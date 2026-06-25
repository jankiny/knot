package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

const (
	workRecordFileName   = "\u5de5\u4f5c\u8bb0\u5f55.md"
	taskSourceDirName    = "00_\u6765\u6e90\u8d44\u6599"
	taskProcessDirName   = "10_\u8fc7\u7a0b\u6587\u4ef6"
	taskOutputDirName    = "20_\u6210\u679c\u8f93\u51fa"
	taskAttachmentDir    = "\u9644\u4ef6"
	defaultSOPTemplateID = "default-task"
)

// SetupRoutes initializes the chi router with common middleware and configures endpoints.
func SetupRoutes() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Post("/mail/connect", handleConnectMail)
		r.Get("/mail/list", handleGetMailList)
		r.Get("/mail/{mail_id}/attachments", handleGetAttachments)
		r.Get("/mail/{mail_id}/detail", handleGetMailDetail)

		r.Post("/folder/create", handleCreateFolder)
		r.Post("/folder/create-with-attachments", handleCreateFolderWithAttachments)
		r.Get("/folder/check-hash", handleCheckHash)

		r.Get("/archive/scan", handleScanWorkFolders)
		r.Post("/archive/move", handleArchiveMove)
		r.Post("/archive/batch-move", handleArchiveBatchMove)
		r.Post("/archive/update-work-record", handleUpdateWorkRecord)
		r.Get("/archive/list", handleArchiveList)
		r.Post("/archive/restore", handleArchiveRestore)

		r.Get("/sop/templates", handleListSOPTemplates)

		r.Post("/report/daily/generate", handleGenerateDailyReport)
		r.Post("/report/weekly/generate", handleGenerateWeeklyReport)
	})

	return r
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]interface{}{
		"detail": message,
	})
}

// -- Folder Handlers --

type FolderRequest struct {
	MailID              string                   `json:"mail_id"`
	Subject             string                   `json:"subject"`
	Date                string                   `json:"date"`
	FromAddr            string                   `json:"from_addr"`
	Body                string                   `json:"body"`
	BasePath            string                   `json:"base_path"`
	FolderName          string                   `json:"folder_name"`
	UseSubFolder        bool                     `json:"use_sub_folder"`
	SubFolderName       string                   `json:"sub_folder_name"`
	SaveMailContent     bool                     `json:"save_mail_content"`
	MailContentFileName string                   `json:"mail_content_file_name"`
	Attachments         []map[string]interface{} `json:"attachments"`
	SaveFormats         []string                 `json:"save_formats"`
	RawContent          string                   `json:"raw_content"`
	Department          string                   `json:"department"`
	Project             string                   `json:"project"`
	Source              string                   `json:"source"`
	Hash                string                   `json:"hash"`
	SOPTemplateID       string                   `json:"sop_template_id"`
}

func getBaseFolder(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath != "" {
		if strings.HasPrefix(basePath, "~") {
			home, _ := os.UserHomeDir()
			basePath = filepath.Join(home, strings.TrimPrefix(basePath, "~"))
		}
		if filepath.IsAbs(basePath) {
			_ = os.MkdirAll(basePath, 0o755)
			return filepath.Clean(basePath)
		}
	}

	home, _ := os.UserHomeDir()
	desktop := filepath.Join(home, "Desktop")
	if _, err := os.Stat(desktop); os.IsNotExist(err) {
		desktop = filepath.Join(home, "桌面")
	}
	return desktop
}

func sanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "task_" + time.Now().Format("20060102150405")
	}
	invalid := regexp.MustCompile(`[\\/:*?"<>|]`)
	name = invalid.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if name == "" {
		return "task_" + time.Now().Format("20060102150405")
	}
	return name
}

func normalizeSource(source string, hasMailID bool) string {
	s := strings.ToLower(strings.TrimSpace(source))
	switch s {
	case "email", "mail", "邮件":
		return "email"
	case "manual", "手动", "quick", "quick_create", "快速创建":
		return "manual"
	}
	if hasMailID {
		return "email"
	}
	return "manual"
}

func buildEmailTXT(req FolderRequest) string {
	return fmt.Sprintf(
		"主题: %s\n发件人: %s\n日期: %s\n\n%s\n",
		req.Subject,
		req.FromAddr,
		req.Date,
		req.Body,
	)
}

