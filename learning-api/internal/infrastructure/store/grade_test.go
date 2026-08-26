package store

import (
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestResolveGradeFollowsAcademicYearRollover(t *testing.T) {
	cases := []struct {
		name            string
		enrollmentYear  string
		enrollmentGrade string
		currentYear     string
		wantGrade       string
		wantGraduated   bool
	}{
		{"入学当年不变", "2025.2026学年", "五年级", "2025.2026学年", "五年级", false},
		{"次年升一级", "2025.2026学年", "五年级", "2026.2027学年", "六年级", false},
		{"跨四年升四级", "2025.2026学年", "五年级", "2029.2030学年", "九年级", false},
		{"升入十年级后继续在读", "2025.2026学年", "五年级", "2030.2031学年", "十年级", false},
		{"超过十二年级封顶并毕业", "2025.2026学年", "五年级", "2033.2034学年", "十二年级", true},
		{"一年级入学滚动", "2020.2021学年", "一年级", "2025.2026学年", "六年级", false},
		{"无法识别的年级原样返回", "2025.2026学年", "预备班", "2026.2027学年", "预备班", false},
		{"缺少入学学年时不推导", "", "五年级", "2026.2027学年", "五年级", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			grade, graduated := resolveGrade(tc.enrollmentYear, tc.enrollmentGrade, tc.currentYear)
			if grade != tc.wantGrade {
				t.Fatalf("grade = %q, want %q", grade, tc.wantGrade)
			}
			if graduated != tc.wantGraduated {
				t.Fatalf("graduated = %v, want %v", graduated, tc.wantGraduated)
			}
		})
	}
}

func TestAcademicYearRolloverHappensOnJulyFirst(t *testing.T) {
	june := time.Date(2026, time.June, 30, 23, 59, 59, 0, time.UTC)
	july := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	students := []learning.Student{{
		ID: "stu-001", Grade: "五年级",
		EnrollmentAcademicYear: "2025.2026学年", EnrollmentGrade: "五年级",
	}}

	refreshStudentGrades(students, june)
	if students[0].Grade != "五年级" {
		t.Fatalf("6 月 30 日应仍是五年级，实际 %q", students[0].Grade)
	}

	refreshStudentGrades(students, july)
	if students[0].Grade != "六年级" {
		t.Fatalf("7 月 1 日应升为六年级，实际 %q", students[0].Grade)
	}
}

func TestApplyDerivedGradeBackfillsLegacyStudentWithoutBaseline(t *testing.T) {
	student := learning.Student{ID: "stu-legacy", Grade: "三年级"}

	applyDerivedGrade(&student, "2026.2027学年")

	if student.EnrollmentGrade != "三年级" {
		t.Fatalf("入学年级应回填为 三年级，实际 %q", student.EnrollmentGrade)
	}
	if student.EnrollmentAcademicYear != "2026.2027学年" {
		t.Fatalf("入学学年应回填为当前学年，实际 %q", student.EnrollmentAcademicYear)
	}
	if student.Grade != "三年级" {
		t.Fatalf("迁移当天年级不应变化，实际 %q", student.Grade)
	}

	// 回填后的基准应当在下一学年正常滚动。
	applyDerivedGrade(&student, "2027.2028学年")
	if student.Grade != "四年级" {
		t.Fatalf("回填后次年应升为四年级，实际 %q", student.Grade)
	}
}
