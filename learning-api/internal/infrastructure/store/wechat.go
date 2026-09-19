package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// resolveOpenID 把小程序 wx.login() 的临时 code 换成稳定的 openId。
// 配置了微信 AppID/Secret 时走真实 jscode2session；否则使用演示映射，
// 保证本地无凭据时登录仍可用（code 即 openId 后缀，如 "student" -> "demo-student"）。
func (s *MemoryStore) resolveOpenID(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", errors.New("wechat code is required")
	}
	if s.wechatResolver != nil {
		return s.wechatResolver(code)
	}
	return "demo-" + code, nil
}

func (s *MemoryStore) resolvePhoneNumber(phoneCode string) (string, error) {
	phoneCode = strings.TrimSpace(phoneCode)
	if phoneCode == "" {
		return "", errors.New("手机号授权已失效，请重新授权")
	}
	if s.phoneResolver == nil {
		return "", errors.New("本地演示模式下请使用演示账号登录")
	}
	return s.phoneResolver(phoneCode)
}

// UseWechatAPI 启用真实微信登录：用 AppID/Secret 调用 jscode2session 换取 openId，并支持手机号授权绑定。
func (s *MemoryStore) useWechatAPIUnlocked(appID, secret string) {
	appID = strings.TrimSpace(appID)
	secret = strings.TrimSpace(secret)
	if appID == "" || secret == "" {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	s.wechatSessionResolver = func(code string) (string, string, error) {
		return wechatCode2Session(client, appID, secret, code)
	}
	s.wechatResolver = func(code string) (string, error) {
		openID, _, err := wechatCode2Session(client, appID, secret, code)
		return openID, err
	}
	s.phoneResolver = func(phoneCode string) (string, error) {
		return wechatPhoneNumber(client, appID, secret, phoneCode)
	}
}

func (s *MemoryStore) useOfficialAccountAPIUnlocked(appID, secret, templateID string) {
	appID = strings.TrimSpace(appID)
	secret = strings.TrimSpace(secret)
	templateID = strings.TrimSpace(templateID)
	if appID == "" || secret == "" || templateID == "" {
		return
	}
	s.officialAccountReady = true
	client := &http.Client{Timeout: 5 * time.Second}
	s.officialNoticeSender = func(notice learning.Notice) error {
		return wechatOfficialTemplateMessage(client, appID, secret, templateID, notice)
	}
}

func (s *MemoryStore) useMiniProgramSubscribeTemplatesUnlocked(templateIDs []string) {
	ids := compactStrings(templateIDs)
	if len(ids) == 0 {
		return
	}
	s.miniProgramSubscribeTemplateIDs = ids
	if s.settings == nil {
		s.settings = map[string]string{}
	}
	s.settings["miniProgramSubscribeStatus"] = "已完成"
}

func wechatCode2Session(client *http.Client, appID, secret, code string) (string, string, error) {
	endpoint := "https://api.weixin.qq.com/sns/jscode2session?" + url.Values{
		"appid":      {appID},
		"secret":     {secret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}.Encode()
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", "", errors.New("微信登录服务暂不可用，请稍后再试")
	}
	defer resp.Body.Close()
	var payload struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", errors.New("微信登录返回异常，请稍后再试")
	}
	if payload.ErrCode != 0 || payload.OpenID == "" {
		return "", "", fmt.Errorf("微信登录失败，请重新授权（%d %s）", payload.ErrCode, payload.ErrMsg)
	}
	return payload.OpenID, payload.UnionID, nil
}

