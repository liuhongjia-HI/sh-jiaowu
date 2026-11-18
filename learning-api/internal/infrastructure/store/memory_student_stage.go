package store

import (
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

var studentCalendarZone = time.FixedZone("Asia/Shanghai", 8*60*60)

type studentCourseStage struct {
	current      int
	past         int
	academicYear string
	termStart    string
}

func (s *MemoryStore) studentCourseStage() studentCourseStage {
	return s.studentCourseStageOn(time.Now().In(studentCalendarZone).Format(time.DateOnly))
}

func (s *MemoryStore) studentCourseStageOn(today string) studentCourseStage {
	if term, ok := s.findCalendarTermForDate(today); ok {
		first := semesterFirstStage(term.Semester)
		_, err := time.Parse(time.DateOnly, term.MidtermDate)
		if first == 0 || err != nil || term.MidtermDate < term.StartDate || term.MidtermDate > term.EndDate {
			return studentCourseStage{}
		}
		current := first
		if today > term.MidtermDate {
			current++
		}
		return studentCourseStage{current: current, past: current - 1, academicYear: term.AcademicYear, termStart: term.StartDate}
	}

	// Between terms, keep authorized courses from the most recently ended term
	// available in history. A calendar with no usable prior term falls back.
	var latestEnd string
	var stage studentCourseStage
	var nextTerm academicCalendarTerm
	for _, term := range s.academicCalendarTerms() {
		first := semesterFirstStage(term.Semester)
		_, startErr := time.Parse(time.DateOnly, term.StartDate)
		_, endErr := time.Parse(time.DateOnly, term.EndDate)
		if first == 0 || startErr != nil || endErr != nil || term.StartDate > term.EndDate {
			continue
		}
		if latestEnd < term.EndDate && term.EndDate < today {
			latestEnd = term.EndDate
			stage = studentCourseStage{past: first + 1, academicYear: term.AcademicYear, termStart: term.StartDate}
		}
		if today < term.StartDate && (nextTerm.StartDate == "" || term.StartDate < nextTerm.StartDate) {
			nextTerm = term
		}
	}
	if latestEnd == "" && nextTerm.StartDate != "" {
		return studentCourseStage{academicYear: nextTerm.AcademicYear, termStart: nextTerm.StartDate}
	}
	return stage
}

func semesterFirstStage(semester string) int {
	switch value := strings.ToUpper(strings.TrimSpace(semester)); {
	case value == "S1", strings.HasPrefix(value, "S1 "), value == "第一学期":
		return 1
	case value == "S2", strings.HasPrefix(value, "S2 "), value == "第二学期":
		return 3
	default:
		return 0
	}
}

func spaceStage(space learningSpace) int {
	first := semesterFirstStage(space.Semester)
	if first == 0 {
		return 0
	}
	switch strings.ToUpper(strings.TrimSpace(space.Phase)) {
	case "Q1", "期中":
		return first
	case "Q2", "期末":
		return first + 1
	default:
		return 0
	}
}

func (stage studentCourseStage) isCurrent(rank int) bool {
	return stage == (studentCourseStage{}) || rank == stage.current && rank != 0
}

func (s *MemoryStore) courseStageRank(courseID, spaceID string) int {
	if spaceID == "" {
		course, _ := s.findCourse(courseID)
		spaceID = course.LearningSpaceID
	}
	space, _ := s.findLearningSpace(spaceID)
	return spaceStage(space)
}

func (s *MemoryStore) splitStudentCourses(studentID string, stage studentCourseStage) (current, history []learning.Course) {
	courses := s.coursesForStudent(studentID)
	// Learning spaces are reused across years. The grant package's school year
	// identifies an older course even when its phase comes later in this year's cycle.
	previousYears := make(map[string]bool)
	if stage.termStart != "" {
		for _, term := range s.academicCalendarTerms() {
			if term.AcademicYear != "" && term.AcademicYear != stage.academicYear && term.EndDate != "" && term.EndDate < stage.termStart {
				previousYears[term.AcademicYear] = true
			}
		}
	}
	previousSpaces := make(map[string]bool)
	if len(previousYears) > 0 {
		for _, grant := range s.grants {
			if grant.StudentID != studentID || !grantActive(grant) || len(s.contentTypesForPackage(grant.PackageID)) == 0 {
				continue
			}
			pkg, _ := s.findPackage(grant.PackageID)
			if previousYears[pkg.AcademicYear] {
				for _, spaceID := range s.learningSpaceIDsForGrant(grant.ID) {
					previousSpaces[spaceID] = true
				}
			}
		}
	}
	for _, course := range courses {
		rank := s.courseStageRank(course.ID, course.LearningSpaceID)
		switch {
		case stage.isCurrent(rank), rank == 0:
			// Keep legacy authorized courses visible when their phase is unknown.
			current = append(current, course)
		case rank <= stage.past, previousSpaces[course.LearningSpaceID]:
			history = append(history, course)
		}
	}
	return current, history
}
