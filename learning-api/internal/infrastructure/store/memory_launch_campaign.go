package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"time"
)

func (s *MemoryStore) LaunchCampaign(p learning.Principal) (*learning.LaunchCampaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	var cfg learning.LaunchCampaignConfig
	raw := s.settings["launchCampaign"]
	if raw == "" || json.Unmarshal([]byte(raw), &cfg) != nil || !cfg.Enabled {
		return nil, nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	if (cfg.StartsAt != "" && now < cfg.StartsAt) || (cfg.EndsAt != "" && now > cfg.EndsAt) {
		return nil, nil
	}
	if cfg.TemplateType == "" {
		cfg.TemplateType = "generic"
	}
	if cfg.PrimaryActionText == "" {
		cfg.PrimaryActionText = "立即了解"
	}
	if cfg.ActionType == "" {
		if cfg.TemplateType == "small_class_reservation" {
			cfg.ActionType = "submit_reservation"
		} else {
			cfg.ActionType = "close"
		}
	}
	return &learning.LaunchCampaign{ID: "launch-default", TemplateType: cfg.TemplateType, Title: cfg.Title, Message: cfg.Message, SubMessage: cfg.SubMessage, ImageURL: cfg.ImageURL, PrimaryActionText: cfg.PrimaryActionText, ActionType: cfg.ActionType, Frequency: cfg.Frequency, TimeOptions: cfg.TimeOptions}, nil
}

func (s *MemoryStore) CreateClassReservation(operator string, p learning.Principal, req learning.ClassReservationRequest) (learning.ClassReservationIntent, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ClassReservationIntent, error) {
			return work.createClassReservationUnlocked(operator, p, req)
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createClassReservationUnlocked(operator, p, req)
}
func (s *MemoryStore) createClassReservationUnlocked(operator string, p learning.Principal, req learning.ClassReservationRequest) (learning.ClassReservationIntent, error) {
	if p.StudentID == "" {
		return learning.ClassReservationIntent{}, errors.New("学生未绑定")
	}
	for _, r := range s.classReservations {
		if r.StudentID == p.StudentID && r.CampaignID == req.CampaignID && r.Status != "closed" {
			return r, nil
		}
	}
	student, ok := s.findStudent(p.StudentID)
	if !ok {
		return learning.ClassReservationIntent{}, errors.New("学生不存在")
	}
	now := time.Now().Format(time.RFC3339)
	r := learning.ClassReservationIntent{ID: fmt.Sprintf("reservation-%d", time.Now().UnixNano()), StudentID: p.StudentID, StudentName: student.Name, Grade: student.Grade, CampaignID: strings.TrimSpace(req.CampaignID), TimeOption: strings.TrimSpace(req.TimeOption), Status: "pending", CreatedAt: now, UpdatedAt: now}
	s.classReservations = append(s.classReservations, r)
	s.prependLogDetail(operator, "提交开屏预约意向", r.StudentName, "")
	return r, nil
}
func (s *MemoryStore) ClassReservations(p learning.Principal) []learning.ClassReservationIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]learning.ClassReservationIntent(nil), s.classReservations...)
}
func (s *MemoryStore) UpdateClassReservation(operator string, p learning.Principal, id string, req learning.ClassReservationUpdateRequest) (learning.ClassReservationIntent, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ClassReservationIntent, error) {
			return work.updateClassReservationUnlocked(operator, p, id, req)
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateClassReservationUnlocked(operator, p, id, req)
}
func (s *MemoryStore) updateClassReservationUnlocked(operator string, p learning.Principal, id string, req learning.ClassReservationUpdateRequest) (learning.ClassReservationIntent, error) {
	for i := range s.classReservations {
		if s.classReservations[i].ID == id {
			if req.Status != "pending" && req.Status != "contacted" && req.Status != "completed" && req.Status != "closed" {
				return learning.ClassReservationIntent{}, errors.New("预约状态不正确")
			}
			s.classReservations[i].Status = req.Status
			s.classReservations[i].Remark = strings.TrimSpace(req.Remark)
			s.classReservations[i].UpdatedAt = time.Now().Format(time.RFC3339)
			s.prependLogDetail(operator, "更新开屏预约意向", id, req.Status)
			return s.classReservations[i], nil
		}
	}
	return learning.ClassReservationIntent{}, errors.New("预约记录不存在")
}
