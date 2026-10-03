package handler

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"time"
)

type MaterialDownloadWorker struct {
	service *learningapp.Service
	root    string
}

func NewMaterialDownloadWorker(service *learningapp.Service, root string) *MaterialDownloadWorker {
	return &MaterialDownloadWorker{service, root}
}
func (w *MaterialDownloadWorker) Recover() error { return w.service.RecoverMaterialDownloads() }
func (w *MaterialDownloadWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		w.processNext(ctx)
		w.cleanup()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func secureMaterialPath(root, file string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	file, err = filepath.Abs(file)
	if err != nil {
		return "", err
	}
	file, err = filepath.EvalSymlinks(file)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("文件不在受控存储目录内")
	}
	return file, nil
}

func secureMaterialArchivePath(root, file string) (string, error) {
	directory := filepath.Join(root, "material-downloads")
	info, err := os.Lstat(directory)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("下载包目录不可重定向")
	}
	directory, err = secureMaterialPath(root, directory)
	if err != nil {
		return "", err
	}
	info, err = os.Lstat(file)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("下载包不是普通文件")
	}
	return secureMaterialPath(directory, file)
}

func (w *MaterialDownloadWorker) processNext(ctx context.Context) bool {
	job, files, found, err := w.service.ClaimMaterialDownload()
	if err != nil || !found {
		return false
	}
	archive, err := w.generate(ctx, job, files)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = w.service.FinishMaterialDownload(job.ID, archive, "")
	}
	if err != nil {
		if archive != "" {
			_ = os.Remove(archive)
		}
		// Shutdown leaves the claimed job recoverable under its original ID.
		if ctx.Err() != nil {
			return true
		}
		_ = w.service.FinishMaterialDownload(job.ID, "", "打包失败：文件缺失、内容变化或权限已撤回，请检查后重新生成")
	}
	return true
}

type materialDownloadContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r materialDownloadContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func (w *MaterialDownloadWorker) generate(ctx context.Context, job learning.MaterialDownloadJob, files []learning.MaterialDownloadFile) (string, error) {
	if len(files) != job.Count || len(files) == 0 {
		return "", errors.New("文件清单不完整")
	}
	dir := filepath.Join(w.root, "material-downloads")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	// Validate the output directory too: an existing symlink must not redirect
	// archives outside the configured storage root.
	dir, err := secureMaterialPath(w.root, dir)
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, job.ID+"-*.tmp")
	if err != nil {
		return "", err
	}
	temporary := file.Name()
	defer func() { _ = file.Close(); _ = os.Remove(temporary) }()
	writer := zip.NewWriter(file)
	names := map[string]bool{}
	total := int64(0)
	for _, item := range files {
		if ctx.Err() != nil {
			_ = writer.Close()
			return "", ctx.Err()
		}
		if item.Name == "" || names[item.Name] || strings.Contains(item.Name, "\\") || strings.HasPrefix(item.Name, "/") || strings.Contains("/"+item.Name+"/", "/../") {
			_ = writer.Close()
			return "", errors.New("打包目录无效或重名")
		}
		names[item.Name] = true
		stored, err := secureMaterialPath(w.root, item.Path)
		if err != nil {
			_ = writer.Close()
			return "", err
		}
		input, err := os.Open(stored)
		if err != nil {
			_ = writer.Close()
			return "", err
		}
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() || item.Size > 0 && item.Size != info.Size() {
			_ = input.Close()
			_ = writer.Close()
			return "", errors.New("原文件已变化")
		}
		total += info.Size()
		if total > 2*1024*1024*1024 {
			_ = input.Close()
			_ = writer.Close()
			return "", errors.New("下载包过大，请分批")
		}
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: item.Name, Method: zip.Deflate})
		if err != nil {
			_ = input.Close()
			_ = writer.Close()
			return "", err
		}
		copied, err := io.Copy(entry, materialDownloadContextReader{ctx, io.LimitReader(input, info.Size()+1)})
		closeErr := input.Close()
		if err != nil || closeErr != nil || copied != info.Size() {
			_ = writer.Close()
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", errors.New("文件读取不完整")
		}
	}
	if ctx.Err() != nil {
		_ = writer.Close()
		return "", ctx.Err()
	}
	manifest, err := writer.Create("manifest.json")
	if err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := json.NewEncoder(manifest).Encode(map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339), "subject": job.Scope.Subject, "materialCount": job.Count, "files": job.Items}); err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	archive := filepath.Join(dir, job.ID+".zip")
	if err := os.Rename(temporary, archive); err != nil {
		return "", err
	}
	return archive, nil
}

func (w *MaterialDownloadWorker) cleanup() {
	paths, err := w.service.ExpiredMaterialArchives()
	if err != nil {
		return
	}
	for _, file := range paths {
		stored, err := secureMaterialArchivePath(w.root, file)
		if os.IsNotExist(err) {
			_ = w.service.AcknowledgeMaterialArchiveRemoval(file)
		} else if err == nil {
			if removeErr := os.Remove(stored); removeErr == nil || os.IsNotExist(removeErr) {
				_ = w.service.AcknowledgeMaterialArchiveRemoval(file)
			}
		}
	}
}
