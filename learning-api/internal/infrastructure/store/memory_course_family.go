package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) CourseFamilies(principal learning.Principal) []learning.CourseFamily {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]learning.CourseFamily, 0, len(s.courseFamilies))
	for _, family := range s.courseFamilies {
		item := family
		item.Curriculum = append([]learning.CurriculumNode(nil), family.Curriculum...)
		item.Courses = []learning.Course{}
		for _, course := range s.courses {
			if course.FamilyID == family.ID && canSeeCourse(principal, course) {
				item.Courses = append(item.Courses, s.decorateCourse(course))
			}
		}
		if len(item.Courses) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func (s *MemoryStore) CreateCourseFamily(operator string, principal learning.Principal, req learning.CourseFamilyCreateRequest) (learning.CourseFamily, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.CourseFamily, error) {
		return work.createCourseFamilyUnlocked(operator, principal, req)
	})
}

func (s *MemoryStore) UpdateCourseFamily(operator string, principal learning.Principal, id string, req learning.CourseFamilyUpdateRequest) (learning.CourseFamily, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.CourseFamily, error) {
		return work.updateCourseFamilyUnlocked(operator, principal, id, req)
	})
}

func (s *MemoryStore) AddCourseFamilyCourse(operator string, principal learning.Principal, id string, req learning.CourseFamilyAddCourseRequest) (learning.Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.Course, error) {
		return work.addCourseFamilyCourseUnlocked(operator, principal, id, req)
	})
}

func (s *MemoryStore) ImportCourseFamily(operator string, principal learning.Principal, req learning.CourseFamilyImportRequest) (learning.CourseFamily, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.CourseFamily, error) {
		return work.importCourseFamilyUnlocked(operator, principal, req)
	})
}

func (s *MemoryStore) importCourseFamilyUnlocked(operator string, principal learning.Principal, req learning.CourseFamilyImportRequest) (learning.CourseFamily, error) {
	if !principal.CanMaintainCourses() {
		return learning.CourseFamily{}, errors.New("当前账号没有维护课程权限")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return learning.CourseFamily{}, errors.New("请输入课程系列名称")
	}
	if len(req.CourseIDs) < 2 {
		return learning.CourseFamily{}, errors.New("请至少选择两个独立班型课程")
	}
	courses := make([]learning.Course, 0, len(req.CourseIDs))
	spaces := make([]learningSpace, 0, len(req.CourseIDs))
	seenIDs := map[string]bool{}
	seenLevels := map[string]bool{}
	for _, rawID := range req.CourseIDs {
		id := strings.TrimSpace(rawID)
		if seenIDs[id] {
			return learning.CourseFamily{}, errors.New("课程不能重复选择")
		}
		seenIDs[id] = true
		course, ok := s.findCourse(id)
		if !ok || course.FamilyID != "" || !canSeeCourse(principal, course) {
			return learning.CourseFamily{}, errors.New("所选课程不存在、无权维护或已属于课程系列")
		}
		space, ok := s.findLearningSpace(course.LearningSpaceID)
		if !ok {
			return learning.CourseFamily{}, errors.New("课程所属学习空间不存在")
		}
		if len(spaces) > 0 && !sameCourseFamilySpace(spaces[0], space) {
			return learning.CourseFamily{}, errors.New("所选课程必须属于同一年级、学科、学期和阶段")
		}
		if seenLevels[space.Level] {
			return learning.CourseFamily{}, errors.New("所选课程包含重复班型")
		}
		seenLevels[space.Level] = true
		if len(course.Curriculum) == 0 {
			return learning.CourseFamily{}, errors.New("所选课程缺少目录，不能合并")
		}
		courses = append(courses, course)
		spaces = append(spaces, space)
	}
	for _, existing := range s.courseFamilies {
		if existing.Name == name && existing.Grade == spaces[0].Grade && subjectsMatch(existing.Subject, spaces[0].Subject) && existing.Semester == spaces[0].Semester && existing.Phase == spaces[0].Phase {
			return learning.CourseFamily{}, errors.New("该范围内课程系列名称已存在")
		}
	}
	canonical := courses[0]
	idMaps := make([]map[string]string, len(courses))
	for index, course := range courses {
		mapping, ok := equivalentCurriculumMap(canonical.Curriculum, course.Curriculum)
		if !ok {
			return learning.CourseFamily{}, fmt.Errorf("课程“%s”的目录与第一门课程不一致，请先调整后再合并", course.Name)
		}
		idMaps[index] = mapping
		for _, plan := range s.teachingPlans {
			if plan.CourseID == course.ID && mapping[plan.LessonID] == "" {
				return learning.CourseFamily{}, fmt.Errorf("课程“%s”存在无法对应的教案章节", course.Name)
			}
		}
		for _, material := range s.materials {
			if material.CourseID == course.ID && material.LessonID != "" && mapping[material.LessonID] == "" {
				return learning.CourseFamily{}, fmt.Errorf("课程“%s”存在无法对应的讲义课节", course.Name)
			}
		}
		for _, homework := range s.homework {
			if homework.CourseID == course.ID && homework.LessonID != "" && mapping[homework.LessonID] == "" {
				return learning.CourseFamily{}, fmt.Errorf("课程“%s”存在无法对应的练习课节", course.Name)
			}
		}
	}
	family := learning.CourseFamily{ID: "course-family-" + time.Now().Format("20060102150405.000000000"), Name: name, Grade: spaces[0].Grade, Subject: spaces[0].Subject, Semester: spaces[0].Semester, Phase: spaces[0].Phase, Curriculum: append([]learning.CurriculumNode(nil), canonical.Curriculum...), Courses: make([]learning.Course, 0, len(courses))}
	for index, course := range courses {
		s.remapDirectorySyncLinks(course.ID, idMaps[index])
		course.DirectorySyncMap = cloneMap(s.courses[findCourseIndex(s.courses, course.ID)].DirectorySyncMap)
		for i := range s.teachingPlans {
			if s.teachingPlans[i].CourseID == course.ID {
				s.teachingPlans[i].LessonID = idMaps[index][s.teachingPlans[i].LessonID]
				if path, err := curriculumPathForLesson(canonical, s.teachingPlans[i].LessonID); err == nil {
					s.teachingPlans[i].Chapter = planChapterLabel(path)
				}
			}
		}
		for i := range s.materials {
			if s.materials[i].CourseID == course.ID && s.materials[i].LessonID != "" {
				s.materials[i].LessonID = idMaps[index][s.materials[i].LessonID]
				if path, err := curriculumPathForLesson(canonical, s.materials[i].LessonID); err == nil {
					s.materials[i].Curriculum = path
				}
			}
		}
		for i := range s.homework {
			if s.homework[i].CourseID == course.ID && s.homework[i].LessonID != "" {
				s.homework[i].LessonID = idMaps[index][s.homework[i].LessonID]
				if path, err := curriculumPathForLesson(canonical, s.homework[i].LessonID); err == nil {
					s.homework[i].Curriculum = path
				}
			}
		}
		course.FamilyID = family.ID
		course.Curriculum = append([]learning.CurriculumNode(nil), family.Curriculum...)
		family.Courses = append(family.Courses, s.decorateCourse(course))
		for i := range s.courses {
			if s.courses[i].ID == course.ID {
				s.courses[i] = course
				break
			}
		}
	}
	s.courseFamilies = append([]learning.CourseFamily{family}, s.courseFamilies...)
	s.prependLogDetail(operator, "合并课程系列", name, fmt.Sprintf("保留 %d 门原课程及其讲义练习", len(courses)))
	return family, nil
}

