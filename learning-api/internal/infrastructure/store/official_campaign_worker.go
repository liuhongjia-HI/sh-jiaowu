package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"starline/learning-api/internal/domain/learning"
)

type campaignRequestMetadata struct {
	Mode        string   `json:"mode"`
	GuardianIDs []string `json:"guardianIds"`
	RequestID   string   `json:"requestId"`
	Digest      string   `json:"digest"`
}

func campaignRequestJSON(c learning.OfficialCampaign) string {
	return mustJSON(campaignRequestMetadata{c.RecipientMode, c.GuardianIDs, c.RequestID, c.RequestDigest})
}
func loadCampaignRequestJSON(c *learning.OfficialCampaign, raw string) error {
	var stored campaignRequestMetadata
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return err
	}
	c.RecipientMode, c.GuardianIDs, c.RequestID, c.RequestDigest = stored.Mode, stored.GuardianIDs, stored.RequestID, stored.Digest
	return nil
}

// Serialize creates, claims and manual retries across processes. Callbacks use
// the normal store mutex and can arrive while an outbound request is in flight.
func (s *MemoryStore) withOfficialCampaignLock(work func() error) error {
	s.officialCampaignMu.Lock()
	defer s.officialCampaignMu.Unlock()
	if s.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		conn, err := s.db.Conn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		var acquired int
		if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT(DATABASE(), ':official-campaigns'), 10)").Scan(&acquired); err != nil {
			return err
		}
		if acquired != 1 {
			return errors.New("发送任务正在处理，请稍后查看记录")
		}
		defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(CONCAT(DATABASE(), ':official-campaigns'))")
		s.mu.Lock()
		err = s.loadOfficialMessagingFromDB()
		s.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return work()
}

func (s *MemoryStore) CreateOfficialCampaign(operator string, req learning.OfficialCampaignCreateRequest) (learning.OfficialCampaign, error) {
	var campaign learning.OfficialCampaign
	err := s.withOfficialCampaignLock(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		var err error
		campaign, err = s.createOfficialCampaignUnlocked(operator, req)
		return err
	})
	if err == nil && !req.Draft {
		go s.deliverOfficialCampaign(campaign.ID, false)
	}
	return campaign, err
}

func (s *MemoryStore) RetryOfficialCampaign(operator, id string) (learning.OfficialCampaign, error) {
	var campaign learning.OfficialCampaign
	err := s.withOfficialCampaignLock(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return persistentMutationError(s, func(work *MemoryStore) error {
			found := -1
			for i, item := range work.officialCampaigns {
				if item.ID == id {
					found = i
					campaign = item
					break
				}
			}
			if found < 0 {
				return errors.New("发送记录不存在")
			}
			count := 0
			for i, item := range work.officialCampaignRecipients {
				if item.CampaignID == id && item.Status == "发送失败" && item.Retryable && item.RetryCount < 4 {
					work.officialCampaignRecipients[i].Status = "待发送"
					count++
				}
			}
			if count == 0 {
				return errors.New("没有可安全重试的失败通知")
			}
			work.officialCampaigns[found].Status = "发送中"
			campaign = work.officialCampaigns[found]
			work.prependLog(operator, "重试公众号模板消息", campaign.TemplateTitle)
			return nil
		})
	})
	if err == nil {
		go s.deliverOfficialCampaign(id, false)
	}
	return campaign, err
}

func (s *MemoryStore) deliverOfficialCampaign(id string, _ bool) {
	_ = s.processOfficialCampaign(id, time.Now())
}

