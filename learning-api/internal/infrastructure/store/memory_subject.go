package store

import (
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
		s.prependLogDetail(operator, "修改学科显示配置", before.Name, auditChangeDetail(before, s.subjects[index]))
		return s.subjects[index], nil
	}
	return learning.SubjectMetadata{}, errors.New("学科不存在")
}
