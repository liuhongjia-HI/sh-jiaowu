package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestExpandRepeatDatesSingle(t *testing.T) {
	dates, err := expandRepeatDates(learning.ScheduleRepeat{}, "2026-05-06")
	if err != nil {
		t.Fatalf("单次课不应报错: %v", err)
	}
	if len(dates) != 1 || dates[0] != "2026-05-06" {
		t.Fatalf("单次课应只生成当天一节，实际 %v", dates)
	}
}

func TestExpandRepeatDatesWeeklyDefaultsToStartWeekday(t *testing.T) {
	// 2026-05-06 是周三，没指定 ByDay 时应按周三重复。
	dates, err := expandRepeatDates(learning.ScheduleRepeat{Freq: "weekly", Interval: 1, Count: 3}, "2026-05-06")
	if err != nil {
		t.Fatalf("展开失败: %v", err)
	}
	want := []string{"2026-05-06", "2026-05-13", "2026-05-20"}
	if len(dates) != len(want) {
		t.Fatalf("应生成 %d 节，实际 %v", len(want), dates)
	}
	for i, date := range want {
		if dates[i] != date {
			t.Fatalf("第 %d 节应为 %s，实际 %s", i+1, date, dates[i])
		}
	}
}

func TestExpandRepeatDatesWeeklyMultiDaySkipsBeforeStart(t *testing.T) {
	// 开始日期是周三，ByDay 含周一：首周的周一早于开始日期，必须跳过。
	dates, err := expandRepeatDates(learning.ScheduleRepeat{
		Freq: "weekly", Interval: 1, ByDay: []int{1, 3}, Until: "2026-05-19",
	}, "2026-05-06")
	if err != nil {
		t.Fatalf("展开失败: %v", err)
	}
	want := []string{"2026-05-06", "2026-05-11", "2026-05-13", "2026-05-18"}
	if strings.Join(dates, ",") != strings.Join(want, ",") {
		t.Fatalf("应为 %v，实际 %v", want, dates)
	}
}

func TestExpandRepeatDatesWeeklyInterval(t *testing.T) {
	dates, err := expandRepeatDates(learning.ScheduleRepeat{Freq: "weekly", Interval: 2, Count: 3}, "2026-05-06")
	if err != nil {
		t.Fatalf("展开失败: %v", err)
	}
	want := []string{"2026-05-06", "2026-05-20", "2026-06-03"}
	if strings.Join(dates, ",") != strings.Join(want, ",") {
		t.Fatalf("每 2 周应为 %v，实际 %v", want, dates)
	}
}

func TestExpandRepeatDatesDaily(t *testing.T) {
	dates, err := expandRepeatDates(learning.ScheduleRepeat{Freq: "daily", Interval: 3, Count: 4}, "2026-05-06")
	if err != nil {
		t.Fatalf("展开失败: %v", err)
	}
	want := []string{"2026-05-06", "2026-05-09", "2026-05-12", "2026-05-15"}
	if strings.Join(dates, ",") != strings.Join(want, ",") {
		t.Fatalf("每 3 天应为 %v，实际 %v", want, dates)
	}
}

func TestExpandRepeatDatesDoesNotSkipHolidays(t *testing.T) {
	// 客户明确：不跳节假日、不跳寒暑假，交给排课人自己管。
	// 2026 年国庆假期区间照排不误。
	dates, err := expandRepeatDates(learning.ScheduleRepeat{Freq: "daily", Interval: 1, Until: "2026-10-05"}, "2026-10-01")
	if err != nil {
		t.Fatalf("展开失败: %v", err)
	}
	if len(dates) != 5 {
		t.Fatalf("节假日不应被跳过，应生成 5 节，实际 %v", dates)
	}
}

func TestExpandRepeatDatesRejectsOverCap(t *testing.T) {
	_, err := expandRepeatDates(learning.ScheduleRepeat{Freq: "daily", Interval: 1, Until: "2029-05-06"}, "2026-05-06")
	if err == nil {
		t.Fatal("超出生成上限时必须拒绝，否则一次误操作能排出上千节课")
	}
}

func TestNormalizeRepeatRequiresEnd(t *testing.T) {
	if _, err := normalizeRepeat(&learning.ScheduleRepeat{Freq: "weekly"}); err == nil {
		t.Fatal("没有结束方式的重复规则必须拒绝，否则无限展开")
	}
	if _, err := normalizeRepeat(&learning.ScheduleRepeat{Freq: "weekly", Until: "2026-06-01", Count: 3}); err == nil {
		t.Fatal("结束方式只能二选一")
	}
}

