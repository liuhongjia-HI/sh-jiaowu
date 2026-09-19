package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func materialSyncFixture(t *testing.T) (*MemoryStore, learning.Principal, learning.Course, learning.Course, learning.Course, learning.Material) {
	t.Helper()
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	source, ok := store.findCourse("course-g05-english-s1-q1")
	if !ok {
		t.Fatal("missing source course")
	}
	createTarget := func(name, spaceID, suffix string) learning.Course {
		course, createErr := store.CreateCourse("超级管理员", admin, learning.CourseUpsertRequest{
			Name: name, LearningSpaceID: spaceID, Status: learning.StatusEnabled,
			Curriculum: []learning.CurriculumNode{
				{ID: suffix + "-unit", Type: learning.CurriculumUnit, Name: source.Curriculum[0].Name, SortOrder: 1},
				{ID: suffix + "-chapter", ParentID: suffix + "-unit", Type: learning.CurriculumChapter, Name: source.Curriculum[1].Name, SortOrder: 1},
				{ID: suffix + "-lesson", ParentID: suffix + "-chapter", Type: learning.CurriculumLesson, Name: source.Curriculum[2].Name, SortOrder: 1},
			},
		})
		if createErr != nil {
			t.Fatalf("create target: %v", createErr)
		}
		return course
	}
	targetSPlus := createTarget("五年级英文 S+", "space-g05-english-s1-q1-splus", "splus")
	targetH := createTarget("五年级英文 H", "space-g05-english-s1-q1-h", "h")
	sourceMaterial, err := store.CreateMaterial("超级管理员", admin, learning.MaterialUploadRequest{
		Title: "第一课新版讲义", CourseID: source.ID, LessonID: source.Curriculum[2].ID, TagCode: "HD", AllowDownload: true,
		File: learning.FileAsset{ID: "sync-file-1", FileName: "HD.pdf", FileType: "PDF", PreviewStatus: "可预览"},
	})
	if err != nil {
		t.Fatalf("create source material: %v", err)
	}
	return store, admin, source, targetSPlus, targetH, sourceMaterial
}

