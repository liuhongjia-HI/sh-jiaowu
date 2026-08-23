package store

import (
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// bindCodeAlphabet 去掉了容易读错/打错的字符（0/O、1/I/L），8 位码在这个
// 31 字符的字母表下有 31^8（约 8500 亿）种组合，靠猜是猜不出来的——真正的
// 防护还是后面登录接口那层按 IP 限流，这里的长度只是第一道门槛。
const bindCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
const bindCodeLength = 8
const bindCodeValidDays = 7

func generateBindCode() (string, error) {
	buf := make([]byte, bindCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, bindCodeLength)
	for i, b := range buf {
		code[i] = bindCodeAlphabet[int(b)%len(bindCodeAlphabet)]
	}
	return string(code), nil
}

// generateStudentBindCodeUnlocked 在后台给学生生成/重置一个绑定码，用来分享
// 给第二个家长（比如已经有一个家长绑过的孩子，另一位家长想用自己的手机号
// 也关联上）。重新生成会让旧码立刻失效——同一时刻只有一个有效码，不需要
// 单独提供"作废"操作。
func (s *MemoryStore) generateStudentBindCodeUnlocked(operator string, principal learning.Principal, id string) (learning.Student, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Student, error) {
			return work.generateStudentBindCodeUnlocked(operator, principal, id)
		})
	}
	if _, err := s.visibleStudent(principal, id); err != nil {
		return learning.Student{}, err
	}
	for i := range s.students {
		if s.students[i].ID != id {
			continue
		}
		code, err := generateBindCode()
		if err != nil {
			return learning.Student{}, errors.New("绑定码生成失败，请重试")
		}
		s.students[i].BindCode = code
		s.students[i].BindCodeExpiresAt = time.Now().AddDate(0, 0, bindCodeValidDays).Format("2006-01-02")
		s.prependLogDetail(operator, "生成学生绑定码", s.students[i].Name, "用于关联其他家长")
		return s.decorateStudent(s.students[i]), nil
	}
	return learning.Student{}, errors.New("学生不存在")
}

// claimGuardianByBindCodeUnlocked 是"凭码关联"整条路径：不看手机号是否命中
// 已有档案，只认这个码对应哪个学生。code 本身不区分大小写、允许多个家长
// 共用同一个码在有效期内各自关联（不是一次性的），失效方式只有"后台重新
// 生成"和"过期"两种。
func (s *MemoryStore) claimGuardianByBindCodeUnlocked(openID string, req learning.WechatLoginRequest) (learning.Principal, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Principal, error) {
			return work.claimGuardianByBindCodeUnlocked(openID, req)
		})
	}
	phone := strings.TrimSpace(req.Phone)
	if phone == "" {
		return learning.Principal{}, errors.New("请授权手机号后再绑定")
	}
	code := strings.ToUpper(strings.TrimSpace(req.BindCode))
	studentIndex := -1
	for i, student := range s.students {
		if student.BindCode != "" && strings.EqualFold(student.BindCode, code) {
			studentIndex = i
			break
		}
	}
	if studentIndex == -1 {
		return learning.Principal{}, errors.New("绑定码不存在，请联系老师确认")
	}
	student := s.students[studentIndex]
	if student.BindCodeExpiresAt != "" && student.BindCodeExpiresAt < time.Now().Format("2006-01-02") {
		return learning.Principal{}, errors.New("绑定码已过期，请联系老师重新获取")
	}
	if student.AccountStatus != "正常" {
		return learning.Principal{}, errors.New("学生账号已停用，请联系老师")
	}
	user, ok := s.findUserByStudentID(student.ID)
	if !ok {
		return learning.Principal{}, errors.New("学生登录账号不存在，请联系老师")
	}
	principal := principalFromUser(user)
	principal.GuardianID = s.ensureGuardianLink(phone, openID, student.ID)
	s.prependLogDetail("微信登录", "家长凭绑定码关联学生", student.Name, "手机号 "+maskPhone(phone))
	return principal, nil
}
