package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
)

const maxPreviewPages = 300

type PreviewWorker struct {
	service     *learningapp.Service
	storageRoot string
}

func NewPreviewWorker(service *learningapp.Service, storageRoot string) *PreviewWorker {
	return &PreviewWorker{service: service, storageRoot: storageRoot}
}

func (w *PreviewWorker) Recover() error {
	return w.service.RecoverPreviewJobs()
}

func (w *PreviewWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if processed := w.processNext(ctx); processed {
			// 给转换工具和数据库留出恢复窗口，失败重试时避免无间隔占满 CPU。
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *PreviewWorker) processNext(ctx context.Context) bool {
	job, ok, err := w.service.ClaimPreviewJob()
	if err != nil || !ok {
		return false
	}
	asset, err := w.service.PreviewJobFile(job.FileID)
	if err == nil {
		var result learning.PreviewResult
		result, err = w.generate(ctx, asset)
		if err == nil {
			err = w.service.CompletePreviewJob(job.ID, result)
		}
	}
	if err != nil {
		_ = w.service.FailPreviewJob(job.ID, err.Error())
	}
	return true
}

func (w *PreviewWorker) generate(ctx context.Context, asset learning.FileAsset) (learning.PreviewResult, error) {
	if _, err := os.Stat(asset.OriginalPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return learning.PreviewResult{}, errors.New("原文件不存在，请重新上传")
		}
		return learning.PreviewResult{}, fmt.Errorf("读取原文件失败: %w", err)
	}
	root, err := filepath.Abs(w.storageRoot)
	if err != nil {
		return learning.PreviewResult{}, errors.New("文件存储目录不可用")
	}
	previewDir := filepath.Join(root, "preview")
	pageDir := filepath.Join(root, "pages", asset.ID)
	if err := os.MkdirAll(previewDir, 0750); err != nil {
		return learning.PreviewResult{}, fmt.Errorf("创建预览目录失败: %w", err)
	}
	if err := os.MkdirAll(pageDir, 0750); err != nil {
		return learning.PreviewResult{}, fmt.Errorf("创建分页目录失败: %w", err)
	}
	previewPath, status := buildPreview(asset.OriginalPath, previewDir, filepath.Ext(asset.OriginalPath))
	if status != "可预览" || previewPath == "" {
		return learning.PreviewResult{}, errors.New("预览PDF生成失败，请检查文件格式和转换服务")
	}
	pageCount, err := countPDFPages(ctx, previewPath)
	if err != nil {
		return learning.PreviewResult{}, fmt.Errorf("课件页数识别失败: %w", err)
	}
	if pageCount > maxPreviewPages {
		return learning.PreviewResult{}, fmt.Errorf("课件共%d页，超过%d页上限", pageCount, maxPreviewPages)
	}
	for page := 1; page <= pageCount; page++ {
		target := filepath.Join(pageDir, fmt.Sprintf("page-%04d.jpg", page))
		if err := rasterizePDFPage(ctx, previewPath, target, page); err != nil {
			return learning.PreviewResult{}, fmt.Errorf("第%d页生成失败: %w", page, err)
		}
	}
	return learning.PreviewResult{PreviewPath: previewPath, PreviewPageDir: pageDir, PreviewPageCount: pageCount}, nil
}
