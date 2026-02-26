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

// ========== Utility Function Tests ==========

func TestGetBaseFolder_WithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	result := getBaseFolder(tmpDir)
	if result != tmpDir {
		t.Errorf("expected %s, got %s", tmpDir, result)
	}
}

func TestGetBaseFolder_EmptyFallsBackToDesktop(t *testing.T) {
	result := getBaseFolder("")
	home, _ := os.UserHomeDir()
	// Should be either Desktop or 桌面
	if !strings.Contains(result, "Desktop") && !strings.Contains(result, "桌面") {
		t.Errorf("expected desktop path under %s, got %s", home, result)
	}
}

func TestGetBaseFolder_TildePath(t *testing.T) {
	result := getBaseFolder("~/test_knot_folder_abc")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "test_knot_folder_abc")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
	// Clean up
	os.RemoveAll(expected)
}

// ========== Route Registration Tests ==========

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
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Should NOT be 404 or 405
			if rr.Code == http.StatusNotFound {
				t.Errorf("endpoint %s %s returned 404, route not registered", ep.method, ep.path)
			}
			if rr.Code == http.StatusMethodNotAllowed {
				t.Errorf("endpoint %s %s returned 405, method not allowed", ep.method, ep.path)
			}
		})
	}
}

// ========== Mail Handler Tests (without real IMAP connection) ==========

