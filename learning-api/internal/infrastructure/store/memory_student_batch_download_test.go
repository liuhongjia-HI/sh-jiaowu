package store

import (
	"encoding/json"
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func studentBatchFixture(t *testing.T) (*MemoryStore, learning.Principal, learning.MaterialDownloadScope) {
	t.Helper()
	s := NewMemoryStore()
	p, err := s.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateDirectGrant("test", learning.DirectGrantCreateRequest{StudentID: p.StudentID, LearningSpaceIDs: []string{"space-g05-english-s1-q1"}, ContentTypeCodes: []string{"course", "handout", "download"}, StartsAt: "2026-01-01", EndsAt: "2027-12-31"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range s.materials {
		if s.materials[i].ID == "mat-g05-english-s1-q1" {
			s.materials[i].FileID = "student-batch-file"
			s.materials[i].AllowDownload = true
		}
	}
	s.fileAssets["student-batch-file"] = learning.FileAsset{ID: "student-batch-file", FileName: "lecture.pdf", OriginalPath: "/test/lecture.pdf", FileSize: 4}
	return s, p, learning.MaterialDownloadScope{CourseIDs: []string{"course-g05-english-s1-q1"}}
}
func TestStudentBatchDownloadSelectionOwnershipAndRevocation(t *testing.T) {
	s, p, scope := studentBatchFixture(t)
	q, err := s.MaterialDownloadSelection(p, scope)
	if err != nil || q.Count != 1 || len(q.Materials) != 1 || q.StudentName == "" {
		t.Fatalf("selection: %#v %v", q, err)
	}
	if _, err = s.MaterialDownloadSelection(p, learning.MaterialDownloadScope{}); err == nil {
		t.Fatal("student could select all courses")
	}
	if _, err = s.MaterialDownloadSelection(p, learning.MaterialDownloadScope{CourseIDs: scope.CourseIDs, MaterialIDs: []string{"other-student-material"}}); err == nil {
		t.Fatal("invalid explicit material accepted")
	}
	scope.MaterialIDs = []string{q.Materials[0].ID}
	job, err := s.CreateMaterialDownload(p, scope)
	if err != nil {
		t.Fatal(err)
	}
	if job.StudentID != p.StudentID || job.StudentName == "" || job.GuardianID != p.GuardianID || len(job.Materials) != 1 {
		t.Fatalf("missing student context: %#v", job)
	}
	other := p
	other.StudentID = "other"
	other.UserID = "other"
	if len(s.MaterialDownloads(other)) != 0 {
		t.Fatal("other account sees task")
	}
	if _, err = s.MaterialDownloadArchive(other, job.ID); err == nil {
		t.Fatal("other account gets archive")
	}
	claimed, files, found, err := s.ClaimMaterialDownload()
	if err != nil || !found || claimed.ID != job.ID || len(files) != 1 {
		t.Fatalf("claim: %#v %v", claimed, err)
	}
	if err = s.FinishMaterialDownload(job.ID, "/test/package.zip", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.MaterialDownloadArchive(p, job.ID); err != nil {
		t.Fatal(err)
	}
	// A JSON database roundtrip keeps the original guardian and student context.
	raw, _ := json.Marshal(storedDownloadJob{job.GuardianID, job, job.OwnerID, job.ArchivePath, job.Items})
	var restored storedDownloadJob
	if err = json.Unmarshal(raw, &restored); err != nil || restored.GuardianID != p.GuardianID || restored.Job.StudentID != p.StudentID {
		t.Fatal("persisted task lost ownership")
	}
	for i := range s.materials {
		if s.materials[i].ID == q.Materials[0].ID {
			s.materials[i].AllowDownload = false
		}
	}
	if _, err = s.MaterialDownloadArchive(p, job.ID); err == nil {
		t.Fatal("revoked download remains available")
	}
	if _, err = s.MaterialDownloadSelection(p, scope); err == nil {
		t.Fatal("revoked explicit selection accepted")
	}
}
func TestStudentBatchDownloadGuardianUnlinkAndExpiry(t *testing.T) {
	s, p, scope := studentBatchFixture(t)
	p.GuardianID = s.ensureGuardianLink("13800000005", "", p.StudentID)
	job, err := s.CreateMaterialDownload(p, scope)
	if err != nil {
		t.Fatal(err)
	}
	impostor := p
	impostor.GuardianID = "another-guardian"
	if len(s.MaterialDownloads(impostor)) > 0 {
		t.Fatal("different guardian sees task")
	}
	if _, err = s.MaterialDownloadArchive(impostor, job.ID); err == nil {
		t.Fatal("different guardian gets task")
	}
	if _, _, found, err := s.ClaimMaterialDownload(); err != nil || !found {
		t.Fatal("guardian task not claimed", err)
	}
	if err = s.FinishMaterialDownload(job.ID, "/test/package.zip", ""); err != nil {
		t.Fatal(err)
	}
	s.materialDownloads[0].ExpiresAt = time.Now().Add(-time.Second).UTC().Format(time.RFC3339)
	if s.MaterialDownloads(p)[0].Status != "已过期" {
		t.Fatal("expiry not displayed")
	}
	if _, err = s.MaterialDownloadArchive(p, job.ID); err == nil {
		t.Fatal("expired task downloadable")
	}
	for i := range s.guardianStudents {
		if s.guardianStudents[i].GuardianID == p.GuardianID && s.guardianStudents[i].StudentID == p.StudentID {
			s.guardianStudents[i].Status = "已解绑"
		}
	}
	if _, err = s.materialDownloadPrincipal(job); err == nil {
		t.Fatal("unlinked guardian remains valid")
	}
}
