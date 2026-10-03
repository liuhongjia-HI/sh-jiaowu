package store

import (
	"strings"
	"testing"
)

func TestTeachingPlanFailureReasonAndRetry(t *testing.T) {
	for _, reason := range []string{
		"LibreOffice 未生成 PDF，请检查 Word/PPT 是否损坏或已加密",
		"课件共301页，超过300页上限",
		"读取原文件失败: open /private/internal/customer.doc: permission denied",
	} {
		t.Run(reason, func(t *testing.T) {
			s := NewMemoryStore()
			p, _, plan := chapterPlanFixture(t, s)
			for attempt := 0; attempt < maxPreviewAttempts; attempt++ {
				job, ok, err := s.ClaimPreviewJob()
				if err != nil || !ok {
					t.Fatalf("claim: %v %v", ok, err)
				}
				if err := s.FailPreviewJob(job.ID, reason); err != nil {
					t.Fatal(err)
				}
			}
			failed, err := s.TeachingPlan(p, plan.ID)
			if err != nil || failed.PreviewStatus != "转换失败" || failed.PreviewError == "" {
				t.Fatalf("failed: %#v %v", failed, err)
			}
			if strings.Contains(failed.PreviewError, "/private/") {
				t.Fatal("internal path leaked")
			}
			if !strings.Contains(reason, "/private/") && failed.PreviewError != strings.ReplaceAll(reason, "LibreOffice 未生成 PDF", "未生成预览") {
				t.Fatal("actionable reason missing")
			}
			if err := s.RetryTeachingPlanPreview("管理员", p, plan.ID); err != nil {
				t.Fatal(err)
			}
			pending, err := s.TeachingPlan(p, plan.ID)
			if err != nil || pending.PreviewError != "" || pending.PreviewStatus != "待转换" {
				t.Fatalf("retry retained old error: %#v %v", pending, err)
			}
		})
	}
}
