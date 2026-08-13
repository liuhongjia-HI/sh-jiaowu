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
	maxStudentAvatarSize      = 2 * 1024 * 1024
	maxStudentAvatarDimension = 4096
)

var studentAvatarTypes = map[string]struct {
	contentType string
	extension   string
}{
	"jpeg": {contentType: "image/jpeg", extension: ".jpg"},
	"png":  {contentType: "image/png", extension: ".png"},
	"gif":  {contentType: "image/gif", extension: ".gif"},
}

type studentAvatarFile struct {
	Path     string
	URL      string
	FileName string
}

// UploadStudentAvatar 接收微信 chooseAvatar 返回的临时文件，并把它转成服务端可长期访问的地址。
// 临时本地路径只在当前小程序设备上有效，不能直接写入学生资料。
func (h *LearningHandler) UploadStudentAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择头像")
		return
	}
	avatar, err := saveStudentAvatar(file)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	principal, ok := middleware.CurrentPrincipal(c)
	if !ok || principal.StudentID == "" {
		_ = os.Remove(avatar.Path)
		Unauthorized(c, "学生登录状态无效，请重新登录")
		return
	}
	updated, err := h.service.UpdateStudentProfile(principal.Name, principal, learning.StudentProfileUpdateRequest{
		AvatarURL: avatar.URL,
	})
	if err != nil {
		_ = os.Remove(avatar.Path)
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

// StudentAvatar 是公开的只读图片地址：微信小程序 image 组件不会为图片请求自动追加 Authorization header。
func (h *LearningHandler) StudentAvatar(c *gin.Context) {
	fileName := strings.TrimSpace(c.Param("asset"))
	if !isSafeStudentAvatarName(fileName) {
		c.Status(http.StatusNotFound)
		return
	}
	root, err := filepath.Abs(filepath.Join("uploads", "avatars"))
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

func saveStudentAvatar(file *multipart.FileHeader) (studentAvatarFile, error) {
	if file == nil || file.Size <= 0 {
		return studentAvatarFile{}, errors.New("头像内容为空，请重新选择")
	}
	if file.Size > maxStudentAvatarSize {
		return studentAvatarFile{}, errors.New("头像太大，请选择 2MB 以内的图片")
	}

	opened, err := file.Open()
	if err != nil {
		return studentAvatarFile{}, errors.New("头像读取失败，请重新选择")
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, maxStudentAvatarSize+1))
	if err != nil {
		return studentAvatarFile{}, errors.New("头像读取失败，请重新选择")
	}
	if len(data) == 0 {
		return studentAvatarFile{}, errors.New("头像内容为空，请重新选择")
	}
	if len(data) > maxStudentAvatarSize {
		return studentAvatarFile{}, errors.New("头像太大，请选择 2MB 以内的图片")
	}

	contentType := http.DetectContentType(data)
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return studentAvatarFile{}, errors.New("头像格式不正确，请选择 JPG、PNG 或 GIF 图片")
	}
	spec, ok := studentAvatarTypes[format]
	if !ok || contentType != spec.contentType {
		return studentAvatarFile{}, errors.New("头像格式不正确，请选择 JPG、PNG 或 GIF 图片")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxStudentAvatarDimension || config.Height > maxStudentAvatarDimension {
		return studentAvatarFile{}, errors.New("头像尺寸过大，请重新选择")
	}

	root, err := filepath.Abs(filepath.Join("uploads", "avatars"))
	if err != nil {
		return studentAvatarFile{}, errors.New("头像目录初始化失败")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return studentAvatarFile{}, errors.New("头像目录创建失败")
	}
	randomPart := make([]byte, 16)
	if _, err := rand.Read(randomPart); err != nil {
		return studentAvatarFile{}, errors.New("头像文件名生成失败")
	}
	fileName := "avatar-" + hex.EncodeToString(randomPart) + spec.extension
	path := filepath.Join(root, fileName)
	created, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return studentAvatarFile{}, errors.New("头像保存失败，请重试")
	}
	if _, err := created.Write(data); err != nil {
		created.Close()
		_ = os.Remove(path)
		return studentAvatarFile{}, errors.New("头像保存失败，请重试")
	}
	if err := created.Close(); err != nil {
		_ = os.Remove(path)
		return studentAvatarFile{}, errors.New("头像保存失败，请重试")
	}
	return studentAvatarFile{
		Path:     path,
		FileName: fileName,
		URL:      "/api/student/avatars/" + fileName,
	}, nil
}

func isSafeStudentAvatarName(fileName string) bool {
	if fileName == "" || filepath.Base(fileName) != fileName || strings.Contains(fileName, "..") || !strings.HasPrefix(fileName, "avatar-") {
		return false
	}
	for _, spec := range studentAvatarTypes {
		if strings.HasSuffix(fileName, spec.extension) {
			return true
		}
	}
	return false
}
