package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// Preview builds a private mutation, including reference moves. Nothing is
// published until the exact preview and all its resolutions are confirmed.
func (s *MemoryStore) directorySyncTarget(p learning.Principal, source learning.Course, targetID string, req learning.CourseDirectorySyncRequest) (out learning.CourseDirectorySyncTarget, planned *MemoryStore) {
	out = learning.CourseDirectorySyncTarget{CourseID: targetID, Added: []string{}, Updated: []string{}}
	target, ok := s.findCourse(targetID)
	fail := func(message string) (learning.CourseDirectorySyncTarget, *MemoryStore) {
		out.Error = message
		return out, nil
	}
	if !ok {
		return fail("目标课程不存在")
	}
	out.CourseName = target.Name
	if target.ID == source.ID || !p.CanMaintainCourses() || !canSeeCourse(p, target) || p.IsTeacherOnly() && !containsString(p.LearningSpaceIDs, target.LearningSpaceID) {
		return fail("没有权限同步该目标课程")
	}
	if !s.sameMaterialSyncScope(source, target) {
		return fail("目标课程必须与源课程的年级、学科、学期和阶段一致")
	}
	if target.Status != learning.StatusEnabled {
		return fail("目标课程已停用")
	}
	shared := source.FamilyID != "" && source.FamilyID == target.FamilyID
	if shared && len(req.Resolutions[targetID]) > 0 {
		return fail("已共用目录，无需指定合并关系，请在课程系列中维护")
	}
	if target.FamilyID != "" && !shared {
		return fail("目标使用另一套共享目录，请在课程系列中维护")
	}
	include := req.IncludeMaterials == nil || *req.IncludeMaterials
	if include && !canUploadHandout(p) {
		return fail("当前账号没有同步课程讲义权限，请选择仅同步目录")
	}
	if _, err := directoryPaths(source.Curriculum); err != nil {
		return fail(err.Error())
	}
	if _, err := directoryPaths(target.Curriculum); err != nil {
		return fail(err.Error())
	}
	original := target
	planned = s.cloneForMutation()
	target.Curriculum = append([]learning.CurriculumNode(nil), target.Curriculum...)
	target.DirectorySyncMap = cloneMap(target.DirectorySyncMap)
	if target.DirectorySyncMap == nil {
		target.DirectorySyncMap = map[string]string{}
	}
	sourceLabels, targetLabels := directoryLabels(source.Curriculum), directoryLabels(target.Curriculum)
	mapping, used := map[string]string{}, map[string]string{}
	nodes := append([]learning.CurriculumNode(nil), source.Curriculum...)
	depths := map[string]int{}
	var depth func(string) int
	bySource := map[string]learning.CurriculumNode{}
	for _, n := range nodes {
		bySource[n.ID] = n
	}
	depth = func(id string) int {
		if id == "" {
			return 0
		}
		if d := depths[id]; d > 0 {
			return d
		}
		d := 1 + depth(bySource[id].ParentID)
		depths[id] = d
		return d
	}
	sort.SliceStable(nodes, func(i, j int) bool { return depth(nodes[i].ID) < depth(nodes[j].ID) })
	for _, node := range nodes {
		parent := mapping[node.ParentID]
		if node.ParentID != "" && parent == "" {
			continue
		} // parent still needs a resolution
		linked := target.DirectorySyncMap[source.ID+":"+node.ID]
		if !directoryContainsNode(target.Curriculum, linked) {
			linked = directorySyncID(source.ID, node.ID, target.ID)
		}
		if shared {
			if !directoryContainsNode(target.Curriculum, node.ID) {
				return fail("共享目录已变化，请刷新课程后重试")
			}
			linked = node.ID
		}
		if !directoryContainsNode(target.Curriculum, linked) {
			linked = ""
		}
		if linked != "" && used[linked] != "" && used[linked] != node.ID {
			return fail("章节对应关系冲突，请先调整目录")
		}
		candidates := []learning.DirectorySyncCandidate{}
		exact := ""
		for _, n := range target.Curriculum {
			if n.Type != node.Type || n.ParentID != parent || used[n.ID] != "" && used[n.ID] != node.ID {
				continue
			}
			sameName := strings.EqualFold(curriculumNodePathLabel(n), curriculumNodePathLabel(node))
			if n.ID == linked || sameName || n.SortOrder == node.SortOrder {
				candidates = append(candidates, learning.DirectorySyncCandidate{ID: n.ID, Label: directoryDisplayLabel(target.Curriculum, n.ID)})
				if sameName {
					exact = n.ID
				}
			}
		}
		// Preserve an established mapping across an intentional move.
		if linked != "" {
			found := false
			for _, c := range candidates {
				if c.ID == linked {
					found = true
				}
			}
			if !found {
				candidates = append(candidates, learning.DirectorySyncCandidate{ID: linked, Label: targetLabels[linked]})
			}
		}
		resolution, explicit := req.Resolutions[targetID][node.ID]
		conflict := !shared && (len(candidates) > 1 || len(candidates) == 1 && linked == "" && exact == "")
		chosen := linked
		if chosen == "" {
			chosen = exact
		}
		if conflict || explicit {
			row := learning.DirectorySyncConflict{SourceNodeID: node.ID, SourceLabel: directoryDisplayLabel(source.Curriculum, node.ID), Candidates: candidates, Resolved: explicit, AllowNew: linked == ""}
			out.Conflicts = append(out.Conflicts, row)
			if !explicit {
				continue
			}
			chosen = resolution.TargetNodeID
			if chosen == "__new__" && linked == directorySyncID(source.ID, node.ID, target.ID) {
				chosen = linked
			}
			valid := chosen == "__new__" && linked == ""
			for _, c := range candidates {
				if c.ID == chosen {
					valid = true
				}
			}
			if !valid {
				return fail("目录对应关系已变化，请重新选择")
			}
			mergeSeen := map[string]bool{}
			for _, oldID := range resolution.MergeNodeIDs {
				if !directoryContainsNode(target.Curriculum, oldID) && target.DirectorySyncMap["merged:"+oldID] == chosen {
					continue
				}
				allowed := false
				for _, c := range candidates {
					if c.ID == oldID {
						allowed = true
					}
				}
				if !allowed || oldID == chosen || chosen == "__new__" || mergeSeen[oldID] || used[oldID] != "" {
					return fail("合并节点无效，请重新选择")
				}
				mergeSeen[oldID] = true
				beforeMerge := append([]learning.CurriculumNode(nil), target.Curriculum...)
				aliases, err := mergeDirectoryBranches(&target, oldID, chosen)
				if err != nil {
					return fail(err.Error())
				}
				planned.moveDirectoryBindings(target.ID, aliases)
				for old, keep := range aliases {
					target.DirectorySyncMap["merged:"+old] = keep
				}
				for key, value := range target.DirectorySyncMap {
					if next := aliases[value]; next != "" {
						target.DirectorySyncMap[key] = next
					}
				}
				mergedIDs := make([]string, 0, len(aliases))
				for old := range aliases {
					mergedIDs = append(mergedIDs, old)
				}
				sort.Strings(mergedIDs)
				for _, old := range mergedIDs {
					out.Merged = append(out.Merged, directoryDisplayLabel(beforeMerge, old)+" → "+directoryDisplayLabel(target.Curriculum, aliases[old]))
				}
			}
		}
		if chosen == "" || chosen == "__new__" {
			chosen = directorySyncID(source.ID, node.ID, target.ID)
		}
		mapping[node.ID] = chosen
		used[chosen] = node.ID
		target.DirectorySyncMap[source.ID+":"+node.ID] = chosen
		copy := node
		copy.ID = chosen
		copy.ParentID = parent
		index := -1
		for i, n := range target.Curriculum {
			if n.ID == chosen {
				index = i
				break
			}
		}
		if index < 0 {
			target.Curriculum = append(target.Curriculum, copy)
			out.Added = append(out.Added, sourceLabels[node.ID])
		} else if target.Curriculum[index] != copy {
			out.Updated = append(out.Updated, directoryDisplayLabel(target.Curriculum, chosen)+" → "+directoryDisplayLabel(source.Curriculum, node.ID))
			target.Curriculum[index] = copy
		}
	}
	for _, n := range original.Curriculum {
		if used[n.ID] == "" && directoryContainsNode(target.Curriculum, n.ID) {
			out.Preserved++
		}
	}
	for _, conflict := range out.Conflicts {
		if !conflict.Resolved {
			return out, nil
		}
	}
	if _, err := directoryPaths(target.Curriculum); err != nil {
		return fail(err.Error())
	}
	target.LessonCount = countCurriculumLessons(target.Curriculum)
	planned.courses[findCourseIndex(planned.courses, target.ID)] = target
	if err := planned.validateCourseContentBindings(target); err != nil {
		return fail(err.Error())
	}
	if include {
		if err := planned.planDirectoryMaterials(p, source, &target, mapping, &out); err != nil {
			return fail(err.Error())
		}
	}
	planned.courses[findCourseIndex(planned.courses, target.ID)] = target
	planned.syncCourseReferences(target)
	// Include actual resources and bindings so uploads/edits/moves after preview
	// invalidate the confirmation, even when the directory itself is unchanged.
	resources := []learning.Material{}
	assets := map[string]learning.FileAsset{}
	for _, m := range s.materials {
		if m.CourseID == source.ID || m.CourseID == target.ID {
			resources = append(resources, m)
			if f, ok := s.fileAssets[m.FileID]; ok {
				assets[m.FileID] = f
			}
		}
	}
	homework := []learning.Homework{}
	plans := []learning.TeachingPlan{}
	for _, h := range s.homework {
		if h.CourseID == target.ID {
			homework = append(homework, h)
		}
	}
	for _, h := range s.teachingPlans {
		if h.CourseID == target.ID {
			plans = append(plans, h)
		}
	}
	raw, _ := json.Marshal([]any{source, original, original.DirectorySyncMap, resources, assets, homework, plans, include, req.Resolutions[targetID]})
	sum := sha256.Sum256(raw)
	out.Snapshot = hex.EncodeToString(sum[:])
	return out, planned
}

