package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

const (
	wechatMiniNameKey       = "wechat.mini.name"
	wechatMiniAppIDKey      = "wechat.mini.appid"
	wechatMiniSecretKey     = "wechat.mini.secret"
	wechatOfficialNameKey   = "wechat.official.name"
	wechatOfficialAppIDKey  = "wechat.official.appid"
	wechatOfficialSecretKey = "wechat.official.secret"
	wechatOfficialOriginal  = "wechat.official.original"
	wechatCallbackTokenKey  = "wechat.official.token"
	wechatEncodingAESKey    = "wechat.official.aeskey"
)

var officialTemplateVariablePattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\.DATA\}\}`)

func (s *MemoryStore) seedOfficialMessagingDemoData() {
	if len(s.officialTemplates) > 0 {
		return
	}
	s.officialTemplates = []learning.OfficialTemplate{{
		ID: "demo-course-change", Title: "课程调整通知", Status: "启用", SyncedAt: time.Now().Format("2006-01-02 15:04:05"),
		Content: "通知标题：{{thing1.DATA}}\n上课时间：{{time2.DATA}}\n通知内容：{{thing3.DATA}}\n补充说明：{{thing4.DATA}}",
		Fields: []learning.OfficialTemplateField{
			{Key: "thing1", Label: "通知标题", MaxLength: 20},
			{Key: "time2", Label: "上课时间", MaxLength: 20},
			{Key: "thing3", Label: "通知内容", MaxLength: 50},
			{Key: "thing4", Label: "补充说明", MaxLength: 50},
		},
	}}
}

func (s *MemoryStore) encryptWechatValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	key := sha256.Sum256([]byte(s.wechatEncryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	return "enc:v1:" + base64.RawStdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (s *MemoryStore) decryptWechatValue(value string) string {
	if !strings.HasPrefix(value, "enc:v1:") {
		return ""
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, "enc:v1:"))
	if err != nil {
		return ""
	}
	key := sha256.Sum256([]byte(s.wechatEncryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return ""
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return ""
	}
	return string(plain)
}

func (s *MemoryStore) wechatSettingsUnlocked() learning.WechatSettings {
	return learning.WechatSettings{
		MiniProgramName:                 s.settings[wechatMiniNameKey],
		MiniProgramAppID:                s.settings[wechatMiniAppIDKey],
		MiniProgramSecretConfigured:     s.decryptWechatValue(s.settings[wechatMiniSecretKey]) != "",
		OfficialAccountName:             s.settings[wechatOfficialNameKey],
		OfficialAccountAppID:            s.settings[wechatOfficialAppIDKey],
		OfficialAccountOriginalID:       s.settings[wechatOfficialOriginal],
		OfficialAccountSecretConfigured: s.decryptWechatValue(s.settings[wechatOfficialSecretKey]) != "",
		CallbackTokenConfigured:         s.decryptWechatValue(s.settings[wechatCallbackTokenKey]) != "",
		EncodingAESKeyConfigured:        s.decryptWechatValue(s.settings[wechatEncodingAESKey]) != "",
		CallbackURL:                     s.wechatCallbackURL,
	}
}

func (s *MemoryStore) WechatSettings() learning.WechatSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wechatSettingsUnlocked()
}

func requireWechatAppID(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("请输入%s AppID", label)
	}
	if !strings.HasPrefix(value, "wx") || len(value) < 10 {
		return fmt.Errorf("%s AppID 格式不正确", label)
	}
	return nil
}

func (s *MemoryStore) updateWechatSettingsUnlocked(operator string, req learning.WechatSettingsUpdateRequest) (learning.WechatSettings, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.WechatSettings, error) {
			return work.updateWechatSettingsUnlocked(operator, req)
		})
	}
	req.MiniProgramName = strings.TrimSpace(req.MiniProgramName)
	req.MiniProgramAppID = strings.TrimSpace(req.MiniProgramAppID)
	req.OfficialAccountName = strings.TrimSpace(req.OfficialAccountName)
	req.OfficialAccountAppID = strings.TrimSpace(req.OfficialAccountAppID)
	req.OfficialAccountOriginalID = strings.TrimSpace(req.OfficialAccountOriginalID)
	if req.MiniProgramName == "" || req.OfficialAccountName == "" {
		return learning.WechatSettings{}, errors.New("请填写小程序和公众号名称")
	}
	if err := requireWechatAppID("小程序", req.MiniProgramAppID); err != nil {
		return learning.WechatSettings{}, err
	}
	if err := requireWechatAppID("公众号", req.OfficialAccountAppID); err != nil {
		return learning.WechatSettings{}, err
	}
	if req.OfficialAccountOriginalID == "" {
		return learning.WechatSettings{}, errors.New("请输入公众号原始 ID")
	}
	miniSecret := strings.TrimSpace(req.MiniProgramAppSecret)
	if miniSecret == "" {
		miniSecret = s.decryptWechatValue(s.settings[wechatMiniSecretKey])
	}
	officialSecret := strings.TrimSpace(req.OfficialAccountAppSecret)
	if officialSecret == "" {
		officialSecret = s.decryptWechatValue(s.settings[wechatOfficialSecretKey])
	}
	callbackToken := strings.TrimSpace(req.CallbackToken)
	if callbackToken == "" {
		callbackToken = s.decryptWechatValue(s.settings[wechatCallbackTokenKey])
	}
	encodingKey := strings.TrimSpace(req.EncodingAESKey)
	if encodingKey == "" {
		encodingKey = s.decryptWechatValue(s.settings[wechatEncodingAESKey])
	}
	if miniSecret == "" || officialSecret == "" || callbackToken == "" || encodingKey == "" {
		return learning.WechatSettings{}, errors.New("首次配置时请完整填写 AppSecret、Token 和 EncodingAESKey")
	}
	if len(callbackToken) < 3 || len(callbackToken) > 32 {
		return learning.WechatSettings{}, errors.New("回调 Token 应为 3 至 32 个字符")
	}
	if len(encodingKey) != 43 {
		return learning.WechatSettings{}, errors.New("EncodingAESKey 应为 43 个字符")
	}
	client := &http.Client{Timeout: 8 * time.Second}
	if _, err := wechatAccessToken(client, req.MiniProgramAppID, miniSecret); err != nil {
		return learning.WechatSettings{}, fmt.Errorf("小程序配置校验失败：%w", err)
	}
	if _, err := wechatAccessToken(client, req.OfficialAccountAppID, officialSecret); err != nil {
		return learning.WechatSettings{}, fmt.Errorf("公众号配置校验失败：%w", err)
	}
	values := map[string]string{
		wechatMiniNameKey: req.MiniProgramName, wechatMiniAppIDKey: req.MiniProgramAppID,
		wechatOfficialNameKey: req.OfficialAccountName, wechatOfficialAppIDKey: req.OfficialAccountAppID,
		wechatOfficialOriginal: req.OfficialAccountOriginalID,
	}
	secretInputs := map[string]string{
		wechatMiniSecretKey: req.MiniProgramAppSecret, wechatOfficialSecretKey: req.OfficialAccountAppSecret,
		wechatCallbackTokenKey: req.CallbackToken, wechatEncodingAESKey: req.EncodingAESKey,
	}
	for key, value := range secretInputs {
		if strings.TrimSpace(value) == "" {
			continue
		}
		encrypted, err := s.encryptWechatValue(value)
		if err != nil {
			return learning.WechatSettings{}, errors.New("微信敏感配置加密失败")
		}
		values[key] = encrypted
	}
	for key, value := range values {
		s.settings[key] = value
	}
	s.applyStoredWechatSettingsUnlocked()
	s.prependLog(operator, "修改微信配置", "小程序与公众号")
	return s.wechatSettingsUnlocked(), nil
}

func (s *MemoryStore) UpdateWechatSettings(operator string, req learning.WechatSettingsUpdateRequest) (learning.WechatSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateWechatSettingsUnlocked(operator, req)
}

func (s *MemoryStore) applyStoredWechatSettingsUnlocked() {
	miniAppID := s.settings[wechatMiniAppIDKey]
	miniSecret := s.decryptWechatValue(s.settings[wechatMiniSecretKey])
	if miniAppID != "" && miniSecret != "" {
		s.useWechatAPIUnlocked(miniAppID, miniSecret)
	}
	officialAppID := s.settings[wechatOfficialAppIDKey]
	officialSecret := s.decryptWechatValue(s.settings[wechatOfficialSecretKey])
	if officialAppID != "" && officialSecret != "" {
		s.useOfficialAccountMessagingUnlocked(officialAppID, officialSecret, miniAppID)
	}
}

func fieldMaxLength(key string) int {
	switch {
	case strings.HasPrefix(key, "phrase"):
		return 5
	case strings.HasPrefix(key, "thing"):
		return 50
	case strings.HasPrefix(key, "time"), strings.HasPrefix(key, "date"):
		return 20
	default:
		return 32
	}
}

func parseOfficialTemplateFields(content string) []learning.OfficialTemplateField {
	seen := map[string]bool{}
	fields := []learning.OfficialTemplateField{}
	for _, line := range strings.Split(content, "\n") {
		matches := officialTemplateVariablePattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			key := match[1]
			if seen[key] {
				continue
			}
			seen[key] = true
			label := strings.TrimSpace(strings.SplitN(line, "：", 2)[0])
			if label == "" || strings.Contains(label, "{{") {
				switch key {
				case "first":
					label = "通知标题"
				case "remark":
					label = "补充说明"
				default:
					label = "模板内容"
				}
			}
			fields = append(fields, learning.OfficialTemplateField{Key: key, Label: label, MaxLength: fieldMaxLength(key)})
		}
	}
	return fields
}

func (s *MemoryStore) OfficialTemplates() []learning.OfficialTemplate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneOfficialTemplates(s.officialTemplates)
}

func (s *MemoryStore) SyncOfficialTemplates(operator string) ([]learning.OfficialTemplate, error) {
	s.mu.Lock()
	syncer := s.officialTemplateSyncer
	s.mu.Unlock()
	if syncer == nil {
		return nil, errors.New("请先在系统设置中完成公众号配置")
	}
	templates, err := syncer()
	if err != nil {
		return nil, err
	}
	for i := range templates {
		templates[i].Fields = parseOfficialTemplateFields(templates[i].Content)
		templates[i].Status = "启用"
		templates[i].SyncedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := persistentMutation(s, func(work *MemoryStore) ([]learning.OfficialTemplate, error) {
		work.officialTemplates = cloneOfficialTemplates(templates)
		work.prependLog(operator, "同步公众号模板", fmt.Sprintf("同步 %d 个模板", len(templates)))
		return cloneOfficialTemplates(work.officialTemplates), nil
	})
	return result, err
}

func (s *MemoryStore) SyncOfficialFollowers(operator string) (learning.OfficialFollowerSyncResult, error) {
	s.mu.Lock()
	syncer := s.officialFollowerSyncer
	s.mu.Unlock()
	if syncer == nil {
		return learning.OfficialFollowerSyncResult{}, errors.New("请先在系统设置中完成公众号配置")
	}
	followers, err := syncer()
	if err != nil {
		return learning.OfficialFollowerSyncResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.OfficialFollowerSyncResult, error) {
		now := time.Now().Format("2006-01-02 15:04:05")
		byOpenID := map[string]learning.OfficialFollower{}
		for _, old := range work.officialFollowers {
			old.Subscribed = false
			old.UnsubscribedAt = now
			byOpenID[old.OpenID] = old
		}
		for _, follower := range followers {
			byOpenID[follower.OpenID] = follower
		}
		work.officialFollowers = make([]learning.OfficialFollower, 0, len(byOpenID))
		guardianUnions := map[string]bool{}
		for _, guardian := range work.guardians {
			if guardian.UnionID != "" {
				guardianUnions[guardian.UnionID] = true
			}
		}
		result := learning.OfficialFollowerSyncResult{TotalCount: len(byOpenID)}
		for _, follower := range byOpenID {
			work.officialFollowers = append(work.officialFollowers, follower)
			if follower.Subscribed {
				result.SubscribedCount++
				if guardianUnions[follower.UnionID] {
					result.MatchedCount++
				}
			}
		}
		result.UnmatchedCount = result.SubscribedCount - result.MatchedCount
		work.prependLog(operator, "同步公众号关注者", fmt.Sprintf("关注 %d，已匹配 %d", result.SubscribedCount, result.MatchedCount))
		return result, nil
	})
}

func (s *MemoryStore) VerifyOfficialCallback(signature, timestamp, nonce string) bool {
	s.mu.Lock()
	token := s.decryptWechatValue(s.settings[wechatCallbackTokenKey])
	s.mu.Unlock()
	if token == "" || signature == "" {
		return false
	}
	values := []string{token, timestamp, nonce}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	return strings.EqualFold(signature, hex.EncodeToString(sum[:]))
}

func (s *MemoryStore) DecryptOfficialCallback(signature, timestamp, nonce, encrypted string) ([]byte, error) {
	s.mu.Lock()
	token := s.decryptWechatValue(s.settings[wechatCallbackTokenKey])
	encodingKey := s.decryptWechatValue(s.settings[wechatEncodingAESKey])
	appID := s.settings[wechatOfficialAppIDKey]
	s.mu.Unlock()
	if token == "" || encodingKey == "" || encrypted == "" {
		return nil, errors.New("公众号安全回调配置不完整")
	}
	parts := []string{token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	expected := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(signature)), []byte(expected)) != 1 {
		return nil, errors.New("公众号回调签名无效")
	}
	key, err := base64.StdEncoding.DecodeString(encodingKey + "=")
	if err != nil || len(key) != 32 {
		return nil, errors.New("EncodingAESKey 配置无效")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("公众号加密消息格式无效")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plain, ciphertext)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > 32 || padding > len(plain) {
		return nil, errors.New("公众号加密消息填充无效")
	}
	plain = plain[:len(plain)-padding]
	if len(plain) < 20 {
		return nil, errors.New("公众号加密消息内容无效")
	}
	xmlLength := int(binary.BigEndian.Uint32(plain[16:20]))
	if xmlLength < 0 || 20+xmlLength > len(plain) {
		return nil, errors.New("公众号加密消息长度无效")
	}
	xmlBody := plain[20 : 20+xmlLength]
	receiverID := string(plain[20+xmlLength:])
	if appID != "" && receiverID != "" && receiverID != appID {
		return nil, errors.New("公众号加密消息接收方不匹配")
	}
	return xmlBody, nil
}

func (s *MemoryStore) HandleOfficialCallback(openID, event string, createTime int64) error {
	openID, event = strings.TrimSpace(openID), strings.ToLower(strings.TrimSpace(event))
	if openID == "" || (event != "subscribe" && event != "unsubscribe") {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		now := time.Now().Format("2006-01-02 15:04:05")
		if createTime > 0 {
			now = time.Unix(createTime, 0).Format("2006-01-02 15:04:05")
		}
		index := -1
		for i := range work.officialFollowers {
			if work.officialFollowers[i].OpenID == openID {
				index = i
				break
			}
		}
		if index < 0 {
			work.officialFollowers = append(work.officialFollowers, learning.OfficialFollower{OpenID: openID})
			index = len(work.officialFollowers) - 1
		}
		follower := work.officialFollowers[index]
		follower.SyncedAt = time.Now().Format("2006-01-02 15:04:05")
		if event == "subscribe" {
			follower.Subscribed = true
			follower.SubscribedAt = now
			follower.UnsubscribedAt = ""
		} else {
			follower.Subscribed = false
			follower.UnsubscribedAt = now
		}
		work.officialFollowers[index] = follower
		return nil
	})
}

type officialAudienceTarget struct {
	GuardianID   string
	GuardianName string
	OpenID       string
	StudentNames []string
}

func (s *MemoryStore) officialAudienceUnlocked(grades []string) (learning.OfficialAudiencePreview, []officialAudienceTarget, error) {
	gradeSet := map[string]bool{}
	for _, grade := range compactStrings(grades) {
		gradeSet[grade] = true
	}
	if len(gradeSet) == 0 {
		return learning.OfficialAudiencePreview{}, nil, errors.New("请至少选择一个年级")
	}
	studentByID := map[string]learning.Student{}
	for _, student := range s.students {
		if gradeSet[student.Grade] && student.AccountStatus == "正常" {
			studentByID[student.ID] = student
		}
	}
	guardianByID := map[string]learning.Guardian{}
	for _, guardian := range s.guardians {
		guardianByID[guardian.ID] = guardian
	}
	followerByUnion := map[string]learning.OfficialFollower{}
	for _, follower := range s.officialFollowers {
		if follower.Subscribed && follower.UnionID != "" {
			followerByUnion[follower.UnionID] = follower
		}
	}
	studentNamesByGuardian := map[string][]string{}
	for _, relation := range s.guardianStudents {
		student, ok := studentByID[relation.StudentID]
		if !ok || relation.Status != learning.GuardianStudentActive {
			continue
		}
		studentNamesByGuardian[relation.GuardianID] = append(studentNamesByGuardian[relation.GuardianID], student.Name)
	}
	preview := learning.OfficialAudiencePreview{StudentCount: len(studentByID), Grades: compactStrings(grades)}
	targets := []officialAudienceTarget{}
	openIDs := map[string]bool{}
	for guardianID, studentNames := range studentNamesByGuardian {
		guardian, ok := guardianByID[guardianID]
		if !ok || guardian.AccountStatus != "正常" {
			continue
		}
		preview.GuardianCount++
		follower, matched := followerByUnion[guardian.UnionID]
		if !matched {
			preview.UnmatchedCount++
			continue
		}
		if openIDs[follower.OpenID] {
			preview.DuplicateCount++
			continue
		}
		openIDs[follower.OpenID] = true
		targets = append(targets, officialAudienceTarget{GuardianID: guardian.ID, GuardianName: firstNonEmpty(guardian.Name, guardian.Nickname, "家长"), OpenID: follower.OpenID, StudentNames: uniqueStrings(studentNames)})
	}
	preview.ReachableCount = len(targets)
	preview.UnreachableCount = preview.GuardianCount - preview.ReachableCount
	if preview.UnmatchedCount > 0 {
		preview.UnreachableReasons = append(preview.UnreachableReasons, fmt.Sprintf("%d 位家长尚未关注公众号或身份未匹配", preview.UnmatchedCount))
	}
	return preview, targets, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range compactStrings(values) {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func (s *MemoryStore) PreviewOfficialAudience(req learning.OfficialAudiencePreviewRequest) (learning.OfficialAudiencePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, _, err := s.officialAudienceUnlocked(req.Grades)
	return preview, err
}

func (s *MemoryStore) OfficialCampaigns() []learning.OfficialCampaign {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := cloneOfficialCampaigns(s.officialCampaigns)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *MemoryStore) OfficialCampaign(id string) (learning.OfficialCampaignDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, campaign := range s.officialCampaigns {
		if campaign.ID != id {
			continue
		}
		recipients := []learning.OfficialCampaignRecipient{}
		for _, recipient := range s.officialCampaignRecipients {
			if recipient.CampaignID == id {
				recipients = append(recipients, recipient)
			}
		}
		return learning.OfficialCampaignDetail{Campaign: campaign, Recipients: recipients}, nil
	}
	return learning.OfficialCampaignDetail{}, errors.New("发送记录不存在")
}

func (s *MemoryStore) createOfficialCampaignUnlocked(operator string, req learning.OfficialCampaignCreateRequest) (learning.OfficialCampaign, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.OfficialCampaign, error) {
			return work.createOfficialCampaignUnlocked(operator, req)
		})
	}
	var template *learning.OfficialTemplate
	for i := range s.officialTemplates {
		if s.officialTemplates[i].ID == strings.TrimSpace(req.TemplateID) && s.officialTemplates[i].Status == "启用" {
			template = &s.officialTemplates[i]
			break
		}
	}
	if template == nil {
		return learning.OfficialCampaign{}, errors.New("请选择有效的公众号模板")
	}
	for _, field := range template.Fields {
		if req.Draft {
			break
		}
		value := strings.TrimSpace(req.Values[field.Key])
		if value == "" {
			return learning.OfficialCampaign{}, fmt.Errorf("请填写%s", field.Label)
		}
		if field.MaxLength > 0 && len([]rune(value)) > field.MaxLength {
			return learning.OfficialCampaign{}, fmt.Errorf("%s不能超过%d个字", field.Label, field.MaxLength)
		}
	}
	preview := learning.OfficialAudiencePreview{}
	targets := []officialAudienceTarget{}
	var err error
	if len(compactStrings(req.Grades)) > 0 {
		preview, targets, err = s.officialAudienceUnlocked(req.Grades)
	} else if !req.Draft {
		err = errors.New("请至少选择一个年级")
	}
	if err != nil {
		return learning.OfficialCampaign{}, err
	}
	if !req.Draft && preview.ReachableCount == 0 {
		return learning.OfficialCampaign{}, errors.New("当前所选年级没有可触达的公众号家长")
	}
	now := time.Now()
	id := "oa-campaign-" + now.Format("20060102150405.000000000")
	status := "草稿"
	if !req.Draft {
		status = "发送中"
	}
	campaign := learning.OfficialCampaign{ID: id, TemplateID: template.ID, TemplateTitle: template.Title, Grades: compactStrings(req.Grades), Values: cloneMap(req.Values), PagePath: strings.TrimSpace(req.PagePath), TargetCount: len(targets), Status: status, CreatedBy: operator, CreatedAt: now.Format("2006-01-02 15:04:05")}
	s.officialCampaigns = append([]learning.OfficialCampaign{campaign}, s.officialCampaigns...)
	if !req.Draft {
		for index, target := range targets {
			s.officialCampaignRecipients = append(s.officialCampaignRecipients, learning.OfficialCampaignRecipient{ID: fmt.Sprintf("%s-%d", id, index+1), CampaignID: id, GuardianID: target.GuardianID, GuardianName: target.GuardianName, OpenID: target.OpenID, StudentNames: strings.Join(target.StudentNames, "、"), Status: "待发送"})
		}
	}
	s.prependLog(operator, map[bool]string{true: "保存公众号消息草稿", false: "发送公众号模板消息"}[req.Draft], template.Title)
	return campaign, nil
}

func (s *MemoryStore) CreateOfficialCampaign(operator string, req learning.OfficialCampaignCreateRequest) (learning.OfficialCampaign, error) {
	s.mu.Lock()
	campaign, err := s.createOfficialCampaignUnlocked(operator, req)
	s.mu.Unlock()
	if err == nil && !req.Draft {
		go s.deliverOfficialCampaign(campaign.ID, false)
	}
	return campaign, err
}

func (s *MemoryStore) RetryOfficialCampaign(operator, id string) (learning.OfficialCampaign, error) {
	s.mu.Lock()
	var found *learning.OfficialCampaign
	for i := range s.officialCampaigns {
		if s.officialCampaigns[i].ID == id {
			s.officialCampaigns[i].Status = "发送中"
			copy := s.officialCampaigns[i]
			found = &copy
			break
		}
	}
	if found == nil {
		s.mu.Unlock()
		return learning.OfficialCampaign{}, errors.New("发送记录不存在")
	}
	s.prependLog(operator, "重试公众号模板消息", found.TemplateTitle)
	s.mu.Unlock()
	go s.deliverOfficialCampaign(id, true)
	return *found, nil
}

func (s *MemoryStore) deliverOfficialCampaign(id string, failedOnly bool) {
	s.mu.Lock()
	var campaign learning.OfficialCampaign
	found := false
	for _, item := range s.officialCampaigns {
		if item.ID == id {
			campaign, found = item, true
			break
		}
	}
	sender := s.officialTemplateSender
	recipients := append([]learning.OfficialCampaignRecipient(nil), s.officialCampaignRecipients...)
	s.mu.Unlock()
	if !found {
		return
	}
	results := map[string]learning.OfficialCampaignRecipient{}
	for _, recipient := range recipients {
		if recipient.CampaignID != id || (failedOnly && recipient.Status != "发送失败") || (!failedOnly && recipient.Status != "待发送") {
			continue
		}
		updated := recipient
		updated.RetryCount++
		if sender == nil {
			updated.Status, updated.FailureReason = "发送失败", "公众号发送配置不可用"
		} else if err := sender(campaign.TemplateID, recipient.OpenID, campaign.Values, campaign.PagePath); err != nil {
			updated.Status, updated.FailureReason = "发送失败", err.Error()
		} else {
			updated.Status, updated.FailureReason, updated.SentAt = "发送成功", "", time.Now().Format("2006-01-02 15:04:05")
		}
		results[recipient.ID] = updated
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = persistentMutation(s, func(work *MemoryStore) (struct{}, error) {
		for i := range work.officialCampaignRecipients {
			if updated, ok := results[work.officialCampaignRecipients[i].ID]; ok {
				work.officialCampaignRecipients[i] = updated
			}
		}
		for i := range work.officialCampaigns {
			if work.officialCampaigns[i].ID != id {
				continue
			}
			work.officialCampaigns[i].SuccessCount = 0
			work.officialCampaigns[i].FailureCount = 0
			for _, recipient := range work.officialCampaignRecipients {
				if recipient.CampaignID != id {
					continue
				}
				if recipient.Status == "发送成功" {
					work.officialCampaigns[i].SuccessCount++
				}
				if recipient.Status == "发送失败" {
					work.officialCampaigns[i].FailureCount++
				}
			}
			if work.officialCampaigns[i].FailureCount > 0 {
				work.officialCampaigns[i].Status = "部分失败"
			} else {
				work.officialCampaigns[i].Status = "发送完成"
			}
			work.officialCampaigns[i].SentAt = time.Now().Format("2006-01-02 15:04:05")
		}
		return struct{}{}, nil
	})
}
