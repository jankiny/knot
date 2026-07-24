package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knot-backend/annualsummary"
	"knot-backend/appdata"
	"knot-backend/contextmanifest"
	"knot-backend/indexer"
	"knot-backend/sources"
	"knot-backend/storage"
)

func TestContextDiscoverAPIUsesRegisteredSourcesWithoutAcceptingPaths(
	t *testing.T,
) {
	dataPath := sourceTestDirectoryPath(t, "stage4_context_api")
	sourcePath := filepath.Join(dataPath, "journal")
	if err := os.MkdirAll(sourcePath, 0o700); err != nil {
		t.Fatalf("create context API source: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourcePath, "2026.07.20 工作日报.md"),
		[]byte("# 工作日报\n\n完成阶段四上下文发现。"),
		0o600,
	); err != nil {
		t.Fatalf("write context API sample: %v", err)
	}

	db, err := storage.Open(context.Background(), appdata.Directory{
		Path:   dataPath,
		Source: appdata.SourceEnvironment,
	})
	if err != nil {
		t.Fatalf("open context API database: %v", err)
	}
	defer db.Close()
	registry := sources.NewRegistry(sources.NewRepository(db))
	root, err := registry.Create(context.Background(), sources.Input{
		Name:        "工作日志",
		Kind:        sources.KindJournal,
		Path:        sourcePath,
		ScopeType:   sources.ScopeGlobal,
		LocalAccess: sources.LocalAccessRead,
		AIAccess:    sources.AIAccessContent,
	})
	if err != nil {
		t.Fatalf("register context API source: %v", err)
	}
	indexRepository := indexer.NewRepository(db)
	runRepository := annualsummary.NewRepository(db)
	gateway := &annualSummaryGatewayStub{}
	handler := SetupRoutesWithDependencies(Dependencies{
		SourceRegistry:       registry,
		IndexRepository:      indexRepository,
		ContextRepository:    contextmanifest.NewRepository(db),
		AIRunRepository:      runRepository,
		AnnualSummaryGateway: gateway,
	})

	scan := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/"+root.ID+"/scan",
		indexer.ScanOptions{},
	)
	if scan.Code != http.StatusOK {
		t.Fatalf("scan context API sample: %d %s", scan.Code, scan.Body.String())
	}

	discover := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/discover",
		contextmanifest.DiscoverRequest{
			TaskType:    contextmanifest.TaskTypePersonalAnnualSummary,
			Query:       "根据过去一年工作资料生成个人工作总结大纲",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
	)
	if discover.Code != http.StatusCreated {
		t.Fatalf(
			"expected 201, got %d body=%s",
			discover.Code,
			discover.Body.String(),
		)
	}
	var manifest contextmanifest.ContextManifest
	if err := json.Unmarshal(discover.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("decode context API response: %v", err)
	}
	if len(manifest.Evidence) != 1 ||
		manifest.Evidence[0].SourceRootID != root.ID ||
		!manifest.RequiresConfirmation {
		t.Fatalf("unexpected discovered manifest: %+v", manifest)
	}
	if strings.Contains(discover.Body.String(), filepath.ToSlash(sourcePath)) ||
		strings.Contains(discover.Body.String(), sourcePath) {
		t.Fatalf(
			"context API exposed an absolute path: %s",
			discover.Body.String(),
		)
	}

	get := performJSONRequest(
		t,
		handler,
		http.MethodGet,
		"/api/context/"+manifest.ID,
		nil,
	)
	if get.Code != http.StatusOK ||
		!strings.Contains(get.Body.String(), `"valid":true`) {
		t.Fatalf("get persisted manifest: %d %s", get.Code, get.Body.String())
	}

	inputTokens := 321
	outputTokens := 123
	gateway.completion = annualsummary.Completion{
		Content: `{
			"title":"个人年度工作总结大纲",
			"target_word_count":1500,
			"sections":[{
				"heading":"年度工作主线",
				"word_count":1500,
				"outline":["概述已完成的阶段工作，并区分团队成果与个人贡献。"],
				"evidence_ids":["` + manifest.Evidence[0].ID + `"]
			}],
			"verified_results":[{
				"statement":"完成阶段四上下文发现。",
				"evidence_ids":["` + manifest.Evidence[0].ID + `"]
			}],
			"missing_information":[]
		}`,
		InputTokens:  &inputTokens,
		OutputTokens: &outputTokens,
	}
	generate := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/"+manifest.ID+"/generate",
		annualsummary.GenerateRequest{
			EvidenceIDs: []string{manifest.Evidence[0].ID},
			ConfirmSend: true,
			AI: annualsummary.AIConfig{
				ConfigID: "deepseek-v4-flash",
				APIURL:   "https://api.deepseek.com",
				APIKey:   "test-key-must-not-be-audited",
				Model:    "deepseek-v4-flash",
				Enabled:  true,
			},
		},
	)
	if generate.Code != http.StatusCreated {
		t.Fatalf(
			"generate annual summary: %d %s",
			generate.Code,
			generate.Body.String(),
		)
	}
	var generated annualsummary.GenerateResponse
	if err := json.Unmarshal(generate.Body.Bytes(), &generated); err != nil {
		t.Fatalf("decode annual summary response: %v", err)
	}
	if generated.RunID == "" ||
		generated.EvidenceCount != 1 ||
		!strings.Contains(generated.Result.Markdown, manifest.Evidence[0].ID) {
		t.Fatalf("unexpected annual summary response: %+v", generated)
	}
	if gateway.calls != 1 ||
		len(gateway.input.Evidence) != 1 ||
		gateway.input.Evidence[0].ID != manifest.Evidence[0].ID {
		t.Fatalf("gateway received unexpected evidence: %+v", gateway.input)
	}
	gatewayPayload, err := json.Marshal(gateway.input)
	if err != nil {
		t.Fatalf("encode gateway input: %v", err)
	}
	if strings.Contains(string(gatewayPayload), sourcePath) ||
		strings.Contains(string(gatewayPayload), "excluded") ||
		strings.Contains(string(gatewayPayload), "unavailable_sources") {
		t.Fatalf("gateway input crossed context boundary: %s", gatewayPayload)
	}

	runAudit := performJSONRequest(
		t,
		handler,
		http.MethodGet,
		"/api/ai-runs/"+generated.RunID,
		nil,
	)
	if runAudit.Code != http.StatusOK ||
		!strings.Contains(runAudit.Body.String(), `"status":"success"`) ||
		strings.Contains(runAudit.Body.String(), "test-key-must-not-be-audited") ||
		strings.Contains(runAudit.Body.String(), sourcePath) ||
		strings.Contains(runAudit.Body.String(), `"excerpt"`) {
		t.Fatalf("unsafe or incomplete AI run audit: %s", runAudit.Body.String())
	}
	runSources := performJSONRequest(
		t,
		handler,
		http.MethodGet,
		"/api/ai-runs/"+generated.RunID+"/sources",
		nil,
	)
	if runSources.Code != http.StatusOK ||
		!strings.Contains(runSources.Body.String(), manifest.Evidence[0].ID) ||
		strings.Contains(runSources.Body.String(), sourcePath) ||
		strings.Contains(runSources.Body.String(), "完成阶段四上下文发现。") {
		t.Fatalf("unsafe or incomplete AI run sources: %s", runSources.Body.String())
	}

	unconfirmed := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/"+manifest.ID+"/generate",
		annualsummary.GenerateRequest{
			EvidenceIDs: []string{manifest.Evidence[0].ID},
			AI: annualsummary.AIConfig{
				ConfigID: "deepseek-v4-flash",
				APIURL:   "https://api.deepseek.com",
				APIKey:   "test-key",
				Model:    "deepseek-v4-flash",
				Enabled:  true,
			},
		},
	)
	if unconfirmed.Code != http.StatusBadRequest || gateway.calls != 1 {
		t.Fatalf(
			"unconfirmed generation reached gateway: %d %s",
			unconfirmed.Code,
			unconfirmed.Body.String(),
		)
	}

	unapprovedDestination := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/"+manifest.ID+"/generate",
		annualsummary.GenerateRequest{
			EvidenceIDs: []string{manifest.Evidence[0].ID},
			ConfirmSend: true,
			AI: annualsummary.AIConfig{
				ConfigID: "custom",
				APIURL:   "https://example.invalid/v1",
				APIKey:   "test-key",
				Model:    "custom",
				Enabled:  true,
			},
		},
	)
	if unapprovedDestination.Code != http.StatusBadRequest || gateway.calls != 1 {
		t.Fatalf(
			"unapproved destination reached gateway: %d %s",
			unapprovedDestination.Code,
			unapprovedDestination.Body.String(),
		)
	}

	if err := os.WriteFile(
		filepath.Join(sourcePath, "2026.07.20 工作日报.md"),
		[]byte("# 工作日报\n\n源文件已在预览后变化，需要重新发现。"),
		0o600,
	); err != nil {
		t.Fatalf("change context API sample: %v", err)
	}
	rescan := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/"+root.ID+"/scan",
		indexer.ScanOptions{Force: true},
	)
	if rescan.Code != http.StatusOK {
		t.Fatalf("rescan changed evidence: %d %s", rescan.Code, rescan.Body.String())
	}
	invalidManifest := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/"+manifest.ID+"/generate",
		annualsummary.GenerateRequest{
			EvidenceIDs: []string{manifest.Evidence[0].ID},
			ConfirmSend: true,
			AI: annualsummary.AIConfig{
				ConfigID: "deepseek-v4-flash",
				APIURL:   "https://api.deepseek.com",
				APIKey:   "test-key",
				Model:    "deepseek-v4-flash",
				Enabled:  true,
			},
		},
	)
	if invalidManifest.Code != http.StatusConflict || gateway.calls != 1 {
		t.Fatalf(
			"invalid manifest reached gateway: %d %s",
			invalidManifest.Code,
			invalidManifest.Body.String(),
		)
	}

	pathAttempt := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/discover",
		map[string]any{
			"task_type":    contextmanifest.TaskTypePersonalAnnualSummary,
			"query":        "年度总结",
			"period_start": "2025-07-24",
			"period_end":   "2026-07-24",
			"scan_paths":   []string{sourcePath},
		},
	)
	if pathAttempt.Code != http.StatusBadRequest {
		t.Fatalf(
			"context API accepted paths: %d %s",
			pathAttempt.Code,
			pathAttempt.Body.String(),
		)
	}

	missing := performJSONRequest(
		t,
		handler,
		http.MethodGet,
		"/api/context/context_forged",
		nil,
	)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected unknown manifest to return 404, got %d", missing.Code)
	}
}