func sanitizePDFLine(line string) string {
	var b strings.Builder
	for _, r := range line {
		switch {
		case r == '\\':
			b.WriteString("\\\\")
		case r == '(':
			b.WriteString("\\(")
		case r == ')':
			b.WriteString("\\)")
		case r >= 32 && r <= 126:
			b.WriteRune(r)
		default:
			b.WriteRune('?')
		}
	}
	return b.String()
}

func writePlainTextPDF(filePath string, title string, body string) error {
	lines := []string{title}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
		if len(lines) >= 36 {
			break
		}
	}
	if len(lines) == 0 {
		lines = []string{"Knot Task Source"}
	}

	var stream bytes.Buffer
	stream.WriteString("BT\n/F1 11 Tf\n50 790 Td\n")
	for i, l := range lines {
		if i > 0 {
			stream.WriteString("T*\n")
		}
		stream.WriteString(fmt.Sprintf("(%s) Tj\n", sanitizePDFLine(l)))
	}
	stream.WriteString("ET")

	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj\n",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", stream.Len(), stream.String()),
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n",
	}

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	offsets[0] = 0
	for i, obj := range objects {
		offsets[i+1] = out.Len()
		out.WriteString(obj)
	}

	startXRef := out.Len()
	out.WriteString(fmt.Sprintf("xref\n0 %d\n", len(objects)+1))
	out.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		out.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	out.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\n", len(objects)+1))
	out.WriteString("startxref\n")
	out.WriteString(fmt.Sprintf("%d\n", startXRef))
	out.WriteString("%%EOF")

	return os.WriteFile(filePath, out.Bytes(), 0o644)
}

func sourceFolderNameForTemplate(tpl SOPTemplate) string {
	if len(tpl.Folders) > 0 {
		if rel, ok := safeTemplateRelativePath(tpl.Folders[0]); ok {
			return rel
		}
	}
	return taskSourceDirName
}

func writeEmailSourceFiles(folderPath string, req FolderRequest, sourceFolderName string) error {
	sourceDir := filepath.Join(folderPath, sourceFolderName)
	if err := os.MkdirAll(filepath.Join(sourceDir, taskAttachmentDir), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "email.txt"), []byte(buildEmailTXT(req)), 0o644); err != nil {
		return err
	}

	pdfTitle := req.Subject
	if strings.TrimSpace(pdfTitle) == "" {
		pdfTitle = "Email Source"
	}
	if err := writePlainTextPDF(filepath.Join(sourceDir, "email.pdf"), pdfTitle, buildEmailTXT(req)); err != nil {
		return err
	}

	return nil
}

func writeManualSourceFiles(folderPath string, sourceFolderName string) error {
	sourceDir := filepath.Join(folderPath, sourceFolderName)
	return os.MkdirAll(sourceDir, 0o755)
}

func formatTaskDate(dateValue string, fallback time.Time) string {
	if t := parseTimeLoose(dateValue); !t.IsZero() {
		return t.Format("2006-01-02")
	}
	return fallback.Format("2006-01-02")
}

func buildWorkRecordTemplate(req FolderRequest, folderName, folderPath, sourceType string, now time.Time, tpl SOPTemplate) string {
	createdDate := now.Format("2006-01-02")
	taskDate := formatTaskDate(req.Date, now)
	title := strings.TrimSpace(req.Subject)
	if title == "" {
		title = folderName
	}
	if strings.TrimSpace(req.Department) == "" {
		req.Department = ""
	}
	if strings.TrimSpace(req.Project) == "" {
		req.Project = ""
	}
	hash := strings.TrimSpace(req.Hash)
	projectPath := filepath.ToSlash(folderPath)

	return fmt.Sprintf(`---
type: task
schema_version: 3
title: %s
status: active
created: %s
updated: %s
task_date: %s
source: %s
department: %s
project: %s
sop_template_id: %s
sop_template_name: %s
project_path: %s
folder_name: %s
archive_status: local_active
hash: %s
tags:
  - 工作材料
---

# %s

## 工作内容

围绕“%s”开展任务资料整理与输出准备工作。

## 工作过程

- %s：创建任务文件夹并完成基础材料归集。

## 当前进展

已完成任务初始化，正在持续完善过程记录与输出内容。

## 下一步

继续补充过程材料，完成成果文件并放入 20_成果输出。
`, title, createdDate, createdDate, taskDate, sourceType, req.Department, req.Project, tpl.ID, tpl.Name, projectPath, folderName, hash, title, title, taskDate)
}

func handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	processFolderCreation(w, r, false)
}

func handleCreateFolderWithAttachments(w http.ResponseWriter, r *http.Request) {
	processFolderCreation(w, r, true)
}

func processFolderCreation(w http.ResponseWriter, r *http.Request, downloadAttachments bool) {
	var req FolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid request parameters")
		return
	}

	baseFolder := getBaseFolder(req.BasePath)
	folderName := sanitizeFolderName(req.FolderName)
	folderPath := filepath.Join(baseFolder, folderName)
	sopTemplate := findSOPTemplate(req.SOPTemplateID)
	sourceFolderName := sourceFolderNameForTemplate(sopTemplate)

	if err := os.MkdirAll(folderPath, 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("创建目录失败: %v", err))
		return
	}
	if err := createTaskStructure(folderPath, sopTemplate); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("创建标准结构失败: %v", err))
		return
	}

	sourceType := normalizeSource(req.Source, strings.TrimSpace(req.MailID) != "")
	if sourceType == "email" {
		if err := writeEmailSourceFiles(folderPath, req, sourceFolderName); err != nil {
			jsonError(w, http.StatusInternalServerError, fmt.Sprintf("保存邮件来源失败: %v", err))
			return
		}
	} else {
		if err := writeManualSourceFiles(folderPath, sourceFolderName); err != nil {
			jsonError(w, http.StatusInternalServerError, fmt.Sprintf("保存需求来源失败: %v", err))
			return
		}
	}

	var downloaded []string
	if downloadAttachments && sourceType == "email" && mailClient != nil && strings.TrimSpace(req.MailID) != "" {
		attachmentsPath := filepath.Join(folderPath, sourceFolderName, taskAttachmentDir)
		d, err := mailClient.DownloadAttachments(req.MailID, attachmentsPath)
		if err == nil {
			downloaded = d
		}
	}

	now := time.Now()
	workRecord := buildWorkRecordTemplate(req, folderName, folderPath, sourceType, now, sopTemplate)
	wrPath := filepath.Join(folderPath, workRecordFileName)
	if err := os.WriteFile(wrPath, []byte(workRecord), 0o644); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("写入工作记录失败: %v", err))
		return
	}

	resp := map[string]interface{}{
		"success":      true,
		"path":         folderPath,
		"content_path": folderPath,
		"work_record":  wrPath,
		"message":      fmt.Sprintf("任务文件夹已创建: %s", folderName),
	}
	if downloadAttachments {
		resp["attachments_downloaded"] = downloaded
	}

	jsonResponse(w, http.StatusOK, resp)
}

func GenerateHash(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}

// scanDirForHash scans a directory for work records matching the given hash.
// Returns list of matching folder info maps with a "status" field.
func scanDirForHash(dirPath, hash, status string) []map[string]interface{} {
	results := []map[string]interface{}{}
	folders, err := collectScannedFolders(dirPath, true)
	if err != nil {
		return results
	}

	for _, folder := range folders {
		if fmt.Sprint(folder["hash"]) != hash {
			continue
		}
		results = append(results, map[string]interface{}{
			"name":       folder["name"],
			"path":       folder["path"],
			"department": folder["department"],
			"project":    folder["project"],
			"source":     folder["source"],
			"status":     status,
		})
	}
	return results
}

func handleCheckHash(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if hash == "" {
		jsonError(w, http.StatusBadRequest, "缺少 hash 参数")
		return
	}

	scanPath := r.URL.Query().Get("scan_path")
	if scanPath == "" {
		scanPath = "~/Desktop"
	}
	scanPath = getBaseFolder(scanPath)

	archivePaths := r.URL.Query()["archive_path"]

	matches := make([]map[string]interface{}, 0)
	matches = append(matches, scanDirForHash(scanPath, hash, "working")...)

	for _, ap := range archivePaths {
		ap = strings.TrimSpace(ap)
		if ap == "" {
			continue
		}
		matches = append(matches, scanDirForHash(getBaseFolder(ap), hash, "archived")...)
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"found":   len(matches) > 0,
		"count":   len(matches),
		"matches": matches,
	})
}
