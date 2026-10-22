package store

import (
	"reflect"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func screenshotDirectoryFixture(t *testing.T) (*MemoryStore, learning.Principal, learning.Course, learning.Course, learning.CourseDirectorySyncRequest) {
	t.Helper()
	s, p, source, target := directorySyncFixture(t)
	source.Curriculum = []learning.CurriculumNode{
		{ID: "src-unit", Type: learning.CurriculumUnit, Name: "Fertile Crescent", SortOrder: 2},
		{ID: "src-chapter", ParentID: "src-unit", Type: learning.CurriculumChapter, Name: "The Sumerians", SortOrder: 2},
	}
	target.Curriculum = []learning.CurriculumNode{
		{ID: "old-unit", Type: learning.CurriculumUnit, SortOrder: 2},
		{ID: "old-chapter", ParentID: "old-unit", Type: learning.CurriculumChapter, Name: "The Sumerians", SortOrder: 2},
	}
	s.courses[findCourseIndex(s.courses, source.ID)].Curriculum = source.Curriculum
	s.courses[findCourseIndex(s.courses, target.ID)].Curriculum = target.Curriculum
	s.materials = append(s.materials,
		learning.Material{ID: "source-published", CourseID: source.ID, LessonID: "src-chapter", Title: "文明起源", TagCode: "HD", FileID: "published-file", FileName: "文明.pdf", Status: learning.StatusEnabled, PublishStatus: "已发布"},
		learning.Material{ID: "source-draft", CourseID: source.ID, LessonID: "src-chapter", Title: "草稿", TagCode: "HD", Status: learning.StatusDraft},
		learning.Material{ID: "target-own", CourseID: target.ID, LessonID: "old-chapter", Title: "目标自有讲义", TagCode: "HD", FileID: "own-file", ViewCount: 23, Status: learning.StatusEnabled},
	)
	s.fileAssets["published-file"] = learning.FileAsset{ID: "published-file", FileName: "文明.pdf"}
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID}, UnitIDs: []string{"src-unit"}}
	return s, p, source, target, req
}

func previewDirectory(t *testing.T, s *MemoryStore, p learning.Principal, req *learning.CourseDirectorySyncRequest) learning.CourseDirectorySyncTarget {
	t.Helper()
	result, err := s.PreviewCourseDirectorySync(p, *req)
	if err != nil || len(result.Targets) != 1 || result.Targets[0].Error != "" {
		t.Fatalf("preview: %#v %v", result, err)
	}
	req.Snapshots = map[string]string{result.Targets[0].CourseID: result.Targets[0].Snapshot}
	return result.Targets[0]
}
func executeDirectory(t *testing.T, s *MemoryStore, p learning.Principal, req learning.CourseDirectorySyncRequest) {
	t.Helper()
	result, err := s.SyncCourseDirectory("管理员", p, req)
	if err != nil || result.Targets[0].Status != "已同步" {
		t.Fatalf("execute: %#v %v", result, err)
	}
}
func directoryMaterial(s *MemoryStore, id string) learning.Material {
	for _, m := range s.materials {
		if m.ID == id {
			return m
		}
	}
	return learning.Material{}
}

func TestDirectorySyncSameNumberRequiresExplicitPreview(t *testing.T) {
	s, p, source, target, req := screenshotDirectoryFixture(t)
	before, _ := s.findCourse(target.ID)
	row := previewDirectory(t, s, p, &req)
	if len(row.Conflicts) != 1 || row.Conflicts[0].Resolved || row.Snapshot != "" {
		t.Fatalf("silent same-number merge: %#v", row)
	}
	req.Snapshots[target.ID] = "forged"
	result, err := s.SyncCourseDirectory("管理员", p, req)
	if err != nil || result.Targets[0].Error == "" {
		t.Fatal("unresolved conflict written")
	}
	after, _ := s.findCourse(target.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("preview or rejection mutated course")
	}
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"src-unit": {TargetNodeID: "old-unit"}}}
	row = previewDirectory(t, s, p, &req)
	if len(row.MaterialsAdded) != 1 {
		t.Fatalf("published-only selection failed: %#v", row)
	}
	executeDirectory(t, s, p, req)
	got, _ := s.findCourse(target.ID)
	if len(got.Curriculum) != 2 || got.Curriculum[0].ID != "old-unit" || got.Curriculum[0].Name != source.Curriculum[0].Name || got.DirectorySyncMap[source.ID+":src-chapter"] != "old-chapter" {
		t.Fatalf("lost existing identity: %#v", got)
	}
	own := directoryMaterial(s, "target-own")
	if own.FileID != "own-file" || own.ViewCount != 23 || own.LessonID != "old-chapter" {
		t.Fatal("overwrote own material")
	}
	logs, count := len(s.logs), len(s.materials)
	executeDirectory(t, s, p, req)
	if len(s.logs) != logs || len(s.materials) != count {
		t.Fatal("retry duplicated resources or logs")
	}
}

