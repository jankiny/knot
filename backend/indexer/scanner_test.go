package indexer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"knot-backend/appdata"
	"knot-backend/extractors"
	"knot-backend/policy"
	"knot-backend/safepath"
	"knot-backend/sources"
	"knot-backend/storage"
)

func TestScannerBuildsUnifiedIncrementalIndex(t *testing.T) {
	readerCalls := 0
	environment := newIndexTestEnvironment(
		t,
		"unified_incremental",
		sources.KindJournal,
		sources.AIAccessContent,
		func(filePath string) (WorkRecordData, error) {
			readerCalls++
			data, err := os.ReadFile(filePath)
			if err != nil {
				return WorkRecordData{}, err
			}
			return WorkRecordData{
				RawContent: data,
				Title:      "索引任务",
				TaskDate:   "2026-07-20",
				Content:    "完成统一索引与安全测试。",
			}, nil
		},
	)

	writeIndexTestFile(t, environment.rootPath, "任务/工作记录.md", "private raw body")
	writeIndexTestFile(t, environment.rootPath, "日/2026-07-23 日报.md", "# 日报\n\n完成日报索引。")
	writeIndexTestFile(t, environment.rootPath, "周/2026.W03 工作周报.md", "# 2026.W03 工作周报\n\n完成周报索引。")
	writeIndexTestFile(t, environment.rootPath, "reference/架构说明.md", "# 架构说明\n\n普通正文不保存。")
	writeIndexTestFile(t, environment.rootPath, "reference/readme.txt", "ordinary text metadata")
	writeIndexTestFile(t, environment.rootPath, "reference/report.pdf", "%PDF-not-parsed")
	writeIndexTestFile(t, environment.rootPath, "node_modules/ignored.md", "# ignored")

	first, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if first.Status != ScanStatusCompleted ||
		first.Indexed != 6 ||
		first.IgnoredDirectories != 1 {
		t.Fatalf("unexpected first scan: %+v", first)
	}
	if readerCalls != 1 {
		t.Fatalf("expected work record parser once, got %d", readerCalls)
	}

	documents := listIndexDocuments(t, environment)
	if len(documents) != 6 {
		t.Fatalf("expected 6 documents, got %+v", documents)
	}
	byPath := documentsByPath(documents)
	assertDocumentType(t, byPath, "任务/工作记录.md", DocumentTypeWorkRecord)
	assertDocumentType(t, byPath, "日/2026-07-23 日报.md", extractors.DocumentTypeJournalDaily)
	assertDocumentType(t, byPath, "周/2026.W03 工作周报.md", extractors.DocumentTypeJournalWeekly)
	assertDocumentType(t, byPath, "reference/架构说明.md", extractors.DocumentTypeMarkdown)
	assertDocumentType(t, byPath, "reference/readme.txt", DocumentTypeText)
	assertDocumentType(t, byPath, "reference/report.pdf", DocumentTypeFile)
	if byPath["reference/架构说明.md"].ContentExcerpt != "" ||
		byPath["reference/readme.txt"].ContentExcerpt != "" ||
		byPath["reference/report.pdf"].ContentExcerpt != "" {
		t.Fatal("ordinary Markdown, TXT, and other files must remain metadata-only")
	}
	if byPath["日/2026-07-23 日报.md"].ContentExcerpt == "" ||
		byPath["周/2026.W03 工作周报.md"].ContentExcerpt == "" {
		t.Fatal("daily and weekly journals must retain bounded excerpts")
	}

	second, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if second.Unchanged != 6 || second.Updated != 0 || second.Indexed != 0 {
		t.Fatalf("expected fully incremental second scan, got %+v", second)
	}
	if readerCalls != 1 {
		t.Fatalf("unchanged work record was parsed again: %d calls", readerCalls)
	}

	forced, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{Force: true},
	)
	if err != nil {
		t.Fatalf("forced rebuild scan: %v", err)
	}
	if forced.Updated != 6 || readerCalls != 2 {
		t.Fatalf(
			"force scan did not rebuild adapters: result=%+v calls=%d",
			forced,
			readerCalls,
		)
	}
}