func (s *MemoryStore) processOfficialCampaign(id string, now time.Time) error {
	return s.withOfficialCampaignLock(func() error {
		s.mu.Lock()
		var campaign learning.OfficialCampaign
		found := false
		for _, item := range s.officialCampaigns {
			if item.ID == id {
				campaign = item
				found = true
				break
			}
		}
		if !found {
			s.mu.Unlock()
			return errors.New("发送记录不存在")
		}
		// A previous process may have sent before losing its reply. Never blindly
		// replay such a claim; only untouched pending recipients can be recovered.
		err := persistentMutationError(s, func(work *MemoryStore) error {
			for i, item := range work.officialCampaignRecipients {
				if item.CampaignID == id && item.Status == "发送中" && now.Sub(parseBusinessTime(item.ClaimedAt)) > time.Minute {
					work.officialCampaignRecipients[i].Status = "结果待确认"
					work.officialCampaignRecipients[i].FailureReason = "发送进程中断，请核对微信回执和手机收件"
					work.officialCampaignRecipients[i].Retryable = false
				}
			}
			work.updateOfficialCampaignDelivery(id)
			return nil
		})
		s.mu.Unlock()
		if err != nil {
			return err
		}
		for {
			s.mu.Lock()
			index := -1
			for i, item := range s.officialCampaignRecipients {
				if item.CampaignID == id && item.Status == "待发送" {
					index = i
					break
				}
			}
			if index < 0 {
				s.mu.Unlock()
				return nil
			}
			recipient := s.officialCampaignRecipients[index]
			previewReq := learning.OfficialAudiencePreviewRequest{Grades: campaign.Grades, RecipientMode: campaign.RecipientMode, GuardianIDs: campaign.GuardianIDs}
			_, targets, audienceErr := s.officialSelectedAudienceUnlocked(previewReq)
			reachable := false
			for _, target := range targets {
				if target.GuardianID == recipient.GuardianID && target.OpenID == recipient.OpenID {
					reachable = true
				}
			}
			err = persistentMutationError(s, func(work *MemoryStore) error {
				current := &work.officialCampaignRecipients[index]
				if audienceErr != nil || !reachable {
					current.Status = "发送失败"
					current.FailureReason = "家长已取消关注、解绑或身份不可用"
					current.Retryable = false
				} else {
					current.Status = "发送中"
					current.ClaimedAt = businessTime(now)
					current.RetryCount++
					current.Retryable = false
				}
				work.updateOfficialCampaignDelivery(id)
				return nil
			})
			sender, legacy := s.officialMessageSender, s.officialTemplateSender
			s.mu.Unlock()
			if err != nil {
				return err
			}
			if !reachable || audienceErr != nil {
				continue
			}
			var messageID string
			var sendErr error
			if sender != nil {
				messageID, sendErr = sendBusinessTemplateSafely(sender, learning.OfficialMessageRequest{TemplateID: campaign.TemplateID, OpenID: recipient.OpenID, Values: campaign.Values, PagePath: campaign.PagePath, ClientMessageID: businessNoticeHash(recipient.ID)})
			} else if legacy != nil {
				sendErr = legacy(campaign.TemplateID, recipient.OpenID, campaign.Values, campaign.PagePath)
			} else {
				sendErr = &officialSendError{reason: "公众号发送配置不可用", configuration: true}
			}
			s.mu.Lock()
			err = persistentMutationError(s, func(work *MemoryStore) error {
				for i, item := range work.officialCampaignRecipients {
					if item.ID != recipient.ID {
						continue
					}
					if item.Status == "已送达" {
						continue
					}
					item.FailureReason = ""
					if sendErr == nil {
						item.Status = "微信已受理"
						item.MessageID = messageID
						item.AcceptedAt = businessTime(time.Now())
						item.SentAt = time.Now().Format("2006-01-02 15:04:05")
						if sender == nil {
							item.Status = "发送成功"
						}
						for _, receipt := range work.businessNoticeReceipts {
							if receipt.MessageID == messageID && receipt.OpenID == item.OpenID {
								applyCampaignReceipt(&item, receipt)
							}
						}
					} else {
						item.Status = "发送失败"
						item.FailureReason = sendErr.Error()
						var typed *officialSendError
						if errors.As(sendErr, &typed) {
							item.Retryable = typed.temporary && item.RetryCount < 4
							if typed.uncertain {
								item.Status = "结果待确认"
								item.Retryable = false
							}
						}
					}
					work.officialCampaignRecipients[i] = item
				}
				work.updateOfficialCampaignDelivery(id)
				return nil
			})
			s.mu.Unlock()
			if err != nil {
				return fmt.Errorf("保存公众号发送结果失败: %w", err)
			}
		}
	})
}

func (s *MemoryStore) RunOfficialCampaignWorker(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.Lock()
			ids := []string{}
			seen := map[string]bool{}
			for _, recipient := range s.officialCampaignRecipients {
				if (recipient.Status == "待发送" || recipient.Status == "发送中") && !seen[recipient.CampaignID] {
					ids = append(ids, recipient.CampaignID)
					seen[recipient.CampaignID] = true
				}
			}
			s.mu.Unlock()
			for _, id := range ids {
				if err := s.processOfficialCampaign(id, now); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}
}
