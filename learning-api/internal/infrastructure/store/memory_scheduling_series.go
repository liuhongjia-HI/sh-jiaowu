package store

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// 单次提交能生成的课次上限。按日重复 + 长跨度很容易一口气排出上千节，
// 没有这个闸，一次误操作就能把课表和通知全冲垮。
const maxGeneratedLessons = 200

// 1=周一 ... 7=周日，与 AvailabilitySlot.DayOfWeek 保持同一套约定。
func weekdayOf(date time.Time) int {
	if date.Weekday() == time.Sunday {
		return 7
	}
	return int(date.Weekday())
}

func normalizeRepeat(repeat *learning.ScheduleRepeat) (learning.ScheduleRepeat, error) {
	if repeat == nil {
		return learning.ScheduleRepeat{}, nil
	}
	out := *repeat
	switch out.Freq {
	case "", "daily", "weekly":
	case "monthly":
		// 客户明确按月与特殊日期后续迭代。字段结构已经能装下，
		// 这里显式拒绝，避免前端先放出来、后端静默当成不重复。
		return learning.ScheduleRepeat{}, errors.New("按月重复暂未开放")
	default:
		return learning.ScheduleRepeat{}, errors.New("不支持的重复方式")
	}
	if out.Freq == "" {
		return learning.ScheduleRepeat{}, nil
	}
	if out.Interval <= 0 {
		out.Interval = 1
	}
	if out.Interval > 52 {
		return learning.ScheduleRepeat{}, errors.New("重复间隔过大")
	}
	if out.Until == "" && out.Count <= 0 {
		return learning.ScheduleRepeat{}, errors.New("请选择重复的结束方式：按日期结束或按次数结束")
	}
	if out.Until != "" && out.Count > 0 {
		return learning.ScheduleRepeat{}, errors.New("结束方式只能二选一")
	}
	if out.Until != "" {
		if _, _, ok := parseDateBound(out.Until); !ok {
			return learning.ScheduleRepeat{}, errors.New("重复结束日期格式应为 YYYY-MM-DD")
		}
	}
	if out.Count > maxGeneratedLessons {
		return learning.ScheduleRepeat{}, errors.New("单次最多生成 " + itoa(maxGeneratedLessons) + " 节课，请缩短重复周期")
	}
	seen := map[int]bool{}
	days := make([]int, 0, len(out.ByDay))
	for _, day := range out.ByDay {
		if day < 1 || day > 7 {
			return learning.ScheduleRepeat{}, errors.New("星期取值应为 1-7")
		}
		if seen[day] {
			continue
		}
		seen[day] = true
		days = append(days, day)
	}
	sort.Ints(days)
	out.ByDay = days
	return out, nil
}