func TestDirectorySyncMergesHistoricalDuplicateWithBindings(t *testing.T) {
	s, p, source, target, req := screenshotDirectoryFixture(t)
	unit, chapter := directorySyncID(source.ID, "src-unit", target.ID), directorySyncID(source.ID, "src-chapter", target.ID)
	original, _ := s.findCourse(target.ID)
	original.Curriculum = append(original.Curriculum,
		learning.CurriculumNode{ID: unit, Type: learning.CurriculumUnit, Name: "Fertile Crescent", SortOrder: 2},
		learning.CurriculumNode{ID: chapter, ParentID: unit, Type: learning.CurriculumChapter, Name: "The Sumerians", SortOrder: 2})
	original.DirectorySyncMap = map[string]string{source.ID + ":src-unit": unit, source.ID + ":src-chapter": chapter}
	s.courses[findCourseIndex(s.courses, target.ID)] = original
	s.materials = append(s.materials, learning.Material{ID: "duplicate-material", CourseID: target.ID, LessonID: chapter, Title: "重复分支已有讲义"})
	s.homework = append(s.homework, learning.Homework{ID: "bound-homework", CourseID: target.ID, LessonID: chapter})
	space, _ := s.findLearningSpace(target.LearningSpaceID)
	s.teachingPlans = append(s.teachingPlans, learning.TeachingPlan{ID: "bound-plan", CourseID: target.ID, LessonID: chapter, Grade: target.Grade, Subject: target.Subject, Semester: space.Semester, Phase: space.Phase})
	row := previewDirectory(t, s, p, &req)
	if len(row.Conflicts) != 1 || len(row.Conflicts[0].Candidates) != 2 {
		t.Fatalf("missing historical conflict: %#v", row)
	}
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"src-unit": {TargetNodeID: "old-unit", MergeNodeIDs: []string{unit}}}}
	row = previewDirectory(t, s, p, &req)
	if len(row.Merged) != 2 || directoryMaterial(s, "duplicate-material").LessonID != chapter {
		t.Fatal("preview mutated references")
	}
	executeDirectory(t, s, p, req)
	got, _ := s.findCourse(target.ID)
	if len(got.Curriculum) != 2 || got.DirectorySyncMap[source.ID+":src-chapter"] != "old-chapter" {
		t.Fatalf("duplicate not folded: %#v", got)
	}
	if directoryMaterial(s, "duplicate-material").LessonID != "old-chapter" {
		t.Fatal("material not migrated")
	}
	for _, h := range s.homework {
		if h.ID == "bound-homework" && h.LessonID != "old-chapter" {
			t.Fatal("homework not migrated")
		}
	}
	for _, h := range s.teachingPlans {
		if h.ID == "bound-plan" && h.LessonID != "old-chapter" {
			t.Fatal("teaching plan not migrated")
		}
	}
	executeDirectory(t, s, p, req)
}

