package handler

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"starline/learning-api/internal/application/learningapp"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestMaterialDownloadCancellationDuringSingleFileLeavesNoArchive(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "large.pdf")
	body := make([]byte, 32*1024*1024)
	if _, err := rand.Read(body); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, body, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		path string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		path, err := NewMaterialDownloadWorker(nil, root).generate(ctx, learning.MaterialDownloadJob{ID: "download-mid-copy", Count: 1}, []learning.MaterialDownloadFile{{Name: "course/large.pdf", Path: source, Size: int64(len(body))}})
		done <- result{path, err}
	}()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case value := <-done:
			t.Fatalf("copy completed before mid-file cancellation: %s %v", value.path, value.err)
		case <-deadline.C:
			cancel()
			<-done
			t.Fatal("archive never began writing")
		case <-ticker.C:
			files, _ := filepath.Glob(filepath.Join(root, "material-downloads", "*.tmp"))
			if len(files) == 0 {
				continue
			}
			info, err := os.Stat(files[0])
			if err != nil || info.Size() < 64*1024 {
				continue
			}
			cancel()
			value := <-done
			if value.path != "" || !errors.Is(value.err, context.Canceled) {
				t.Fatalf("cancelled copy advertised complete archive: %s %v", value.path, value.err)
			}
			entries, err := os.ReadDir(filepath.Join(root, "material-downloads"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("cancel left files: %#v %v", entries, err)
			}
			return
		}
	}
}

type archiveCleanupRepository struct {
	learningapp.Repository
	paths                []string
	acknowledged         []string
	failAcknowledgements int
}

func (r *archiveCleanupRepository) ExpiredMaterialArchives() ([]string, error) {
	return append([]string(nil), r.paths...), nil
}
func (r *archiveCleanupRepository) AcknowledgeMaterialArchiveRemoval(path string) error {
	if r.failAcknowledgements > 0 {
		r.failAcknowledgements--
		return errors.New("temporary persistence failure")
	}
	r.acknowledged = append(r.acknowledged, path)
	for i, value := range r.paths {
		if value == path {
			r.paths = append(r.paths[:i], r.paths[i+1:]...)
			break
		}
	}
	return nil
}

func TestMaterialDownloadCleanupRetriesAcknowledgementAfterDeletion(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "material-downloads")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(directory, "expired.zip")
	if err := os.WriteFile(archive, []byte("expired"), 0600); err != nil {
		t.Fatal(err)
	}
	repo := &archiveCleanupRepository{paths: []string{archive}, failAcknowledgements: 1}
	worker := NewMaterialDownloadWorker(learningapp.NewService(repo), root)
	worker.cleanup()
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("archive was not removed")
	}
	if len(repo.paths) != 1 {
		t.Fatal("failed persistence lost retry record")
	}
	worker.cleanup()
	worker.cleanup()
	if len(repo.paths) != 0 || len(repo.acknowledged) != 1 {
		t.Fatal("missing archive cleanup did not recover")
	}
}

