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
		{"GET", "/api/archive/scan"},
		{"POST", "/api/archive/move"},
		{"POST", "/api/archive/batch-move"},
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
		Subject:            "测试邮件",
		Date:               "2026-02-23",
		FromAddr:           "test@example.com",
		Body:               "这是邮件正文内容",
		BasePath:           tmpDir,
		FolderName:         "2026.02.23_测试邮件",
		SaveMailContent:    true,
		MailContentFileName: "邮件正文",
		SaveFormats:        []string{"txt"},
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
}

func TestHandleCreateFolder_WithSubFolder(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	body := FolderRequest{
		Subject:            "子目录测试",
		Date:               "2026-02-23",
		BasePath:           tmpDir,
		FolderName:         "2026.02.23_子目录测试",
		UseSubFolder:       true,
		SubFolderName:      "邮件",
		SaveMailContent:    true,
		Body:               "内容",
		MailContentFileName: "邮件正文",
		SaveFormats:        []string{"txt"},
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
		Subject:          "工作记录测试",
		Date:             "2026-02-23",
		FromAddr:         "boss@company.com",
		BasePath:         tmpDir,
		FolderName:       "2026.02.23_工作记录测试",
		Department:       "技术部",
		CreateWorkRecord: true,
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/folder/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify 工作记录.md was created
	wrFile := filepath.Join(tmpDir, "2026.02.23_工作记录测试", "工作记录.md")
	if _, err := os.Stat(wrFile); os.IsNotExist(err) {
		t.Errorf("工作记录.md was not created at %s", wrFile)
	}

	content, _ := os.ReadFile(wrFile)
	contentStr := string(content)

	if !strings.Contains(contentStr, "技术部") {
		t.Error("工作记录.md does not contain department")
	}
	if !strings.Contains(contentStr, "工作记录测试") {
		t.Error("工作记录.md does not contain subject")
	}
	if !strings.Contains(contentStr, "boss@company.com") {
		t.Error("工作记录.md does not contain from address")
	}
}

func TestHandleCreateFolder_EmlFormat(t *testing.T) {
	tmpDir := t.TempDir()
	router := SetupRoutes()

	rawEml := "Subject: Test\nFrom: test@example.com\n\nHello"
	body := FolderRequest{
		Subject:            "EML测试",
		Date:               "2026-02-23",
		BasePath:           tmpDir,
		FolderName:         "2026.02.23_EML测试",
		SaveMailContent:    true,
		Body:               "Hello",
		RawContent:         rawEml,
		MailContentFileName: "邮件正文",
		SaveFormats:        []string{"eml"},
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

func TestHandleArchiveScan_ReturnsEmptyList(t *testing.T) {
	router := SetupRoutes()

	req := httptest.NewRequest("GET", "/api/archive/scan", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestHandleArchiveMove_ReturnsOK(t *testing.T) {
	router := SetupRoutes()

	req := httptest.NewRequest("POST", "/api/archive/move", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHandleArchiveBatchMove_ReturnsOK(t *testing.T) {
	router := SetupRoutes()

	req := httptest.NewRequest("POST", "/api/archive/batch-move", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
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
