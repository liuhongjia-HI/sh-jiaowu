package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// errQPDFUnavailable 表示服务器没有安装 qpdf，无法生成带权限限制的 PDF。
var errQPDFUnavailable = errors.New("qpdf unavailable")

const pdfProtectionTimeout = 2 * time.Minute

func qpdfAvailable() bool {
	_, err := exec.LookPath("qpdf")
	return err == nil
}

// protectPDFArgs 集中维护 qpdf 的安全参数，避免学生拿到可复制/可编辑的普通 PDF。
// 空 user password 保持微信 wx.openDocument 的无感打开体验；owner password 仅在服务端
// 进程中短暂存在，用于让遵循 PDF 权限的阅读器拒绝复制和修改。
func protectPDFArgs(ownerPassword, sourcePath, targetPath string) []string {
	return []string{
		"--encrypt", "", ownerPassword, "256",
		"--print=none", "--modify=none", "--extract=n",
		"--", sourcePath, targetPath,
	}
}

func protectPDF(ctx context.Context, sourcePath, targetPath string) error {
	if !qpdfAvailable() {
		return errQPDFUnavailable
	}
	ownerPassword, err := randomOwnerPassword()
	if err != nil {
		return fmt.Errorf("生成 PDF 权限密钥失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, pdfProtectionTimeout)
	defer cancel()
	cmd := execCommandContext(ctx, "qpdf", protectPDFArgs(ownerPassword, sourcePath, targetPath)...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("生成受保护 PDF 失败: %w", err)
	}
	if err := os.Chmod(targetPath, 0600); err != nil {
		return fmt.Errorf("设置受保护 PDF 权限失败: %w", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("受保护 PDF 未生成: %w", err)
	}
	return nil
}

func randomOwnerPassword() (string, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(secret), nil
}
