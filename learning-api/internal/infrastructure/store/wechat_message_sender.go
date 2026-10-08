package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"starline/learning-api/internal/domain/learning"
)

var wechatTokenCache = struct {
	sync.Mutex
	items map[string]cachedWechatToken
}{items: map[string]cachedWechatToken{}}

type cachedWechatToken struct {
	token   string
	expires time.Time
}

func wechatAccessToken(client *http.Client, appID, secret string) (string, error) {
	key := businessNoticeHash(appID, secret)
	wechatTokenCache.Lock()
	defer wechatTokenCache.Unlock()
	if item, ok := wechatTokenCache.items[key]; ok && time.Now().Before(item.expires) {
		return item.token, nil
	}
	token, lifetime, err := fetchWechatAccessToken(client, appID, secret)
	if err != nil {
		return "", err
	}
	wechatTokenCache.items[key] = cachedWechatToken{token: token, expires: time.Now().Add(lifetime)}
	return token, nil
}
func invalidateWechatToken(appID, secret string) {
	wechatTokenCache.Lock()
	delete(wechatTokenCache.items, businessNoticeHash(appID, secret))
	wechatTokenCache.Unlock()
}

func validateOfficialMessageValues(values map[string]string) error {
	if len(values) == 0 {
		return &officialSendError{reason: "模板字段为空", configuration: true}
	}
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			return &officialSendError{reason: "模板字段 " + key + " 不能为空", configuration: true}
		}
		for _, char := range value {
			if unicode.IsControl(char) {
				return &officialSendError{reason: "模板字段不能包含换行或控制字符", configuration: true}
			}
		}
		if (strings.HasPrefix(key, "thing") || strings.HasPrefix(key, "const")) && len([]rune(value)) > 20 {
			return &officialSendError{reason: key + " 最多20个字符", configuration: true}
		}
		if strings.HasPrefix(key, "time") {
			parts := strings.Split(value, "~")
			if len(parts) > 2 {
				return &officialSendError{reason: "时间段格式不正确", configuration: true}
			}
			for _, part := range parts {
				valid := false
				for _, format := range []string{"2006-01-02 15:04", "2006-01-02 15:04:05", "2006年1月2日 15:04", "15:04", "15:04:05"} {
					if _, err := time.Parse(format, strings.TrimSpace(part)); err == nil {
						valid = true
						break
					}
				}
				if !valid {
					return &officialSendError{reason: "时间字段格式不正确", configuration: true}
				}
			}
		}
	}
	return nil
}