// expandRepeatDates 把重复规则展开成具体日期。不跳节假日、不跳寒暑假——
// 客户明确这件事交给排课人自己管，系统不代为判断。
func expandRepeatDates(repeat learning.ScheduleRepeat, startDate string) ([]string, error) {
	start, hasStart, ok := parseDateBound(startDate)
	if !ok || !hasStart {
		return nil, errors.New("开始日期格式应为 YYYY-MM-DD")
	}
	if repeat.Freq == "" {
		return []string{start.Format("2006-01-02")}, nil
	}
	var until time.Time
	hasUntil := false
	if repeat.Until != "" {
		parsed, has, ok := parseDateBound(repeat.Until)
		if !ok || !has {
			return nil, errors.New("重复结束日期格式应为 YYYY-MM-DD")
		}
		if parsed.Before(start) {
			return nil, errors.New("重复结束日期不能早于开始日期")
		}
		until, hasUntil = parsed, true
	}

	dates := make([]string, 0, 16)
	appendDate := func(date time.Time) bool {
		if date.Before(start) {
			return true
		}
		if hasUntil && date.After(until) {
			return false
		}
		dates = append(dates, date.Format("2006-01-02"))
		return !(repeat.Count > 0 && len(dates) >= repeat.Count)
	}

	if repeat.Freq == "daily" {
		for cursor := start; ; cursor = cursor.AddDate(0, 0, repeat.Interval) {
			if !appendDate(cursor) {
				break
			}
			if len(dates) > maxGeneratedLessons {
				break
			}
		}
	} else {
		byDay := repeat.ByDay
		if len(byDay) == 0 {
			// 没指定星期就按开始日期那一天重复，等价于 Outlook 的默认行为。
			byDay = []int{weekdayOf(start)}
		}
		// 定位到开始日期所在周的周一，之后按 interval 周推进。
		weekStart := start.AddDate(0, 0, -(weekdayOf(start) - 1))
		running := true
		for week := weekStart; running; week = week.AddDate(0, 0, 7*repeat.Interval) {
			for _, day := range byDay {
				if !appendDate(week.AddDate(0, 0, day-1)) {
					running = false
					break
				}
			}
			if len(dates) > maxGeneratedLessons {
				break
			}
			// 没有 until 也没有 count 时 normalizeRepeat 已经拦下，这里不会死循环。
			if hasUntil && week.AddDate(0, 0, 7*repeat.Interval).After(until) {
				break
			}
		}
	}

	if len(dates) == 0 {
		return nil, errors.New("按当前重复规则排不出任何课次，请检查星期与结束日期")
	}
	if len(dates) > maxGeneratedLessons {
		return nil, errors.New("单次最多生成 " + itoa(maxGeneratedLessons) + " 节课，请缩短重复周期")
	}
	sort.Strings(dates)
	// 同一天只排一节：ByDay 里理论上不会重复（normalizeRepeat 已去重），
	// 这里再兜一次，保证批内不会出现自己跟自己撞课的日期。
	unique := dates[:0]
	for i, date := range dates {
		if i == 0 || date != dates[i-1] {
			unique = append(unique, date)
		}
	}
	return unique, nil
}

// resolveEditScope 决定这次修改落在哪些课次上。
// 对重复课次，调用方必须显式给出范围——不给就报错，而不是默默挑一个。
// 「拖一节课却挪走整学期」正是因为以前没有这个必答项。
func resolveEditScope(existing learning.ScheduleClass, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if existing.SeriesID == "" {
		// 单次课没有系列可言，不需要也不应该弹三选一。
		return learning.EditScopeThis, nil
	}
	if existing.Detached {
		// 已经单独调整过的课次不再跟随系列，也不能反过来带动系列。
		if raw != "" && raw != learning.EditScopeThis {
			return "", errors.New("这节课已单独调整过，只能单独修改")
		}
		return learning.EditScopeThis, nil
	}
	switch raw {
	case learning.EditScopeThis, learning.EditScopeThisAndFuture, learning.EditScopeAll:
		return raw, nil
	case "":
		return "", errors.New("这是重复课程，请选择修改范围：仅此课次 / 此课次及后续 / 整个系列")
	default:
		return "", errors.New("不支持的修改范围")
	}
}

// dayOffset 返回 to 相对 from 相差多少天，用于把一次拖动平移到系列里的其它课次。
// 系列改期的语义是「整体平移」而不是「全都改到同一天」——
// 后者会把一学期的课全压在一天上。
func dayOffset(from, to string) (int, error) {
	fromDate, hasFrom, ok := parseDateBound(from)
	if !ok || !hasFrom {
		return 0, errors.New("原上课日期无效")
	}
	toDate, hasTo, ok := parseDateBound(to)
	if !ok || !hasTo {
		return 0, errors.New("新上课日期格式应为 YYYY-MM-DD")
	}
	return int(toDate.Sub(fromDate).Hours() / 24), nil
}

func shiftDate(date string, offsetDays int) (string, error) {
	parsed, has, ok := parseDateBound(date)
	if !ok || !has {
		return "", errors.New("上课日期无效")
	}
	return parsed.AddDate(0, 0, offsetDays).Format("2006-01-02"), nil
}

