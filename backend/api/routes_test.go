package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetBaseFolder_WithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	result := getBaseFolder(tmpDir)
	if result != tmpDir {
		t.Fatalf("expected %s, got %s", tmpDir, result)
	}
}

func TestGetBaseFolder_TildePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := getBaseFolder("~/knot_test_home_path")
	expected := filepath.Join(home, "knot_test_home_path")
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
	_ = os.RemoveAll(expected)
}

func TestSetupRoutes_AllEndpointsRegistered(t *testing.T) {
	router := SetupRoutes()
	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/api/mail/connect"},
		{"GET", "/api/mail/list"},
		{"GET", "/api/mail/123/attachments"},
		{"GET", "/api/mail/123/detail"},
		{"POST", "/api/folder/create"},
		{"POST", "/api/folder/create-with-attachments"},
		{"GET", "/api/folder/check-hash"},
		{"GET", "/api/archive/scan"},
		{"POST", "/api/archive/move"},
		{"POST", "/api/archive/batch-move"},
		{"POST", "/api/archive/update-work-record"},
		{"POST", "/api/report/daily/generate"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code == http.StatusNotFound {
			t.Fatalf("%s %s returned 404", ep.method, ep.path)
		}
		if rr.Code == http.StatusMethodNotAllowed {
			t.Fatalf("%s %s returned 405", ep.method, ep.path)
		}
	}
}

func TestHandleCreateFolder_ManualStructure_NewTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		BasePath:   tmpDir,
		FolderName: "2026.04.20_manual_task",
		Subject:    "Material Cleanup",
		Source:     "manual",
		Department: "ops",
		Hash:       "manualhash001",
	}

	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/folder/create", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	folderPath := filepath.Join(tmpDir, "2026.04.20_manual_task")
	mustExist := []string{
		filepath.Join(folderPath, taskSourceDirName),
		filepath.Join(folderPath, taskProcessDirName),
		filepath.Join(folderPath, taskOutputDirName),
		filepath.Join(folderPath, workRecordFileName),
	}
	for _, p := range mustExist {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Fatalf("expected path not found: %s", p)
		}
	}

	wrContent, _ := os.ReadFile(filepath.Join(folderPath, workRecordFileName))
	wr := string(wrContent)
	for _, expect := range []string{
		"schema_version: 3",
		"title: Material Cleanup",
		"department: ops",
		"folder_name: 2026.04.20_manual_task",
	} {
		if !strings.Contains(wr, expect) {
			t.Fatalf("work record missing %q\n%s", expect, wr)
		}
	}
}

func TestReadWorkRecord_ExtractCoreContent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, workRecordFileName)
	content := `---
type: task
schema_version: 3
title: API Sync
status: active
created: 2026-04-20
updated: 2026-04-20
source: manual
department: dev
project_path: C:/Task
folder_name: 2026.04.20_api_sync
archive_status: local_active
hash: h001
---

# API Sync

This task is progressing with new integration updates.`
	_ = os.WriteFile(filePath, []byte(content), 0o644)

	parsed, err := readWorkRecord(filePath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if parsed.Info.Title != "API Sync" {
		t.Fatalf("title mismatch: %s", parsed.Info.Title)
	}
	if strings.TrimSpace(parsed.Info.Content) == "" {
		t.Fatalf("expected core content")
	}
	if !strings.Contains(parsed.Info.RawContent, "API Sync") {
		t.Fatalf("expected raw content to keep original body")
	}
}

