package store

import (
	"errors"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) bannersUnlocked() []learning.Banner {
	out := make([]learning.Banner, 0, len(s.banners))
	for _, item := range s.banners {
		out = append(out, decorateBannerStatus(item))
	}
	sortBanners(out)
	return out
}

// activeStudentBannersUnlocked 是学生端小程序首页实际会用到的那部分：
// 只留启用中、且落在生效时间段内的 banner，按排序号排好直接给前端渲染，
// 前端不用再拿 status 自己判断一遍要不要展示。
func (s *MemoryStore) activeStudentBannersUnlocked() []learning.Banner {
	out := make([]learning.Banner, 0, len(s.banners))
	for _, item := range s.banners {
		decorated := decorateBannerStatus(item)
		if decorated.Status != "生效中" {
			continue
		}
		out = append(out, decorated)
	}
	sortBanners(out)
	return out
}

func sortBanners(items []learning.Banner) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
}

func decorateBannerStatus(item learning.Banner) learning.Banner {
	if !item.Enabled {
		item.Status = "已停用"
		return item
	}
	today := time.Now().Format("2006-01-02")
	if item.StartsAt != "" && item.StartsAt > today {
		item.Status = "未开始"
		return item
	}
	if item.EndsAt != "" && item.EndsAt < today {
		item.Status = "已结束"
		return item
	}
	item.Status = "生效中"
	return item
}

func (s *MemoryStore) createBannerUnlocked(operator string, req learning.BannerUpsertRequest) (learning.Banner, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Banner, error) {
			return work.createBannerUnlocked(operator, req)
		})
	}
	req, err := normalizeBannerRequest(req)
	if err != nil {
		return learning.Banner{}, err
	}
	item := learning.Banner{
		ID:        "banner-" + time.Now().Format("20060102150405.000000000"),
		ImageURL:  req.ImageURL,
		Title:     req.Title,
		LinkType:  req.LinkType,
		LinkValue: req.LinkValue,
		SortOrder: req.SortOrder,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
		Enabled:   req.Enabled,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	s.banners = append(s.banners, item)
	s.prependLog(operator, "新增轮播图", bannerLogTarget(item))
	return decorateBannerStatus(item), nil
}

func (s *MemoryStore) updateBannerUnlocked(operator string, id string, req learning.BannerUpsertRequest) (learning.Banner, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Banner, error) {
			return work.updateBannerUnlocked(operator, id, req)
		})
	}
	req, err := normalizeBannerRequest(req)
	if err != nil {
		return learning.Banner{}, err
	}
	for i := range s.banners {
		if s.banners[i].ID != id {
			continue
		}
		s.banners[i].ImageURL = req.ImageURL
		s.banners[i].Title = req.Title
		s.banners[i].LinkType = req.LinkType
		s.banners[i].LinkValue = req.LinkValue
		s.banners[i].SortOrder = req.SortOrder
		s.banners[i].StartsAt = req.StartsAt
		s.banners[i].EndsAt = req.EndsAt
		s.banners[i].Enabled = req.Enabled
		s.prependLog(operator, "更新轮播图", bannerLogTarget(s.banners[i]))
		return decorateBannerStatus(s.banners[i]), nil
	}
	return learning.Banner{}, errors.New("轮播图不存在")
}

func (s *MemoryStore) deleteBannerUnlocked(operator string, id string) error {
	if s.db != nil {
		return persistentMutationError(s, func(work *MemoryStore) error {
			return work.deleteBannerUnlocked(operator, id)
		})
	}
	for i := range s.banners {
		if s.banners[i].ID != id {
			continue
		}
		name := bannerLogTarget(s.banners[i])
		s.banners = append(s.banners[:i], s.banners[i+1:]...)
		s.prependLog(operator, "删除轮播图", name)
		return nil
	}
	return errors.New("轮播图不存在")
}

func normalizeBannerRequest(req learning.BannerUpsertRequest) (learning.BannerUpsertRequest, error) {
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	req.Title = strings.TrimSpace(req.Title)
	req.LinkType = strings.TrimSpace(req.LinkType)
	req.LinkValue = strings.TrimSpace(req.LinkValue)
	req.StartsAt = strings.TrimSpace(req.StartsAt)
	req.EndsAt = strings.TrimSpace(req.EndsAt)
	if req.ImageURL == "" {
		return req, errors.New("请上传轮播图图片")
	}
	if req.LinkType == "" {
		req.LinkType = "none"
	}
	if req.LinkType != "none" && req.LinkType != "page" && req.LinkType != "url" {
		return req, errors.New("跳转类型不正确")
	}
	if req.LinkType != "none" && req.LinkValue == "" {
		return req, errors.New("请填写跳转目标")
	}
	if req.StartsAt != "" && req.EndsAt != "" && req.StartsAt > req.EndsAt {
		return req, errors.New("生效开始时间不能晚于结束时间")
	}
	return req, nil
}

func bannerLogTarget(item learning.Banner) string {
	if item.Title != "" {
		return item.Title
	}
	return item.ID
}
