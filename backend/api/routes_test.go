package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestHandleGetMailList_NoConnection(t *testing.T) {
	mailClient = nil
	router := SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/mail/list", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["detail"] != "请先连接邮箱" {
		t.Fatalf("unexpected detail: %v", resp["detail"])
	}
}

func TestHandleCreateFolder_ManualStructure(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		BasePath:   tmpDir,
		FolderName: "2026.04.20_Manual_Task",
		Subject:    "整理合同需求",
		Source:     "manual",
		Department: "法务部",
		Hash:       "manualhash001",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	folderPath := filepath.Join(tmpDir, "2026.04.20_Manual_Task")
	for _, p := range []string{
		filepath.Join(folderPath, "00_Source"),
		filepath.Join(folderPath, "10_Process"),
		filepath.Join(folderPath, "20_Output"),
		filepath.Join(folderPath, "00_Source", "references"),
		filepath.Join(folderPath, "00_Source", "requirement.md"),
		filepath.Join(folderPath, workRecordFileName),
	} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Fatalf("expected path not found: %s", p)
		}
	}

	wrContent, _ := os.ReadFile(filepath.Join(folderPath, workRecordFileName))
	wr := string(wrContent)
	for _, expect := range []string{
		"schema_version: 2",
		"source: manual",
		"department: 法务部",
		"hash: manualhash001",
		"# 2026.04.20_Manual_Task",
	} {
		if !strings.Contains(wr, expect) {
			t.Fatalf("work record missing %q\n%s", expect, wr)
		}
	}
}

func TestHandleCreateFolder_EmailStructure(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		BasePath:   tmpDir,
		FolderName: "2026.04.20_Email_Task",
		Subject:    "会议纪要确认",
		Date:       "2026-04-20 10:00:00",
		FromAddr:   "alice@example.com",
		Body:       "请确认会议纪要并补充意见。",
		MailID:     "1001",
		Source:     "email",
		Hash:       "emailhash001",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/folder/create-with-attachments", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	folderPath := filepath.Join(tmpDir, "2026.04.20_Email_Task")
	for _, p := range []string{
		filepath.Join(folderPath, "00_Source", "email.txt"),
		filepath.Join(folderPath, "00_Source", "email.pdf"),
		filepath.Join(folderPath, "00_Source", "attachments"),
	} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Fatalf("expected path not found: %s", p)
		}
	}

	emailText, _ := os.ReadFile(filepath.Join(folderPath, "00_Source", "email.txt"))
	if !strings.Contains(string(emailText), "会议纪要确认") {
		t.Fatalf("email.txt does not contain subject")
	}

	wrContent, _ := os.ReadFile(filepath.Join(folderPath, workRecordFileName))
	wr := string(wrContent)
	if !strings.Contains(wr, "source: email") {
		t.Fatalf("work record should be email source")
	}
	if !strings.Contains(wr, "alice@example.com") {
		t.Fatalf("work record should contain initiator")
	}
}

func TestParseWorkRecord_LegacyFrontmatterCompatible(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, workRecordFileName)
	content := `---
归属部门: 财务部
创建时间: 2026-04-20 09:00
来源: 邮件
标识: legacyhash001
---
# 工作记录

legacy body`
	_ = os.WriteFile(filePath, []byte(content), 0o644)

	info, err := parseWorkRecord(filePath)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if info.Department != "财务部" {
		t.Fatalf("department mismatch: %s", info.Department)
	}
	if info.Source != "邮件" {
		t.Fatalf("source mismatch: %s", info.Source)
	}
	if info.Hash != "legacyhash001" {
		t.Fatalf("hash mismatch: %s", info.Hash)
	}
}

