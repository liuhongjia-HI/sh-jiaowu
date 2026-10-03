package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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
		// 允许客户打印学习版，但仍禁止修改和提取内容。
		"--print=full", "--modify=none", "--extract=n",
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
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		if !qpdfWarningsOnly(err) || ctx.Err() != nil {
			return fmt.Errorf("生成受保护 PDF 失败: %w; %s", err, safePDFDiagnostic(diagnostic.String(), ownerPassword))
		}
		// qpdf 的 3 表示只有警告。只有输出通过结构和权限检查后才允许下发。
		if err := validateProtectedPDF(ctx, targetPath); err != nil {
			return err
		}
		log.Printf("event=student_pdf_protection_warning diagnostic=%q", safePDFDiagnostic(diagnostic.String(), ownerPassword))
	}
	if err := os.Chmod(targetPath, 0600); err != nil {
		return fmt.Errorf("设置受保护 PDF 权限失败: %w", err)
	}
	if info, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("受保护 PDF 未生成: %w", err)
	} else if !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("受保护 PDF 输出为空或不是普通文件")
	}
	return nil
}

func qpdfWarningsOnly(err error) bool {
	var exitError *exec.ExitError
	return errors.As(err, &exitError) && exitError.ExitCode() == 3
}

func safePDFDiagnostic(text, ownerPassword string) string {
	text = strings.ReplaceAll(text, ownerPassword, "[redacted]")
	text = strings.TrimSpace(text)
	if len(text) > 4096 {
		text = text[:4096] + "..."
	}
	return text
}

func validateProtectedPDF(ctx context.Context, path string) error {
	check := execCommandContext(ctx, "qpdf", "--check", path)
	var diagnostic bytes.Buffer
	check.Stderr = &diagnostic
	if err := check.Run(); err != nil && (!qpdfWarningsOnly(err) || ctx.Err() != nil) {
		return fmt.Errorf("受保护 PDF 结构校验失败: %w", err)
	}
	metadata := execCommandContext(ctx, "qpdf", "--json", "--json-key=encrypt", path)
	output, err := metadata.Output()
	if err != nil && (!qpdfWarningsOnly(err) || ctx.Err() != nil) {
		return fmt.Errorf("读取受保护 PDF 权限失败: %w", err)
	}
	var document struct {
		Encrypt struct {
			Encrypted           bool `json:"encrypted"`
			UserPasswordMatched bool `json:"userpasswordmatched"`
			Parameters          struct {
				Bits   int    `json:"bits"`
				Method string `json:"method"`
			} `json:"parameters"`
			Capabilities map[string]bool `json:"capabilities"`
		} `json:"encrypt"`
	}
	if err := json.Unmarshal(output, &document); err != nil {
		return fmt.Errorf("解析受保护 PDF 权限失败: %w", err)
	}
	encryption := document.Encrypt
	if !encryption.Encrypted || !encryption.UserPasswordMatched || encryption.Parameters.Bits != 256 || encryption.Parameters.Method != "AESv3" {
		return errors.New("受保护 PDF 未满足 AES-256 无密码打开要求")
	}
	for _, capability := range []string{"extract", "modify", "modifyannotations", "modifyassembly", "modifyforms", "modifyother", "printhigh", "printlow"} {
		allowed, present := encryption.Capabilities[capability]
		want := capability == "printhigh" || capability == "printlow"
		if !present || allowed != want {
			return errors.New("受保护 PDF 权限校验失败")
		}
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
