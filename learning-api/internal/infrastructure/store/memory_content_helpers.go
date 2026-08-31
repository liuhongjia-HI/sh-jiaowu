package store

import (
	"html"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func countSubmittedStudents(students []learning.HomeworkSubmissionStudent) int {
	count := 0
	for _, student := range students {
		if student.SubmissionID != "" {
			count++
		}
	}
	return count
}

func (s *MemoryStore) deliverNotice(notice learning.Notice) learning.Notice {
	if notice.Channel == "" {
		notice.Channel = "站内通知"
	}
	if notice.Channel == "公众号模板消息" {
		if notice.RecipientOpenID == "" {
			notice.Status = "待配置"
			notice.FailureReason = "需先关联公众号并填写接收人 openid。"
			return notice
		}
		if s.officialNoticeSender == nil {
			notice.Status = "待配置"
			notice.FailureReason = "需配置 WECHAT_OFFICIAL_ACCOUNT_APPID、WECHAT_OFFICIAL_ACCOUNT_SECRET、WECHAT_OFFICIAL_ACCOUNT_TEMPLATE_ID。"
			return notice
		}
		notice.Status = "发送中"
		notice.FailureReason = ""
		s.pendingNoticeDeliveries = mergePendingNoticeDeliveries(s.pendingNoticeDeliveries, []learning.Notice{notice})
		return notice
	}
	notice.Status = "已发送"
	notice.FailureReason = ""
	return notice
}

func (s *MemoryStore) prependNoticeRecord(notice learning.Notice) {
	if notice.Channel == "公众号模板消息" {
		station := stationNoticeForOfficialNotice(notice)
		s.notices = append([]learning.Notice{notice, station}, s.notices...)
		return
	}
	s.notices = append([]learning.Notice{notice}, s.notices...)
}

func (s *MemoryStore) ensureStationNoticeForOfficialNotice(notice learning.Notice) {
	if notice.Channel != "公众号模板消息" {
		return
	}
	stationID := stationNoticeID(notice.ID)
	for _, item := range s.notices {
		if item.ID == stationID {
			return
		}
	}
	station := stationNoticeForOfficialNotice(notice)
	s.notices = append([]learning.Notice{station}, s.notices...)
}

func stationNoticeForOfficialNotice(notice learning.Notice) learning.Notice {
	station := notice
	station.ID = stationNoticeID(notice.ID)
	station.Channel = "站内通知"
	station.RecipientOpenID = ""
	station.Status = "已发送"
	station.FailureReason = ""
	station.RetryCount = 0
	return station
}

func stationNoticeID(id string) string {
	return strings.TrimSpace(id) + "-station"
}

func appendCourseUnique(values []learning.Course, addition learning.Course) []learning.Course {
	for _, value := range values {
		if value.ID == addition.ID {
			return values
		}
	}
	return append(values, addition)
}

func appendMaterialUnique(values []learning.Material, addition learning.Material) []learning.Material {
	for _, value := range values {
		if value.ID == addition.ID {
			return values
		}
	}
	return append(values, addition)
}

func appendHomeworkUnique(values []learning.Homework, addition learning.Homework) []learning.Homework {
	for _, value := range values {
		if value.ID == addition.ID {
			return values
		}
	}
	return append(values, cloneHomework(addition))
}

func bankItemQuestion(item learning.QuestionBankItem) learning.Question {
	answer := item.Answer
	answers := cleanPhrases(item.Answers)
	if answer == "" && len(answers) > 0 {
		answer = answers[0]
	}
	return learning.Question{
		ID: item.ID, Title: item.Title, Type: item.Type, Stem: item.Stem, Options: append([]string(nil), item.Options...),
		Answer: answer, Answers: answers, Score: item.Score,
	}
}

func shortQuestionTitle(stem string) string {
	title := richTextPlainText(stem)
	if len([]rune(title)) <= 24 {
		return title
	}
	return string([]rune(title)[:24]) + "..."
}

func sanitizeRichText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = richTextScriptPattern.ReplaceAllString(value, "")
	value = richTextEventAttrPattern.ReplaceAllString(value, "")
	value = richTextJSURLAttrPattern.ReplaceAllString(value, "")
	return strings.TrimSpace(richTextTagPattern.ReplaceAllStringFunc(value, sanitizeRichTextTag))
}

func sanitizeRichTextTag(tag string) string {
	match := richTextTagNamePattern.FindStringSubmatch(tag)
	if len(match) < 2 {
		return ""
	}
	name := strings.ToLower(match[1])
	closing := strings.HasPrefix(strings.TrimSpace(tag), "</")
	switch name {
	case "strong", "b", "ul", "ol", "li", "p":
		if closing {
			return "</" + name + ">"
		}
		return "<" + name + ">"
	case "br":
		return "<br />"
	case "span":
		if closing {
			return "</span>"
		}
		color := richTextColorPattern.FindStringSubmatch(tag)
		if len(color) >= 2 {
			return `<span style="color:` + color[1] + `">`
		}
		return "<span>"
	case "img":
		if closing {
			return ""
		}
		src := richTextHTTPSImagePattern.FindStringSubmatch(tag)
		if len(src) >= 2 {
			return `<img src="` + html.EscapeString(src[1]) + `" alt="老师建议配图" />`
		}
		return ""
	default:
		return ""
	}
}