func TestContextDiscoverAPIWithoutPersistentServicesIsUnavailable(t *testing.T) {
	response := performJSONRequest(
		t,
		SetupRoutes(),
		http.MethodPost,
		"/api/context/discover",
		contextmanifest.DiscoverRequest{
			TaskType:    contextmanifest.TaskTypePersonalAnnualSummary,
			Query:       "年度总结",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
	)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestAnnualSummaryFailureIsAuditedWithoutModelContent(t *testing.T) {
	dataPath := sourceTestDirectoryPath(t, "stage5_failed_audit")
	sourcePath := filepath.Join(dataPath, "journal")
	if err := os.MkdirAll(sourcePath, 0o700); err != nil {
		t.Fatalf("create failed-audit source: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourcePath, "2026.07.20 工作日报.md"),
		[]byte("# 工作日报\n\n完成只读智能闭环测试。"),
		0o600,
	); err != nil {
		t.Fatalf("write failed-audit sample: %v", err)
	}

	db, err := storage.Open(context.Background(), appdata.Directory{
		Path:   dataPath,
		Source: appdata.SourceEnvironment,
	})
	if err != nil {
		t.Fatalf("open failed-audit database: %v", err)
	}
	defer db.Close()
	registry := sources.NewRegistry(sources.NewRepository(db))
	root, err := registry.Create(context.Background(), sources.Input{
		Name:        "工作日志",
		Kind:        sources.KindJournal,
		Path:        sourcePath,
		ScopeType:   sources.ScopeGlobal,
		LocalAccess: sources.LocalAccessRead,
		AIAccess:    sources.AIAccessContent,
	})
	if err != nil {
		t.Fatalf("register failed-audit source: %v", err)
	}
	gateway := &annualSummaryGatewayStub{
		completion: annualsummary.Completion{
			Content: `{"title":"invalid"}`,
		},
	}
	handler := SetupRoutesWithDependencies(Dependencies{
		SourceRegistry:       registry,
		IndexRepository:      indexer.NewRepository(db),
		ContextRepository:    contextmanifest.NewRepository(db),
		AIRunRepository:      annualsummary.NewRepository(db),
		AnnualSummaryGateway: gateway,
	})
	scan := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/"+root.ID+"/scan",
		indexer.ScanOptions{},
	)
	if scan.Code != http.StatusOK {
		t.Fatalf("scan failed-audit source: %d %s", scan.Code, scan.Body.String())
	}
	discover := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/discover",
		contextmanifest.DiscoverRequest{
			TaskType:    contextmanifest.TaskTypePersonalAnnualSummary,
			Query:       "生成个人工作总结大纲",
			PeriodStart: "2025-07-24",
			PeriodEnd:   "2026-07-24",
		},
	)
	var manifest contextmanifest.ContextManifest
	if err := json.Unmarshal(discover.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("decode failed-audit manifest: %v", err)
	}
	generate := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/context/"+manifest.ID+"/generate",
		annualsummary.GenerateRequest{
			EvidenceIDs: []string{manifest.Evidence[0].ID},
			ConfirmSend: true,
			AI: annualsummary.AIConfig{
				ConfigID: "deepseek-v4-flash",
				APIURL:   "https://api.deepseek.com",
				APIKey:   "failed-audit-key",
				Model:    "deepseek-v4-flash",
				Enabled:  true,
			},
		},
	)
	if generate.Code != http.StatusBadGateway {
		t.Fatalf("expected model JSON failure, got %d %s", generate.Code, generate.Body.String())
	}
	var failure struct {
		RunID string `json:"ai_run_id"`
	}
	if err := json.Unmarshal(generate.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode failed generation: %v", err)
	}
	audit := performJSONRequest(
		t,
		handler,
		http.MethodGet,
		"/api/ai-runs/"+failure.RunID,
		nil,
	)
	if audit.Code != http.StatusOK ||
		!strings.Contains(audit.Body.String(), `"status":"failed"`) ||
		!strings.Contains(audit.Body.String(), annualsummary.ErrorCodeInvalidModelOutput) ||
		strings.Contains(audit.Body.String(), "failed-audit-key") ||
		strings.Contains(audit.Body.String(), `{"title":"invalid"}`) {
		t.Fatalf("unsafe failed AI run audit: %s", audit.Body.String())
	}
}

type annualSummaryGatewayStub struct {
	completion annualsummary.Completion
	err        error
	input      annualsummary.ModelInput
	calls      int
}

func (gateway *annualSummaryGatewayStub) Complete(
	_ context.Context,
	_ annualsummary.AIConfig,
	input annualsummary.ModelInput,
) (annualsummary.Completion, error) {
	gateway.calls++
	gateway.input = input
	if gateway.err != nil {
		return annualsummary.Completion{}, gateway.err
	}
	if gateway.completion.Content == "" {
		return annualsummary.Completion{}, errors.New("missing test completion")
	}
	return gateway.completion, nil
}
