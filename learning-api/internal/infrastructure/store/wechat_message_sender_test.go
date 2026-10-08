package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
	"time"
)

type noticeTransport func(*http.Request) (*http.Response, error)

func (f noticeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func noticeResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func TestBusinessWechatSenderCachesTokenRefreshesAndKeepsDedupID(t *testing.T) {
	app := "sender-test-" + t.Name()
	invalidateWechatToken(app, "fixture")
	tokens, sends := 0, 0
	client := &http.Client{Transport: noticeTransport(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/token") {
			tokens++
			return noticeResponse(`{"access_token":"fixture-token","expires_in":7200}`), nil
		}
		sends++
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["client_msg_id"] != "same-id" || body["template_id"] != "real-template-id" {
			t.Fatalf("wrong payload %#v", body)
		}
		mini, ok := body["miniprogram"].(map[string]any)
		if !ok || mini["appid"] != "mini-fixture" || mini["pagepath"] != "pages/notice-detail/index?id=fixture" {
			t.Fatal("mini-program jump payload used wrong config or path")
		}
		if sends == 1 {
			return noticeResponse(`{"errcode":40014}`), nil
		}
		return noticeResponse(`{"errcode":0,"msgid":123456789}`), nil
	})}
	sender := newOfficialMessageSender(client, app, "fixture", "mini-fixture")
	req := learning.OfficialMessageRequest{OpenID: "recipient", TemplateID: "real-template-id", ClientMessageID: "same-id", Values: map[string]string{"thing1": "课程"}, PagePath: "pages/notice-detail/index?id=fixture"}
	for i := 0; i < 2; i++ {
		id, err := sender(req)
		if err != nil || id != "123456789" {
			t.Fatalf("send result %s %v", id, err)
		}
	}
	if tokens != 2 || sends != 3 {
		t.Fatalf("token refresh/cache incorrect: tokens=%d sends=%d", tokens, sends)
	}
}
func TestBusinessWechatSenderClassifiesMissingReceiptAndConfiguration(t *testing.T) {
	for _, tc := range []struct {
		body              string
		uncertain, config bool
	}{{`{"errcode":0}`, true, false}, {`{"errcode":47003,"errmsg":"invalid fields"}`, false, true}, {`{"errcode":40013,"errmsg":"invalid appid"}`, false, true}} {
		app := "classification-" + tc.body
		invalidateWechatToken(app, "fixture")
		client := &http.Client{Transport: noticeTransport(func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/token") {
				return noticeResponse(`{"access_token":"fixture","expires_in":7200}`), nil
			}
			return noticeResponse(tc.body), nil
		})}
		_, err := newOfficialMessageSender(client, app, "fixture", "")(learning.OfficialMessageRequest{OpenID: "recipient", TemplateID: "template", Values: map[string]string{"thing1": "课程"}})
		var typed *officialSendError
		if !errors.As(err, &typed) || typed.uncertain != tc.uncertain || typed.configuration != tc.config {
			t.Fatalf("wrong classification: %v", err)
		}
	}
}
func TestBusinessManualCampaignEarlyReceiptIsPersistentDeliveryState(t *testing.T) {
	s := businessFixture(t)
	s.students[0].Grade = "G5"
	s.students[1].Grade = "G5"
	s.officialCampaigns = []learning.OfficialCampaign{{ID: "campaign", TemplateID: "template", Grades: []string{"G5"}, Values: map[string]string{"thing1": "课程"}, TargetCount: 1}}
	s.officialCampaignRecipients = []learning.OfficialCampaignRecipient{{ID: "recipient", CampaignID: "campaign", GuardianID: "g1", OpenID: "o1", Status: "待发送"}}
	s.officialMessageSender = func(req learning.OfficialMessageRequest) (string, error) {
		if err := s.HandleOfficialDeliveryReceipt("o1", "msg-manual", "success"); err != nil {
			t.Fatal(err)
		}
		return "msg-manual", nil
	}
	s.deliverOfficialCampaign("campaign", false)
	item := s.officialCampaignRecipients[0]
	if item.Status != "已送达" || item.MessageID != "msg-manual" || item.DeliveredAt == "" || item.AcceptedAt == "" {
		t.Fatalf("manual delivery lost receipt: %#v", item)
	}
	if err := s.HandleOfficialDeliveryReceipt("o1", "msg-manual", "failed:user block"); err != nil {
		t.Fatal(err)
	}
	if s.officialCampaignRecipients[0].Status != "已送达" {
		t.Fatal("late failure overwrote delivery success")
	}
}

