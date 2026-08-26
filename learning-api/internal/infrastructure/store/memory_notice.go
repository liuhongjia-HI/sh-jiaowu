package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) noticesUnlocked(principal learning.Principal) []learning.Notice {
	out := make([]learning.Notice, 0, len(s.notices))
	for _, notice := range s.notices {
		if s.canSeeNotice(principal, notice) {
			out = append(out, notice)
		}
	}
	return out
}

func (s *MemoryStore) createNoticeUnlocked(operator string, principal learning.Principal, req learning.NoticeCreateRequest) (learning.Notice, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Notice, error) {
			return work.createNoticeUnlocked(operator, principal, req)
		})
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Title = strings.TrimSpace(req.Title)
	req.Target = strings.TrimSpace(req.Target)
	req.Summary = strings.TrimSpace(req.Summary)
	req.Channel = strings.TrimSpace(req.Channel)
	req.RecipientOpenID = strings.TrimSpace(req.RecipientOpenID)
	req.RelatedType = strings.TrimSpace(req.RelatedType)
	req.RelatedID = strings.TrimSpace(req.RelatedID)
	if req.Type == "" {
		req.Type = "通知"
	}
	if req.Channel == "" {
		req.Channel = "站内通知"
	}
	if req.Title == "" {
		return learning.Notice{}, errors.New("请输入通知标题")
	}
	if req.Target == "" {
		return learning.Notice{}, errors.New("请选择或填写接收对象")
	}
	if req.Summary == "" {
		return learning.Notice{}, errors.New("请输入通知内容")
	}
	if !s.canSendNoticeTo(principal, req.Target, req.Title, req.Summary) {
		return learning.Notice{}, errors.New("不能发送到未负责的学生范围")
	}
	if req.Channel == "公众号模板消息" && req.RecipientOpenID == "" {
		req.RecipientOpenID = s.officialAccountOpenIDForTarget(req.Target)
	}
	notice := learning.Notice{
		ID:              "notice-" + time.Now().Format("20060102150405.000000000"),
		Type:            req.Type,
		Title:           req.Title,
		Target:          req.Target,
		Summary:         req.Summary,
		Channel:         req.Channel,
		RecipientOpenID: req.RecipientOpenID,
		RelatedType:     req.RelatedType,
		RelatedID:       req.RelatedID,
	}
	notice = s.deliverNotice(notice)
	s.prependNoticeRecord(notice)
	s.prependLog(operator, "发送通知", notice.Target+" / "+notice.Title)
	return notice, nil
}

func (s *MemoryStore) retryNoticeUnlocked(operator string, principal learning.Principal, id string) (learning.Notice, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Notice, error) {
			return work.retryNoticeUnlocked(operator, principal, id)
		})
	}
	id = strings.TrimSpace(id)
	for index := range s.notices {
		if s.notices[index].ID != id {
			continue
		}
		notice := s.notices[index]
		if !s.canSeeNotice(principal, notice) {
			return learning.Notice{}, errors.New("不能补发未负责范围的通知")
		}
		if notice.Channel == "公众号模板消息" && notice.RecipientOpenID == "" {
			notice.RecipientOpenID = s.officialAccountOpenIDForTarget(notice.Target)
		}
		notice.RetryCount++
		notice = s.deliverNotice(notice)
		s.notices[index] = notice
		s.ensureStationNoticeForOfficialNotice(notice)
		s.prependLog(operator, "补发通知", notice.Target+" / "+notice.Title)
		return notice, nil
	}
	return learning.Notice{}, errors.New("通知记录不存在")
}

func (s *MemoryStore) canSeeNotice(principal learning.Principal, notice learning.Notice) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	subjects := subjectsForCourses(s.coursesUnlocked(principal))
	return canSeeSubject(principal, subjects, notice.Target) ||
		canSeeSubject(principal, subjects, notice.Title) ||
		canSeeSubject(principal, subjects, notice.Summary)
}

func readinessFromConfirmedSetting(key, title, value, readyMessage, action string) learning.ReadinessItem {
	value = strings.TrimSpace(value)
	if value == "已完成" || value == "已配置" || value == "已确认" {
		return learning.ReadinessItem{Key: key, Title: title, Status: "ready", Message: readyMessage}
	}
	if value == "" {
		value = "待确认"
	}
	return learning.ReadinessItem{
		Key:     key,
		Title:   title,
		Status:  "missing",
		Message: title + "当前为“" + value + "”。",
		Action:  action,
	}
}

func readinessFromDomain(value string) learning.ReadinessItem {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "https://") && !strings.Contains(value, "localhost") && !strings.Contains(value, "127.0.0.1") {
		return learning.ReadinessItem{
			Key:     "productionApiDomain",
			Title:   "生产接口域名",
			Status:  "ready",
			Message: "已配置生产接口域名：" + value,
		}
	}
	if value == "" {
		value = "待配置"
	}
	return learning.ReadinessItem{
		Key:     "productionApiDomain",
		Title:   "生产接口域名",
		Status:  "missing",
		Message: "当前生产接口域名为“" + value + "”。",
		Action:  "配置 https 生产接口域名，并确保已加入小程序合法域名。",
	}
}

