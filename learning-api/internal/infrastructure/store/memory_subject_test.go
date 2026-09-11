package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestDeleteUnusedSubjectMetadata(t *testing.T) {
	store := NewMemoryStore()
	store.subjects = append(store.subjects, learning.SubjectMetadata{
		ID: "politics", Name: "政治", ShortLabel: "Pol", Color: "#888888", Status: "启用",
	})

	listed := store.Subjects()
	var politics *learning.SubjectMetadata
	for index := range listed {
		if listed[index].ID == "politics" {
			politics = &listed[index]
		}
		if listed[index].ID == "english" && listed[index].Deletable {
			t.Fatalf("built-in subject should not be deletable: %#v", listed[index])
		}
	}
	if politics == nil || !politics.Deletable {
		t.Fatalf("unused leftover subject should be deletable, got %#v", politics)
	}

	if err := store.DeleteSubjectMetadata("测试管理员", "politics"); err != nil {
		t.Fatalf("delete unused subject: %v", err)
	}
	for _, item := range store.Subjects() {
		if item.ID == "politics" {
			t.Fatal("deleted subject should disappear from the list")
		}
	}
}

func TestDeleteBuiltInSubjectMetadataRejected(t *testing.T) {
	store := NewMemoryStore()
	err := store.DeleteSubjectMetadata("测试管理员", "english")
	if err == nil || !strings.Contains(err.Error(), "系统内置学科不能删除") {
		t.Fatalf("expected built-in subject delete to fail, got %v", err)
	}
	err = store.DeleteSubjectMetadata("测试管理员", "history")
	if err == nil || !strings.Contains(err.Error(), "系统内置学科不能删除") {
		t.Fatalf("expected retired built-in subject delete to fail, got %v", err)
	}
	if err := store.DeleteSubjectMetadata("测试管理员", "missing"); err == nil || !strings.Contains(err.Error(), "学科不存在") {
		t.Fatalf("expected missing subject delete to fail, got %v", err)
	}
}

func TestDeleteReferencedCustomSubjectRejected(t *testing.T) {
	store := NewMemoryStore()
	store.subjects = append(store.subjects, learning.SubjectMetadata{
		ID: "biology", Name: "生物", ShortLabel: "Bio", Color: "#228B22", Status: "启用",
	})
	store.courses = append(store.courses, learning.Course{ID: "course-bio", Name: "生物课", Subject: "生物", Status: learning.StatusEnabled})
	err := store.DeleteSubjectMetadata("测试管理员", "biology")
	if err == nil || !strings.Contains(err.Error(), "仍有课程使用该学科") {
		t.Fatalf("expected referenced subject delete to fail, got %v", err)
	}
}
