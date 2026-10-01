package store

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
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
	}{{`{"errcode":0}`, true, false}, {`{"errcode":47003,"errmsg":"invalid fields"}`, false, true}} {
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
