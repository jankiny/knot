package contextmanifest

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"knot-backend/appdata"
	"knot-backend/indexer"
	"knot-backend/safepath"
	"knot-backend/sources"
	"knot-backend/storage"
)

func TestDiscoverBuildsStablePolicyCheckedEvidenceManifest(t *testing.T) {
	environment := newContextTestEnvironment(t, "discover")
	journal := environment.createRoot(
		t,
		"工作日志",
		sources.KindJournal,
		"journal",
		sources.AIAccessContent,
		true,
	)
	current := environment.createRoot(
		t,
		"当前工作",
		sources.KindCurrentWork,
		"current",
		sources.AIAccessContent,
		true,
	)
	metadata := environment.createRoot(
		t,
		"归档元数据",
		sources.KindWorkArchive,
		"metadata",
		sources.AIAccessMetadata,
		true,
	)
	noAI := environment.createRoot(
		t,
		"禁止 AI",
		sources.KindCurrentWork,
		"NoAI_private",
		sources.AIAccessContent,
		true,
	)
	offline := environment.createRoot(
		t,
		"离线归档",
		sources.KindWorkArchive,
		"missing_archive",
		sources.AIAccessContent,
		false,
	)

	writeContextTestFile(
		t,
		journal.Path,
		"2026.07.01 工作日报.md",
		"# 2026.07.01 工作日报\n\n完成接口联调，推进安全测试。",
	)
	writeContextTestFile(
		t,
		current.Path,
		"2026.06.20_Knot/工作记录.md",
		"完成 Context Manifest，并交付阶段成果。",
	)
	writeContextTestFile(
		t,
		current.Path,
		"2026.06.21_Restricted/工作记录.md",
		"NOAI_OVERRIDE 不得进入 evidence。",
	)
	writeContextTestFile(
		t,
		metadata.Path,
		"2026.05.10 项目归档.md",
		"# 项目归档\n\n正文不得进入 evidence。",
	)
	writeContextTestFile(
		t,
		noAI.Path,
		"2026.04.01 私密日报.md",
		"# 私密日报\n\n不得进入 evidence。",
	)

	for _, root := range []sources.SourceRoot{journal, current, metadata, noAI} {
		if _, err := environment.scanner.ScanSource(
			context.Background(),
			root.ID,
			indexer.ScanOptions{},
		); err != nil {
			t.Fatalf("scan source %q: %v", root.ID, err)
		}
	}

	request := DiscoverRequest{
		TaskType:    TaskTypePersonalAnnualSummary,
		Query:       "根据过去一年工作资料生成个人工作总结大纲",
		PeriodStart: "2025-07-24",
		PeriodEnd:   "2026-07-24",
	}
	first, err := environment.resolver.Discover(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatalf("discover context: %v", err)
	}
	second, err := environment.resolver.Discover(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatalf("repeat context discovery: %v", err)
	}

	if first.ID == second.ID {
		t.Fatal("separate persisted manifests must have different IDs")
	}
	if !first.Valid || first.Status != StatusReady ||
		!first.RequiresConfirmation || first.RequiresRediscovery {
		t.Fatalf("unexpected manifest state: %+v", first)
	}
	if len(first.Evidence) != 3 {
		t.Fatalf("expected three allowed evidence items, got %+v", first.Evidence)
	}
	if first.Evidence[0].SourceType != indexer.DocumentTypeWorkRecord {
		t.Fatalf("work record should rank first, got %+v", first.Evidence)
	}
	if !sameEvidenceOrder(first.Evidence, second.Evidence) {
		t.Fatalf(
			"same request and index must produce stable evidence: first=%+v second=%+v",
			first.Evidence,
			second.Evidence,
		)
	}

	var metadataEvidence *EvidenceItem
	for index := range first.Evidence {
		item := &first.Evidence[index]
		if item.SourceRootID == noAI.ID {
			t.Fatalf("NoAI evidence leaked into manifest: %+v", item)
		}
		if item.SourceRootID == metadata.ID {
			metadataEvidence = item
		}
	}
	if metadataEvidence == nil ||
		metadataEvidence.AIAccessEffective != sources.AIAccessMetadata ||
		metadataEvidence.Excerpt != "" {
		t.Fatalf("metadata evidence exposed content: %+v", metadataEvidence)
	}
	if !hasExcludedReason(first.Excluded, "sensitive_path_marker") {
		t.Fatalf("missing NoAI exclusion explanation: %+v", first.Excluded)
	}
	if !hasExcludedReason(first.Excluded, "ai_access_denied") {
		t.Fatalf(
			"missing work-record override exclusion: %+v",
			first.Excluded,
		)
	}
	if len(first.UnavailableSources) != 1 ||
		first.UnavailableSources[0].SourceRootID != offline.ID ||
		first.UnavailableSources[0].ReasonCode != "root_offline" {
		t.Fatalf(
			"offline source was not returned as a gap: %+v",
			first.UnavailableSources,
		)
	}
	if first.EstimatedInputTokens > first.TokenBudget {
		t.Fatalf("token budget exceeded: %+v", first)
	}

	payload, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	for _, absolutePath := range []string{
		journal.Path,
		current.Path,
		metadata.Path,
		noAI.Path,
	} {
		if strings.Contains(string(payload), absolutePath) {
			t.Fatalf("manifest exposed absolute path %q: %s", absolutePath, payload)
		}
	}
}