func (s *MemoryStore) useOfficialAccountMessagingUnlocked(appID, secret, miniProgramAppID string) {
	appID, secret, miniProgramAppID = strings.TrimSpace(appID), strings.TrimSpace(secret), strings.TrimSpace(miniProgramAppID)
	if appID == "" || secret == "" {
		return
	}
	client := &http.Client{Timeout: 8 * time.Second}
	s.officialAccountReady = true
	s.officialTemplateSyncer = func() ([]learning.OfficialTemplate, error) {
		token, err := wechatAccessToken(client, appID, secret)
		if err != nil {
			return nil, err
		}
		endpoint := "https://api.weixin.qq.com/cgi-bin/template/get_all_private_template?access_token=" + url.QueryEscape(token)
		resp, err := client.Get(endpoint)
		if err != nil {
			return nil, errors.New("公众号模板同步服务暂不可用")
		}
		defer resp.Body.Close()
		var payload struct {
			TemplateList []struct {
				TemplateID string `json:"template_id"`
				Title      string `json:"title"`
				Content    string `json:"content"`
				Example    string `json:"example"`
			} `json:"template_list"`
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, errors.New("公众号模板同步返回异常")
		}
		if payload.ErrCode != 0 {
			return nil, fmt.Errorf("公众号模板同步失败（%d %s）", payload.ErrCode, payload.ErrMsg)
		}
		out := make([]learning.OfficialTemplate, 0, len(payload.TemplateList))
		for _, item := range payload.TemplateList {
			out = append(out, learning.OfficialTemplate{ID: item.TemplateID, Title: item.Title, Content: item.Content, Example: item.Example})
		}
		return out, nil
	}
	s.officialTemplateSender = func(templateID, openID string, values map[string]string, pagePath string) error {
		token, err := wechatAccessToken(client, appID, secret)
		if err != nil {
			return err
		}
		data := map[string]any{}
		for key, value := range values {
			data[key] = map[string]string{"value": value}
		}
		body := map[string]any{"touser": openID, "template_id": templateID, "data": data}
		if miniProgramAppID != "" && strings.TrimSpace(pagePath) != "" {
			body["miniprogram"] = map[string]string{"appid": miniProgramAppID, "pagepath": strings.TrimSpace(pagePath)}
		}
		encoded, _ := json.Marshal(body)
		endpoint := "https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=" + url.QueryEscape(token)
		resp, err := client.Post(endpoint, "application/json", strings.NewReader(string(encoded)))
		if err != nil {
			return errors.New("公众号模板消息服务暂不可用")
		}
		defer resp.Body.Close()
		var payload struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return errors.New("公众号模板消息返回异常")
		}
		if payload.ErrCode != 0 {
			return fmt.Errorf("公众号模板消息发送失败（%d %s）", payload.ErrCode, payload.ErrMsg)
		}
		return nil
	}
	s.officialFollowerSyncer = func() ([]learning.OfficialFollower, error) {
		token, err := wechatAccessToken(client, appID, secret)
		if err != nil {
			return nil, err
		}
		openIDs := []string{}
		nextOpenID := ""
		for {
			endpoint := "https://api.weixin.qq.com/cgi-bin/user/get?" + url.Values{"access_token": {token}, "next_openid": {nextOpenID}}.Encode()
			resp, err := client.Get(endpoint)
			if err != nil {
				return nil, errors.New("公众号关注者同步服务暂不可用")
			}
			var payload struct {
				Count int `json:"count"`
				Data  struct {
					OpenIDs []string `json:"openid"`
				} `json:"data"`
				NextOpenID string `json:"next_openid"`
				ErrCode    int    `json:"errcode"`
				ErrMsg     string `json:"errmsg"`
			}
			err = json.NewDecoder(resp.Body).Decode(&payload)
			resp.Body.Close()
			if err != nil {
				return nil, errors.New("公众号关注者列表返回异常")
			}
			if payload.ErrCode != 0 {
				return nil, fmt.Errorf("公众号关注者同步失败（%d %s）", payload.ErrCode, payload.ErrMsg)
			}
			openIDs = append(openIDs, payload.Data.OpenIDs...)
			if payload.Count == 0 || payload.NextOpenID == "" || payload.NextOpenID == nextOpenID {
				break
			}
			nextOpenID = payload.NextOpenID
		}
		followers := make([]learning.OfficialFollower, 0, len(openIDs))
		for start := 0; start < len(openIDs); start += 100 {
			end := start + 100
			if end > len(openIDs) {
				end = len(openIDs)
			}
			users := make([]map[string]string, 0, end-start)
			for _, openID := range openIDs[start:end] {
				users = append(users, map[string]string{"openid": openID, "lang": "zh_CN"})
			}
			encoded, _ := json.Marshal(map[string]any{"user_list": users})
			endpoint := "https://api.weixin.qq.com/cgi-bin/user/info/batchget?access_token=" + url.QueryEscape(token)
			resp, err := client.Post(endpoint, "application/json", strings.NewReader(string(encoded)))
			if err != nil {
				return nil, errors.New("公众号关注者资料同步服务暂不可用")
			}
			var payload struct {
				UserInfoList []struct {
					Subscribe     int    `json:"subscribe"`
					OpenID        string `json:"openid"`
					UnionID       string `json:"unionid"`
					SubscribeTime int64  `json:"subscribe_time"`
				} `json:"user_info_list"`
				ErrCode int    `json:"errcode"`
				ErrMsg  string `json:"errmsg"`
			}
			err = json.NewDecoder(resp.Body).Decode(&payload)
			resp.Body.Close()
			if err != nil {
				return nil, errors.New("公众号关注者资料返回异常")
			}
			if payload.ErrCode != 0 {
				return nil, fmt.Errorf("公众号关注者资料同步失败（%d %s）", payload.ErrCode, payload.ErrMsg)
			}
			for _, user := range payload.UserInfoList {
				subscribedAt := ""
				if user.SubscribeTime > 0 {
					subscribedAt = time.Unix(user.SubscribeTime, 0).Format("2006-01-02 15:04:05")
				}
				followers = append(followers, learning.OfficialFollower{OpenID: user.OpenID, UnionID: user.UnionID, Subscribed: user.Subscribe == 1, SubscribedAt: subscribedAt, SyncedAt: time.Now().Format("2006-01-02 15:04:05")})
			}
		}
		return followers, nil
	}
}