func newOfficialMessageSender(client *http.Client, appID, secret, miniAppID string, miniSecrets ...string) func(learning.OfficialMessageRequest) (string, error) {
	return func(request learning.OfficialMessageRequest) (string, error) {
		if err := validateOfficialMessageValues(request.Values); err != nil {
			return "", err
		}
		if request.OpenID == "" || request.TemplateID == "" {
			return "", &officialSendError{reason: "接收人或模板未配置", configuration: true}
		}
		data := map[string]any{}
		for key, value := range request.Values {
			data[key] = map[string]string{"value": value}
		}
		body := map[string]any{"touser": request.OpenID, "template_id": request.TemplateID, "data": data}
		if request.URL != "" {
			target, err := url.Parse(request.URL)
			if err != nil || target.Scheme != "https" || target.Hostname() == "" || target.User != nil || target.Fragment != "" || strings.ContainsAny(request.URL, "\r\n") || request.PagePath != "" {
				return "", &officialSendError{reason: "网页跳转配置不正确", configuration: true}
			}
			body["url"] = target.String()
		}
		if request.ClientMessageID != "" {
			body["client_msg_id"] = request.ClientMessageID
		}
		if request.PagePath != "" {
			path := strings.TrimPrefix(request.PagePath, "/")
			if miniAppID == "" || !strings.HasPrefix(path, "pages/") || strings.Contains(path, "..") {
				return "", &officialSendError{reason: "小程序跳转配置不正确", configuration: true}
			}
			body["miniprogram"] = map[string]string{"appid": miniAppID, "pagepath": path}
		}
		log.Printf("event=official_template_send official_app_id=%q mini_app_id=%q has_mini_program=%t template_id=%q", appID, miniAppID, request.PagePath != "", request.TemplateID)
		encoded, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		refreshed, linkFallback := false, false
		for attempt := 0; attempt < 4; attempt++ {
			token, err := wechatAccessToken(client, appID, secret)
			if err != nil {
				return "", &officialSendError{reason: "公众号授权服务暂不可用", temporary: true}
			}
			endpoint := "https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=" + url.QueryEscape(token)
			response, err := client.Post(endpoint, "application/json", bytes.NewReader(encoded))
			if err != nil {
				return "", &officialSendError{reason: "微信请求超时或连接中断，接收结果待确认", uncertain: true}
			}
			var payload struct {
				ErrCode   int         `json:"errcode"`
				ErrMsg    string      `json:"errmsg"`
				MessageID json.Number `json:"msgid"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&payload)
			response.Body.Close()
			if decodeErr != nil {
				return "", &officialSendError{reason: "微信响应无法解析，接收结果待确认", uncertain: true}
			}
			if (payload.ErrCode == 40001 || payload.ErrCode == 40014 || payload.ErrCode == 42001) && !refreshed {
				refreshed = true
				invalidateWechatToken(appID, secret)
				continue
			}
			// A definite rejection has no delivered message. Preserve the destination
			// and deduplication ID when native mini-program navigation is rejected.
			if payload.ErrCode == 40013 && request.PagePath != "" && !linkFallback && len(miniSecrets) > 0 && miniSecrets[0] != "" {
				link, err := wechatMiniProgramURLLink(client, miniAppID, miniSecrets[0], request.PagePath)
				if err != nil {
					return "", err
				}
				delete(body, "miniprogram")
				body["url"] = link
				encoded, err = json.Marshal(body)
				if err != nil {
					return "", err
				}
				linkFallback = true
				log.Printf("event=official_template_jump_fallback mini_app_id=%q reason=40013 route=url_link", miniAppID)
				continue
			}
			if payload.ErrCode != 0 {
				temporary := payload.ErrCode == -1 || payload.ErrCode == 45009 || payload.ErrCode == 45011
				config := payload.ErrCode == 40013 || payload.ErrCode == 40037 || payload.ErrCode == 47003 || payload.ErrCode == 40165
				return "", &officialSendError{reason: fmt.Sprintf("微信发送失败（%d %s）", payload.ErrCode, payload.ErrMsg), temporary: temporary, configuration: config}
			}
			if payload.MessageID == "" {
				return "", &officialSendError{reason: "微信未返回消息编号，接收结果待确认", uncertain: true}
			}
			return payload.MessageID.String(), nil
		}
		return "", &officialSendError{reason: "公众号授权失败", configuration: true}
	}
}

// The link resolves to the same released page and query as the native jump.
func wechatMiniProgramURLLink(client *http.Client, appID, secret, pagePath string) (string, error) {
	token, err := wechatAccessToken(client, appID, secret)
	if err != nil {
		return "", &officialSendError{reason: "小程序链接授权服务暂不可用", temporary: true}
	}
	target, err := url.Parse(strings.TrimPrefix(pagePath, "/"))
	if err != nil || !strings.HasPrefix(target.Path, "pages/") || target.IsAbs() || target.Host != "" || target.Fragment != "" {
		return "", &officialSendError{reason: "小程序链接目标不正确", configuration: true}
	}
	body, _ := json.Marshal(map[string]any{"path": target.Path, "query": target.RawQuery, "env_version": "release", "expire_type": 1, "expire_interval": 30})
	response, err := client.Post("https://api.weixin.qq.com/wxa/generate_urllink?access_token="+url.QueryEscape(token), "application/json", bytes.NewReader(body))
	if err != nil {
		return "", &officialSendError{reason: "小程序链接生成服务暂不可用", temporary: true}
	}
	defer response.Body.Close()
	var result struct {
		ErrCode int    `json:"errcode"`
		URLLink string `json:"url_link"`
	}
	if json.NewDecoder(response.Body).Decode(&result) != nil {
		return "", &officialSendError{reason: "小程序链接生成响应异常", temporary: true}
	}
	link, parseErr := url.Parse(result.URLLink)
	if result.ErrCode != 0 || parseErr != nil || link.Scheme != "https" || (link.Host != "wxmpurl.cn" && link.Host != "wxaurl.cn") || link.User != nil || link.Fragment != "" {
		return "", &officialSendError{reason: fmt.Sprintf("小程序链接生成失败（%d），原生跳转被微信拒绝（40013）", result.ErrCode), configuration: true}
	}
	return link.String(), nil
}
