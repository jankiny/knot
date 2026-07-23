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
	"knot-backend/indexer"
	"knot-backend/sources"
	"knot-backend/storage"
)

func TestIndexScanAPIUsesRegisteredRootAndExistingWorkRecordParser(t *testing.T) {
	handler, repository, root, closeDatabase := indexScanTestHandler(
		t,
		"api_index_scan",
		"source",
	)
	defer closeDatabase()

	taskPath := filepath.Join(root.Path, "2026.07.23_阶段三")
	if err := os.MkdirAll(taskPath, 0o700); err != nil {
		t.Fatalf("create task path: %v", err)
	}
	record := `---
type: task
schema_version: 3
title: 阶段三增量索引
created: 2026-07-23
task_date: 2026-07-23
project: Knot
ai_access: content
---
# 阶段三增量索引

## 工作内容

完成 migration 与有限扫描。

## 当前进展

所有候选均通过安全路径解析。
`
	if err := os.WriteFile(
		filepath.Join(taskPath, workRecordFileName),
		[]byte(record),
		0o600,
	); err != nil {
		t.Fatalf("write work record: %v", err)
	}

	response := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/"+root.ID+"/scan",
		indexer.ScanOptions{},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	var result indexer.ScanResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode scan result: %v", err)
	}
	if result.Status != indexer.ScanStatusCompleted || result.Indexed != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}
	if strings.Contains(response.Body.String(), filepath.ToSlash(root.Path)) {
		t.Fatalf("scan response exposed an absolute path: %s", response.Body.String())
	}

	documents, err := repository.ListBySource(context.Background(), root.ID)
	if err != nil {
		t.Fatalf("list indexed documents: %v", err)
	}
	if len(documents) != 1 ||
		documents[0].DocumentType != indexer.DocumentTypeWorkRecord ||
		documents[0].Title != "阶段三增量索引" ||
		documents[0].TaskDate != "2026-07-23" ||
		!strings.Contains(documents[0].ContentExcerpt, "migration") {
		t.Fatalf("unexpected indexed work record: %+v", documents)
	}

	second := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/"+root.ID+"/scan",
		nil,
	)
	if second.Code != http.StatusOK ||
		!strings.Contains(second.Body.String(), `"unchanged":1`) {
		t.Fatalf("expected incremental rescan, got %d %s", second.Code, second.Body.String())
	}

	deletedRoot := performJSONRequest(
		t,
		handler,
		http.MethodDelete,
		"/api/source-roots/"+root.ID,
		nil,
	)
	if deletedRoot.Code != http.StatusNoContent {
		t.Fatalf(
			"source deletion regressed after indexing: %d %s",
			deletedRoot.Code,
			deletedRoot.Body.String(),
		)
	}
	documents, err = repository.ListBySource(context.Background(), root.ID)
	if err != nil || len(documents) != 0 {
		t.Fatalf("rebuildable index rows were not cascaded: %+v err=%v", documents, err)
	}
}

func TestIndexScanAllAndRequestContract(t *testing.T) {
	handler, _, root, closeDatabase := indexScanTestHandler(
		t,
		"api_scan_all",
		"source",
	)
	defer closeDatabase()
	if err := os.WriteFile(
		filepath.Join(root.Path, "note.txt"),
		[]byte("metadata"),
		0o600,
	); err != nil {
		t.Fatalf("write scan sample: %v", err)
	}

	all := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/scan",
		map[string]any{"force": true},
	)
	if all.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", all.Code, all.Body.String())
	}
	var result indexer.ScanAllResult
	if err := json.Unmarshal(all.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode all scan result: %v", err)
	}
	if result.SourceRoots != 1 ||
		len(result.Results) != 1 ||
		result.Results[0].SourceRootID != root.ID {
		t.Fatalf("unexpected all-root scan: %+v", result)
	}

	absolutePathAttempt := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/"+root.ID+"/scan",
		map[string]any{
			"force":         false,
			"absolute_path": root.Path,
		},
	)
	if absolutePathAttempt.Code != http.StatusBadRequest {
		t.Fatalf(
			"scan API unexpectedly accepted an absolute path: %d %s",
			absolutePathAttempt.Code,
			absolutePathAttempt.Body.String(),
		)
	}

	missing := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/source-roots/root_forged/scan",
		nil,
	)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected unknown root ID to return 404, got %d", missing.Code)
	}
}

func TestIndexScanAPIWithoutIndexRepositoryIsUnavailable(t *testing.T) {
	response := performJSONRequest(
		t,
		SetupRoutes(),
		http.MethodPost,
		"/api/source-roots/root_missing/scan",
		nil,
	)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", response.Code, response.Body.String())
	}
}

func indexScanTestHandler(
	t *testing.T,
	name string,
	sourceDirectoryName string,
) (http.Handler, *indexer.Repository, sources.SourceRoot, func()) {
	t.Helper()
	dataPath := sourceTestDirectoryPath(t, "stage3_"+name)
	sourcePath := filepath.Join(dataPath, sourceDirectoryName)
	if err := os.MkdirAll(sourcePath, 0o700); err != nil {
		t.Fatalf("create index API source: %v", err)
	}
	db, err := storage.Open(context.Background(), appdata.Directory{
		Path:   dataPath,
		Source: appdata.SourceEnvironment,
	})
	if err != nil {
		t.Fatalf("open index API database: %v", err)
	}
	registry := sources.NewRegistry(sources.NewRepository(db))
	root, err := registry.Create(context.Background(), sources.Input{
		Name:        "阶段三资料",
		Kind:        sources.KindCurrentWork,
		Path:        sourcePath,
		ScopeType:   sources.ScopeGlobal,
		LocalAccess: sources.LocalAccessRead,
		AIAccess:    sources.AIAccessContent,
	})
	if err != nil {
		_ = db.Close()
		t.Fatalf("register index API source: %v", err)
	}
	repository := indexer.NewRepository(db)
	handler := SetupRoutesWithDependencies(Dependencies{
		SourceRegistry:  registry,
		IndexRepository: repository,
	})
	return handler, repository, root, func() {
		_ = db.Close()
	}
}