func TestHandleScanWorkFolders_ReturnsCoreAndRawContent(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "2026.04.20_task_a")
	_ = os.MkdirAll(taskDir, 0o755)

	workRecord := `---
type: task
schema_version: 3
title: Data Pack
status: active
created: 2026-04-20
updated: 2026-04-20
source: manual
department: pm
project_path: C:/Task
folder_name: 2026.04.20_task_a
archive_status: local_active
hash: hashabc
---
# Data Pack

Core progress note.`
	_ = os.WriteFile(filepath.Join(taskDir, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, "/api/archive/scan?scan_path="+tmpDir, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Count   int `json:"count"`
		Folders []struct {
			Content    string `json:"content"`
			RawContent string `json:"raw_content"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Count != 1 || len(resp.Folders) != 1 {
		t.Fatalf("expected one folder, got count=%d len=%d", resp.Count, len(resp.Folders))
	}
	if strings.TrimSpace(resp.Folders[0].Content) == "" {
		t.Fatalf("expected non-empty core content")
	}
	if !strings.Contains(resp.Folders[0].RawContent, "# Data Pack") {
		t.Fatalf("expected raw content with markdown title")
	}
}

func TestHandleUpdateWorkRecord_RenameFolderAndUpdateFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	folder := filepath.Join(tmpDir, "2026.04.20_old-title")
	_ = os.MkdirAll(folder, 0o755)

	workRecord := `---
type: task
schema_version: 3
title: old-title
status: active
created: 2026-04-20
updated: 2026-04-20
source: manual
department: ops
project_path: C:/Task
folder_name: 2026.04.20_old-title
archive_status: local_active
hash: h001
---

# old-title

old content`
	_ = os.WriteFile(filepath.Join(folder, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	body, _ := json.Marshal(UpdateWorkRecordRequest{
		FolderPath:   folder,
		Title:        "new-title",
		Content:      "# new-title\n\nupdated content",
		RenameFolder: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/archive/update-work-record", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	newFolder := filepath.Join(tmpDir, "2026.04.20_new-title")
	if _, err := os.Stat(newFolder); err != nil {
		t.Fatalf("expected renamed folder to exist: %v", err)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatalf("expected old folder removed")
	}

	updated, err := os.ReadFile(filepath.Join(newFolder, workRecordFileName))
	if err != nil {
		t.Fatalf("read updated record failed: %v", err)
	}
	text := string(updated)
	for _, expect := range []string{
		"title: new-title",
		"folder_name: 2026.04.20_new-title",
		"project_path: " + filepath.ToSlash(newFolder),
		"# new-title",
	} {
		if !strings.Contains(text, expect) {
			t.Fatalf("missing %q in updated record", expect)
		}
	}
}

func TestHandleUpdateWorkRecord_RenameConflict(t *testing.T) {
	tmpDir := t.TempDir()
	folder := filepath.Join(tmpDir, "2026.04.20_old-title")
	conflict := filepath.Join(tmpDir, "2026.04.20_new-title")
	_ = os.MkdirAll(folder, 0o755)
	_ = os.MkdirAll(conflict, 0o755)

	workRecord := `---
type: task
schema_version: 3
title: old-title
status: active
created: 2026-04-20
updated: 2026-04-20
source: manual
department:
project_path: C:/Task
folder_name: 2026.04.20_old-title
archive_status: local_active
hash: h001
---
# old-title`
	_ = os.WriteFile(filepath.Join(folder, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	body, _ := json.Marshal(UpdateWorkRecordRequest{
		FolderPath:   folder,
		Title:        "new-title",
		RenameFolder: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/archive/update-work-record", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestBuildDailyReportInput_UsesCoreContent(t *testing.T) {
	input := buildDailyReportInput("2026-04-23", "task-A", "ops", "core note")
	for _, expected := range []string{
		"2026-04-23",
		"task-A",
		"ops",
		"core note",
	} {
		if !strings.Contains(input, expected) {
			t.Fatalf("expected %q in input, got:\n%s", expected, input)
		}
	}
}

func TestHandleGenerateDailyReport_Fallback(t *testing.T) {
	tmpDir := t.TempDir()
	folderPath := filepath.Join(tmpDir, "2026.04.20_daily_task")
	_ = os.MkdirAll(folderPath, 0o755)

	workRecord := `---
type: task
schema_version: 3
title: daily-task
status: active
created: 2026-04-20
updated: 2026-04-20
source: manual
department: ops
project_path: C:/Task
folder_name: 2026.04.20_daily_task
archive_status: local_active
hash:
---
# daily-task

content from actual work.`
	_ = os.WriteFile(filepath.Join(folderPath, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	reqBody := DailyReportGenerateRequest{
		Date: "2026-04-20",
		Items: []DailyReportItem{
			{FolderPath: folderPath},
		},
		AI: DailyReportAIConfig{Enabled: false},
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/report/daily/generate", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Logs    []struct {
			Content string `json:"content"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !resp.Success || len(resp.Logs) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if strings.TrimSpace(resp.Logs[0].Content) == "" {
		t.Fatalf("expected non-empty fallback daily log")
	}
}

func TestNormalizePathKey_WindowsCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path key behavior")
	}

	a := normalizePathKey(`C:\Workspace\20_Dev\Task\`)
	b := normalizePathKey(`c:/workspace/20_dev/task`)
	if a != b {
		t.Fatalf("expected equal keys, got %s and %s", a, b)
	}
}

func TestCollectScannedFolders_RecursiveSkipsNestedTaskWhenParentIsTask(t *testing.T) {
	tmpDir := t.TempDir()

	parent := filepath.Join(tmpDir, "2026.04.22_parent")
	child := filepath.Join(parent, "nested", "2026.04.22_child")
	_ = os.MkdirAll(child, 0o755)

	parentRecord := `---
type: task
schema_version: 3
title: Parent
status: active
created: 2026-04-22
updated: 2026-04-22
source: manual
department:
project_path: D:/Workspace/Parent
folder_name: 2026.04.22_parent
archive_status: local_active
hash: parenthash
---
# Parent
`
	childRecord := `---
type: task
schema_version: 3
title: Child
status: active
created: 2026-04-22
updated: 2026-04-22
source: manual
department:
project_path: D:/Workspace/Child
folder_name: 2026.04.22_child
archive_status: local_active
hash: childhash
---
# Child
`

	_ = os.WriteFile(filepath.Join(parent, workRecordFileName), []byte(parentRecord), 0o644)
	_ = os.WriteFile(filepath.Join(child, workRecordFileName), []byte(childRecord), 0o644)

	folders, err := collectScannedFolders(tmpDir, true)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	if len(folders) != 1 {
		t.Fatalf("expected only parent task folder, got %d", len(folders))
	}

	gotPath := folders[0]["path"].(string)
	if normalizePathKey(gotPath) != normalizePathKey(parent) {
		t.Fatalf("expected %s, got %s", parent, gotPath)
	}
}