func directoryDisplayLabel(nodes []learning.CurriculumNode, id string) string {
	byID := map[string]learning.CurriculumNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	parts := []string{}
	for id != "" {
		n, ok := byID[id]
		if !ok {
			break
		}
		label := fmt.Sprintf("%s %d", strings.ToUpper(string(n.Type)[:1])+string(n.Type)[1:], n.SortOrder)
		if n.Name != "" {
			label += " · " + n.Name
		}
		parts = append([]string{label}, parts...)
		id = n.ParentID
	}
	return strings.Join(parts, " / ")
}

// Only explicitly selected branches are merged. Unique child matches are folded;
// unmatched children retain their IDs and move beneath the retained parent.
func mergeDirectoryBranches(course *learning.Course, oldID, keepID string) (map[string]string, error) {
	aliases := map[string]string{}
	var merge func(string, string) error
	merge = func(old, keep string) error {
		var a, b learning.CurriculumNode
		for _, n := range course.Curriculum {
			if n.ID == old {
				a = n
			}
			if n.ID == keep {
				b = n
			}
		}
		if a.ID == "" || b.ID == "" || a.Type != b.Type || a.ParentID != b.ParentID {
			return errors.New("只能合并同一上级下的同类目录")
		}
		children := []learning.CurriculumNode{}
		for _, n := range course.Curriculum {
			if n.ParentID == old {
				children = append(children, n)
			}
		}
		for _, child := range children {
			matches := []string{}
			for _, n := range course.Curriculum {
				if n.ParentID == keep && n.Type == child.Type && (strings.EqualFold(curriculumNodePathLabel(n), curriculumNodePathLabel(child)) || n.SortOrder == child.SortOrder) {
					matches = append(matches, n.ID)
				}
			}
			if len(matches) > 1 {
				return errors.New("下级章节存在多个对应项，请先整理后合并")
			}
			// Make them siblings before recursively folding their descendants.
			for i := range course.Curriculum {
				if course.Curriculum[i].ID == child.ID {
					course.Curriculum[i].ParentID = keep
				}
			}
			if len(matches) == 1 {
				if err := merge(child.ID, matches[0]); err != nil {
					return err
				}
			}
		}
		aliases[old] = keep
		kept := []learning.CurriculumNode{}
		for _, n := range course.Curriculum {
			if n.ID != old {
				kept = append(kept, n)
			}
		}
		course.Curriculum = kept
		return nil
	}
	if err := merge(oldID, keepID); err != nil {
		return nil, err
	}
	return aliases, nil
}