func TestBusinessWechatSenderWebTargetAndInvalidTargets(t *testing.T) {
	for _, tc := range []struct {
		target, path string
		valid        bool
	}{
		{"https://school.example/teaching-plans?plan=plan-20261003120000.123456000", "", true},
		{"http://school.example/teaching-plans", "", false},
		{"javascript:alert(1)", "", false},
		{"//school.example/teaching-plans", "", false},
		{"https://user:secret@school.example/teaching-plans", "", false},
		{"https://school.example/teaching-plans#secret", "", false},
		{"https://school.example/teaching-plans\r\n", "", false},
		{"https://school.example/teaching-plans", "pages/notice-detail/index?id=fixture", false},
	} {
		t.Run(tc.target+tc.path, func(t *testing.T) {
			app := "web-target-" + businessNoticeHash(t.Name())
			invalidateWechatToken(app, "fixture")
			calls := 0
			client := &http.Client{Transport: noticeTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				if strings.HasSuffix(req.URL.Path, "/token") {
					return noticeResponse(`{"access_token":"fixture","expires_in":7200}`), nil
				}
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["url"] != tc.target || body["miniprogram"] != nil || body["client_msg_id"] != "web-target-id" {
					t.Fatalf("wrong webpage payload: %+v", body)
				}
				return noticeResponse(`{"errcode":0,"msgid":987}`), nil
			})}
			id, err := newOfficialMessageSender(client, app, "fixture", "")(learning.OfficialMessageRequest{URL: tc.target, PagePath: tc.path, OpenID: "fixture-open", TemplateID: "fixture-template", Values: map[string]string{"thing1": "内部教案"}, ClientMessageID: "web-target-id"})
			if tc.valid {
				if err != nil || id != "987" || calls != 2 {
					t.Fatalf("valid URL rejected: %s %v %d", id, err, calls)
				}
			} else {
				var typed *officialSendError
				if !errors.As(err, &typed) || !typed.configuration || calls != 0 {
					t.Fatalf("invalid target reached transport: %v calls=%d", err, calls)
				}
			}
		})
	}
}

func TestBusinessWorkerPreservesWebTargetAndDedupIdentity(t *testing.T) {
	s := businessFixture(t)
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("web-worker-lesson", "web-worker-series")}
	})
	target := "https://school.example/teaching-plans?plan=plan-fixture"
	for i := range s.businessNoticeTasks {
		s.businessNoticeTasks[i].URL = target
		s.businessNoticeTasks[i].PagePath = ""
	}
	sent := map[string]bool{}
	s.officialMessageSender = func(req learning.OfficialMessageRequest) (string, error) {
		if req.URL != target || req.PagePath != "" || req.ClientMessageID == "" || sent[req.ClientMessageID] {
			t.Fatalf("worker changed target or replayed: %+v", req)
		}
		sent[req.ClientMessageID] = true
		return req.ClientMessageID, nil
	}
	if err := s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.ProcessBusinessNotices(time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 3 {
		t.Fatalf("wrong unique recipient tasks: %d", len(sent))
	}
}

func TestBusinessWechatSenderMiniLinkFallbackPreservesTargetAndDedup(t *testing.T) {
	for _, reject := range []int{40013, 47003, 0} {
		t.Run(fmt.Sprint(reject), func(t *testing.T) {
			app := "fallback-" + t.Name()
			mini := "mini-" + t.Name()
			invalidateWechatToken(app, "official-secret")
			invalidateWechatToken(mini, "mini-secret")
			sends, links := 0, 0
			client := &http.Client{Transport: noticeTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/cgi-bin/token" {
					return noticeResponse(`{"access_token":"fixture","expires_in":7200}`), nil
				}
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if req.URL.Path == "/wxa/generate_urllink" {
					links++
					if body["path"] != "pages/notice-detail/index" || body["query"] != "id=notice-1&studentId=student-1" || body["env_version"] != "release" {
						t.Fatal("link lost its authorized destination")
					}
					return noticeResponse(`{"errcode":0,"url_link":"https://wxmpurl.cn/fixture"}`), nil
				}
				sends++
				if body["touser"] != "recipient" || body["client_msg_id"] != "same-dedup" || body["template_id"] != "template" {
					t.Fatal("fallback changed message identity")
				}
				if sends == 1 {
					if body["miniprogram"] == nil {
						t.Fatal("native jump was not attempted first")
					}
					if reject == 0 {
						return nil, errors.New("connection lost")
					}
					return noticeResponse(fmt.Sprintf(`{"errcode":%d}`, reject)), nil
				}
				if body["miniprogram"] != nil || body["url"] != "https://wxmpurl.cn/fixture" {
					t.Fatal("fallback did not use generated link")
				}
				return noticeResponse(`{"errcode":0,"msgid":123}`), nil
			})}
			id, err := newOfficialMessageSender(client, app, "official-secret", mini, "mini-secret")(learning.OfficialMessageRequest{OpenID: "recipient", TemplateID: "template", Values: map[string]string{"thing1": "课程"}, PagePath: "pages/notice-detail/index?id=notice-1&studentId=student-1", ClientMessageID: "same-dedup"})
			if reject == 40013 {
				if err != nil || id != "123" || sends != 2 || links != 1 {
					t.Fatalf("fallback failed: %v %s %d %d", err, id, sends, links)
				}
			} else if err == nil || sends != 1 || links != 0 {
				t.Fatal("retried a field rejection or uncertain send")
			}
		})
	}
}