// Match siblings in curriculum order and compare their full hierarchy. A single
// title or ordinal is never enough to map a bound lesson across courses.
func equivalentCurriculumMap(canonical, candidate []learning.CurriculumNode) (map[string]string, bool) {
	if len(canonical) != len(candidate) {
		return nil, false
	}
	mapping := map[string]string{}
	var visit func(string, string) bool
	visit = func(leftParent, rightParent string) bool {
		children := func(nodes []learning.CurriculumNode, parent string) []learning.CurriculumNode {
			out := []learning.CurriculumNode{}
			for _, node := range nodes {
				if node.ParentID == parent {
					out = append(out, node)
				}
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].SortOrder != out[j].SortOrder {
					return out[i].SortOrder < out[j].SortOrder
				}
				return out[i].ID < out[j].ID
			})
			return out
		}
		left, right := children(canonical, leftParent), children(candidate, rightParent)
		if len(left) != len(right) {
			return false
		}
		for index := range left {
			if left[index].Type != right[index].Type || strings.TrimSpace(left[index].Name) != strings.TrimSpace(right[index].Name) || left[index].SortOrder != right[index].SortOrder {
				return false
			}
			mapping[right[index].ID] = left[index].ID
			if !visit(left[index].ID, right[index].ID) {
				return false
			}
		}
		return true
	}
	if !visit("", "") || len(mapping) != len(candidate) {
		return nil, false
	}
	return mapping, true
}