func TestScannerNeverStoresNoAIContent(t *testing.T) {
	readerCalls := 0
	environment := newIndexTestEnvironment(
		t,
		"noai_content",
		sources.KindPrivate,
		sources.AIAccessContent,
		func(filePath string) (WorkRecordData, error) {
			readerCalls++
			data, err := os.ReadFile(filePath)
			return WorkRecordData{
				RawContent: data,
				Title:      "不应解析",
				Content:    "高度敏感正文",
			}, err
		},
	)
	noAIPath := filepath.Join(filepath.Dir(environment.rootPath), "Private_NoAI")
	if err := os.Rename(environment.rootPath, noAIPath); err != nil {
		t.Fatalf("rename source to NoAI path: %v", err)
	}
	environment.rootPath = noAIPath
	updatedRoot, err := environment.registry.Update(
		context.Background(),
		environment.root.ID,
		sources.Input{
			Name:        "NoAI",
			Kind:        sources.KindPrivate,
			Path:        noAIPath,
			ScopeType:   sources.ScopeGlobal,
			LocalAccess: sources.LocalAccessRead,
			AIAccess:    sources.AIAccessContent,
		},
	)
	if err != nil {
		t.Fatalf("update source to NoAI path: %v", err)
	}
	environment.root = updatedRoot
	writeIndexTestFile(t, environment.rootPath, "任务/工作记录.md", "高度敏感正文")

	result, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("scan NoAI root: %v", err)
	}
	if result.MetadataOnly != 1 || readerCalls != 0 {
		t.Fatalf("NoAI content was read unexpectedly: result=%+v calls=%d", result, readerCalls)
	}
	document := listIndexDocuments(t, environment)[0]
	if document.AIAccessEffective != sources.AIAccessNone ||
		document.ContentExcerpt != "" ||
		!strings.HasPrefix(document.ContentHash, "sha256:") {
		t.Fatalf("NoAI document stored more than bounded metadata: %+v", document)
	}
}

func TestScannerAppliesWorkRecordOverrideWithoutSavingExcerpt(t *testing.T) {
	environment := newIndexTestEnvironment(
		t,
		"work_record_override",
		sources.KindCurrentWork,
		sources.AIAccessMetadata,
		func(filePath string) (WorkRecordData, error) {
			data, err := os.ReadFile(filePath)
			return WorkRecordData{
				RawContent: data,
				Title:      "私密任务",
				TaskDate:   "2026-07-20",
				Content:    "不得写入数据库的正文",
				AIAccess:   "noai",
			}, err
		},
	)
	writeIndexTestFile(t, environment.rootPath, "任务/工作记录.md", "不得写入数据库的正文")

	if _, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	); err != nil {
		t.Fatalf("scan metadata work record: %v", err)
	}
	document := listIndexDocuments(t, environment)[0]
	if document.AIAccessEffective != sources.AIAccessNone ||
		document.ContentExcerpt != "" {
		t.Fatalf("work-record NoAI override was not enforced: %+v", document)
	}

	var storedExcerptCount int
	if err := environment.repository.db.QueryRow(
		`SELECT COUNT(*) FROM indexed_documents
		 WHERE id = ? AND content_excerpt IS NOT NULL`,
		document.ID,
	).Scan(&storedExcerptCount); err != nil {
		t.Fatalf("inspect stored excerpt: %v", err)
	}
	if storedExcerptCount != 0 {
		t.Fatal("NoAI excerpt must be SQL NULL")
	}
}

func TestScannerMarksMovedDeletedOfflineAndRecoveredDocuments(t *testing.T) {
	environment := newIndexTestEnvironment(
		t,
		"lifecycle",
		sources.KindReference,
		sources.AIAccessContent,
		nil,
	)
	writeIndexTestFile(t, environment.rootPath, "old.md", "# Old")
	if _, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	if err := os.Rename(
		filepath.Join(environment.rootPath, "old.md"),
		filepath.Join(environment.rootPath, "moved.md"),
	); err != nil {
		t.Fatalf("move indexed file: %v", err)
	}
	movedResult, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("scan moved file: %v", err)
	}
	if movedResult.Indexed != 1 || movedResult.Deleted != 1 {
		t.Fatalf("unexpected move result: %+v", movedResult)
	}
	byPath := documentsByPath(listIndexDocuments(t, environment))
	if byPath["old.md"].IndexStatus != IndexStatusDeleted ||
		byPath["moved.md"].IndexStatus != IndexStatusReady {
		t.Fatalf("unexpected moved document states: %+v", byPath)
	}

	offlinePath := environment.rootPath + "_offline"
	if err := os.Rename(environment.rootPath, offlinePath); err != nil {
		t.Fatalf("take source offline: %v", err)
	}
	offlineResult, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("scan offline source: %v", err)
	}
	if offlineResult.Status != ScanStatusUnavailable {
		t.Fatalf("expected unavailable result, got %+v", offlineResult)
	}
	byPath = documentsByPath(listIndexDocuments(t, environment))
	if byPath["moved.md"].IndexStatus != IndexStatusStale {
		t.Fatalf("online document was not marked stale: %+v", byPath["moved.md"])
	}

	if err := os.Rename(offlinePath, environment.rootPath); err != nil {
		t.Fatalf("restore source: %v", err)
	}
	recovered, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("scan recovered source: %v", err)
	}
	if recovered.Status != ScanStatusCompleted {
		t.Fatalf("unexpected recovery result: %+v", recovered)
	}
	byPath = documentsByPath(listIndexDocuments(t, environment))
	if byPath["moved.md"].IndexStatus != IndexStatusReady {
		t.Fatalf("stale document did not recover: %+v", byPath["moved.md"])
	}
}