func TestDiscoverEnforcesEvidenceBudgetAndExplainsExclusion(t *testing.T) {
	environment := newContextTestEnvironment(t, "budget")
	root := environment.createRoot(
		t,
		"日志",
		sources.KindJournal,
		"journal",
		sources.AIAccessContent,
		true,
	)
	writeContextTestFile(
		t,
		root.Path,
		"2026.07.01 工作日报.md",
		"# 日报\n\n完成第一项工作。",
	)
	writeContextTestFile(
		t,
		root.Path,
		"2026.07.02 工作日报.md",
		"# 日报\n\n完成第二项工作。",
	)
	if _, err := environment.scanner.ScanSource(
		context.Background(),
		root.ID,
		indexer.ScanOptions{},
	); err != nil {
		t.Fatalf("scan budget source: %v", err)
	}
	environment.resolver.limits.MaxEvidence = 1

	manifest, err := environment.resolver.Discover(
		context.Background(),
		DiscoverRequest{
			TaskType:    TaskTypePersonalAnnualSummary,
			Query:       "年度工作总结",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
	)
	if err != nil {
		t.Fatalf("discover bounded context: %v", err)
	}
	if len(manifest.Evidence) != 1 ||
		!hasExcludedReason(manifest.Excluded, ReasonTokenBudget) {
		t.Fatalf("evidence limit was not explained: %+v", manifest)
	}
}

func TestDiscoverRejectsFileChangedAfterIndexing(t *testing.T) {
	environment := newContextTestEnvironment(t, "outdated_index")
	root := environment.createRoot(
		t,
		"日志",
		sources.KindJournal,
		"journal",
		sources.AIAccessContent,
		true,
	)
	relativePath := "2026.07.02 工作日报.md"
	writeContextTestFile(
		t,
		root.Path,
		relativePath,
		"# 日报\n\n完成初始工作。",
	)
	if _, err := environment.scanner.ScanSource(
		context.Background(),
		root.ID,
		indexer.ScanOptions{},
	); err != nil {
		t.Fatalf("scan source: %v", err)
	}
	writeContextTestFile(
		t,
		root.Path,
		relativePath,
		"# 日报\n\n文件已在索引后变化，且尚未重新扫描。",
	)

	manifest, err := environment.resolver.Discover(
		context.Background(),
		DiscoverRequest{
			TaskType:    TaskTypePersonalAnnualSummary,
			Query:       "年度工作总结",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
	)
	if err != nil {
		t.Fatalf("discover with outdated index: %v", err)
	}
	if len(manifest.Evidence) != 0 ||
		!hasExcludedReason(manifest.Excluded, ReasonIndexOutdated) {
		t.Fatalf("outdated index was used as evidence: %+v", manifest)
	}
}

func TestManifestInvalidatesOnHashPolicyAndExpiryChanges(t *testing.T) {
	environment := newContextTestEnvironment(t, "invalidation")
	root := environment.createRoot(
		t,
		"日志",
		sources.KindJournal,
		"journal",
		sources.AIAccessContent,
		true,
	)
	relativePath := "2026.07.03 工作日报.md"
	writeContextTestFile(
		t,
		root.Path,
		relativePath,
		"# 日报\n\n完成初始版本。",
	)
	if _, err := environment.scanner.ScanSource(
		context.Background(),
		root.ID,
		indexer.ScanOptions{},
	); err != nil {
		t.Fatalf("scan source: %v", err)
	}
	manifest, err := environment.resolver.Discover(
		context.Background(),
		DiscoverRequest{
			TaskType:    TaskTypePersonalAnnualSummary,
			Query:       "年度工作总结",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
	)
	if err != nil {
		t.Fatalf("discover context: %v", err)
	}

	writeContextTestFile(
		t,
		root.Path,
		relativePath,
		"# 日报\n\n完成已经变化且长度不同的新版本。",
	)
	if _, err := environment.scanner.ScanSource(
		context.Background(),
		root.ID,
		indexer.ScanOptions{Force: true},
	); err != nil {
		t.Fatalf("rescan changed source: %v", err)
	}
	changed, err := environment.resolver.Get(
		context.Background(),
		manifest.ID,
	)
	if err != nil {
		t.Fatalf("get changed manifest: %v", err)
	}
	if changed.Valid ||
		!containsString(changed.InvalidatedReasons, ReasonDocumentHashChanged) {
		t.Fatalf("hash change did not invalidate manifest: %+v", changed)
	}

	enabled := false
	_, err = environment.registry.Update(
		context.Background(),
		root.ID,
		sources.Input{
			Name:        root.Name,
			Kind:        root.Kind,
			Path:        root.Path,
			ScopeType:   root.ScopeType,
			Enabled:     &enabled,
			LocalAccess: root.LocalAccess,
			AIAccess:    sources.AIAccessNone,
		},
	)
	if err != nil {
		t.Fatalf("restrict source policy: %v", err)
	}
	policyChanged, err := environment.resolver.Get(
		context.Background(),
		manifest.ID,
	)
	if err != nil {
		t.Fatalf("get policy-changed manifest: %v", err)
	}
	if !containsString(policyChanged.InvalidatedReasons, "root_disabled") {
		t.Fatalf("policy change did not invalidate manifest: %+v", policyChanged)
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, manifest.ExpiresAt)
	if err != nil {
		t.Fatalf("parse manifest expiry: %v", err)
	}
	environment.resolver.now = func() time.Time {
		return expiresAt.Add(time.Second)
	}
	expired, err := environment.resolver.Get(
		context.Background(),
		manifest.ID,
	)
	if err != nil {
		t.Fatalf("get expired manifest: %v", err)
	}
	if !containsString(expired.InvalidatedReasons, ReasonManifestExpired) {
		t.Fatalf("expiry did not invalidate manifest: %+v", expired)
	}
}

func TestDiscoverRejectsUnsupportedTaskAndInvalidPeriods(t *testing.T) {
	environment := newContextTestEnvironment(t, "validation")
	testCases := []DiscoverRequest{
		{
			TaskType:    "other",
			Query:       "总结",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
		{
			TaskType:    TaskTypePersonalAnnualSummary,
			Query:       "",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
		{
			TaskType:    TaskTypePersonalAnnualSummary,
			Query:       "总结",
			PeriodStart: "2026-07-24",
			PeriodEnd:   "2025-07-24",
		},
	}
	for _, request := range testCases {
		if _, err := environment.resolver.Discover(
			context.Background(),
			request,
		); err == nil {
			t.Fatalf("expected validation error for %+v", request)
		}
	}
}

type contextTestEnvironment struct {
	dataPath        string
	db              *sql.DB
	registry        *sources.Registry
	indexRepository *indexer.Repository
	scanner         *indexer.Scanner
	resolver        *Resolver
}

func newContextTestEnvironment(
	t *testing.T,
	name string,
) *contextTestEnvironment {
	t.Helper()
	dataPath := contextTestDataPath(t, name)
	db, err := storage.Open(context.Background(), appdata.Directory{
		Path:   dataPath,
		Source: appdata.SourceEnvironment,
	})
	if err != nil {
		t.Fatalf("open context test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_ = os.RemoveAll(dataPath)
	})

	registry := sources.NewRegistry(sources.NewRepository(db))
	indexRepository := indexer.NewRepository(db)
	pathResolver := safepath.NewResolver(registry)
	scanner := indexer.NewScanner(
		registry,
		pathResolver,
		indexRepository,
		indexer.NewWorkRecordAdapter(func(filePath string) (
			indexer.WorkRecordData,
			error,
		) {
			data, err := os.ReadFile(filePath)
			aiAccess := "content"
			if strings.Contains(string(data), "NOAI_OVERRIDE") {
				aiAccess = "noai"
			}
			return indexer.WorkRecordData{
				RawContent: data,
				Title:      "Knot 智能闭环",
				TaskDate:   "2026-06-20",
				Content:    string(data),
				AIAccess:   aiAccess,
			}, err
		}),
		indexer.NewMarkdownAdapter(),
		indexer.NewTextAdapter(),
		indexer.NewFileAdapter(),
	)
	resolver := NewResolver(
		registry,
		pathResolver,
		indexRepository,
		NewRepository(db),
	)
	return &contextTestEnvironment{
		dataPath:        dataPath,
		db:              db,
		registry:        registry,
		indexRepository: indexRepository,
		scanner:         scanner,
		resolver:        resolver,
	}
}

func (environment *contextTestEnvironment) createRoot(
	t *testing.T,
	name string,
	kind sources.Kind,
	directoryName string,
	aiAccess sources.AIAccess,
	createDirectory bool,
) sources.SourceRoot {
	t.Helper()
	rootPath := filepath.Join(environment.dataPath, directoryName)
	if createDirectory {
		if err := os.MkdirAll(rootPath, 0o700); err != nil {
			t.Fatalf("create context source root: %v", err)
		}
	}
	root, err := environment.registry.Create(
		context.Background(),
		sources.Input{
			Name:        name,
			Kind:        kind,
			Path:        rootPath,
			ScopeType:   sources.ScopeGlobal,
			LocalAccess: sources.LocalAccessRead,
			AIAccess:    aiAccess,
		},
	)
	if err != nil {
		t.Fatalf("register context source root: %v", err)
	}
	return root
}

func contextTestDataPath(t *testing.T, name string) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	safeName := strings.NewReplacer("/", "_", "\\", "_", " ", "_").
		Replace(name)
	dataPath, err := filepath.Abs(filepath.Join(
		workingDirectory,
		"..",
		"..",
		"data",
		"_test_stage4",
		safeName,
	))
	if err != nil {
		t.Fatalf("resolve context test path: %v", err)
	}
	if err := os.RemoveAll(dataPath); err != nil {
		t.Fatalf("reset context test path: %v", err)
	}
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatalf("create context test path: %v", err)
	}
	return dataPath
}

func writeContextTestFile(
	t *testing.T,
	rootPath string,
	relativePath string,
	content string,
) {
	t.Helper()
	filePath := filepath.Join(rootPath, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		t.Fatalf("create context test file parent: %v", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write context test file: %v", err)
	}
}

func sameEvidenceOrder(left, right []EvidenceItem) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID ||
			left[index].DocumentID != right[index].DocumentID {
			return false
		}
	}
	return true
}

func hasExcludedReason(items []ExcludedItem, reason string) bool {
	for _, item := range items {
		if item.ReasonCode == reason {
			return true
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