func TestHandleConnectMail_InvalidBody(t *testing.T) {
	router := SetupRoutes()

	req := httptest.NewRequest("POST", "/api/mail/connect", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGetMailList_NoConnection(t *testing.T) {
	// Ensure mailClient is nil
	mailClient = nil

	router := SetupRoutes()
	req := httptest.NewRequest("GET", "/api/mail/list", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if detail, ok := resp["detail"].(string); !ok || detail != "请先连接邮件服务器" {
		t.Errorf("expected '请先连接邮件服务器', got '%v'", resp["detail"])
	}
}

func TestHandleGetAttachments_NoConnection(t *testing.T) {
	mailClient = nil
	router := SetupRoutes()

	req := httptest.NewRequest("GET", "/api/mail/999/attachments", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGetMailDetail_NoConnection(t *testing.T) {
	mailClient = nil
	router := SetupRoutes()

	req := httptest.NewRequest("GET", "/api/mail/999/detail", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ========== Folder Handler Tests ==========

func TestHandleCreateFolder_Success(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		Subject:             "测试邮件",
		Date:                "2026-02-23",
		FromAddr:            "test@example.com",
		Body:                "这是邮件正文内容",
		BasePath:            tmpDir,
		FolderName:          "2026.02.23_测试邮件",
		SaveMailContent:     true,
		MailContentFileName: "邮件正文",
		SaveFormats:         []string{"txt"},
		Source:              "邮件",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}

	// Verify folder was created
	folderPath := filepath.Join(tmpDir, "2026.02.23_测试邮件")
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		t.Errorf("folder was not created at %s", folderPath)
	}

	// Verify txt file was created
	txtFile := filepath.Join(folderPath, "邮件正文.txt")
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		t.Errorf("txt file was not created at %s", txtFile)
	}

	// Verify txt content
	content, _ := os.ReadFile(txtFile)
	if !strings.Contains(string(content), "测试邮件") {
		t.Errorf("txt file does not contain subject")
	}
	if !strings.Contains(string(content), "这是邮件正文内容") {
		t.Errorf("txt file does not contain body")
	}

	// Verify 工作记录.md is always created (YAML frontmatter format)
	wrFile := filepath.Join(folderPath, "工作记录.md")
	if _, err := os.Stat(wrFile); os.IsNotExist(err) {
		t.Errorf("工作记录.md was not created at %s", wrFile)
	}
	wrContent, _ := os.ReadFile(wrFile)
	wrStr := string(wrContent)
	if !strings.HasPrefix(strings.TrimSpace(wrStr), "---") {
		t.Error("工作记录.md does not have YAML frontmatter")
	}
	if !strings.Contains(wrStr, "来源: 邮件") {
		t.Error("工作记录.md does not contain source")
	}
}

func TestHandleCreateFolder_WithSubFolder(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		Subject:             "子目录测试",
		Date:                "2026-02-23",
		BasePath:            tmpDir,
		FolderName:          "2026.02.23_子目录测试",
		UseSubFolder:        true,
		SubFolderName:       "邮件",
		SaveMailContent:     true,
		Body:                "内容",
		MailContentFileName: "邮件正文",
		SaveFormats:         []string{"txt"},
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify sub folder was created
	subFolder := filepath.Join(tmpDir, "2026.02.23_子目录测试", "邮件")
	if _, err := os.Stat(subFolder); os.IsNotExist(err) {
		t.Errorf("sub folder was not created at %s", subFolder)
	}

	// Verify file saved in sub folder
	txtFile := filepath.Join(subFolder, "邮件正文.txt")
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		t.Errorf("txt file was not created in sub folder at %s", txtFile)
	}
}

func TestHandleCreateFolder_WithWorkRecord(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		Subject:    "工作记录测试",
		Date:       "2026-02-23",
		FromAddr:   "boss@company.com",
		BasePath:   tmpDir,
		FolderName: "2026.02.23_工作记录测试",
		Department: "技术部",
		Source:     "邮件",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify 工作记录.md was created with YAML frontmatter
	wrFile := filepath.Join(tmpDir, "2026.02.23_工作记录测试", "工作记录.md")
	if _, err := os.Stat(wrFile); os.IsNotExist(err) {
		t.Errorf("工作记录.md was not created at %s", wrFile)
	}

	content, _ := os.ReadFile(wrFile)
	contentStr := string(content)

	if !strings.Contains(contentStr, "归属部门: 技术部") {
		t.Error("工作记录.md does not contain department in frontmatter")
	}
	if !strings.Contains(contentStr, "来源: 邮件") {
		t.Error("工作记录.md does not contain source in frontmatter")
	}
}

func TestHandleCreateFolder_WithoutDepartment(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		Subject:    "无部门测试",
		Date:       "2026-02-23",
		BasePath:   tmpDir,
		FolderName: "2026.02.23_无部门测试",
		Source:     "快速创建",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// 工作记录.md should still be created even without department
	wrFile := filepath.Join(tmpDir, "2026.02.23_无部门测试", "工作记录.md")
	if _, err := os.Stat(wrFile); os.IsNotExist(err) {
		t.Errorf("工作记录.md should always be created")
	}

	content, _ := os.ReadFile(wrFile)
	contentStr := string(content)
	if !strings.Contains(contentStr, "归属部门:") {
		t.Error("工作记录.md should have 归属部门 field (even if empty)")
	}
	if !strings.Contains(contentStr, "来源: 快速创建") {
		t.Error("工作记录.md should contain source '快速创建'")
	}
}

func TestHandleCreateFolder_EmlFormat(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	rawEml := "Subject: Test\nFrom: test@example.com\n\nHello"
	body := FolderRequest{
		Subject:             "EML测试",
		Date:                "2026-02-23",
		BasePath:            tmpDir,
		FolderName:          "2026.02.23_EML测试",
		SaveMailContent:     true,
		Body:                "Hello",
		RawContent:          rawEml,
		MailContentFileName: "邮件正文",
		SaveFormats:         []string{"eml"},
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	emlFile := filepath.Join(tmpDir, "2026.02.23_EML测试", "邮件正文.eml")
	if _, err := os.Stat(emlFile); os.IsNotExist(err) {
		t.Errorf("eml file was not created at %s", emlFile)
	}

	content, _ := os.ReadFile(emlFile)
	if string(content) != rawEml {
		t.Errorf("eml content mismatch")
	}
}

func TestHandleCreateFolder_InvalidBody(t *testing.T) {
	router := SetupRoutes()

	req := httptest.NewRequest("POST", "/api/folder/create", strings.NewReader("broken json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ========== Archive Handler Tests ==========

func TestHandleScanWorkFolders_WithFolders(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test folders with work records
	folder1 := filepath.Join(tmpDir, "2026.02.25_项目汇报")
	os.MkdirAll(folder1, 0755)
	wr1 := "---\n归属部门: 技术部\n创建时间: 2026-02-25 10:00\n来源: 邮件\n---\n# 工作记录\n\n> 此文件由 Knot（绳结）自动创建。\n<!-- 请在此记录 -->\n\n今天完成了项目汇报\n"
	os.WriteFile(filepath.Join(folder1, "工作记录.md"), []byte(wr1), 0644)

	folder2 := filepath.Join(tmpDir, "2026.02.26_会议纪要")
	os.MkdirAll(folder2, 0755)
	wr2 := "---\n归属部门: \n创建时间: 2026-02-26 14:00\n来源: 快速创建\n---\n# 工作记录\n\n> 此文件由 Knot（绳结）自动创建。\n<!-- 请在此记录 -->\n\n"
	os.WriteFile(filepath.Join(folder2, "工作记录.md"), []byte(wr2), 0644)

	// Create a folder without work record (should not be scanned)
	folder3 := filepath.Join(tmpDir, "random_folder")
	os.MkdirAll(folder3, 0755)
	os.WriteFile(filepath.Join(folder3, "readme.txt"), []byte("hello"), 0644)

	router := SetupRoutes()
	req := httptest.NewRequest("GET", "/api/archive/scan?scan_path="+tmpDir, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["success"] != true {
		t.Error("expected success=true")
	}

	folders := resp["folders"].([]interface{})
	if len(folders) != 2 {
		t.Errorf("expected 2 folders, got %d", len(folders))
	}

	// Find folder1 in results
	for _, f := range folders {
		fm := f.(map[string]interface{})
		if fm["name"] == "2026.02.25_项目汇报" {
			if fm["department"] != "技术部" {
				t.Errorf("expected department '技术部', got '%v'", fm["department"])
			}
			if fm["source"] != "邮件" {
				t.Errorf("expected source '邮件', got '%v'", fm["source"])
			}
			if !strings.Contains(fm["content"].(string), "项目汇报") {
				t.Error("expected content to contain '项目汇报'")
			}
		}
	}
}

func TestHandleScanWorkFolders_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	req := httptest.NewRequest("GET", "/api/archive/scan?scan_path="+tmpDir, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("expected success=true")
	}
	folders := resp["folders"].([]interface{})
	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}
}

func TestHandleArchiveMove_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Source folder
	srcFolder := filepath.Join(tmpDir, "src", "2026.02.25_测试归档")
	os.MkdirAll(srcFolder, 0755)
	os.WriteFile(filepath.Join(srcFolder, "工作记录.md"), []byte("---\n归属部门: 技术部\n创建时间: 2026-02-25\n来源: 邮件\n---\n# 工作记录\n"), 0644)

	// Archive destination
	archiveDir := filepath.Join(tmpDir, "archive")
	os.MkdirAll(archiveDir, 0755)

	router := SetupRoutes()
	body, _ := json.Marshal(ArchiveMoveRequest{
		FolderPath:  srcFolder,
		ArchivePath: archiveDir,
	})
	req := httptest.NewRequest("POST", "/api/archive/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("expected success=true")
	}

	// Verify moved to archive/2026/
	destPath := filepath.Join(archiveDir, "2026", "2026.02.25_测试归档")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Errorf("folder was not moved to %s", destPath)
	}

	// Source should no longer exist
	if _, err := os.Stat(srcFolder); !os.IsNotExist(err) {
		t.Error("source folder should have been moved")
	}
}

func TestHandleArchiveBatchMove_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two source folders
	src1 := filepath.Join(tmpDir, "src", "2026.01.01_task1")
	src2 := filepath.Join(tmpDir, "src", "2026.02.01_task2")
	os.MkdirAll(src1, 0755)
	os.MkdirAll(src2, 0755)
	os.WriteFile(filepath.Join(src1, "工作记录.md"), []byte("---\n归属部门: A\n创建时间: x\n来源: 邮件\n---\n"), 0644)
	os.WriteFile(filepath.Join(src2, "工作记录.md"), []byte("---\n归属部门: B\n创建时间: x\n来源: 邮件\n---\n"), 0644)

	archiveDir := filepath.Join(tmpDir, "archive")
	os.MkdirAll(archiveDir, 0755)

	router := SetupRoutes()
	body, _ := json.Marshal(BatchMoveRequest{
		Items: []ArchiveMoveRequest{
			{FolderPath: src1, ArchivePath: archiveDir},
			{FolderPath: src2, ArchivePath: archiveDir},
		},
	})
	req := httptest.NewRequest("POST", "/api/archive/batch-move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success_count"].(float64) != 2 {
		t.Errorf("expected 2 successes, got %v", resp["success_count"])
	}
}

func TestHandleUpdateWorkRecord_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a folder with work record
	folder := filepath.Join(tmpDir, "2026.02.25_update_test")
	os.MkdirAll(folder, 0755)
	wr := "---\n归属部门: \n创建时间: 2026-02-25 10:00\n来源: 邮件\n---\n# 工作记录\n\n> 此文件由 Knot（绳结）自动创建。\n<!-- 请在此记录 -->\n\n"
	os.WriteFile(filepath.Join(folder, "工作记录.md"), []byte(wr), 0644)

	router := SetupRoutes()
	body, _ := json.Marshal(UpdateWorkRecordRequest{
		FolderPath: folder,
		Department: "办公室",
		Content:    "已完成汇报",
	})
	req := httptest.NewRequest("POST", "/api/archive/update-work-record", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	// Verify the file was updated
	content, _ := os.ReadFile(filepath.Join(folder, "工作记录.md"))
	contentStr := string(content)
	if !strings.Contains(contentStr, "归属部门: 办公室") {
		t.Error("department was not updated")
	}
	if !strings.Contains(contentStr, "已完成汇报") {
		t.Error("content was not updated")
	}
	// Preserved fields
	if !strings.Contains(contentStr, "创建时间: 2026-02-25 10:00") {
		t.Error("create time should be preserved")
	}
}

// ========== JSON Response Tests ==========

func TestJsonResponse_SetsContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonResponse(rr, http.StatusOK, map[string]string{"key": "value"})

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func TestJsonError_ReturnsDetailField(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonError(rr, http.StatusBadRequest, "test error")

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["detail"] != "test error" {
		t.Errorf("expected detail='test error', got '%v'", resp["detail"])
	}

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ========== Hash Tests ==========

func TestGenerateHash_Consistency(t *testing.T) {
	hash1 := GenerateHash("test|2026-02-25|sender@example.com")
	hash2 := GenerateHash("test|2026-02-25|sender@example.com")
	if hash1 != hash2 {
		t.Errorf("same input should produce same hash, got %s and %s", hash1, hash2)
	}
	if len(hash1) != 16 {
		t.Errorf("hash should be 16 chars, got %d", len(hash1))
	}

	// Different input should produce different hash
	hash3 := GenerateHash("different|2026-02-26|other@example.com")
	if hash1 == hash3 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestHandleCreateFolder_WithHash(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	hash := GenerateHash("邮件主题|2026-02-25|test@example.com")
	body := FolderRequest{
		Subject:    "邮件主题",
		Date:       "2026-02-25",
		BasePath:   tmpDir,
		FolderName: "2026.02.25_邮件主题",
		Source:     "邮件",
		Hash:       hash,
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	// Verify 工作记录.md contains the hash
	wrFile := filepath.Join(tmpDir, "2026.02.25_邮件主题", "工作记录.md")
	content, _ := os.ReadFile(wrFile)
	contentStr := string(content)
	if !strings.Contains(contentStr, "标识: "+hash) {
		t.Errorf("工作记录.md should contain '标识: %s', got:\n%s", hash, contentStr)
	}
}

func TestHandleCheckHash_Found(t *testing.T) {
	tmpDir := t.TempDir()

	hash := GenerateHash("test-check|2026-02-25|sender@test.com")

	// Create a folder with work record containing the hash
	folderPath := filepath.Join(tmpDir, "2026.02.25_test-check")
	os.MkdirAll(folderPath, 0755)
	wr := "---\n归属部门: 技术部\n创建时间: 2026-02-25 10:00\n来源: 邮件\n标识: " + hash + "\n---\n# 工作记录\n"
	os.WriteFile(filepath.Join(folderPath, "工作记录.md"), []byte(wr), 0644)

	router := SetupRoutes()
	req := httptest.NewRequest("GET", "/api/folder/check-hash?hash="+hash+"&scan_path="+tmpDir, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["found"] != true {
		t.Error("expected found=true")
	}
	if resp["count"].(float64) != 1 {
		t.Errorf("expected count=1, got %v", resp["count"])
	}
	matches := resp["matches"].([]interface{})
	m := matches[0].(map[string]interface{})
	if m["name"] != "2026.02.25_test-check" {
		t.Errorf("expected name '2026.02.25_test-check', got '%v'", m["name"])
	}
	if m["status"] != "working" {
		t.Errorf("expected status 'working', got '%v'", m["status"])
	}
}

func TestHandleCheckHash_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	router := SetupRoutes()
	req := httptest.NewRequest("GET", "/api/folder/check-hash?hash=nonexistenthash&scan_path="+tmpDir, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["found"] != false {
		t.Error("expected found=false")
	}
}
