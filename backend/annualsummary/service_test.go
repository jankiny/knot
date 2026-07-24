package annualsummary

import (
	"encoding/json"
	"strings"
	"testing"

	"knot-backend/contextmanifest"
	"knot-backend/policy"
)

func TestMakeModelInputKeepsOnlyApprovedEvidenceFields(t *testing.T) {
	input := makeModelInput(
		contextmanifest.ContextManifest{
			TaskType:    contextmanifest.TaskTypePersonalAnnualSummary,
			Query:       "年度总结",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
		[]contextmanifest.EvidenceItem{
			{
				ID:                "evidence_content",
				DocumentID:        "doc_content",
				SourceRootID:      "root_content",
				SourceType:        "journal_daily",
				Title:             "工作日报",
				Date:              "2026-07-20",
				Project:           "Knot",
				Excerpt:           "完成阶段五。",
				Reason:            "内部排序说明",
				AIAccessEffective: policy.AIAccessContent,
			},
			{
				ID:                "evidence_metadata",
				DocumentID:        "doc_metadata",
				SourceRootID:      "root_metadata",
				SourceType:        "file",
				Title:             "年度资料.docx",
				Date:              "2026-07-19",
				Excerpt:           "不得发送的测试正文",
				Reason:            "内部元数据说明",
				AIAccessEffective: policy.AIAccessMetadata,
			},
		},
	)

	if len(input.Evidence) != 2 {
		t.Fatalf("unexpected model evidence: %+v", input)
	}
	if input.Evidence[0].Excerpt != "完成阶段五。" {
		t.Fatalf("content evidence was removed: %+v", input.Evidence[0])
	}
	if input.Evidence[1].Excerpt != "" {
		t.Fatalf("metadata evidence leaked content: %+v", input.Evidence[1])
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("encode model input: %v", err)
	}
	serialized := strings.TrimSpace(string(payload))
	for _, forbidden := range []string{
		"doc_content",
		"root_content",
		"内部排序说明",
		"不得发送的测试正文",
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("model input leaked %q: %s", forbidden, serialized)
		}
	}
}

func TestParseModelResultRejectsUnknownEvidenceReference(t *testing.T) {
	_, err := parseModelResult(`{
		"title":"个人年度总结大纲",
		"target_word_count":1500,
		"sections":[{
			"heading":"工作主线",
			"word_count":1500,
			"outline":["梳理年度工作主线"],
			"evidence_ids":["evidence_forged"]
		}],
		"verified_results":[],
		"missing_information":[]
	}`, map[string]struct{}{"evidence_allowed": {}})
	if err == nil || !strings.Contains(err.Error(), "unknown evidence ID") {
		t.Fatalf("expected unknown reference validation error, got %v", err)
	}
}

func TestValidateAIDestinationUsesApprovedOfficialHTTPSHosts(t *testing.T) {
	allowed := []string{
		"https://api.deepseek.com",
		"https://api.openai.com/v1",
	}
	for _, value := range allowed {
		if err := validateAIDestination(value); err != nil {
			t.Fatalf("expected destination %q to be allowed: %v", value, err)
		}
	}

	blocked := []string{
		"http://api.deepseek.com",
		"https://api.deepseek.com.evil.invalid",
		"https://example.invalid/v1",
		"https://user:pass@api.openai.com/v1",
	}
	for _, value := range blocked {
		if err := validateAIDestination(value); err == nil {
			t.Fatalf("expected destination %q to be rejected", value)
		}
	}
}