func TestDirectorySyncMaterialUpdatesStalePreviewAndSourceRemoval(t *testing.T) {
	s, p, source, target, req := screenshotDirectoryFixture(t)
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"src-unit": {TargetNodeID: "old-unit"}}}
	previewDirectory(t, s, p, &req)
	executeDirectory(t, s, p, req)
	got, _ := s.findCourse(target.ID)
	id := got.DirectorySyncMap["material:"+source.ID+":source-published"]
	for i := range s.materials {
		if s.materials[i].ID == id {
			s.materials[i].ViewCount = 8
		}
	}
	previewDirectory(t, s, p, &req)
	for i := range s.materials {
		if s.materials[i].ID == "source-published" {
			s.materials[i].Title = "文明起源第二版"
			s.materials[i].FileID = "revision-file"
			s.materials[i].AllowDownload = true
		}
	}
	s.fileAssets["revision-file"] = learning.FileAsset{ID: "revision-file", FileName: "第二版.pdf"}
	result, err := s.SyncCourseDirectory("管理员", p, req)
	if err != nil || result.Targets[0].Error == "" || directoryMaterial(s, id).FileID != "published-file" {
		t.Fatal("stale material preview accepted")
	}
	row := previewDirectory(t, s, p, &req)
	if len(row.MaterialsUpdated) != 1 || len(row.MaterialsAdded) != 0 {
		t.Fatalf("lost material identity: %#v", row)
	}
	executeDirectory(t, s, p, req)
	m := directoryMaterial(s, id)
	if m.FileID != "revision-file" || m.ViewCount != 8 || !m.AllowDownload {
		t.Fatalf("update failed: %#v", m)
	}
	for i := range s.materials {
		if s.materials[i].ID == "source-published" {
			s.materials = append(s.materials[:i], s.materials[i+1:]...)
			break
		}
	}
	previewDirectory(t, s, p, &req)
	executeDirectory(t, s, p, req)
	if directoryMaterial(s, id).ID == "" || directoryMaterial(s, "target-own").FileID != "own-file" {
		t.Fatal("source removal deleted target resources")
	}
}

func TestDirectorySyncOnlyDirectoryMissingFileAndInvalidResolution(t *testing.T) {
	s, p, _, target, req := screenshotDirectoryFixture(t)
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"src-unit": {TargetNodeID: "old-unit"}}}
	delete(s.fileAssets, "published-file")
	result, _ := s.PreviewCourseDirectorySync(p, req)
	if result.Targets[0].Error == "" {
		t.Fatal("missing file silently skipped")
	}
	no := false
	req.IncludeMaterials = &no
	row := previewDirectory(t, s, p, &req)
	if len(row.MaterialsAdded) != 0 {
		t.Fatal("directory-only copied material")
	}
	executeDirectory(t, s, p, req)
	req.Resolutions[target.ID]["src-unit"] = learning.DirectorySyncResolution{TargetNodeID: "old-unit", MergeNodeIDs: []string{"outside-course-node"}}
	result, _ = s.PreviewCourseDirectorySync(p, req)
	if result.Targets[0].Error == "" {
		t.Fatal("accepted foreign merge node")
	}
}

func TestDirectorySyncExplicitNewBranchRemainsIdempotent(t *testing.T) {
	s, p, _, target, req := screenshotDirectoryFixture(t)
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"src-unit": {TargetNodeID: "__new__"}}}
	previewDirectory(t, s, p, &req)
	executeDirectory(t, s, p, req)
	executeDirectory(t, s, p, req)
	got, _ := s.findCourse(target.ID)
	if len(got.Curriculum) != 4 {
		t.Fatal("explicit distinct branch duplicated on retry")
	}
}

