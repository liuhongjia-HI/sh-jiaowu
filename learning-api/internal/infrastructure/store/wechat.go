package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
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
func (s *MemoryStore) UseWechatAPI(appID, secret string) {
	appID = strings.TrimSpace(appID)
	secret = strings.TrimSpace(secret)
	if appID == "" || secret == "" {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	s.wechatResolver = func(code string) (string, error) {
		return wechatCode2Session(client, appID, secret, code)
	}
	s.phoneResolver = func(phoneCode string) (string, error) {
		return wechatPhoneNumber(client, appID, secret, phoneCode)
	}
}

func wechatCode2Session(client *http.Client, appID, secret, code string) (string, error) {
	endpoint := "https://api.weixin.qq.com/sns/jscode2session?" + url.Values{
		"appid":      {appID},
		"secret":     {secret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}.Encode()
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", errors.New("微信登录服务暂不可用，请稍后再试")
	}
	defer resp.Body.Close()
	var payload struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", errors.New("微信登录返回异常，请稍后再试")
	}
	if payload.ErrCode != 0 || payload.OpenID == "" {
		return "", fmt.Errorf("微信登录失败，请重新授权（%d %s）", payload.ErrCode, payload.ErrMsg)
	}
	return payload.OpenID, nil
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
