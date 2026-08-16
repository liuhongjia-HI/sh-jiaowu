package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestQuestionsFilterByGradeAndSubject(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	all := store.Questions(principal, learning.QuestionBankQuery{})
	if len(all) == 0 {
		t.Fatal("expected demo question bank items")
	}
	grade, subject := all[0].Grade, all[0].Subject
	items := store.Questions(principal, learning.QuestionBankQuery{Grade: grade, Subject: subject})
	if len(items) == 0 {
		t.Fatal("expected matching question bank items")
	}
	for _, item := range items {
		if item.Grade != grade || item.Subject != subject {
			t.Fatalf("expected exact grade and subject filter, got %#v", item)
		}
	}
}

func TestQuestionsFilterByKeyword(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	all := store.Questions(principal, learning.QuestionBankQuery{})
	if len(all) == 0 {
		t.Fatal("expected demo question bank items")
	}
	items := store.Questions(principal, learning.QuestionBankQuery{Keyword: all[0].Stem})
	if len(items) == 0 {
		t.Fatal("expected keyword to match question stem")
	}
}
