package store

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

var subjectColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *MemoryStore) subjectsUnlocked() []learning.SubjectMetadata {
	out := append([]learning.SubjectMetadata(nil), s.subjects...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Name < out[j].Name
	})
	for index := range out {
		out[index].Deletable = s.subjectDeleteBlockReason(out[index]) == ""
	}
	return out
}

func (s *MemoryStore) updateSubjectMetadataUnlocked(operator, id string, req learning.SubjectMetadataUpdateRequest) (learning.SubjectMetadata, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.SubjectMetadata, error) {
			return work.updateSubjectMetadataUnlocked(operator, id, req)
		})
	}
	id = strings.TrimSpace(id)
	req.ShortLabel = strings.TrimSpace(req.ShortLabel)
	req.Color = strings.ToUpper(strings.TrimSpace(req.Color))
	req.Status = strings.TrimSpace(req.Status)
	if req.ShortLabel == "" {
		return learning.SubjectMetadata{}, errors.New("请填写学科简称")
	}
	if len([]rune(req.ShortLabel)) > 20 {
		return learning.SubjectMetadata{}, errors.New("学科简称不能超过20个字符")
	}
	if !subjectColorPattern.MatchString(req.Color) {
		return learning.SubjectMetadata{}, errors.New("颜色应为 #RRGGBB 格式")
	}
	if req.SortOrder < 0 {
		return learning.SubjectMetadata{}, errors.New("排序不能小于0")
	}
	if req.Status != "启用" && req.Status != "停用" {
		return learning.SubjectMetadata{}, errors.New("学科状态只能为启用或停用")
	}
	for index := range s.subjects {
		if s.subjects[index].ID != id {
			continue
		}
		before := s.subjects[index]
		s.subjects[index].ShortLabel = req.ShortLabel
		s.subjects[index].Color = req.Color
		s.subjects[index].SortOrder = req.SortOrder
		s.subjects[index].Status = req.Status
		if req.Status == "启用" {
			s.ensureLearningSpaces(currentAcademicYear())
		}
		s.prependLogDetail(operator, "修改学科显示配置", before.Name, auditChangeDetail(before, s.subjects[index]))
		updated := s.subjects[index]
		updated.Deletable = s.subjectDeleteBlockReason(updated) == ""
		return updated, nil
	}
	return learning.SubjectMetadata{}, errors.New("学科不存在")
}

// AppendSubjectMetadata 仅用于测试注入残留学科，正式环境没有创建学科接口。
func (s *MemoryStore) AppendSubjectMetadata(item learning.SubjectMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subjects = append(s.subjects, item)
}

func (s *MemoryStore) deleteSubjectMetadataUnlocked(operator, id string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error {
			return work.deleteSubjectMetadataUnlocked(operator, id)
		})
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("学科不存在")
	}
	for index, item := range s.subjects {
		if item.ID != id {
			continue
		}
		if reason := s.subjectDeleteBlockReason(item); reason != "" {
			return errors.New(reason)
		}
		s.removeLeftoverSubjectFromCatalog(item.Name)
		s.subjects = append(s.subjects[:index], s.subjects[index+1:]...)
		s.prependLogDetail(operator, "删除学科显示配置", item.Name, "")
		return nil
	}
	return errors.New("学科不存在")
}

func isDefaultSubjectID(id string) bool {
	for _, item := range defaultSubjectMetadata() {
		if item.ID == id {
			return true
		}
	}
	return false
}

func (s *MemoryStore) subjectDeleteBlockReason(item learning.SubjectMetadata) string {
	if isDefaultSubjectID(item.ID) {
		return "系统内置学科不能删除。如不再开设，请改为停用"
	}
	return s.leftoverSubjectUsageReason(item.Name)
}

func (s *MemoryStore) leftoverSubjectUsageReason(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, course := range s.courses {
		if subjectsMatch(course.Subject, name) {
			return "仍有课程使用该学科，不能删除。如不再开设，请改为停用"
		}
	}
	for _, pkg := range s.packages {
		if subjectsMatch(pkg.Subject, name) {
			return "仍有课程方案使用该学科，不能删除。如不再开设，请改为停用"
		}
	}
	for _, material := range s.materials {
		if subjectsMatch(material.Subject, name) {
			return "仍有课程讲义使用该学科，不能删除。如不再开设，请改为停用"
		}
	}
	for _, homework := range s.homework {
		if subjectsMatch(homework.Subject, name) {
			return "仍有课后练习使用该学科，不能删除。如不再开设，请改为停用"
		}
	}
	for _, question := range s.questionBank {
		if subjectsMatch(question.Subject, name) {
			return "仍有题库题目使用该学科，不能删除。如不再开设，请改为停用"
		}
	}
	for _, assignment := range s.tutoringAssignments {
		if subjectsMatch(assignment.SubjectID, name) || subjectsMatch(assignment.SubjectName, name) {
			return "仍有辅导关系使用该学科，不能删除。如不再开设，请改为停用"
		}
	}
	for _, score := range s.scoreRecords {
		if subjectsMatch(score.Subject, name) {
			return "仍有成绩记录使用该学科，不能删除。如不再开设，请改为停用"
		}
	}
	return ""
}

func (s *MemoryStore) removeLeftoverSubjectFromCatalog(name string) {
	if s.settings == nil {
		return
	}
	raw := strings.TrimSpace(s.settings[gradeSubjectCatalogSetting])
	if raw == "" {
		return
	}
	var items []learning.GradeSubjectMetadata
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		return
	}
	kept := make([]learning.GradeSubjectMetadata, 0, len(items))
	removed := false
	for _, item := range items {
		if subjectsMatch(item.Subject, name) {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	if !removed {
		return
	}
	if len(kept) == 0 {
		delete(s.settings, gradeSubjectCatalogSetting)
		return
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		return
	}
	s.settings[gradeSubjectCatalogSetting] = string(encoded)
}