func TestHandleScanWorkFolders_Recursive(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "2026", "2026.04.20_Task_A")
	_ = os.MkdirAll(taskDir, 0o755)

	workRecord := `---
type: task
schema_version: 2
status: active
created: 2026-04-20
source: manual
department: 行政部
project_path: D:/Workspace/Task
archive_status: local_active
hash: hashabc
---
# 2026.04.20_Task_A
`
	_ = os.WriteFile(filepath.Join(taskDir, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, "/api/archive/scan?scan_path="+tmpDir+"&recursive=true", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if int(resp["count"].(float64)) != 1 {
		t.Fatalf("expected count=1, got %v", resp["count"])
	}
}

func TestHandleArchiveMove_UpdatesWorkRecordStatus(t *testing.T) {
	tmpDir := t.TempDir()
	srcFolder := filepath.Join(tmpDir, "src", "2026.04.20_ArchiveTask")
	_ = os.MkdirAll(srcFolder, 0o755)

	workRecord := `---
type: task
schema_version: 2
status: active
created: 2026-04-20
source: manual
department: 综合部
project_path: D:/Workspace/Task
archive_status: local_active
hash: archivehash001
---
# 2026.04.20_ArchiveTask

## 归档记录

- 本地状态：active
- 归档位置：
- 归档时间：
`
	_ = os.WriteFile(filepath.Join(srcFolder, workRecordFileName), []byte(workRecord), 0o644)

	archiveDir := filepath.Join(tmpDir, "archive")
	_ = os.MkdirAll(archiveDir, 0o755)

	router := SetupRoutes()
	body, _ := json.Marshal(ArchiveMoveRequest{
		FolderPath:  srcFolder,
		ArchivePath: archiveDir,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/archive/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	destPath := filepath.Join(archiveDir, "2026", "2026.04.20_ArchiveTask", workRecordFileName)
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read archived work record failed: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "status: archived") {
		t.Fatalf("missing archived status")
	}
	if !strings.Contains(text, "archive_status: local_archive") {
		t.Fatalf("missing local_archive status")
	}
	if !strings.Contains(text, "- 本地状态：archived") {
		t.Fatalf("missing archive section update")
	}
}

func TestHandleUpdateWorkRecord_Success(t *testing.T) {
	tmpDir := t.TempDir()
	folder := filepath.Join(tmpDir, "2026.04.20_UpdateTask")
	_ = os.MkdirAll(folder, 0o755)

	workRecord := `---
type: task
schema_version: 2
status: active
created: 2026-04-20
source: manual
department:
project_path: D:/Workspace/Task
archive_status: local_active
hash:
---
# 2026.04.20_UpdateTask
`
	_ = os.WriteFile(filepath.Join(folder, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	body, _ := json.Marshal(UpdateWorkRecordRequest{
		FolderPath: folder,
		Department: "市场部",
		Content:    "# 自定义内容\n\n- 今天完成了扫描",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/archive/update-work-record", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	content, _ := os.ReadFile(filepath.Join(folder, workRecordFileName))
	text := string(content)
	if !strings.Contains(text, "department: 市场部") {
		t.Fatalf("department not updated")
	}
	if !strings.Contains(text, "# 自定义内容") {
		t.Fatalf("content not updated")
	}
}

func TestHandleCheckHash_Found(t *testing.T) {
	tmpDir := t.TempDir()
	folderPath := filepath.Join(tmpDir, "2026.04.20_HashTask")
	_ = os.MkdirAll(folderPath, 0o755)

	workRecord := `---
type: task
schema_version: 2
status: active
created: 2026-04-20
source: manual
department: 研发部
project_path: D:/Workspace/Task
archive_status: local_active
hash: hashcheck001
---
# 2026.04.20_HashTask
`
	_ = os.WriteFile(filepath.Join(folderPath, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, "/api/folder/check-hash?hash=hashcheck001&scan_path="+tmpDir, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["found"] != true {
		t.Fatalf("expected found=true")
	}
	if int(resp["count"].(float64)) != 1 {
		t.Fatalf("expected count=1, got %v", resp["count"])
	}
}

func TestHandleGenerateDailyReport_Fallback(t *testing.T) {
	tmpDir := t.TempDir()
	folderPath := filepath.Join(tmpDir, "2026.04.20_DailyTask")
	_ = os.MkdirAll(folderPath, 0o755)

	workRecord := `---
type: task
schema_version: 2
status: active
created: 2026-04-20
source: manual
department: 运营部
project_path: D:/Workspace/Task
archive_status: local_active
hash:
---
# 2026.04.20_DailyTask

## 任务目标

完成资料整理并形成对外发送版本。
`
	_ = os.WriteFile(filepath.Join(folderPath, workRecordFileName), []byte(workRecord), 0o644)

	router := SetupRoutes()
	reqBody := DailyReportGenerateRequest{
		Date: "2026-04-20",
		Items: []DailyReportItem{
			{FolderPath: folderPath},
		},
		AI: DailyReportAIConfig{
			Enabled: false,
		},
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/report/daily/generate", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success"] != true {
		t.Fatalf("expected success=true")
	}
	logs, ok := resp["logs"].([]interface{})
	if !ok || len(logs) != 1 {
		t.Fatalf("expected one log, got %v", resp["logs"])
	}
	logItem := logs[0].(map[string]interface{})
	content := logItem["content"].(string)
	if !strings.Contains(content, "完成了") {
		t.Fatalf("unexpected log content: %s", content)
	}
}