func TestMaterialDownloadCleanupRetainsFailuresAndRejectsRedirects(t *testing.T) {
	for _, scenario := range []string{"redirected-directory", "linked-archive", "nonempty-directory", "permission-denied"} {
		t.Run(scenario, func(t *testing.T) {
			if scenario == "permission-denied" && os.Geteuid() == 0 {
				t.Skip("root bypasses filesystem permission failure")
			}
			root := t.TempDir()
			directory := filepath.Join(root, "material-downloads")
			archive := filepath.Join(directory, "expired.zip")
			outside := filepath.Join(t.TempDir(), "expired.zip")
			if err := os.WriteFile(outside, []byte("must remain"), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "redirected-directory" {
				if err := os.Symlink(filepath.Dir(outside), directory); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(directory, 0700); err != nil {
					t.Fatal(err)
				}
				if scenario == "linked-archive" {
					// A link within the same archive directory must not delete a
					// different live archive merely because it is inside the root.
					outside = filepath.Join(directory, "live.zip")
					if err := os.WriteFile(outside, []byte("must remain"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(outside, archive); err != nil {
						t.Fatal(err)
					}
				} else if scenario == "nonempty-directory" {
					if err := os.Mkdir(archive, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(archive, "block"), []byte("blocked"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.WriteFile(archive, []byte("expired"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(directory, 0500); err != nil {
						t.Fatal(err)
					}
					defer os.Chmod(directory, 0700)
				}
			}
			repo := &archiveCleanupRepository{paths: []string{archive}}
			worker := NewMaterialDownloadWorker(learningapp.NewService(repo), root)
			worker.cleanup()
			if body, err := os.ReadFile(outside); err != nil || string(body) != "must remain" {
				t.Fatalf("unrelated archive touched: %v %q", err, body)
			}
			if len(repo.paths) != 1 || len(repo.acknowledged) != 0 {
				t.Fatal("failed cleanup lost its retry record")
			}
			if scenario == "redirected-directory" {
				if err := os.Remove(directory); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(directory, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Chmod(directory, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(archive); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(archive, []byte("expired"), 0600); err != nil {
				t.Fatal(err)
			}
			worker.cleanup()
			worker.cleanup()
			if len(repo.paths) != 0 || len(repo.acknowledged) != 1 {
				t.Fatal("cleanup did not recover idempotently")
			}
			if _, err := os.Stat(archive); !os.IsNotExist(err) {
				t.Fatal("expired archive still exists")
			}
		})
	}
}

func TestMaterialDownloadArchiveCompleteContents(t *testing.T) {
	root := t.TempDir()
	files := []learning.MaterialDownloadFile{}
	items := []learning.MaterialDownloadItem{}
	for i, value := range []string{"第一份讲义", "第二份讲义"} {
		source := filepath.Join(root, string(rune('a'+i))+".pdf")
		if err := os.WriteFile(source, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		name := "课程/第一课/" + string(rune('a'+i)) + "_讲义.pdf"
		files = append(files, learning.MaterialDownloadFile{Name: name, Path: source, Size: int64(len(value))})
		items = append(items, learning.MaterialDownloadItem{Name: name, Size: int64(len(value))})
	}
	job := learning.MaterialDownloadJob{ID: "download-test", Count: 2, Scope: learning.MaterialDownloadScope{Subject: "english"}, Items: items}
	archive, err := NewMaterialDownloadWorker(nil, root).generate(context.Background(), job, files)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 3 {
		t.Fatalf("incomplete archive: %d", len(reader.File))
	}
	for i, entry := range reader.File {
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			if entry.Name != files[i].Name || string(data) != []string{"第一份讲义", "第二份讲义"}[i] {
				t.Fatalf("wrong contents: %s %q", entry.Name, data)
			}
		} else {
			var manifest struct {
				MaterialCount int                             `json:"materialCount"`
				Files         []learning.MaterialDownloadItem `json:"files"`
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.MaterialCount != 2 || len(manifest.Files) != 2 || strings.Contains(string(data), root) {
				t.Fatalf("invalid manifest: %s", data)
			}
		}
	}
}

func TestMaterialDownloadRejectsIncompleteOrUnsafeArchive(t *testing.T) {
	for _, scenario := range []string{"missing", "changed-size", "duplicate", "traversal", "outside", "symlink", "count", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.pdf")
			if err := os.WriteFile(source, []byte("abc"), 0600); err != nil {
				t.Fatal(err)
			}
			files := []learning.MaterialDownloadFile{{Name: "课程/a.pdf", Path: source, Size: 3}, {Name: "课程/b.pdf", Path: source, Size: 3}}
			job := learning.MaterialDownloadJob{ID: "download-test", Count: 2}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "missing":
				files[1].Path = filepath.Join(root, "missing.pdf")
			case "changed-size":
				files[1].Size = 4
			case "duplicate":
				files[1].Name = files[0].Name
			case "traversal":
				files[1].Name = "../escape.pdf"
			case "outside", "symlink":
				outside := filepath.Join(t.TempDir(), "private.pdf")
				if err := os.WriteFile(outside, []byte("abc"), 0600); err != nil {
					t.Fatal(err)
				}
				files[1].Path = outside
				if scenario == "symlink" {
					link := filepath.Join(root, "link.pdf")
					if err := os.Symlink(outside, link); err != nil {
						t.Fatal(err)
					}
					files[1].Path = link
				}
			case "count":
				job.Count = 3
			case "cancelled":
				cancel()
			}
			archive, err := NewMaterialDownloadWorker(nil, root).generate(ctx, job, files)
			if err == nil || archive != "" {
				t.Fatalf("unsafe archive accepted: %s %v", archive, err)
			}
			entries, err := os.ReadDir(filepath.Join(root, "material-downloads"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if len(entries) > 0 {
				t.Fatalf("partial files left behind: %#v", entries)
			}
		})
	}
}

func TestMaterialDownloadRejectsRedirectedOutputDirectory(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	source := filepath.Join(root, "source.pdf")
	if err := os.WriteFile(source, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "material-downloads")); err != nil {
		t.Fatal(err)
	}
	archive, err := NewMaterialDownloadWorker(nil, root).generate(context.Background(), learning.MaterialDownloadJob{ID: "download-test", Count: 1}, []learning.MaterialDownloadFile{{Name: "course/a.pdf", Path: source, Size: 3}})
	if err == nil || archive != "" {
		t.Fatalf("redirected output accepted: %s %v", archive, err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside root: %#v %v", entries, err)
	}
}

func TestMaterialDownloadTwoThousandEntriesRemainComplete(t *testing.T) {
	root := t.TempDir()
	files := make([]learning.MaterialDownloadFile, 2000)
	items := make([]learning.MaterialDownloadItem, len(files))
	expected := map[string]string{}
	for i := range files {
		body := fmt.Sprintf("unique material %04d\n", i)
		source := filepath.Join(root, fmt.Sprintf("source-%04d.txt", i))
		if err := os.WriteFile(source, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("课程/章节/资料-%04d.txt", i)
		files[i] = learning.MaterialDownloadFile{Name: name, Path: source, Size: int64(len(body))}
		items[i] = learning.MaterialDownloadItem{MaterialID: fmt.Sprint(i), FileID: fmt.Sprint(i), Name: name, Size: int64(len(body))}
		expected[name] = body
	}
	archive, err := NewMaterialDownloadWorker(nil, root).generate(context.Background(), learning.MaterialDownloadJob{ID: "download-2000-files", Count: len(files), Items: items}, files)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 2001 {
		t.Fatalf("ZIP entry count %d", len(reader.File))
	}
	for _, file := range reader.File {
		entry, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(entry)
		closeErr := entry.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("read: %v %v", err, closeErr)
		}
		if file.Name == "manifest.json" {
			var manifest struct {
				MaterialCount int                             `json:"materialCount"`
				Files         []learning.MaterialDownloadItem `json:"files"`
			}
			if err := json.Unmarshal(body, &manifest); err != nil || manifest.MaterialCount != 2000 || len(manifest.Files) != 2000 {
				t.Fatal("incomplete manifest", err)
			}
			continue
		}
		want, ok := expected[file.Name]
		if !ok || string(body) != want {
			t.Fatal("unexpected or corrupt entry", file.Name)
		}
		delete(expected, file.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("missing %d entries", len(expected))
	}
	entries, err := os.ReadDir(filepath.Join(root, "material-downloads"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "download-2000-files.zip" {
		t.Fatal("temporary files remain", err)
	}
}
