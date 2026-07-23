package extractors

import (
	"strings"
	"testing"
)

func TestExtractMarkdownRecognizesDailyAndWeeklyJournals(t *testing.T) {
	daily := ExtractMarkdown(
		[]byte("---\ntitle: 工作日报\n---\n# 不使用此标题\n\n- 完成阶段测试\n"),
		"日/2026-07-23 日报.md",
		MarkdownOptions{Journal: true, MaxExcerptRunes: 20},
	)
	if daily.DocumentType != DocumentTypeJournalDaily ||
		daily.TaskDate != "2026-07-23" ||
		daily.Title != "工作日报" ||
		!strings.Contains(daily.Excerpt, "完成阶段测试") {
		t.Fatalf("unexpected daily extraction: %+v", daily)
	}

	weekly := ExtractMarkdown(
		[]byte("# 2026.W03 工作周报\n\n本周完成增量索引。\n"),
		"周/2026/2026.W03 工作周报.md",
		MarkdownOptions{Journal: true, MaxExcerptRunes: 100},
	)
	if weekly.DocumentType != DocumentTypeJournalWeekly ||
		weekly.TaskDate != "2026-01-12" ||
		weekly.Title != "2026.W03 工作周报" ||
		!strings.Contains(weekly.Excerpt, "增量索引") {
		t.Fatalf("unexpected weekly extraction: %+v", weekly)
	}
}

func TestExtractMarkdownKeepsOrdinaryMarkdownMetadataOnly(t *testing.T) {
	result := ExtractMarkdown(
		[]byte("# 架构说明\n\n这段普通 Markdown 正文不应进入片段。\n"),
		"reference/架构说明.md",
		MarkdownOptions{Journal: false, MaxExcerptRunes: 100},
	)
	if result.DocumentType != DocumentTypeMarkdown {
		t.Fatalf("expected ordinary markdown, got %+v", result)
	}
	if result.Title != "架构说明" || result.Excerpt != "" {
		t.Fatalf("ordinary markdown must be metadata-only: %+v", result)
	}
}

func TestNormalizeDateRejectsInvalidDates(t *testing.T) {
	if got := NormalizeDate("2026-02-30"); got != "" {
		t.Fatalf("expected invalid date to be rejected, got %q", got)
	}
	if got := NormalizeDate("updated 2026.7.3"); got != "2026-07-03" {
		t.Fatalf("unexpected normalized date %q", got)
	}
}
