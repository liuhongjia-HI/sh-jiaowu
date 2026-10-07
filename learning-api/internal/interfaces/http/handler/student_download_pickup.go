package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"
	"strings"
	"time"
)

// Approval is ephemeral: restart expires approvals, while persisted jobs survive.
type downloadPickup struct {
	JobID, BrowserKey string
	Expires           time.Time
	Principal         learning.Principal
	Approved          bool
}

func pickupNonce() (string, error) {
	var b [24]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func (h *LearningHandler) CreateDownloadPickup(c *gin.Context) {
	var req struct {
		JobID string `json:"jobId"`
	}
	if c.ShouldBindJSON(&req) != nil || !strings.HasPrefix(req.JobID, "download-") || len(req.JobID) != 41 {
		BadRequest(c, "领取链接无效")
		return
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(req.JobID, "download-")); err != nil {
		BadRequest(c, "领取链接无效")
		return
	}
	challenge, err := pickupNonce()
	if err != nil {
		BadRequest(c, "暂时无法生成二维码")
		return
	}
	key, err := pickupNonce()
	if err != nil {
		BadRequest(c, "暂时无法生成二维码")
		return
	}
	h.pickupMu.Lock()
	if h.pickups == nil {
		h.pickups = map[string]downloadPickup{}
	}
	for id, item := range h.pickups {
		if !time.Now().Before(item.Expires) {
			delete(h.pickups, id)
		}
	}
	if len(h.pickups) >= 1000 {
		h.pickupMu.Unlock()
		BadRequest(c, "领取请求较多，请稍后重试")
		return
	}
	expires := time.Now().Add(10 * time.Minute)
	h.pickups[challenge] = downloadPickup{JobID: req.JobID, BrowserKey: key, Expires: expires}
	h.pickupMu.Unlock()
	c.Header("Cache-Control", "no-store")
	OK(c, gin.H{"challenge": challenge, "browserKey": key, "expiresAt": expires.UTC().Format(time.RFC3339)})
}
func (h *LearningHandler) pickup(c *gin.Context, browser bool) (downloadPickup, bool) {
	h.pickupMu.Lock()
	item, ok := h.pickups[c.Param("challenge")]
	h.pickupMu.Unlock()
	if !ok || !time.Now().Before(item.Expires) {
		BadRequest(c, "二维码已过期，请在电脑上刷新")
		return item, false
	}
	if browser && subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")), []byte(item.BrowserKey)) != 1 {
		Forbidden(c, "请从生成二维码的电脑领取")
		return item, false
	}
	return item, true
}
func (h *LearningHandler) studentPickupJob(p learning.Principal, id string) (learning.MaterialDownloadJob, bool) {
	for _, job := range h.service.MaterialDownloads(p) {
		if job.ID == id && job.StudentID == p.StudentID && job.StudentID != "" {
			return job, true
		}
	}
	return learning.MaterialDownloadJob{}, false
}
func (h *LearningHandler) StudentDownloadPickup(c *gin.Context) {
	item, ok := h.pickup(c, false)
	if !ok {
		return
	}
	p, _ := middleware.CurrentPrincipal(c)
	job, ok := h.studentPickupJob(p, item.JobID)
	if !ok {
		Forbidden(c, "请切换到创建任务的学生账号后扫码")
		return
	}
	if _, err := h.service.MaterialDownloadArchive(p, job.ID); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, job)
}
func (h *LearningHandler) ConfirmDownloadPickup(c *gin.Context) {
	item, ok := h.pickup(c, false)
	if !ok {
		return
	}
	p, _ := middleware.CurrentPrincipal(c)
	if _, ok := h.studentPickupJob(p, item.JobID); !ok {
		Forbidden(c, "请切换到创建任务的学生账号后扫码")
		return
	}
	if _, err := h.service.MaterialDownloadArchive(p, item.JobID); err != nil {
		BadRequest(c, err.Error())
		return
	}
	h.pickupMu.Lock()
	current, exists := h.pickups[c.Param("challenge")]
	if !exists || !time.Now().Before(current.Expires) {
		h.pickupMu.Unlock()
		BadRequest(c, "二维码已过期，请刷新")
		return
	}
	if current.Approved && (current.Principal.UserID != p.UserID || current.Principal.GuardianID != p.GuardianID) {
		h.pickupMu.Unlock()
		Forbidden(c, "该领取已确认")
		return
	}
	current.Principal, current.Approved = p, true
	h.pickups[c.Param("challenge")] = current
	h.pickupMu.Unlock()
	OK(c, gin.H{"confirmed": true})
}
func (h *LearningHandler) pickupPrincipal(c *gin.Context, item downloadPickup) (learning.Principal, bool) {
	p, err := h.service.PrincipalByUserID(item.Principal.UserID)
	if err != nil || p.TokenVersion != item.Principal.TokenVersion || p.StudentID != item.Principal.StudentID || item.Principal.GuardianID != "" && !h.service.GuardianStudentActive(item.Principal.GuardianID, p.StudentID) {
		Forbidden(c, "账号或学生关系已变化，请重新扫码")
		return p, false
	}
	p.GuardianID = item.Principal.GuardianID
	return p, true
}
func (h *LearningHandler) DownloadPickupStatus(c *gin.Context) {
	item, ok := h.pickup(c, true)
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	if !item.Approved {
		OK(c, gin.H{"status": "waiting"})
		return
	}
	p, ok := h.pickupPrincipal(c, item)
	if !ok {
		return
	}
	if _, err := h.service.MaterialDownloadArchive(p, item.JobID); err != nil {
		Forbidden(c, err.Error())
		return
	}
	job, ok := h.studentPickupJob(p, item.JobID)
	if !ok {
		Forbidden(c, "下载任务已失效")
		return
	}
	OK(c, gin.H{"status": "approved", "job": job})
}
func (h *LearningHandler) DownloadPickupArchive(c *gin.Context) {
	item, ok := h.pickup(c, true)
	if !ok {
		return
	}
	if !item.Approved {
		Forbidden(c, "请先在小程序确认领取")
		return
	}
	p, ok := h.pickupPrincipal(c, item)
	if !ok {
		return
	}
	archive, err := h.service.MaterialDownloadArchive(p, item.JobID)
	if err != nil {
		Forbidden(c, err.Error())
		return
	}
	stored, err := secureMaterialArchivePath(h.fileStorageRoot, archive)
	if err != nil {
		_ = h.service.InvalidateMaterialDownload(p, item.JobID)
		BadRequest(c, "下载包不可用，请在小程序重新生成")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(stored, "课程讲义.zip")
}