func TestScannerPreservesRegisteredProjectScope(t *testing.T) {
	environment := newIndexTestEnvironment(
		t,
		"project_scope",
		sources.KindWorkArchive,
		sources.AIAccessContent,
		nil,
	)
	projectID := "project-knot"
	projectName := "Knot"
	updatedRoot, err := environment.registry.Update(
		context.Background(),
		environment.root.ID,
		sources.Input{
			Name:        "项目归档",
			Kind:        sources.KindWorkArchive,
			Path:        environment.rootPath,
			ScopeType:   sources.ScopeProject,
			ScopeID:     &projectID,
			ScopeName:   &projectName,
			LocalAccess: sources.LocalAccessRead,
			AIAccess:    sources.AIAccessContent,
		},
	)
	if err != nil {
		t.Fatalf("set project scope: %v", err)
	}
	environment.root = updatedRoot
	writeIndexTestFile(t, environment.rootPath, "项目说明.md", "# 项目说明")

	if _, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	); err != nil {
		t.Fatalf("scan project source: %v", err)
	}
	document := listIndexDocuments(t, environment)[0]
	if document.ProjectID == nil || *document.ProjectID != projectID {
		t.Fatalf("project scope was not preserved: %+v", document)
	}
}

func TestScannerEnforcesLimitsAndCancellation(t *testing.T) {
	environment := newIndexTestEnvironment(
		t,
		"limits",
		sources.KindReference,
		sources.AIAccessContent,
		nil,
	)
	writeIndexTestFile(t, environment.rootPath, "a.txt", "first")
	writeIndexTestFile(t, environment.rootPath, "b.txt", "second")
	environment.scanner.limits.MaxFiles = 1

	result, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("limited scan: %v", err)
	}
	if result.Status != ScanStatusPartial ||
		!hasIssue(result.Issues, ReasonScanLimit) {
		t.Fatalf("expected partial scan limit result, got %+v", result)
	}

	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = environment.scanner.ScanSource(
		cancelledContext,
		environment.root.ID,
		ScanOptions{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestScannerIndexesOversizedFileAsMetadata(t *testing.T) {
	environment := newIndexTestEnvironment(
		t,
		"oversized",
		sources.KindReference,
		sources.AIAccessContent,
		nil,
	)
	writeIndexTestFile(t, environment.rootPath, "large.txt", "larger than four bytes")
	environment.scanner.limits.MaxFileBytes = 4

	if _, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	); err != nil {
		t.Fatalf("scan oversized file: %v", err)
	}
	document := listIndexDocuments(t, environment)[0]
	if document.IndexStatus != IndexStatusReady ||
		document.LastErrorCode != string(policy.ReasonSizeLimit) ||
		document.ContentHash != "" ||
		document.ContentExcerpt != "" {
		t.Fatalf("oversized file must be metadata-only: %+v", document)
	}
}

func TestScannerReportsLinkEscapeWithoutReadingTarget(t *testing.T) {
	environment := newIndexTestEnvironment(
		t,
		"link_escape",
		sources.KindReference,
		sources.AIAccessContent,
		nil,
	)
	outsideDirectory := filepath.Join(filepath.Dir(environment.rootPath), "outside")
	if err := os.MkdirAll(outsideDirectory, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	outsideFile := filepath.Join(outsideDirectory, "private.txt")
	if err := os.WriteFile(outsideFile, []byte("must not be indexed"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkPath := filepath.Join(environment.rootPath, "escape.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		if runtime.GOOS != "windows" {
			t.Skipf("symbolic links are unavailable: %v", err)
		}
		linkPath = filepath.Join(environment.rootPath, "escape")
		output, junctionErr := exec.Command(
			"cmd.exe",
			"/c",
			"mklink",
			"/J",
			linkPath,
			outsideDirectory,
		).CombinedOutput()
		if junctionErr != nil {
			t.Skipf(
				"symbolic links and junctions are unavailable: %v output=%s",
				junctionErr,
				strings.TrimSpace(string(output)),
			)
		}
	}
	t.Cleanup(func() {
		_ = os.Remove(linkPath)
	})

	result, err := environment.scanner.ScanSource(
		context.Background(),
		environment.root.ID,
		ScanOptions{},
	)
	if err != nil {
		t.Fatalf("scan link escape: %v", err)
	}
	if !hasIssue(result.Issues, string(policy.ReasonSymlinkEscape)) {
		t.Fatalf("expected structured symlink escape issue, got %+v", result)
	}
	for _, document := range listIndexDocuments(t, environment) {
		if document.ContentHash != "" || document.ContentExcerpt != "" {
			t.Fatalf("escaped target content was indexed: %+v", document)
		}
	}
}

type indexTestEnvironment struct {
	rootPath   string
	root       sources.SourceRoot
	registry   *sources.Registry
	repository *Repository
	scanner    *Scanner
}

func newIndexTestEnvironment(
	t *testing.T,
	name string,
	kind sources.Kind,
	aiAccess sources.AIAccess,
	reader WorkRecordReader,
) *indexTestEnvironment {
	t.Helper()
	dataPath := indexTestDataPath(t, name)
	rootPath := filepath.Join(dataPath, "source")
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		t.Fatalf("create source root: %v", err)
	}

	db, err := storage.Open(context.Background(), appdata.Directory{
		Path:   dataPath,
		Source: appdata.SourceEnvironment,
	})
	if err != nil {
		t.Fatalf("open index test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_ = os.RemoveAll(dataPath)
	})

	registry := sources.NewRegistry(sources.NewRepository(db))
	root, err := registry.Create(context.Background(), sources.Input{
		Name:        "Test source",
		Kind:        kind,
		Path:        rootPath,
		ScopeType:   sources.ScopeGlobal,
		LocalAccess: sources.LocalAccessRead,
		AIAccess:    aiAccess,
	})
	if err != nil {
		t.Fatalf("create source root: %v", err)
	}
	repository := NewRepository(db)
	adapters := []Adapter{}
	if reader != nil {
		adapters = append(adapters, NewWorkRecordAdapter(reader))
	}
	adapters = append(
		adapters,
		NewMarkdownAdapter(),
		NewTextAdapter(),
		NewFileAdapter(),
	)
	scanner := NewScanner(
		registry,
		safepath.NewResolver(registry),
		repository,
		adapters...,
	)
	return &indexTestEnvironment{
		rootPath:   rootPath,
		root:       root,
		registry:   registry,
		repository: repository,
		scanner:    scanner,
	}
}

func indexTestDataPath(t *testing.T, name string) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	safeName := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
	dataPath, err := filepath.Abs(filepath.Join(
		workingDirectory,
		"..",
		"..",
		"data",
		"_test_stage3",
		safeName,
	))
	if err != nil {
		t.Fatalf("resolve index test path: %v", err)
	}
	if err := os.RemoveAll(dataPath); err != nil {
		t.Fatalf("reset index test path: %v", err)
	}
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatalf("create index test path: %v", err)
	}
	return dataPath
}

