package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestMaterialFileOrderReachesStudentWithoutChangingUploadTime(t *testing.T) {
	store := NewMemoryStore()
	teacher, _ := store.PrincipalByUserID("user-teacher")
	student, _ := store.PrincipalByUserID("user-student-001")
	const courseID = "course-g05-english-s1-q1"
	created := make([]learning.Material, 0, 2)
	for _, tag := range []string{"HD", "HW"} {
		item, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
			Title: tag + " 阅读资料", CourseID: courseID, LearningSpaceID: "space-g05-english-s1-q1", LessonID: courseID + "-lesson-1", TagCode: tag,
		})
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, item)
	}
	ids := []string{created[1].ID, created[0].ID}
	for _, item := range store.Materials(teacher, learning.MaterialQuery{}) {
		if item.CourseID == courseID && item.ID != created[0].ID && item.ID != created[1].ID {
			ids = append(ids, item.ID)
		}
	}
	if err := store.ReorderMaterials("英语老师", teacher, learning.MaterialReorderRequest{CourseID: courseID, MaterialIDs: ids}); err != nil {
		t.Fatal(err)
	}
	detail, err := store.StudentCourseDetail(student, courseID)
	if err != nil {
		t.Fatal(err)
	}
	var ordered []string
	for _, station := range detail.Stations {
		if station.MaterialID == created[0].ID || station.MaterialID == created[1].ID {
			ordered = append(ordered, station.MaterialID)
		}
	}
	if len(ordered) != 2 || ordered[0] != created[1].ID || ordered[1] != created[0].ID {
		t.Fatalf("student station order differs from saved order: %v", ordered)
	}
	for _, item := range detail.Materials {
		for _, original := range created {
			if item.ID == original.ID && item.CreatedAt != original.CreatedAt {
				t.Fatalf("reorder changed upload time: %s", item.ID)
			}
		}
	}
}
