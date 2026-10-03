package store

import (
	"errors"
	"starline/learning-api/internal/domain/learning"
)

// Existing grants are retained when a subject is disabled. New grants must come
// from the active catalog; editing a name must not silently revoke old access.
func (s *MemoryStore) validateTeacherRangeChanges(req learning.TeacherUpsertRequest, previous learning.User) error {
	active := func(space learningSpace) bool {
		if space.Status != learning.StatusEnabled {
			return false
		}
		for _, subject := range s.subjects {
			if subjectsMatch(subject.Name, space.Subject) {
				return subject.Status == "启用"
			}
		}
		return false
	}
	checkIDs := func(ids, old []string) error {
		for _, id := range ids {
			if containsString(old, id) {
				continue
			}
			space, ok := s.findLearningSpace(id)
			if !ok || !active(space) {
				return errors.New("新增教师范围必须选择已启用的年级与学科")
			}
		}
		return nil
	}
	if err := checkIDs(req.LearningSpaceIDs, previous.LearningSpaceIDs); err != nil {
		return err
	}
	if req.TeacherLibrary == nil {
		return nil
	}
	old := previous.TeacherLibrary
	if old == nil {
		old = &learning.TeacherLibraryPolicy{SpaceIDs: previous.LearningSpaceIDs}
	}
	oldReadIDs := cloneStrings(old.SpaceIDs)
	for _, space := range s.learningSpaces {
		for _, scope := range old.Scopes {
			if subjectsMatch(scope.Subject, space.Subject) && (scope.Grade == "" || scope.Grade == space.Grade) {
				oldReadIDs = append(oldReadIDs, space.ID)
				break
			}
		}
	}
	if err := checkIDs(req.TeacherLibrary.SpaceIDs, oldReadIDs); err != nil {
		return err
	}
	for _, scope := range req.TeacherLibrary.Scopes {
		retained := false
		for _, existing := range old.Scopes {
			if scope.Grade == existing.Grade && subjectsMatch(scope.Subject, existing.Subject) {
				retained = true
				break
			}
		}
		if retained {
			continue
		}
		valid := false
		for _, space := range s.learningSpaces {
			if (scope.Grade == "" || space.Grade == scope.Grade) && subjectsMatch(scope.Subject, space.Subject) && active(space) {
				valid = true
				break
			}
		}
		if !valid {
			return errors.New("新增查阅范围必须选择已启用的年级与学科")
		}
	}
	return nil
}