func TestDirectorySyncMultipleTargetsKeepIndependentSnapshotsAndUnitScope(t *testing.T) {
	s, p, source, target, req := screenshotDirectoryFixture(t)
	otherNodes := []learning.CurriculumNode{
		{ID: "unselected-unit", Type: learning.CurriculumUnit, Name: "未选单元", SortOrder: 3},
		{ID: "unselected-chapter", ParentID: "unselected-unit", Type: learning.CurriculumChapter, Name: "未选章节", SortOrder: 1},
	}
	i := findCourseIndex(s.courses, source.ID)
	s.courses[i].Curriculum = append(s.courses[i].Curriculum, otherNodes...)
	s.materials = append(s.materials, learning.Material{ID: "unselected-handout", CourseID: source.ID, LessonID: "unselected-chapter", Title: "不应复制", TagCode: "HD", FileID: "outside-file", Status: learning.StatusEnabled})
	second, err := s.CreateCourse("管理员", p, learning.CourseUpsertRequest{Name: "第三班型课程", LearningSpaceID: "space-g05-english-s1-q1-h", Curriculum: []learning.CurriculumNode{
		{ID: "second-unit", Type: learning.CurriculumUnit, Name: "Fertile Crescent", SortOrder: 2},
		{ID: "second-chapter", ParentID: "second-unit", Type: learning.CurriculumChapter, Name: "The Sumerians", SortOrder: 2},
	}})
	if err != nil {
		t.Fatal(err)
	}
	req.TargetCourseIDs = append(req.TargetCourseIDs, second.ID)
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"src-unit": {TargetNodeID: "old-unit"}}}
	preview, err := s.PreviewCourseDirectorySync(p, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Snapshots = map[string]string{}
	for _, row := range preview.Targets {
		if row.Error != "" || len(row.MaterialsAdded) != 1 {
			t.Fatalf("scope: %#v", row)
		}
		req.Snapshots[row.CourseID] = row.Snapshot
	}
	result, err := s.SyncCourseDirectory("管理员", p, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range result.Targets {
		if row.Status != "已同步" {
			t.Fatalf("earlier target invalidated later snapshot: %#v", row)
		}
	}
	for _, m := range s.materials {
		if (m.CourseID == target.ID || m.CourseID == second.ID) && m.Title == "不应复制" {
			t.Fatal("copied unselected Unit")
		}
	}
}

func TestDirectorySyncHandoutPermissionCannotBeBypassed(t *testing.T) {
	s, _, source, target, req := screenshotDirectoryFixture(t)
	teacher, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	teacher.LearningSpaceIDs = []string{source.LearningSpaceID, target.LearningSpaceID}
	teacher.TeacherLibrary = &learning.TeacherLibraryPolicy{CanManageCourses: true, Scopes: []learning.TeacherLibraryScope{{Grade: source.Grade, Subject: source.Subject}}}
	teacher.CanUploadHandout = false
	preview, err := s.PreviewCourseDirectorySync(teacher, req)
	if err != nil || preview.Targets[0].Error != "当前账号没有同步课程讲义权限，请选择仅同步目录" {
		t.Fatalf("permission: %#v %v", preview, err)
	}
	no := false
	req.IncludeMaterials = &no
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {"src-unit": {TargetNodeID: "old-unit"}}}
	previewDirectory(t, s, teacher, &req)
	executeDirectory(t, s, teacher, req)
	got, _ := s.findCourse(target.ID)
	if got.DirectorySyncMap["material:"+source.ID+":source-published"] != "" {
		t.Fatal("directory permission copied handouts")
	}
}

func TestDirectorySyncSharedFamilyCopiesHandoutsWithoutMergingDirectory(t *testing.T) {
	s, p, source, target := directorySyncFixture(t)
	for _, id := range []string{source.ID, target.ID} {
		i := findCourseIndex(s.courses, id)
		s.courses[i].FamilyID = "shared-family"
		s.courses[i].Curriculum = source.Curriculum
	}
	s.materials = append(s.materials, learning.Material{ID: "family-handout", CourseID: source.ID, LessonID: source.Curriculum[0].ID, Title: "共享目录讲义", FileID: "family-file", TagCode: "HD", Status: learning.StatusEnabled})
	s.fileAssets["family-file"] = learning.FileAsset{ID: "family-file"}
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID}}
	row := previewDirectory(t, s, p, &req)
	if len(row.Added)+len(row.Updated) != 0 || len(row.MaterialsAdded) != 1 {
		t.Fatalf("shared preview: %#v", row)
	}
	executeDirectory(t, s, p, req)
	req.Resolutions = map[string]map[string]learning.DirectorySyncResolution{target.ID: {source.Curriculum[0].ID: {TargetNodeID: "__new__"}}}
	result, _ := s.PreviewCourseDirectorySync(p, req)
	if result.Targets[0].Error == "" {
		t.Fatal("shared directory accepts per-course merge")
	}
}
