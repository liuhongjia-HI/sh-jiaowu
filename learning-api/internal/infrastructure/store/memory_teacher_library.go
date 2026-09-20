package store

import (
	"encoding/json"
	"errors"
	"starline/learning-api/internal/domain/learning"
)

func cloneTeacherLibrary(p *learning.TeacherLibraryPolicy) *learning.TeacherLibraryPolicy {
	if p == nil {
		return nil
	}
	out := *p
	out.SpaceIDs = append([]string{}, p.SpaceIDs...)
	out.Scopes = append([]learning.TeacherLibraryScope{}, p.Scopes...)
	out.RecentMaterialIDs = cloneStrings(p.RecentMaterialIDs)
	return &out
}
func teacherLibraryJSON(p *learning.TeacherLibraryPolicy) any {
	if p == nil {
		return nil
	}
	data, _ := json.Marshal(p)
	return string(data)
}
func materialCurriculumSearch(m learning.Material) string {
	return m.Curriculum.Unit + " " + m.Curriculum.Chapter + " " + m.Curriculum.Lesson
}

func (s *MemoryStore) canReadTeacherCourse(p learning.Principal, c learning.Course) bool {
	if !p.IsTeacherOnly() || p.TeacherLibrary == nil {
		return canSeeCourse(p, c)
	}
	policy := p.TeacherLibrary
	if !policy.CanViewDrafts && c.Status != learning.StatusEnabled {
		return false
	}
	if containsString(policy.SpaceIDs, c.LearningSpaceID) {
		return true
	}
	space, ok := s.findLearningSpace(c.LearningSpaceID)
	if !ok {
		return false
	}
	for _, scope := range policy.Scopes {
		if subjectsMatch(scope.Subject, space.Subject) && (scope.Grade == "" || scope.Grade == space.Grade) {
			return true
		}
	}
	return false
}
func (s *MemoryStore) teacherLibraryMaterialsUnlocked(p learning.Principal) []learning.Material {
	out := []learning.Material{}
	for _, m := range s.materials {
		c, ok := s.findCourse(m.CourseID)
		if !ok || c.LearningSpaceID != m.LearningSpaceID || !s.canReadTeacherCourse(p, c) {
			continue
		}
		if p.IsTeacherOnly() && p.TeacherLibrary != nil && !p.TeacherLibrary.CanViewDrafts && !materialPublished(m.Status) {
			continue
		}
		m = s.decorateMaterial(m)
		if !p.CanDownloadTeacherMaterial() {
			m.DownloadURL = ""
		}
		out = append(out, m)
	}
	return orderMaterialsByCourse(out)
}
func (s *MemoryStore) TeacherLibrary(p learning.Principal) (learning.TeacherLibrary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := learning.TeacherLibrary{Policy: cloneTeacherLibrary(p.TeacherLibrary), Courses: []learning.Course{}, Materials: s.teacherLibraryMaterialsUnlocked(p), Spaces: []learning.LearningSpace{}, RecentMaterialIDs: []string{}, CanDownload: p.CanDownloadTeacherMaterial()}
	for _, c := range s.courses {
		if s.canReadTeacherCourse(p, c) {
			decorated := s.decorateCourse(c)
			decorated.MaterialNum = 0
			decorated.HomeworkNum = 0
			for _, m := range out.Materials {
				if m.CourseID == c.ID {
					decorated.MaterialNum++
				}
			}
			out.Courses = append(out.Courses, decorated)
		}
	}
	for _, space := range s.learningSpacesUnlocked() {
		if s.canReadTeacherCourse(p, learning.Course{LearningSpaceID: space.ID, Status: learning.StatusEnabled}) {
			out.Spaces = append(out.Spaces, space)
		}
	}
	if p.TeacherLibrary != nil {
		visible := map[string]bool{}
		for _, m := range out.Materials {
			visible[m.ID] = true
		}
		for _, id := range p.TeacherLibrary.RecentMaterialIDs {
			if visible[id] {
				out.RecentMaterialIDs = append(out.RecentMaterialIDs, id)
			}
		}
	}
	return out, nil
}
func (s *MemoryStore) RecordTeacherMaterialView(p learning.Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordTeacherMaterialViewUnlocked(p, id)
}
func (s *MemoryStore) recordTeacherMaterialViewUnlocked(p learning.Principal, id string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error { return work.recordTeacherMaterialViewUnlocked(p, id) })
	}
	found := false
	for _, m := range s.teacherLibraryMaterialsUnlocked(p) {
		if m.ID == id {
			found = true
			break
		}
	}
	if !found {
		return errors.New("资料不存在或已不在负责范围内")
	}
	for i, u := range s.users {
		if u.ID != p.UserID {
			continue
		}
		policy := cloneTeacherLibrary(u.TeacherLibrary)
		if policy == nil {
			policy = &learning.TeacherLibraryPolicy{SpaceIDs: cloneStrings(u.LearningSpaceIDs), CanDownload: true, CanManageCourses: true, CanViewDrafts: true}
		}
		ids := []string{id}
		for _, old := range policy.RecentMaterialIDs {
			if old != id && len(ids) < 20 {
				ids = append(ids, old)
			}
		}
		policy.RecentMaterialIDs = ids
		s.users[i].TeacherLibrary = policy
		return nil
	}
	return errors.New("账号不存在")
}