func (s *MemoryStore) createCourseFamilyUnlocked(operator string, principal learning.Principal, req learning.CourseFamilyCreateRequest) (learning.CourseFamily, error) {
	if !principal.CanMaintainCourses() {
		return learning.CourseFamily{}, errors.New("当前账号没有维护课程权限")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return learning.CourseFamily{}, errors.New("请输入课程系列名称")
	}
	curriculum, err := normalizeCurriculum(req.Curriculum)
	if err != nil {
		return learning.CourseFamily{}, err
	}
	if len(req.LearningSpaceIDs) == 0 {
		return learning.CourseFamily{}, errors.New("请至少选择一个班型")
	}
	spaces := make([]learningSpace, 0, len(req.LearningSpaceIDs))
	seen := map[string]bool{}
	seenLevels := map[string]bool{}
	for _, rawID := range req.LearningSpaceIDs {
		spaceID := strings.TrimSpace(rawID)
		if seen[spaceID] {
			return learning.CourseFamily{}, errors.New("同一班型不能重复选择")
		}
		seen[spaceID] = true
		space, ok := s.findLearningSpace(spaceID)
		if !ok || space.Status == learning.StatusDisabled {
			return learning.CourseFamily{}, errors.New("班型对应的学习空间不存在或已停用")
		}
		if len(spaces) > 0 && !sameCourseFamilySpace(spaces[0], space) {
			return learning.CourseFamily{}, errors.New("班型必须属于同一年级、学科、学期和阶段")
		}
		if seenLevels[space.Level] {
			return learning.CourseFamily{}, errors.New("同一课程系列不能重复选择相同班型")
		}
		seenLevels[space.Level] = true
		candidate := learning.Course{LearningSpaceID: space.ID, Grade: space.Grade, Subject: space.Subject}
		if !canSeeCourse(principal, candidate) {
			return learning.CourseFamily{}, errors.New("不能维护未负责的班型课程范围")
		}
		spaces = append(spaces, space)
	}
	for _, existing := range s.courseFamilies {
		if existing.Name == name && existing.Grade == spaces[0].Grade && subjectsMatch(existing.Subject, spaces[0].Subject) && existing.Semester == spaces[0].Semester && existing.Phase == spaces[0].Phase {
			return learning.CourseFamily{}, errors.New("该范围内课程系列名称已存在")
		}
	}
	stamp := time.Now().Format("20060102150405.000000000")
	family := learning.CourseFamily{ID: "course-family-" + stamp, Name: name, Grade: spaces[0].Grade, Subject: spaces[0].Subject, Semester: spaces[0].Semester, Phase: spaces[0].Phase, Curriculum: curriculum, Courses: []learning.Course{}}
	for index, space := range spaces {
		courseName := familyCourseName(name, space.Level)
		if s.courseNameExists("", courseName) {
			return learning.CourseFamily{}, fmt.Errorf("课程名称“%s”已存在", courseName)
		}
		course := learning.Course{ID: fmt.Sprintf("course-family-member-%s-%d", stamp, index), FamilyID: family.ID, Name: courseName, Grade: space.Grade, Subject: space.Subject, LearningSpaceID: space.ID, Curriculum: append([]learning.CurriculumNode(nil), curriculum...), LessonCount: countCurriculumLessons(curriculum), Status: learning.StatusEnabled}
		family.Courses = append(family.Courses, course)
	}
	s.courseFamilies = append([]learning.CourseFamily{family}, s.courseFamilies...)
	for _, course := range family.Courses {
		s.courses = append([]learning.Course{course}, s.courses...)
	}
	s.prependLogDetail(operator, "创建课程系列", name, fmt.Sprintf("%s/%s/%s/%s；%d 个班型", family.Grade, family.Subject, family.Semester, family.Phase, len(spaces)))
	return family, nil
}