func TestMaterialSyncCreatesMultipleTargetsAndIsIdempotent(t *testing.T) {
	store, admin, source, targetSPlus, targetH, sourceMaterial := materialSyncFixture(t)
	req := learning.MaterialSyncRequest{
		SourceCourseID: source.ID, SourceLessonID: source.Curriculum[2].ID, MaterialIDs: []string{sourceMaterial.ID},
		Targets: []learning.MaterialSyncTarget{{CourseID: targetSPlus.ID, LessonID: "splus-lesson"}, {CourseID: targetH.ID, LessonID: "h-lesson"}},
	}
	preview, err := store.PreviewMaterialSync(admin, req)
	if err != nil {
		t.Fatalf("preview sync: %v", err)
	}
	if len(preview.Targets) != 2 || preview.Targets[0].Items[0].Action != "create" {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	req.Snapshot = preview.Snapshot
	result, err := store.SyncMaterials("超级管理员", admin, req)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(result.Targets) != 2 || result.Targets[0].Created != 1 || result.Targets[1].Created != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, target := range []learning.Course{targetSPlus, targetH} {
		matches := store.materialSlotMatches(target.ID, map[string]string{targetSPlus.ID: "splus-lesson", targetH.ID: "h-lesson"}[target.ID], "HD")
		if len(matches) != 1 || matches[0].FileID != sourceMaterial.FileID || matches[0].ID == sourceMaterial.ID || matches[0].ViewCount != 0 {
			t.Fatalf("unexpected synced material: %#v", matches)
		}
	}
	noticeCount := len(store.notices)
	repeated, err := store.SyncMaterials("超级管理员", admin, req)
	if err != nil || !repeated.AlreadySynced {
		t.Fatalf("repeat should be idempotent, result=%#v err=%v", repeated, err)
	}
	if len(store.notices) != noticeCount {
		t.Fatal("idempotent retry must not create notices")
	}
}

func TestMaterialSyncAddsAlongsideExistingSameTag(t *testing.T) {
	store, admin, source, target, _, sourceMaterial := materialSyncFixture(t)
	existing, err := store.CreateMaterial("超级管理员", admin, learning.MaterialUploadRequest{
		Title: "旧讲义", CourseID: target.ID, LessonID: "splus-lesson", TagCode: "HD",
		File: learning.FileAsset{ID: "old-file", FileName: "old.pdf", FileType: "PDF"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for index := range store.materials {
		if store.materials[index].ID == existing.ID {
			store.materials[index].ViewCount = 17
			store.materials[index].SortOrder = 9
		}
	}
	req := learning.MaterialSyncRequest{SourceCourseID: source.ID, SourceLessonID: source.Curriculum[2].ID, MaterialIDs: []string{sourceMaterial.ID}, Targets: []learning.MaterialSyncTarget{{CourseID: target.ID, LessonID: "splus-lesson"}}}
	preview, err := store.PreviewMaterialSync(admin, req)
	if err != nil || preview.Targets[0].Items[0].Action != "create" {
		t.Fatalf("preview create: %#v %v", preview, err)
	}
	req.Snapshot = preview.Snapshot
	result, err := store.SyncMaterials("超级管理员", admin, req)
	if err != nil || result.Targets[0].Created != 1 || result.Targets[0].Replaced != 0 {
		t.Fatalf("create alongside existing material: %#v %v", result, err)
	}
	matches := store.materialSlotMatches(target.ID, "splus-lesson", "HD")
	if len(matches) != 2 {
		t.Fatalf("both same-tag materials should remain: %#v", matches)
	}
	var keptExisting, addedSource bool
	for _, match := range matches {
		keptExisting = keptExisting || (match.ID == existing.ID && match.FileID == "old-file" && match.ViewCount == 17 && match.SortOrder == 9)
		addedSource = addedSource || (match.ID != existing.ID && match.FileID == sourceMaterial.FileID)
	}
	if !keptExisting || !addedSource {
		t.Fatalf("sync should preserve the existing item and add the source item: %#v", matches)
	}
}

func TestMaterialSyncValidatesScopeAndDuplicateTargets(t *testing.T) {
	store, admin, source, target, _, sourceMaterial := materialSyncFixture(t)
	base := learning.MaterialSyncRequest{SourceCourseID: source.ID, SourceLessonID: source.Curriculum[2].ID, MaterialIDs: []string{sourceMaterial.ID}}
	base.Targets = []learning.MaterialSyncTarget{{CourseID: target.ID, LessonID: "splus-lesson"}, {CourseID: target.ID, LessonID: "splus-lesson"}}
	if _, err := store.PreviewMaterialSync(admin, base); err == nil || !strings.Contains(err.Error(), "只能选择一次") {
		t.Fatalf("duplicate target should fail, got %v", err)
	}
	other, err := store.CreateCourse("超级管理员", admin, learning.CourseUpsertRequest{Name: "六年级英文", LearningSpaceID: "space-g06-english-s1-q1", Status: learning.StatusEnabled, Curriculum: []learning.CurriculumNode{
		{ID: "other-unit", Type: learning.CurriculumUnit, Name: "Unit 1"},
		{ID: "other-chapter", ParentID: "other-unit", Type: learning.CurriculumChapter, Name: "Chapter 1"},
		{ID: "other-lesson", ParentID: "other-chapter", Type: learning.CurriculumLesson, Name: "基础巩固"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	base.Targets = []learning.MaterialSyncTarget{{CourseID: other.ID, LessonID: "other-lesson"}}
	if _, err := store.PreviewMaterialSync(admin, base); err == nil || !strings.Contains(err.Error(), "年级") {
		t.Fatalf("different scope should fail, got %v", err)
	}
}

func TestMaterialSyncKeepsSharedFileAndStudentAccessScopedByMaterial(t *testing.T) {
	store, admin, source, target, _, sourceMaterial := materialSyncFixture(t)
	req := learning.MaterialSyncRequest{SourceCourseID: source.ID, SourceLessonID: source.Curriculum[2].ID, MaterialIDs: []string{sourceMaterial.ID}, Targets: []learning.MaterialSyncTarget{{CourseID: target.ID, LessonID: "splus-lesson"}}}
	preview, err := store.PreviewMaterialSync(admin, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Snapshot = preview.Snapshot
	result, err := store.SyncMaterials("超级管理员", admin, req)
	if err != nil {
		t.Fatal(err)
	}
	targetMaterialID := result.Targets[0].MaterialIDs[0]
	store.packages = append(store.packages, learning.Package{ID: "pkg-sync-student", Name: "S+ 讲义", Status: learning.StatusEnabled})
	store.packageSpaces = append(store.packageSpaces, packageSpace{PackageID: "pkg-sync-student", LearningSpaceID: target.LearningSpaceID})
	store.contentTypes = append(store.contentTypes, packageContentType{PackageID: "pkg-sync-student", ContentType: "handout"})
	store.grants = append(store.grants, packageGrant{ID: "grant-sync-student", StudentID: "stu-001", PackageID: "pkg-sync-student", StartsAt: "2020-01-01", EndsAt: "2099-12-31", Status: "active"})
	store.spaceAccess = append(store.spaceAccess, learningSpaceAccess{StudentID: "stu-001", LearningSpaceID: target.LearningSpaceID, PackageGrantID: "grant-sync-student", StartsAt: "2020-01-01", EndsAt: "2099-12-31", Status: "active"})
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	if material, err := store.StudentMaterial(student, targetMaterialID); err != nil || material.FileID != sourceMaterial.FileID {
		t.Fatalf("student should access target material: %#v %v", material, err)
	}
	if err := store.DeleteMaterial("超级管理员", admin, targetMaterialID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StudentMaterial(student, targetMaterialID); err == nil {
		t.Fatal("deleted target material must no longer grant student access")
	}
	if _, exists := store.fileAssets[sourceMaterial.FileID]; !exists {
		t.Fatal("deleting one shared material must keep the file asset")
	}
	if matches := store.materialSlotMatches(source.ID, source.Curriculum[2].ID, sourceMaterial.TagCode); len(matches) != 1 || matches[0].FileID != sourceMaterial.FileID {
		t.Fatalf("source material must remain available: %#v", matches)
	}
}

func TestMaterialSyncRollsBackWholeBatchWhenTargetChanges(t *testing.T) {
	store, admin, source, firstTarget, secondTarget, sourceMaterial := materialSyncFixture(t)
	req := learning.MaterialSyncRequest{
		SourceCourseID: source.ID, SourceLessonID: source.Curriculum[2].ID, MaterialIDs: []string{sourceMaterial.ID},
		Targets: []learning.MaterialSyncTarget{{CourseID: firstTarget.ID, LessonID: "splus-lesson"}, {CourseID: secondTarget.ID, LessonID: "h-lesson"}},
	}
	preview, err := store.PreviewMaterialSync(admin, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Snapshot = preview.Snapshot
	for index := range store.courses {
		if store.courses[index].ID == secondTarget.ID {
			store.courses[index].Curriculum = store.courses[index].Curriculum[:2]
		}
	}
	if _, err := store.SyncMaterials("超级管理员", admin, req); err == nil {
		t.Fatal("changed second target should reject the batch")
	}
	if matches := store.materialSlotMatches(firstTarget.ID, "splus-lesson", "HD"); len(matches) != 0 {
		t.Fatalf("first target must remain unchanged after batch failure: %#v", matches)
	}
}
