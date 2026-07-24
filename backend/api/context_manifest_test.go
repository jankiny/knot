package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	handler := SetupRoutesWithDependencies(Dependencies{
		SourceRegistry:    registry,
		IndexRepository:   indexRepository,
		ContextRepository: contextmanifest.NewRepository(db),
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