func (s *MemoryStore) officialAccountConfigReadiness() learning.ReadinessItem {
	if s.officialAccountReady && s.officialNoticeSender != nil {
		return learning.ReadinessItem{
			Key:     "officialAccountConfig",
			Title:   "公众号发送配置",
			Status:  "ready",
			Message: "已配置公众号 AppID、Secret 和模板 ID，系统可尝试发送模板消息。",
		}
	}
	return learning.ReadinessItem{
		Key:     "officialAccountConfig",
		Title:   "公众号发送配置",
		Status:  "missing",
		Message: "公众号模板消息环境变量未完整配置。",
		Action:  "配置 WECHAT_OFFICIAL_ACCOUNT_APPID、WECHAT_OFFICIAL_ACCOUNT_SECRET、WECHAT_OFFICIAL_ACCOUNT_TEMPLATE_ID 后重启服务。",
	}
}

func (s *MemoryStore) studentOfficialAccountReadiness() learning.ReadinessItem {
	opened := map[string]bool{}
	for _, grant := range s.grants {
		if grantActive(grant) {
			opened[grant.StudentID] = true
		}
	}
	total := 0
	linked := 0
	for _, student := range s.students {
		if !opened[student.ID] {
			continue
		}
		total++
		if strings.TrimSpace(student.OfficialAccountOpenID) != "" {
			linked++
		}
	}
	if total == 0 {
		return learning.ReadinessItem{
			Key:     "studentOfficialAccountOpenID",
			Title:   "学生公众号关联",
			Status:  "warning",
			Message: "当前没有已开通套餐的学生，暂时无法计算公众号 openid 覆盖率。",
			Action:  "先为学生开通学习套餐，再补充公众号 openid。",
		}
	}
	message := fmt.Sprintf("已开通学生公众号 openid 覆盖 %d/%d。", linked, total)
	if linked == total {
		return learning.ReadinessItem{
			Key:     "studentOfficialAccountOpenID",
			Title:   "学生公众号关联",
			Status:  "ready",
			Message: message,
		}
	}
	return learning.ReadinessItem{
		Key:     "studentOfficialAccountOpenID",
		Title:   "学生公众号关联",
		Status:  "warning",
		Message: message,
		Action:  "在学生档案补充公众号 openid，或通过关注公众号后的绑定流程自动回填。",
	}
}

func (s *MemoryStore) logsUnlocked() []learning.OperationLog {
	return append([]learning.OperationLog(nil), s.logs...)
}

func (s *MemoryStore) settingsUnlocked() map[string]string {
	out := make(map[string]string, len(s.settings))
	for key, value := range s.settings {
		out[key] = value
	}
	return out
}

func (s *MemoryStore) updateSettingUnlocked(operator string, req learning.SettingUpdateRequest) (map[string]string, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (map[string]string, error) { return work.updateSettingUnlocked(operator, req) })
	}
	req.Key = strings.TrimSpace(req.Key)
	req.Value = strings.TrimSpace(req.Value)
	if req.Key == "" {
		return nil, errors.New("请选择要修改的设置项")
	}
	if req.Value == "" {
		return nil, errors.New("设置值不能为空")
	}
	if _, ok := s.settings[req.Key]; !ok {
		return nil, errors.New("设置项不存在")
	}
	if err := s.validateSettingValue(req.Key, req.Value); err != nil {
		return nil, err
	}
	before := map[string]string{req.Key: s.settings[req.Key]}
	s.settings[req.Key] = req.Value
	after := map[string]string{req.Key: s.settings[req.Key]}
	s.prependLogDetail(operator, "修改系统设置", settingLabel(req.Key), auditChangeDetail(before, after))
	return s.settingsUnlocked(), nil
}

func (s *MemoryStore) validateSettingValue(key, value string) error {
	if key == "downloadPolicy" && value != "仅在线预览" && value != "允许下载带水印PDF" {
		return errors.New("下载规则只能选择“仅在线预览”或“允许下载带水印PDF”")
	}
	if key == "grantDefaultStart" || key == "grantDefaultEnd" { // 已由校历取代，保留校验以兼容历史存量值
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return errors.New("默认时间格式应为 YYYY-MM-DD")
		}
		start, end := s.settings["grantDefaultStart"], s.settings["grantDefaultEnd"]
		if key == "grantDefaultStart" {
			start = value
		} else {
			end = value
		}
		if start != "" && end != "" && end < start {
			return errors.New("默认结束日期不能早于开始日期")
		}
	}
	if key == "academicCalendar" {
		var terms []academicCalendarTerm
		if err := json.Unmarshal([]byte(value), &terms); err != nil || len(terms) == 0 {
			return errors.New("校历需填写有效的学期列表")
		}
		for _, term := range terms {
			if strings.TrimSpace(term.AcademicYear) == "" {
				return errors.New("校历每一条都要填学年")
			}
			if strings.TrimSpace(term.Semester) == "" {
				return errors.New("校历每一条都要填学期")
			}
			start, startErr := time.Parse("2006-01-02", term.StartDate)
			end, endErr := time.Parse("2006-01-02", term.EndDate)
			if startErr != nil || endErr != nil || end.Before(start) {
				return errors.New("校历日期无效或结束日期早于开始日期")
			}
		}
	}
	return nil
}