func TestNormalizeRepeatRejectsMonthlyForNow(t *testing.T) {
	if _, err := normalizeRepeat(&learning.ScheduleRepeat{Freq: "monthly", Count: 3}); err == nil {
		t.Fatal("按月重复本期未开放，必须显式拒绝而不是静默当成不重复")
	}
}

func TestNormalizeRepeatNilMeansSingleLesson(t *testing.T) {
	repeat, err := normalizeRepeat(nil)
	if err != nil || repeat.Freq != "" {
		t.Fatalf("repeat 为空应表示只排一节课，实际 %+v / %v", repeat, err)
	}
}

func TestWeeklyLessonDatesExpandsLegacyRange(t *testing.T) {
	// 升级前一条「每周三，2026-06-01 至 2026-06-30」的记录，
	// 应还原成区间内所有周三。
	dates := weeklyLessonDates(3, "2026-06-01", "2026-06-30")
	want := []string{"2026-06-03", "2026-06-10", "2026-06-17", "2026-06-24"}
	if strings.Join(dates, ",") != strings.Join(want, ",") {
		t.Fatalf("应展开为 %v，实际 %v", want, dates)
	}
}

func TestWeeklyLessonDatesWithoutEndDateYieldsSingleLesson(t *testing.T) {
	// 没有结束日期的历史脏数据不能凭空展开出无穷课次。
	dates := weeklyLessonDates(3, "2026-06-01", "")
	if len(dates) != 1 || dates[0] != "2026-06-01" {
		t.Fatalf("缺结束日期时应只留一节，实际 %v", dates)
	}
}

func TestWeeklyLessonDatesWhenRangeHasNoSuchWeekday(t *testing.T) {
	// 区间里根本没有那个星期几时，至少要保住这条记录，不能丢。
	dates := weeklyLessonDates(3, "2026-06-04", "2026-06-06")
	if len(dates) != 1 || dates[0] != "2026-06-04" {
		t.Fatalf("区间内无对应星期时应保留原记录，实际 %v", dates)
	}
}

func TestWeeklyLessonDatesRejectsUnparsableStart(t *testing.T) {
	if dates := weeklyLessonDates(3, "", "2026-06-30"); dates != nil {
		t.Fatalf("开始日期解析不出来时应交给调用方兜底，实际 %v", dates)
	}
}

func TestScheduleClassNameFollowsCustomerConvention(t *testing.T) {
	students := []learning.CandidateStudent{{Name: "Zoe"}, {Name: "Arthur"}}
	got := scheduleClassName("Clara", "五年级", "Eng", students)
	// 对照客户 Outlook 里的标题：Clara G5 Eng Zoe&Arthur
	if got != "Clara G5 Eng Zoe&Arthur" {
		t.Fatalf("标题应为 Clara G5 Eng Zoe&Arthur，实际 %q", got)
	}
}

func TestScheduleClassNameWithoutStudents(t *testing.T) {
	// 预留时段（还没定学生）时只拼到科目，不留空占位。
	got := scheduleClassName("Beth", "八年级", "Math", nil)
	if got != "Beth G8 Math" {
		t.Fatalf("无学生时应为 Beth G8 Math，实际 %q", got)
	}
}

func TestScheduleClassNameKeepsUnknownGradeVisible(t *testing.T) {
	// 认不出的年级原样保留：标题里出现一个陌生年级，
	// 比出现一段空白更容易被发现和纠正。
	got := scheduleClassName("Gavin", "预备班", "Geo", nil)
	if got != "Gavin 预备班 Geo" {
		t.Fatalf("未知年级应原样保留，实际 %q", got)
	}
}

func TestGradeCodeMapsToOutlookNotation(t *testing.T) {
	cases := map[string]string{"一年级": "G1", "五年级": "G5", "九年级": "G9"}
	for grade, want := range cases {
		if got := gradeCode(grade); got != want {
			t.Fatalf("%s 应为 %s，实际 %s", grade, want, got)
		}
	}
}

func TestSubjectShortLabelFallsBackToFullName(t *testing.T) {
	store := NewMemoryStore()
	if got := store.subjectShortLabel("英文"); got != "Eng" {
		t.Fatalf("英文应取短标签 Eng，实际 %q", got)
	}
	// 元数据里没有的学科退回全名，不能返回空——标题少一段比多一段难查。
	if got := store.subjectShortLabel("音乐"); got != "音乐" {
		t.Fatalf("未配置的学科应退回全名，实际 %q", got)
	}
}

func TestSubjectShortLabelSurvivesBrokenSettings(t *testing.T) {
	store := NewMemoryStore()
	store.settings["subjectColors"] = "{ 这不是合法 JSON"
	if got := store.subjectShortLabel("数学"); got != "Math" {
		t.Fatalf("设置写坏时应退回内置默认值，实际 %q", got)
	}
}
