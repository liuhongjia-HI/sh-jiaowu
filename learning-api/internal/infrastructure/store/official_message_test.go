package store

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestParseOfficialTemplateFieldsKeepsDisplayOrder(t *testing.T) {
	fields := parseOfficialTemplateFields("通知标题：{{thing1.DATA}}\n上课时间：{{time2.DATA}}\n通知内容：{{thing3.DATA}}")
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	if fields[0].Key != "thing1" || fields[0].Label != "通知标题" || fields[1].Key != "time2" {
		t.Fatalf("unexpected parsed fields: %#v", fields)
	}
}

func TestWechatSecretEncryptionAndMasking(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SkipBaseData: true, EncryptionKey: "test-secret-key"})
	encrypted, err := store.encryptWechatValue("wechat-secret")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "wechat-secret") {
		t.Fatal("encrypted value leaked plaintext")
	}
	store.settings[wechatMiniSecretKey] = encrypted
	settings := store.WechatSettings()
	if !settings.MiniProgramSecretConfigured {
		t.Fatal("expected configured marker")
	}
}

func TestOfficialAudienceUsesGuardianUnionIDAndDeduplicates(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	store.students = []learning.Student{{ID: "s1", Name: "学生甲", Grade: "五年级", AccountStatus: "正常"}, {ID: "s2", Name: "学生乙", Grade: "五年级", AccountStatus: "正常"}}
	store.guardians = []learning.Guardian{{ID: "g1", Name: "家长甲", UnionID: "union-1", AccountStatus: "正常"}}
	store.guardianStudents = []learning.GuardianStudent{{GuardianID: "g1", StudentID: "s1", Status: learning.GuardianStudentActive}, {GuardianID: "g1", StudentID: "s2", Status: learning.GuardianStudentActive}}
	store.officialFollowers = []learning.OfficialFollower{{OpenID: "official-openid", UnionID: "union-1", Subscribed: true}}
	preview, targets, err := store.officialAudienceUnlocked([]string{"五年级"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.StudentCount != 2 || preview.GuardianCount != 1 || preview.ReachableCount != 1 || len(targets) != 1 {
		t.Fatalf("unexpected preview: %#v targets=%#v", preview, targets)
	}
}

func TestOfficialCallbackSignature(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SkipBaseData: true, EncryptionKey: "test-secret-key"})
	encrypted, err := store.encryptWechatValue("callback-token")
	if err != nil {
		t.Fatal(err)
	}
	store.settings[wechatCallbackTokenKey] = encrypted
	parts := []string{"callback-token", "1700000000", "nonce"}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	if !store.VerifyOfficialCallback(hex.EncodeToString(sum[:]), "1700000000", "nonce") {
		t.Fatal("expected valid callback signature")
	}
	if store.VerifyOfficialCallback("bad", "1700000000", "nonce") {
		t.Fatal("expected invalid callback signature")
	}
}
