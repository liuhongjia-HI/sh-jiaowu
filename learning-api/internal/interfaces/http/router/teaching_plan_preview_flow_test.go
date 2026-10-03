package router_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/handler"
)

func TestTeachingPlanRealWordConversion(t *testing.T) {
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Skip("LibreOffice required for actual Word conversion")
	}
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Independent Word teaching plan</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="12240" w:h="15840"/></w:sectPr></w:body></w:document>`,
	}
	for name, content := range parts {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	verifyTeachingPlanOfficeConversion(t, "备课.docx", archive.Bytes())
}

func verifyTeachingPlanOfficeConversion(t *testing.T, filename string, original []byte) []byte {
	t.Helper()
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, _ := app.store.PrincipalByUserID("user-super")
	var plan learning.TeachingPlan
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", token, map[string]string{"grade": "五年级", "subject": "英文"}, "file", filename, original, http.StatusOK, &plan)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan struct{})
	go func() { handler.NewPreviewWorker(learningapp.NewService(app.store), root).Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	for {
		ready, err := app.store.TeachingPlan(p, plan.ID)
		if err != nil {
			t.Fatal(err)
		}
		if ready.PreviewStatus == "转换失败" {
			t.Fatalf("Office conversion failed: %#v", ready)
		}
		if ready.PreviewStatus == "可预览" {
			plan = ready
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Office conversion timeout")
		case <-time.After(50 * time.Millisecond):
		}
	}
	var previewBytes []byte
	for _, path := range []string{plan.PreviewURL, plan.DownloadURL} {
		request, _ := http.NewRequest(http.MethodGet, app.server.URL+path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("file response: %d %v", response.StatusCode, err)
		}
		if path == plan.PreviewURL {
			previewBytes = body
			if !bytes.HasPrefix(body, []byte("%PDF-")) || !bytes.Contains(body, []byte("%%EOF")) || len(body) < 1000 {
				t.Fatal("Office preview is not a complete PDF")
			}
			if directory := os.Getenv("STARLINE_PREVIEW_TEST_OUTPUT"); directory != "" {
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, filename+".preview.pdf"), body, 0644); err != nil {
					t.Fatal(err)
				}
			}
		} else if !bytes.Equal(body, original) {
			t.Fatal("Office original was replaced by preview")
		}
	}
	if _, err := exec.LookPath("gs"); err == nil {
		asset, err := app.store.TeachingPlanFile(p, plan.ID)
		if err != nil || asset.PreviewPageCount < 1 {
			t.Fatalf("page images missing: %#v %v", asset, err)
		}
		for page := 1; page <= asset.PreviewPageCount; page++ {
			file, err := os.Open(filepath.Join(asset.PreviewPageDir, fmt.Sprintf("page-%04d.jpg", page)))
			if err != nil {
				t.Fatal(err)
			}
			config, kind, err := image.DecodeConfig(file)
			file.Close()
			if err != nil || kind != "jpeg" || config.Width <= 0 || config.Height <= 0 {
				t.Fatalf("invalid page %d image: %s %#v %v", page, kind, config, err)
			}
			if directory := os.Getenv("STARLINE_PREVIEW_TEST_OUTPUT"); directory != "" {
				contents, err := os.ReadFile(filepath.Join(asset.PreviewPageDir, fmt.Sprintf("page-%04d.jpg", page)))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%s.page-%04d.jpg", filename, page)), contents, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return previewBytes
}

func TestTeachingPlanRealLegacyOfficeConversion(t *testing.T) {
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Skip("LibreOffice required")
	}
	for _, tc := range []struct {
		ext, filter, source string
		pages               int
	}{
		{ext: ".doc", filter: "doc:MS Word 97", source: "legacy.rtf", pages: 3},
		{ext: ".ppt", filter: "ppt:MS PowerPoint 97", source: "teaching-plan.pptx"},
	} {
		t.Run(tc.ext, func(t *testing.T) {
			root := t.TempDir()
			var original []byte
			if tc.ext == ".doc" {
				original = []byte(`{\rtf1\ansi\deff0 {\fonttbl {\f0 Arial;}}\f0\fs32 Legacy first page\page Legacy second page\page Legacy third page}`)
			} else {
				var err error
				original, err = os.ReadFile("testdata/" + tc.source)
				if err != nil {
					t.Fatal(err)
				}
			}
			source := filepath.Join(root, tc.source)
			if err := os.WriteFile(source, original, 0600); err != nil {
				t.Fatal(err)
			}
			outputDir := filepath.Join(root, "legacy")
			if err := os.Mkdir(outputDir, 0700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "soffice", "-env:UserInstallation=file://"+filepath.ToSlash(filepath.Join(root, "profile")), "--headless", "--convert-to", tc.filter, "--outdir", outputDir, source)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("legacy fixture conversion: %v %s", err, output)
			}
			legacy, err := os.ReadFile(filepath.Join(outputDir, strings.TrimSuffix(tc.source, filepath.Ext(tc.source))+tc.ext))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(legacy, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
				t.Fatal("fixture is not a binary Office document")
			}
			preview := verifyTeachingPlanOfficeConversion(t, "旧版备课"+tc.ext, legacy)
			if tc.pages > 0 {
				info := exec.CommandContext(ctx, "pdfinfo", "-")
				info.Stdin = bytes.NewReader(preview)
				output, err := info.CombinedOutput()
				if err != nil || !regexp.MustCompile(fmt.Sprintf(`(?m)^Pages:\s+%d\s*$`, tc.pages)).Match(output) {
					t.Fatalf("converted page count: %v %s", err, output)
				}
			}
		})
	}
}

func TestTeachingPlanRealPPTConversion(t *testing.T) {
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Skip("LibreOffice required for actual PPT conversion")
	}
	contents, err := os.ReadFile("testdata/teaching-plan.pptx")
	if err != nil {
		t.Fatal(err)
	}
	verifyTeachingPlanOfficeConversion(t, "备课.pptx", contents)
}

func TestTeachingPlanRealNearLimitOfficeConversion(t *testing.T) {
	directory := os.Getenv("STARLINE_LARGE_OFFICE_FIXTURE_DIR")
	if directory == "" {
		t.Skip("explicit large Office fixture directory required")
	}
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Fatal("actual LibreOffice required for large Office verification")
	}
	for _, extension := range []string{"docx", "pptx"} {
		t.Run(extension, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join(directory, "large-teaching-plan."+extension))
			if err != nil {
				t.Fatal(err)
			}
			if len(contents) <= 48*1024*1024 || len(contents) >= 50*1024*1024 {
				t.Fatalf("fixture is outside near-limit range: %d", len(contents))
			}
			preview := verifyTeachingPlanOfficeConversion(t, "大体积备课."+extension, contents)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "pdfinfo", "-")
			command.Stdin = bytes.NewReader(preview)
			output, err := command.CombinedOutput()
			if err != nil || !regexp.MustCompile(`(?m)^Pages:\s+2\s*$`).Match(output) {
				t.Fatalf("large Office lost pages: %s %v", output, err)
			}
			t.Logf("actual Office upload and conversion: original=%d bytes preview=%d bytes pages=2", len(contents), len(preview))
		})
	}
}

func TestTeachingPlanHTTPRejectsOversizeAndAcceptsNextFile(t *testing.T) {
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	oversize := make([]byte, 51*1024*1024)
	copy(oversize, "%PDF-1.4\n")
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", token, map[string]string{"grade": "五年级", "subject": "英文"}, "file", "超大备课.pdf", oversize, http.StatusBadRequest, nil)
	var list learning.TeachingPlanList
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", token, nil, http.StatusOK, &list)
	if len(list.Plans) != 0 {
		t.Fatal("oversize file created a plan")
	}
	if files, err := os.ReadDir(filepath.Join(root, "original")); err == nil && len(files) > 0 {
		t.Fatal("oversize original saved")
	}
	var accepted learning.TeachingPlan
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", token, map[string]string{"grade": "五年级", "subject": "英文"}, "file", "正常备课.pdf", teachingPlanPDF("Small file after oversized rejection"), http.StatusOK, &accepted)
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", token, nil, http.StatusOK, &list)
	if len(list.Plans) != 1 || list.Plans[0].ID != accepted.ID || list.Plans[0].FileName != "正常备课.pdf" {
		t.Fatalf("next valid file not retained independently: %#v", list)
	}
}

func TestTeachingPlanRealBrokenWordReasonAndRetry(t *testing.T) {
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Skip("LibreOffice required")
	}
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, data := range map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<broken document`,
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	original := archive.Bytes()
	var plan learning.TeachingPlan
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", token, map[string]string{"grade": "五年级", "subject": "英文"}, "file", "损坏备课.docx", original, http.StatusOK, &plan)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan struct{})
	go func() { handler.NewPreviewWorker(learningapp.NewService(app.store), root).Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	for {
		app.doJSON(t, http.MethodGet, "/api/teaching-plans/"+plan.ID, token, nil, http.StatusOK, &plan)
		if plan.PreviewStatus == "转换失败" {
			break
		}
		select {
		case <-ctx.Done():
			p, _ := app.store.PrincipalByUserID("user-super")
			asset, err := app.store.TeachingPlanFile(p, plan.ID)
			t.Fatalf("damaged document did not fail: status=%s reason=%s err=%v", asset.PreviewStatus, asset.PreviewError, err)
		case <-time.After(time.Second):
		}
	}
	if !strings.Contains(plan.PreviewError, "请检查") || strings.Contains(plan.PreviewError, "LibreOffice") {
		t.Fatalf("missing actionable reason: %#v", plan)
	}
	var list learning.TeachingPlanList
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", token, nil, http.StatusOK, &list)
	if len(list.Plans) != 1 || list.Plans[0].PreviewError != plan.PreviewError || len(list.UnreadPlanIDs) != 1 {
		t.Fatalf("list lost reason or consumed unread: %#v", list)
	}
	app.doJSON(t, http.MethodGet, plan.PreviewURL, token, nil, http.StatusBadRequest, nil)
	request, _ := http.NewRequest(http.MethodGet, app.server.URL+plan.DownloadURL, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(body, original) {
		t.Fatal("failed conversion damaged original download")
	}
	app.doJSON(t, http.MethodGet, "/api/teaching-plans/"+plan.ID, app.loginStudent(t), nil, http.StatusForbidden, nil)
	cancel()
	<-done
	app.doJSON(t, http.MethodPost, "/api/teaching-plans/"+plan.ID+"/preview/retry", token, map[string]any{}, http.StatusOK, nil)
	id := plan.ID
	plan = learning.TeachingPlan{}
	app.doJSON(t, http.MethodGet, "/api/teaching-plans/"+id, token, nil, http.StatusOK, &plan)
	if plan.PreviewStatus != "待转换" || plan.PreviewError != "" {
		t.Fatalf("retry retained error: %#v", plan)
	}
}

// A complete single-page PDF with xref offsets, rather than a PDF header stub.
func teachingPlanPDF(text string) []byte {
	stream := "BT /F1 18 Tf 50 740 Td (" + text + ") Tj ET\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return out.Bytes()
}

func TestTeachingPlanRealPDFPreviewIsolationAndMissingFileRecovery(t *testing.T) {
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, _ := app.store.PrincipalByUserID("user-super")
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "真实教案预览验收", LearningSpaceID: "space-g05-english-s1-q1", Status: learning.StatusEnabled, Curriculum: []learning.CurriculumNode{{ID: "preview-u1", Type: learning.CurriculumUnit, Name: "第一课", SortOrder: 1}, {ID: "preview-u2", Type: learning.CurriculumUnit, Name: "第二课", SortOrder: 2}}}, http.StatusOK, &course)
	plans := make([]learning.TeachingPlan, 2)
	contents := [][]byte{teachingPlanPDF("First independent teaching plan"), teachingPlanPDF("Second independent teaching plan")}
	for i := range plans {
		doMultipart(t, app, http.MethodPost, "/api/teaching-plans", token, map[string]string{"grade": course.Grade, "subject": course.Subject, "courseId": course.ID, "lessonId": fmt.Sprintf("preview-u%d", i+1)}, "file", fmt.Sprintf("教案%d.pdf", i+1), contents[i], http.StatusOK, &plans[i])
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	done := make(chan struct{})
	go func() { handler.NewPreviewWorker(learningapp.NewService(app.store), root).Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	waitReady := func(id string) learning.TeachingPlan {
		for {
			plan, err := app.store.TeachingPlan(p, id)
			if err != nil {
				t.Fatal(err)
			}
			if plan.PreviewStatus == "可预览" {
				return plan
			}
			if plan.PreviewStatus == "转换失败" {
				t.Fatalf("preview failed: %#v", plan)
			}
			select {
			case <-ctx.Done():
				t.Fatal("preview did not complete")
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	getFile := func(path string, expected []byte) {
		request, _ := http.NewRequest(http.MethodGet, app.server.URL+path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(body, expected) {
			t.Fatalf("file mismatch: status=%d size=%d err=%v", response.StatusCode, len(body), err)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("private document cache policy missing")
		}
	}
	for i := range plans {
		plans[i] = waitReady(plans[i].ID)
		if plans[i].LessonID != fmt.Sprintf("preview-u%d", i+1) {
			t.Fatal("chapter changed while converting")
		}
		getFile(plans[i].PreviewURL, contents[i])
		getFile(plans[i].DownloadURL, contents[i])
		app.doJSON(t, http.MethodPost, "/api/teaching-plans/"+plans[i].ID+"/view", token, learning.TeachingPlanReadRequest{Version: plans[i].ReadVersion}, http.StatusOK, nil)
	}
	var list learning.TeachingPlanList
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", token, nil, http.StatusOK, &list)
	if len(list.UnreadPlanIDs) != 0 {
		t.Fatal("HTTP reading records did not clear unread plans")
	}
	teacher, _ := app.store.PrincipalByUserID("user-teacher")
	if _, err := app.store.UpdateTeacher("管理员", p, teacher.UserID, learning.TeacherUpsertRequest{Name: teacher.Name, Phone: teacher.Phone, CampusID: "campus-main", AccountStatus: "正常", LearningSpaceIDs: []string{course.LearningSpaceID}, TeacherLibrary: &learning.TeacherLibraryPolicy{SpaceIDs: []string{course.LearningSpaceID}, CanManageCourses: false}}); err != nil {
		t.Fatal(err)
	}
	readOnlyToken := app.loginAdmin(t, "13800000004")
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", readOnlyToken, nil, http.StatusOK, &list)
	if len(list.UnreadPlanIDs) != 2 {
		t.Fatal("another user's read state consumed teacher unread plans")
	}
	app.doJSON(t, http.MethodPost, "/api/teaching-plans/"+plans[0].ID+"/view", readOnlyToken, learning.TeachingPlanReadRequest{Version: plans[0].ReadVersion}, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/teaching-plans", readOnlyToken, nil, http.StatusOK, &list)
	if len(list.UnreadPlanIDs) != 1 || list.UnreadPlanIDs[0] != plans[1].ID {
		t.Fatal("read-only teacher unread update failed")
	}
	app.doJSON(t, http.MethodPost, "/api/teaching-plans/"+plans[0].ID+"/view", app.loginStudent(t), learning.TeachingPlanReadRequest{Version: plans[0].ReadVersion}, http.StatusForbidden, nil)
	asset, err := app.store.TeachingPlanFile(p, plans[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(asset.PreviewPath); err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodGet, plans[0].PreviewURL, token, nil, http.StatusBadRequest, nil)
	getFile(plans[1].PreviewURL, contents[1])
	app.doJSON(t, http.MethodPost, "/api/teaching-plans/"+plans[0].ID+"/preview/retry", token, map[string]any{}, http.StatusOK, nil)
	ready := waitReady(plans[0].ID)
	getFile(ready.PreviewURL, contents[0])
}

func teachingPlanLargeImagePDF() []byte {
	const width, height = 4096, 4266
	pixels := make([]byte, width*height*3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			pixels[offset] = byte(x * 255 / width)
			pixels[offset+1] = byte(y * 255 / height)
			pixels[offset+2] = 128
		}
	}
	stream := "BT /F1 18 Tf 40 750 Td (Near-limit teaching plan: large image) Tj ET\nq 384 0 0 400 40 100 cm /Im0 Do Q\n"
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> /XObject << /Im0 6 0 R >> >> /Contents 5 0 R >>"),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
		[]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream)),
	}
	var out bytes.Buffer
	out.Grow(len(pixels) + 2048)
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(object)
		out.WriteString("\nendobj\n")
	}
	offsets = append(offsets, out.Len())
	fmt.Fprintf(&out, "6 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length %d >>\nstream\n", width, height, len(pixels))
	out.Write(pixels)
	out.WriteString("\nendstream\nendobj\n")
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return out.Bytes()
}

func TestTeachingPlanNearLimitPDFOverTCPPreservesOriginalAndPreview(t *testing.T) {
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, _ := app.store.PrincipalByUserID("user-super")
	server := httptest.NewServer(app.server.transport.handler)
	defer server.Close()
	app.server.URL = server.URL
	http.DefaultClient.Transport = server.Client().Transport
	contents := teachingPlanLargeImagePDF()
	if len(contents) <= 49*1024*1024 || len(contents) >= 50*1024*1024 {
		t.Fatalf("invalid near-limit fixture size: %d", len(contents))
	}
	expectedHash := sha256.Sum256(contents)
	var plan learning.TeachingPlan
	doMultipart(t, app, http.MethodPost, "/api/teaching-plans", token, map[string]string{"grade": "五年级", "subject": "英文"}, "file", "近上限备课.pdf", contents, http.StatusOK, &plan)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { handler.NewPreviewWorker(learningapp.NewService(app.store), root).Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	for {
		current, err := app.store.TeachingPlan(p, plan.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.PreviewStatus == "可预览" {
			plan = current
			break
		}
		if current.PreviewStatus == "转换失败" {
			t.Fatalf("large PDF failed: %#v", current)
		}
		select {
		case <-ctx.Done():
			t.Fatal("large PDF preview timeout")
		case <-time.After(50 * time.Millisecond):
		}
	}
	for _, target := range []string{plan.PreviewURL, plan.DownloadURL} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+target, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.New()
		size, err := io.Copy(hash, response.Body)
		closeErr := response.Body.Close()
		if err != nil || closeErr != nil || response.StatusCode != http.StatusOK || size != int64(len(contents)) || !bytes.Equal(hash.Sum(nil), expectedHash[:]) {
			t.Fatalf("large file incomplete: %d %d %v %v", response.StatusCode, size, err, closeErr)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("private cache policy missing")
		}
	}
	asset, err := app.store.TeachingPlanFile(p, plan.ID)
	if err != nil || asset.FileSize != int64(len(contents)) {
		t.Fatal("incorrect stored size", err)
	}
	if _, err := exec.LookPath("gs"); err == nil {
		if asset.PreviewPageCount != 1 {
			t.Fatalf("near-limit PDF missing page image: %#v", asset)
		}
		file, err := os.Open(filepath.Join(asset.PreviewPageDir, "page-0001.jpg"))
		if err != nil {
			t.Fatal(err)
		}
		config, kind, err := image.DecodeConfig(file)
		file.Close()
		if err != nil || kind != "jpeg" || config.Width <= 0 || config.Height <= 0 {
			t.Fatal("near-limit PDF page image invalid", err)
		}
		if directory := os.Getenv("STARLINE_PREVIEW_TEST_OUTPUT"); directory != "" {
			contents, err := os.ReadFile(filepath.Join(asset.PreviewPageDir, "page-0001.jpg"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "近上限备课.page-0001.jpg"), contents, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := exec.LookPath("pdfinfo"); err == nil {
		output, err := exec.CommandContext(ctx, "pdfinfo", asset.PreviewPath).CombinedOutput()
		if err != nil || !regexp.MustCompile(`(?m)^Pages:\s+1\s*$`).Match(output) {
			t.Fatalf("invalid large PDF: %s %v", output, err)
		}
		t.Log("Poppler parsed the large image PDF as one page")
	}
	if directory := os.Getenv("STARLINE_PREVIEW_TEST_OUTPUT"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "近上限备课.preview.pdf"), contents, 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("actual TCP upload, preview and download verified: %d bytes", len(contents))
}