func wechatPhoneNumber(client *http.Client, appID, secret, phoneCode string) (string, error) {
	token, err := wechatAccessToken(client, appID, secret)
	if err != nil {
		return "", err
	}
	endpoint := "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=" + url.QueryEscape(token)
	requestBody, err := json.Marshal(struct {
		Code string `json:"code"`
	}{Code: phoneCode})
	if err != nil {
		return "", errors.New("手机号授权请求生成失败")
	}
	body := strings.NewReader(string(requestBody))
	resp, err := client.Post(endpoint, "application/json", body)
	if err != nil {
		return "", errors.New("微信手机号授权服务暂不可用，请稍后再试")
	}
	defer resp.Body.Close()
	var payload struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber string `json:"phoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", errors.New("微信手机号授权返回异常，请稍后再试")
	}
	if payload.ErrCode != 0 || payload.PhoneInfo.PhoneNumber == "" {
		return "", fmt.Errorf("手机号授权失败，请重新授权（%d %s）", payload.ErrCode, payload.ErrMsg)
	}
	return payload.PhoneInfo.PhoneNumber, nil
}

func wechatAccessToken(client *http.Client, appID, secret string) (string, error) {
	endpoint := "https://api.weixin.qq.com/cgi-bin/token?" + url.Values{
		"grant_type": {"client_credential"},
		"appid":      {appID},
		"secret":     {secret},
	}.Encode()
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", errors.New("微信授权服务暂不可用，请稍后再试")
	}
	defer resp.Body.Close()
	var payload struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", errors.New("微信授权返回异常，请稍后再试")
	}
	if payload.ErrCode != 0 || payload.AccessToken == "" {
		return "", fmt.Errorf("微信授权失败，请稍后再试（%d %s）", payload.ErrCode, payload.ErrMsg)
	}
	return payload.AccessToken, nil
}

func wechatOfficialTemplateMessage(client *http.Client, appID, secret, templateID string, notice learning.Notice) error {
	token, err := wechatAccessToken(client, appID, secret)
	if err != nil {
		return err
	}
	if strings.TrimSpace(notice.RecipientOpenID) == "" {
		return errors.New("缺少公众号接收人 openid")
	}
	endpoint := "https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=" + url.QueryEscape(token)
	requestBody, err := json.Marshal(struct {
		ToUser     string         `json:"touser"`
		TemplateID string         `json:"template_id"`
		Data       map[string]any `json:"data"`
	}{
		ToUser:     notice.RecipientOpenID,
		TemplateID: templateID,
		Data: map[string]any{
			"first":    map[string]string{"value": notice.Title},
			"keyword1": map[string]string{"value": notice.Type},
			"keyword2": map[string]string{"value": notice.Target},
			"remark":   map[string]string{"value": notice.Summary},
		},
	})
	if err != nil {
		return errors.New("公众号模板消息请求生成失败")
	}
	resp, err := client.Post(endpoint, "application/json", strings.NewReader(string(requestBody)))
	if err != nil {
		return errors.New("公众号模板消息服务暂不可用，请稍后重试")
	}
	defer resp.Body.Close()
	var payload struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return errors.New("公众号模板消息返回异常，请稍后重试")
	}
	if payload.ErrCode != 0 {
		return fmt.Errorf("公众号模板消息发送失败（%d %s）", payload.ErrCode, payload.ErrMsg)
	}
	return nil
}
