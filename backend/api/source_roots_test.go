package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knot-backend/appdata"
	"knot-backend/sources"
	"knot-backend/storage"
)

func TestSourceRootCRUDAPI(t *testing.T) {
	router, closeDatabase, sourcePath := sourceRootTestRouter(t, "api_crud")
	defer closeDatabase()

	createBody := map[string]any{
		"name":         "工作日志",
		"kind":         "journal",
		"path":         sourcePath,
		"scope_type":   "global",
		"enabled":      true,
		"recursive":    true,
		"local_access": "read",
		"ai_access":    "content",
	}
	createResponse := performJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/source-roots",
		createBody,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	for _, internalField := range []string{"path_key", "created_at", "updated_at"} {
		if strings.Contains(createResponse.Body.String(), internalField) {
			t.Fatalf("response exposed internal field %q: %s", internalField, createResponse.Body.String())
		}
	}

	var created sources.SourceRoot
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created root: %v", err)
	}
	if created.ID == "" || created.Kind != sources.KindJournal {
		t.Fatalf("unexpected created root: %+v", created)
	}

	listResponse := performJSONRequest(t, router, http.MethodGet, "/api/source-roots", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var listBody struct {
		SourceRoots []sources.SourceRoot `json:"source_roots"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode source list: %v", err)
	}
	if len(listBody.SourceRoots) != 1 || listBody.SourceRoots[0].ID != created.ID {
		t.Fatalf("unexpected source list: %+v", listBody.SourceRoots)
	}

	createBody["name"] = "年度日志"
	createBody["enabled"] = false
	updateResponse := performJSONRequest(
		t,
		router,
		http.MethodPut,
		"/api/source-roots/"+created.ID,
		createBody,
	)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	var updated sources.SourceRoot
	if err := json.Unmarshal(updateResponse.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated root: %v", err)
	}
	if updated.Name != "年度日志" || updated.Enabled {
		t.Fatalf("unexpected updated root: %+v", updated)
	}

	deleteResponse := performJSONRequest(
		t,
		router,
		http.MethodDelete,
		"/api/source-roots/"+created.ID,
		nil,
	)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestSourceRootAPIReportsDuplicateAndMissingPaths(t *testing.T) {
	router, closeDatabase, sourcePath := sourceRootTestRouter(t, "api_conflict")
	defer closeDatabase()

	body := map[string]any{
		"name": "Reference",
		"kind": "reference",
		"path": sourcePath,
	}
	first := performJSONRequest(t, router, http.MethodPost, "/api/source-roots", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first create to succeed, got %d", first.Code)
	}

	body["name"] = "Duplicate"
	second := performJSONRequest(t, router, http.MethodPost, "/api/source-roots", body)
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", second.Code, second.Body.String())
	}

	missingPath := filepath.Join(filepath.Dir(sourcePath), "missing-drive")
	body["name"] = "Offline"
	body["path"] = missingPath
	missing := performJSONRequest(t, router, http.MethodPost, "/api/source-roots", body)
	if missing.Code != http.StatusCreated {
		t.Fatalf("expected missing source to be recorded, got %d body=%s", missing.Code, missing.Body.String())
	}
	var root sources.SourceRoot
	if err := json.Unmarshal(missing.Body.Bytes(), &root); err != nil {
		t.Fatalf("decode missing root: %v", err)
	}
	if root.Availability != sources.AvailabilityMissing {
		t.Fatalf("expected missing availability, got %q", root.Availability)
	}
}

func TestSourceRootAPIRejectsUnsupportedAccessEnums(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
	}{
		{name: "local access", field: "local_access", value: "owner"},
		{name: "AI access", field: "ai_access", value: "full_text"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, closeDatabase, sourcePath := sourceRootTestRouter(
				t,
				"api_invalid_"+test.field,
			)
			defer closeDatabase()

			body := map[string]any{
				"name":         "Reference",
				"kind":         "reference",
				"path":         sourcePath,
				"local_access": "read",
				"ai_access":    "content",
			}
			body[test.field] = test.value
			response := performJSONRequest(
				t,
				router,
				http.MethodPost,
				"/api/source-roots",
				body,
			)
			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected 400, got %d body=%s",
					response.Code,
					response.Body.String(),
				)
			}
			if !strings.Contains(response.Body.String(), test.field) {
				t.Fatalf("expected field name in error: %s", response.Body.String())
			}
		})
	}
}

func TestRegisteredNoAIRootAllowsLocalReadWriteButRemainsAIDenied(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	dataPath, err := filepath.Abs(filepath.Join(
		workingDirectory,
		"..",
		"..",
		"data",
		"_test_stage2",
		"api_registered_noai",
	))
	if err != nil {
		t.Fatalf("resolve test data path: %v", err)
	}
	if err := os.RemoveAll(dataPath); err != nil {
		t.Fatalf("reset test data path: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dataPath)
	})

	sourcePath := filepath.Join(dataPath, "PhotoLibrary_NoAI")
	taskPath := filepath.Join(sourcePath, "2026.07.20_city")
	if err := os.MkdirAll(taskPath, 0o700); err != nil {
		t.Fatalf("create task path: %v", err)
	}
	record := `---
type: photo_project
title: 城市照片
task_date: 2026-07-20
ai_access: content
---
# 城市照片

## 当前进展

已完成选片。
`
	if err := os.WriteFile(
		filepath.Join(taskPath, workRecordFileName),
		[]byte(record),
		0o600,
	); err != nil {
		t.Fatalf("write work record: %v", err)
	}

	db, err := storage.Open(context.Background(), appdata.Directory{
		Path:   dataPath,
		Source: appdata.SourceEnvironment,
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	registry := sources.NewRegistry(sources.NewRepository(db))
	createdRoot, err := registry.Create(context.Background(), sources.Input{
		Name:        "照片库",
		Kind:        sources.KindPrivate,
		Path:        sourcePath,
		ScopeType:   sources.ScopeGlobal,
		LocalAccess: sources.LocalAccessReadWrite,
		AIAccess:    sources.AIAccessContent,
	})
	if err != nil {
		t.Fatalf("register source root: %v", err)
	}
	router := SetupRoutesWithDependencies(Dependencies{SourceRegistry: registry})

	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/archive/list?archive_path="+url.QueryEscape(sourcePath),
		nil,
	)
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK ||
		!strings.Contains(listResponse.Body.String(), `"total":1`) {
		t.Fatalf(
			"registered NoAI root was not locally readable: status=%d body=%s",
			listResponse.Code,
			listResponse.Body.String(),
		)
	}

	updateResponse := performJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/archive/update-work-record",
		UpdateWorkRecordRequest{
			FolderPath: taskPath,
			Content:    "# 城市照片\n\n仅在本地更新。",
		},
	)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf(
			"registered read_write NoAI root was not locally writable: status=%d body=%s",
			updateResponse.Code,
			updateResponse.Body.String(),
		)
	}

	if _, err := registry.Update(context.Background(), createdRoot.ID, sources.Input{
		Name:        "照片库",
		Kind:        sources.KindPrivate,
		Path:        sourcePath,
		ScopeType:   sources.ScopeGlobal,
		LocalAccess: sources.LocalAccessRead,
		AIAccess:    sources.AIAccessContent,
	}); err != nil {
		t.Fatalf("restrict source root to read-only: %v", err)
	}
	readOnlyResponse := performJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/archive/update-work-record",
		UpdateWorkRecordRequest{
			FolderPath: taskPath,
			Content:    "# 不应写入",
		},
	)
	if readOnlyResponse.Code != http.StatusForbidden {
		t.Fatalf(
			"read-only NoAI root unexpectedly allowed a write: status=%d body=%s",
			readOnlyResponse.Code,
			readOnlyResponse.Body.String(),
		)
	}

	aiResponse := performJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/report/daily/generate",
		DailyReportGenerateRequest{
			Date:  "2026-07-20",
			Items: []DailyReportItem{{FolderPath: taskPath}},
			AI: DailyReportAIConfig{
				Enabled: true,
				APIURL:  "https://example.com/v1",
				APIKey:  "test-key",
				Model:   "test-model",
			},
		},
	)
	if aiResponse.Code != http.StatusForbidden {
		t.Fatalf(
			"registered NoAI path must remain denied to AI: status=%d body=%s",
			aiResponse.Code,
			aiResponse.Body.String(),
		)
	}
}

func TestUnregisteredSensitivePathRemainsDeniedToLocalWrite(t *testing.T) {
	taskPath := filepath.Join(
		sourceTestDirectoryPath(t, "unregistered_noai"),
		"Unregistered_NoAI",
	)
	if err := os.MkdirAll(taskPath, 0o700); err != nil {
		t.Fatalf("create task path: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(taskPath, workRecordFileName),
		[]byte("# Private task\n"),
		0o600,
	); err != nil {
		t.Fatalf("write work record: %v", err)
	}

	response := performJSONRequest(
		t,
		SetupRoutes(),
		http.MethodPost,
		"/api/archive/update-work-record",
		UpdateWorkRecordRequest{
			FolderPath: taskPath,
			Content:    "# Changed",
		},
	)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestLegacyImportAPIRejectsUnrelatedSettingsFields(t *testing.T) {
	router, closeDatabase, sourcePath := sourceRootTestRouter(t, "api_legacy_unknown")
	defer closeDatabase()

	body := map[string]any{
		"folder_path":           sourcePath,
		"mailPasswordEncrypted": "must-not-be-accepted",
	}
	response := performJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/source-roots/import-legacy",
		body,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestSourceRootAPIWithoutRegistryReturnsServiceUnavailable(t *testing.T) {
	response := performJSONRequest(t, SetupRoutes(), http.MethodGet, "/api/source-roots", nil)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", response.Code, response.Body.String())
	}
}

func sourceRootTestRouter(
	t *testing.T,
	name string,
) (http.Handler, func(), string) {
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
		"_test_stage1",
		safeName,
	))
	if err != nil {
		t.Fatalf("resolve test data path: %v", err)
	}
	if err := os.RemoveAll(dataPath); err != nil {
		t.Fatalf("reset test data path: %v", err)
	}
	sourcePath := filepath.Join(dataPath, "source")
	if err := os.MkdirAll(sourcePath, 0o700); err != nil {
		t.Fatalf("create source path: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dataPath)
	})

	db, err := storage.Open(context.Background(), appdata.Directory{
		Path:   dataPath,
		Source: appdata.SourceEnvironment,
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	registry := sources.NewRegistry(sources.NewRepository(db))
	router := SetupRoutesWithDependencies(Dependencies{SourceRegistry: registry})
	return router, func() {
		_ = db.Close()
	}, sourcePath
}

func sourceTestDirectoryPath(t *testing.T, name string) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	path, err := filepath.Abs(filepath.Join(
		workingDirectory,
		"..",
		"..",
		"data",
		"_test_stage2",
		name,
	))
	if err != nil {
		t.Fatalf("resolve test directory: %v", err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("reset test directory: %v", err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(path)
	})
	return path
}

func performJSONRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