// weeklyLessonDates 把「每周 dayOfWeek，startDate 到 endDate」展开成具体日期。
// 只给存量数据迁移用：升级前的一条记录代表的是一整串课，
// 升级后要还原成一节一条。
func weeklyLessonDates(dayOfWeek int, startDate, endDate string) []string {
	start, hasStart, ok := parseDateBound(startDate)
	if !ok || !hasStart {
		return nil
	}
	end, hasEnd, ok := parseDateBound(endDate)
	if !ok || !hasEnd || end.Before(start) {
		// 没有结束日期的历史记录当作只有一节，避免凭空造出无穷课次。
		return []string{start.Format("2006-01-02")}
	}
	if dayOfWeek < 1 || dayOfWeek > 7 {
		return []string{start.Format("2006-01-02")}
	}
	// 从 start 起找到第一个符合星期的日子。
	cursor := start
	for i := 0; i < 7 && weekdayOf(cursor) != dayOfWeek; i++ {
		cursor = cursor.AddDate(0, 0, 1)
	}
	if cursor.After(end) {
		// 区间里根本没有这个星期几，退回开始日期，至少保住这条记录。
		return []string{start.Format("2006-01-02")}
	}
	dates := make([]string, 0, 16)
	for ; !cursor.After(end); cursor = cursor.AddDate(0, 0, 7) {
		dates = append(dates, cursor.Format("2006-01-02"))
	}
	return dates
}

// gradeCode 把「五年级」转成客户在 Outlook 里用的 G5 记号。
// 认不出来的年级原样返回，不要吞掉——排课标题上出现一个陌生年级，
// 比出现一个空白更容易被发现和纠正。
func gradeCode(grade string) string {
	index := gradeIndexOf(grade)
	if index < 0 {
		return strings.TrimSpace(grade)
	}
	return "G" + itoa(index+1)
}

// subjectColorEntries 读系统设置里的学科元数据。解析失败就退回内置默认值——
// 一个手滑写坏的设置项不该让所有课次的标题丢掉科目。
func (s *MemoryStore) subjectColorEntries() []subjectColorEntry {
	raw := strings.TrimSpace(s.settings["subjectColors"])
	if raw == "" {
		return defaultSubjectColors()
	}
	var entries []subjectColorEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil || len(entries) == 0 {
		return defaultSubjectColors()
	}
	return entries
}

// subjectShortLabel 取学科的短标签（Eng / Math / Geo / …）。
// 标签维护在系统设置的 subjectColors 里，和前端共用同一份元数据；
// 设置里查不到就退回学科全名。
func (s *MemoryStore) subjectShortLabel(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}
	for _, entry := range s.subjectColorEntries() {
		if entry.Subject == subject {
			if label := strings.TrimSpace(entry.ShortLabel); label != "" {
				return label
			}
			return subject
		}
	}
	return subject
}

// scheduleClassName 按客户的命名约定拼课次标题：教师 年级 科目 学生。
//
// 客户长期用 Outlook 排课，标题一直是这个顺序（Clara G5 Eng Zoe&Arthur）。
// 这个顺序是有道理的：课表上一眼要认的是「谁的课、几年级、什么科」，
// 我们原来的「英文 1V1 小班」把班型放在最前面，而班型恰恰是最不需要
// 在标题里读的信息——它在课程块上已经有独立标签了。
//
// 学生还没定时（预留时段）就只到科目为止，不留空占位。
func scheduleClassName(teacherName, grade, subjectLabel string, students []learning.CandidateStudent) string {
	parts := make([]string, 0, 4)
	for _, part := range []string{strings.TrimSpace(teacherName), gradeCode(grade), strings.TrimSpace(subjectLabel)} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	names := make([]string, 0, len(students))
	for _, student := range students {
		if name := strings.TrimSpace(student.Name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		parts = append(parts, strings.Join(names, "&"))
	}
	return strings.Join(parts, " ")
}