func writeIndexTestFile(
	t *testing.T,
	rootPath string,
	relativePath string,
	content string,
) {
	t.Helper()
	filePath := filepath.Join(rootPath, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		t.Fatalf("create test file parent: %v", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func listIndexDocuments(
	t *testing.T,
	environment *indexTestEnvironment,
) []IndexedDocument {
	t.Helper()
	documents, err := environment.repository.ListBySource(
		context.Background(),
		environment.root.ID,
	)
	if err != nil {
		t.Fatalf("list indexed documents: %v", err)
	}
	return documents
}

func documentsByPath(
	documents []IndexedDocument,
) map[string]IndexedDocument {
	result := make(map[string]IndexedDocument, len(documents))
	for _, document := range documents {
		result[document.RelativePath] = document
	}
	return result
}

func assertDocumentType(
	t *testing.T,
	documents map[string]IndexedDocument,
	relativePath string,
	expected string,
) {
	t.Helper()
	document, ok := documents[relativePath]
	if !ok {
		t.Fatalf("document %q was not indexed", relativePath)
	}
	if document.DocumentType != expected {
		t.Fatalf(
			"expected %q type %q, got %+v",
			relativePath,
			expected,
			document,
		)
	}
}

func hasIssue(issues []ScanIssue, reason string) bool {
	for _, issue := range issues {
		if issue.Reason == reason {
			return true
		}
	}
	return false
}