func (s *MemoryStore) moveDirectoryBindings(courseID string, aliases map[string]string) {
	for i := range s.materials {
		if s.materials[i].CourseID == courseID {
			if id := aliases[s.materials[i].LessonID]; id != "" {
				s.materials[i].LessonID = id
			}
		}
	}
	for i := range s.homework {
		if s.homework[i].CourseID == courseID {
			if id := aliases[s.homework[i].LessonID]; id != "" {
				s.homework[i].LessonID = id
			}
		}
	}
	for i := range s.teachingPlans {
		if s.teachingPlans[i].CourseID == courseID {
			if id := aliases[s.teachingPlans[i].LessonID]; id != "" {
				s.teachingPlans[i].LessonID = id
			}
		}
	}
	s.remapDirectorySyncLinks(courseID, aliases)
}

func (s *MemoryStore) planDirectoryMaterials(p learning.Principal, source learning.Course, target *learning.Course, mapping map[string]string, out *learning.CourseDirectorySyncTarget) error {
	selected := map[string]bool{}
	for _, n := range source.Curriculum {
		selected[n.ID] = true
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	touched := map[string]bool{}
	sources := append([]learning.Material(nil), s.materials...)
	for _, src := range sources {
		if src.CourseID != source.ID || !selected[src.LessonID] || !materialVisibleToStudents(src) {
			continue
		}
		if src.FileID == "" || strings.TrimSpace(src.TagCode) == "" {
			return errors.New("已发布讲义缺少标签或文件，请先补全：" + src.Title)
		}
		if _, ok := s.fileAssets[src.FileID]; !ok {
			return errors.New("已发布讲义文件不存在：" + src.Title)
		}
		destination := mapping[src.LessonID]
		path, err := curriculumPathForLesson(*target, destination)
		if err != nil {
			return errors.New("讲义目标必须是有效的末级目录：" + src.Title)
		}
		key := "material:" + source.ID + ":" + src.ID
		id := target.DirectorySyncMap[key]
		if id == "" {
			id = "material-" + directorySyncID(source.ID, src.ID, target.ID)
		}
		index := -1
		for i, m := range s.materials {
			if m.ID == id && m.CourseID == target.ID {
				index = i
				break
			}
		}
		next := src
		next.ID = id
		next.CourseID = target.ID
		next.Course = target.Name
		next.LearningSpaceID = target.LearningSpaceID
		next.Grade = target.Grade
		next.Subject = target.Subject
		next.Semester = s.semesterForSpace(target.LearningSpaceID)
		next.LessonID = destination
		next.Curriculum = path
		next.OwnerTeacherID = p.UserID
		next.OwnerTeacherName = p.Name
		next.ViewCount = 0
		next.CreatedAt = now
		next.UpdatedAt = now
		next.SortOrder = s.nextMaterialSortOrder(target.ID)
		if index >= 0 {
			before := s.materials[index]
			next.ViewCount = before.ViewCount
			next.CreatedAt = before.CreatedAt
			next.SortOrder = before.SortOrder
			next.OwnerTeacherID = before.OwnerTeacherID
			next.OwnerTeacherName = before.OwnerTeacherName
			next.UpdatedAt = before.UpdatedAt
			if next != before {
				next.UpdatedAt = now
				s.materials[index] = next
				out.MaterialsUpdated = append(out.MaterialsUpdated, src.Title)
			}
		} else {
			s.materials = append(s.materials, next)
			out.MaterialsAdded = append(out.MaterialsAdded, src.Title)
		}
		target.DirectorySyncMap[key] = id
		touched[id] = true
	}
	for _, m := range s.materials {
		if m.CourseID == target.ID && !touched[m.ID] {
			out.MaterialsPreserved++
		}
	}
	return nil
}

func directoryContainsNode(nodes []learning.CurriculumNode, id string) bool {
	for _, n := range nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}
