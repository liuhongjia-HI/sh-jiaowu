package store

import (
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// gradeSequence 是一至十二年级序列，年级推导与基础学习空间共用同一份定义。
var gradeSequence = []string{
	"一年级", "二年级", "三年级", "四年级", "五年级", "六年级",
	"七年级", "八年级", "九年级", "十年级", "十一年级", "十二年级",
}

// gradeIndexOf 返回年级在序列中的位置，无法识别时返回 -1。
func gradeIndexOf(grade string) int {
	target := strings.TrimSpace(grade)
	for index, item := range gradeSequence {
		if item == target {
			return index
		}
	}
	return -1
}

// academicYearStart 取学年字符串的起始年份，例如 "2025.2026学年" 返回 2025。
func academicYearStart(academicYear string) (int, bool) {
	trimmed := strings.TrimSpace(academicYear)
	dot := strings.Index(trimmed, ".")
	if dot <= 0 {
		return 0, false
	}
	year, err := strconv.Atoi(trimmed[:dot])
	if err != nil || year <= 0 {
		return 0, false
	}
	return year, true
}

// resolveGrade 按入学基准推导当前年级。
// 每年 7 月 1 日随学年切换升一级，升到十二年级封顶，超出后标记为已毕业。
func resolveGrade(enrollmentAcademicYear, enrollmentGrade, currentAcademicYear string) (string, bool) {
	baseIndex := gradeIndexOf(enrollmentGrade)
	if baseIndex < 0 {
		return strings.TrimSpace(enrollmentGrade), false
	}
	enrollStart, ok := academicYearStart(enrollmentAcademicYear)
	if !ok {
		return gradeSequence[baseIndex], false
	}
	currentStart, ok := academicYearStart(currentAcademicYear)
	if !ok {
		return gradeSequence[baseIndex], false
	}
	index := baseIndex + currentStart - enrollStart
	if index < 0 {
		index = 0
	}
	if index >= len(gradeSequence) {
		return gradeSequence[len(gradeSequence)-1], true
	}
	return gradeSequence[index], false
}

// applyDerivedGrade 用入学基准覆盖学生的当前年级。
// 缺少基准的历史档案按“当前学年入学”回填，迁移当天年级保持不变。
func applyDerivedGrade(student *learning.Student, currentAcademicYear string) {
	if student == nil {
		return
	}
	if strings.TrimSpace(student.EnrollmentGrade) == "" {
		student.EnrollmentGrade = strings.TrimSpace(student.Grade)
	}
	if strings.TrimSpace(student.EnrollmentAcademicYear) == "" {
		student.EnrollmentAcademicYear = currentAcademicYear
	}
	student.Grade, student.Graduated = resolveGrade(student.EnrollmentAcademicYear, student.EnrollmentGrade, currentAcademicYear)
}

// effectiveStudentGrade 兼容历史档案：当前年级缺失时使用入学年级。
func effectiveStudentGrade(student learning.Student) string {
	grade := strings.TrimSpace(student.Grade)
	if grade == "" || grade == "待完善" {
		grade = strings.TrimSpace(student.EnrollmentGrade)
	}
	return grade
}

// refreshStudentGrades 在学生集合上重新推导年级，用于加载和跨学年刷新。
func refreshStudentGrades(students []learning.Student, now time.Time) {
	currentAcademicYear := academicYearForDate(now)
	for i := range students {
		applyDerivedGrade(&students[i], currentAcademicYear)
	}
}