func (s *MemoryStore) updateCourseFamilyUnlocked(operator string, principal learning.Principal, id string, req learning.CourseFamilyUpdateRequest) (learning.CourseFamily, error) {
	if !principal.CanMaintainCourses() {
		return learning.CourseFamily{}, errors.New("当前账号没有维护课程权限")
	}
	index := s.courseFamilyIndex(id)
	if index < 0 {
		return learning.CourseFamily{}, errors.New("课程系列不存在")
	}
	family := s.courseFamilies[index]
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return learning.CourseFamily{}, errors.New("请输入课程系列名称")
	}
	curriculum, err := normalizeCurriculum(req.Curriculum)
	if err != nil {
		return learning.CourseFamily{}, err
	}
	for _, other := range s.courseFamilies {
		if other.ID != id && other.Name == name && other.Grade == family.Grade && subjectsMatch(other.Subject, family.Subject) && other.Semester == family.Semester && other.Phase == family.Phase {
			return learning.CourseFamily{}, errors.New("该范围内课程系列名称已存在")
		}
	}
	updatedCourses := make([]learning.Course, 0)
	for _, course := range s.courses {
		if course.FamilyID != id {
			continue
		}
		if !canSeeCourse(principal, course) {
			return learning.CourseFamily{}, errors.New("当前账号不能修改该系列的全部班型")
		}
		space, ok := s.findLearningSpace(course.LearningSpaceID)
		if !ok {
			return learning.CourseFamily{}, errors.New("班型对应的学习空间不存在")
		}
		course.Name = familyCourseName(name, space.Level)
		if s.courseNameExists(course.ID, course.Name) {
			return learning.CourseFamily{}, fmt.Errorf("课程名称“%s”已存在", course.Name)
		}
		course.Curriculum = append([]learning.CurriculumNode(nil), curriculum...)
		course.LessonCount = countCurriculumLessons(curriculum)
		if err := s.validateCourseContentBindings(course); err != nil {
			return learning.CourseFamily{}, err
		}
		// Removing a bound leaf would orphan content, even though adding children is checked above.
		valid := map[string]bool{}
		for _, node := range curriculum {
			valid[node.ID] = true
		}
		for _, material := range s.materials {
			if material.CourseID == course.ID && material.LessonID != "" && !valid[material.LessonID] {
				return learning.CourseFamily{}, errors.New("目录中仍有班型讲义，不能删除对应课节")
			}
		}
		for _, homework := range s.homework {
			if homework.CourseID == course.ID && homework.LessonID != "" && !valid[homework.LessonID] {
				return learning.CourseFamily{}, errors.New("目录中仍有班型练习，不能删除对应课节")
			}
		}
		updatedCourses = append(updatedCourses, course)
	}
	for i := range s.courses {
		for _, course := range updatedCourses {
			if s.courses[i].ID == course.ID {
				s.courses[i] = course
				s.syncCourseReferences(course)
			}
		}
	}
	family.Name = name
	family.Curriculum = curriculum
	family.Courses = updatedCourses
	s.courseFamilies[index] = family
	s.prependLogDetail(operator, "编辑共享课程目录", name, fmt.Sprintf("影响 %d 个班型", len(updatedCourses)))
	return family, nil
}

func (s *MemoryStore) addCourseFamilyCourseUnlocked(operator string, principal learning.Principal, id string, req learning.CourseFamilyAddCourseRequest) (learning.Course, error) {
	if !principal.CanMaintainCourses() {
		return learning.Course{}, errors.New("当前账号没有维护课程权限")
	}
	index := s.courseFamilyIndex(id)
	if index < 0 {
		return learning.Course{}, errors.New("课程系列不存在")
	}
	family := s.courseFamilies[index]
	space, ok := s.findLearningSpace(strings.TrimSpace(req.LearningSpaceID))
	if !ok || space.Status == learning.StatusDisabled {
		return learning.Course{}, errors.New("班型对应的学习空间不存在或已停用")
	}
	if family.Grade != space.Grade || !subjectsMatch(family.Subject, space.Subject) || family.Semester != space.Semester || family.Phase != space.Phase {
		return learning.Course{}, errors.New("班型必须属于课程系列的年级、学科、学期和阶段")
	}
	for _, course := range s.courses {
		if course.FamilyID != id {
			continue
		}
		if !canSeeCourse(principal, course) {
			return learning.Course{}, errors.New("当前账号不能修改该系列的全部班型")
		}
		existingSpace, exists := s.findLearningSpace(course.LearningSpaceID)
		if course.LearningSpaceID == space.ID || (exists && existingSpace.Level == space.Level) {
			return learning.Course{}, errors.New("该班型已关联到课程系列")
		}
	}
	courseName := familyCourseName(family.Name, space.Level)
	if s.courseNameExists("", courseName) {
		return learning.Course{}, errors.New("课程名称已存在")
	}
	course := learning.Course{ID: "course-family-member-" + time.Now().Format("20060102150405.000000000"), FamilyID: id, Name: courseName, Grade: space.Grade, Subject: space.Subject, LearningSpaceID: space.ID, Curriculum: append([]learning.CurriculumNode(nil), family.Curriculum...), LessonCount: countCurriculumLessons(family.Curriculum), Status: learning.StatusEnabled}
	if !canSeeCourse(principal, course) {
		return learning.Course{}, errors.New("不能维护未负责的班型课程范围")
	}
	s.courses = append([]learning.Course{course}, s.courses...)
	s.prependLogDetail(operator, "添加课程班型", family.Name, space.Level)
	return s.decorateCourse(course), nil
}

func (s *MemoryStore) courseFamilyIndex(id string) int {
	for index, family := range s.courseFamilies {
		if family.ID == strings.TrimSpace(id) {
			return index
		}
	}
	return -1
}

func sameCourseFamilySpace(left, right learningSpace) bool {
	return left.Grade == right.Grade && subjectsMatch(left.Subject, right.Subject) && left.Semester == right.Semester && left.Phase == right.Phase
}

func familyCourseName(name, level string) string {
	return strings.TrimSpace(name) + " · " + strings.TrimSpace(level)
}