func richTextPlainText(value string) string {
	value = html.UnescapeString(value)
	var builder strings.Builder
	inTag := false
	lastSpace := false
	for _, char := range value {
		switch char {
		case '<':
			inTag = true
			if !lastSpace {
				builder.WriteRune(' ')
				lastSpace = true
			}
		case '>':
			inTag = false
		default:
			if inTag {
				continue
			}
			if char == '\n' || char == '\r' || char == '\t' || char == ' ' {
				if !lastSpace {
					builder.WriteRune(' ')
					lastSpace = true
				}
				continue
			}
			builder.WriteRune(char)
			lastSpace = false
		}
	}
	title := strings.TrimSpace(builder.String())
	if title == "" {
		return "图片题"
	}
	return title
}

func questionIDs(questions []learning.Question) []string {
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}
	return ids
}

func normalizedQuestionAnswers(question learning.Question) []string {
	if len(question.Answers) > 0 {
		return cleanPhrases(question.Answers)
	}
	if strings.TrimSpace(question.Answer) != "" {
		return []string{strings.TrimSpace(question.Answer)}
	}
	return nil
}

func sameChoiceSet(left, right []string) bool {
	a := cleanPhrases(left)
	b := cleanPhrases(right)
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for index := range a {
		if !strings.EqualFold(a[index], b[index]) {
			return false
		}
	}
	return true
}

func grantEndsAt(grant packageGrant) string {
	if grant.EndsAt != "" {
		return grant.EndsAt
	}
	return grant.EffectiveUntil
}

func grantOpenedAt(grant packageGrant) string {
	if grant.OpenedAt != "" {
		return grant.OpenedAt
	}
	if grant.StartsAt != "" {
		_, normalized, err := normalizeGrantTimestamp(grant.StartsAt, false)
		if err == nil {
			return normalized
		}
	}
	return time.Now().Format("2006-01-02 15:04:05")
}

func grantActive(grant packageGrant) bool {
	status := grant.Status
	if status == "" {
		status = "active"
	}
	return status == "active" && grantPeriodActive(grant.StartsAt, grantEndsAt(grant))
}

func grantPermissionState(grant packageGrant) string {
	if grant.Status == "revoked" || grantPeriodExpired(grantEndsAt(grant)) {
		return "已到期"
	}
	if grantPeriodNotStarted(grant.StartsAt) {
		return "未开始"
	}
	return "生效中"
}

func grantPeriodActive(startsAt, endsAt string) bool {
	now := time.Now()
	if startsAt != "" {
		start, _, err := normalizeGrantTimestamp(startsAt, false)
		if err != nil || now.Before(start) {
			return false
		}
	}
	if endsAt == "" {
		return false
	}
	end, _, err := normalizeGrantTimestamp(endsAt, true)
	return err == nil && !now.After(end)
}

func grantPeriodExpired(endsAt string) bool {
	if endsAt == "" {
		return true
	}
	end, _, err := normalizeGrantTimestamp(endsAt, true)
	return err != nil || time.Now().After(end)
}

func grantPeriodNotStarted(startsAt string) bool {
	if startsAt == "" {
		return false
	}
	start, _, err := normalizeGrantTimestamp(startsAt, false)
	return err != nil || time.Now().Before(start)
}

func contentTypeLabel(value string) string {
	switch value {
	case "course":
		return "课程"
	case "question":
		return "题"
	case "handout":
		return "学习资料"
	case "download":
		return "下载讲义"
	default:
		return value
	}
}

func validContentType(value string) bool {
	return value == "course" || value == "question" || value == "handout" || value == "download"
}

func packageTypeLabel(values []string) string {
	labels := make([]string, 0, len(values))
	for _, value := range []string{"course", "question", "handout", "download"} {
		if containsString(values, value) {
			labels = append(labels, contentTypeLabel(value))
		}
	}
	if len(labels) == 0 {
		return "自定义"
	}
	return strings.Join(labels, "+")
}

func courseNames(courses []learning.Course) []string {
	out := make([]string, 0, len(courses))
	for _, course := range courses {
		out = append(out, course.Name)
	}
	return out
}

func subjectsForCourses(courses []learning.Course) []string {
	out := make([]string, 0, len(courses))
	for _, course := range courses {
		out = appendUnique(out, course.Subject)
	}
	return out
}
