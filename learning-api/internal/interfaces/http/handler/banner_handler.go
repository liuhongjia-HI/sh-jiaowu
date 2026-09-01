package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

const (
	maxBannerImageSize      = 5 * 1024 * 1024
	maxBannerImageDimension = 4096
)

var bannerImageTypes = map[string]struct {
	contentType string
	extension   string
}{
	"jpeg": {contentType: "image/jpeg", extension: ".jpg"},
	"png":  {contentType: "image/png", extension: ".png"},
}

func (h *LearningHandler) Banners(c *gin.Context) {
	OK(c, h.service.Banners())
}

func (h *LearningHandler) CreateBanner(c *gin.Context) {
	req, ok := bindBanner(c)
	if !ok {
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateBanner(operator.(string), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}

func (h *LearningHandler) UpdateBanner(c *gin.Context) {
	req, ok := bindBanner(c)
	if !ok {
		return
	}
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateBanner(operator.(string), c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) DeleteBanner(c *gin.Context) {
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.DeleteBanner(operator.(string), c.Param("id")); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"id": c.Param("id")})
}

// StudentBanners 只返回启用中、且在生效时间段内的轮播图，供小程序首页直接渲染。
func (h *LearningHandler) StudentBanners(c *gin.Context) {
	OK(c, h.service.ActiveStudentBanners())
}

func bindBanner(c *gin.Context) (learning.BannerUpsertRequest, bool) {
	var req learning.BannerUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return learning.BannerUpsertRequest{}, false
	}
	return req, true
}

// UploadBannerImage 单独上传轮播图图片，返回可直接存进 Banner.ImageURL 的地址。
// 和记录本身的增删改分开：调整排序、生效时间或下线一条轮播图都不需要重新传图。
func (h *LearningHandler) UploadBannerImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择轮播图图片")
		return
	}
	asset, err := saveBannerImage(file, h.fileStorageRoot)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"imageUrl": asset.URL})
}

// UploadGradeSubjectImage 复用受校验的图片存储，供年级课程目录上传学科封面。
// 返回的公开只读地址可被小程序 image 组件直接加载。
func (h *LearningHandler) UploadGradeSubjectImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择学科图片")
		return
	}
	asset, err := saveBannerImage(file, h.fileStorageRoot)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"imageUrl": asset.URL})
}

// BannerImage 是公开的只读图片地址：微信小程序 image 组件不会为图片请求自动追加
// Authorization header，轮播图又必须在登录前（首页刚打开）就能显示。
func (h *LearningHandler) BannerImage(c *gin.Context) {
	fileName := strings.TrimSpace(c.Param("asset"))
	if !isSafeBannerImageName(fileName) {
		c.Status(http.StatusNotFound)
		return
	}
	root, err := filepath.Abs(filepath.Join(h.fileStorageRoot, "banners"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	path := filepath.Join(root, fileName)
	if filepath.Base(path) != fileName {
		c.Status(http.StatusNotFound)
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(path)
}

type bannerImageFile struct {
	URL string
}

func saveBannerImage(file *multipart.FileHeader, storageRoot string) (bannerImageFile, error) {
	if file == nil || file.Size <= 0 {
		return bannerImageFile{}, errors.New("图片内容为空，请重新选择")
	}
	if file.Size > maxBannerImageSize {
		return bannerImageFile{}, errors.New("图片太大，请选择 5MB 以内的图片")
	}

	opened, err := file.Open()
	if err != nil {
		return bannerImageFile{}, errors.New("图片读取失败，请重新选择")
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, maxBannerImageSize+1))
	if err != nil {
		return bannerImageFile{}, errors.New("图片读取失败，请重新选择")
	}
	if len(data) == 0 {
		return bannerImageFile{}, errors.New("图片内容为空，请重新选择")
	}
	if len(data) > maxBannerImageSize {
		return bannerImageFile{}, errors.New("图片太大，请选择 5MB 以内的图片")
	}

	contentType := http.DetectContentType(data)
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return bannerImageFile{}, errors.New("图片格式不正确，请选择 JPG 或 PNG 图片")
	}
	spec, ok := bannerImageTypes[format]
	if !ok || contentType != spec.contentType {
		return bannerImageFile{}, errors.New("图片格式不正确，请选择 JPG 或 PNG 图片")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxBannerImageDimension || config.Height > maxBannerImageDimension {
		return bannerImageFile{}, errors.New("图片尺寸过大，请重新选择")
	}

	root, err := filepath.Abs(filepath.Join(storageRoot, "banners"))
	if err != nil {
		return bannerImageFile{}, errors.New("图片目录初始化失败")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return bannerImageFile{}, errors.New("图片目录创建失败")
	}
	randomPart := make([]byte, 16)
	if _, err := rand.Read(randomPart); err != nil {
		return bannerImageFile{}, errors.New("图片文件名生成失败")
	}
	fileName := "banner-" + hex.EncodeToString(randomPart) + spec.extension
	path := filepath.Join(root, fileName)
	created, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return bannerImageFile{}, errors.New("图片保存失败，请重试")
	}
	if _, err := created.Write(data); err != nil {
		created.Close()
		_ = os.Remove(path)
		return bannerImageFile{}, errors.New("图片保存失败，请重试")
	}
	if err := created.Close(); err != nil {
		_ = os.Remove(path)
		return bannerImageFile{}, errors.New("图片保存失败，请重试")
	}
	return bannerImageFile{URL: "/api/banners/images/" + fileName}, nil
}

func isSafeBannerImageName(fileName string) bool {
	if fileName == "" || filepath.Base(fileName) != fileName || strings.Contains(fileName, "..") || !strings.HasPrefix(fileName, "banner-") {
		return false
	}
	for _, spec := range bannerImageTypes {
		if strings.HasSuffix(fileName, spec.extension) {
			return true
		}
	}
	return false
}
