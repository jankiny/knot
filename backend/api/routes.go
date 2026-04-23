package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"knot-backend/mail"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

const (
	workRecordFileName = "\u5de5\u4f5c\u8bb0\u5f55.md"
	taskSourceDirName  = "00_\u6765\u6e90\u8d44\u6599"
	taskProcessDirName = "10_\u8fc7\u7a0b\u6587\u4ef6"
	taskOutputDirName  = "20_\u6210\u679c\u8f93\u51fa"
	taskAttachmentDir  = "\u9644\u4ef6"
)

var mailClient *mail.MailClient

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

		r.Post("/report/daily/generate", handleGenerateDailyReport)
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

// -- Mail Handlers --

type MailConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseSSL   bool   `json:"use_ssl"`
}

func handleConnectMail(w http.ResponseWriter, r *http.Request) {
	var config MailConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if mailClient != nil {
		mailClient.Disconnect()
	}

	mailClient = mail.NewMailClient(config.Server, config.Port, config.Username, config.Password, config.UseSSL)
	if err := mailClient.Connect(); err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("连接失败: %v", err))
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "message": "连接成功"})
}

func handleGetMailList(w http.ResponseWriter, r *http.Request) {
	if mailClient == nil {
		jsonError(w, http.StatusBadRequest, "请先连接邮箱")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))

	mails, err := mailClient.FetchMailList(limit, days)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "data": mails})
}

func handleGetAttachments(w http.ResponseWriter, r *http.Request) {
	if mailClient == nil {
		jsonError(w, http.StatusBadRequest, "请先连接邮箱")
		return
	}

	mailID := chi.URLParam(r, "mail_id")
	attachments, err := mailClient.FetchAttachments(mailID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "data": attachments})
}

func handleGetMailDetail(w http.ResponseWriter, r *http.Request) {
	if mailClient == nil {
		jsonError(w, http.StatusBadRequest, "请先连接邮箱")
		return
	}

	mailID := chi.URLParam(r, "mail_id")
	detail, err := mailClient.FetchMailDetail(mailID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "data": detail})
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
	Source              string                   `json:"source"`
	Hash                string                   `json:"hash"`
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

func createTaskStructure(folderPath string) error {
	dirs := []string{
		filepath.Join(folderPath, taskSourceDirName),
		filepath.Join(folderPath, taskProcessDirName),
		filepath.Join(folderPath, taskOutputDirName),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
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

func writeEmailSourceFiles(folderPath string, req FolderRequest) error {
	sourceDir := filepath.Join(folderPath, taskSourceDirName)
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

func writeManualSourceFiles(folderPath string) error {
	sourceDir := filepath.Join(folderPath, taskSourceDirName)
	return os.MkdirAll(sourceDir, 0o755)
}

func buildWorkRecordTemplate(req FolderRequest, folderName, folderPath, sourceType string, now time.Time) string {
	createdDate := now.Format("2006-01-02")
	title := strings.TrimSpace(req.Subject)
	if title == "" {
		title = folderName
	}
	if strings.TrimSpace(req.Department) == "" {
		req.Department = ""
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
source: %s
department: %s
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
`, title, createdDate, createdDate, sourceType, req.Department, projectPath, folderName, hash, title, title, createdDate)
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

	if err := os.MkdirAll(folderPath, 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("创建目录失败: %v", err))
		return
	}
	if err := createTaskStructure(folderPath); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("创建标准结构失败: %v", err))
		return
	}

	sourceType := normalizeSource(req.Source, strings.TrimSpace(req.MailID) != "")
	if sourceType == "email" {
		if err := writeEmailSourceFiles(folderPath, req); err != nil {
			jsonError(w, http.StatusInternalServerError, fmt.Sprintf("保存邮件来源失败: %v", err))
			return
		}
	} else {
		if err := writeManualSourceFiles(folderPath); err != nil {
			jsonError(w, http.StatusInternalServerError, fmt.Sprintf("保存需求来源失败: %v", err))
			return
		}
	}

	var downloaded []string
	if downloadAttachments && sourceType == "email" && mailClient != nil && strings.TrimSpace(req.MailID) != "" {
		attachmentsPath := filepath.Join(folderPath, taskSourceDirName, taskAttachmentDir)
		d, err := mailClient.DownloadAttachments(req.MailID, attachmentsPath)
		if err == nil {
			downloaded = d
		}
	}

	now := time.Now()
	workRecord := buildWorkRecordTemplate(req, folderName, folderPath, sourceType, now)
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

// -- Archive Handlers --

// WorkRecordInfo holds parsed info from 工作记录.md
type WorkRecordInfo struct {
	Title         string `json:"title"`
	Department    string `json:"department"`
	CreateTime    string `json:"create_time"`
	UpdateTime    string `json:"update_time"`
	Source        string `json:"source"`
	Content       string `json:"content"`
	RawContent    string `json:"raw_content"`
	Hash          string `json:"hash"`
	Status        string `json:"status"`
	ArchiveStatus string `json:"archive_status"`
	SchemaVersion int    `json:"schema_version"`
	ProjectPath   string `json:"project_path"`
	FolderName    string `json:"folder_name"`
}

type parsedWorkRecord struct {
	Info         *WorkRecordInfo
	FrontLines   []string
	Body         string
	HasFrontmatt bool
}

func splitFrontmatter(text string) ([]string, string, bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, text, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, text, false
	}
	return lines[1:end], strings.Join(lines[end+1:], "\n"), true
}

func normalizeKey(key string) string {
	k := strings.TrimSpace(strings.ToLower(key))
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, " ", "")
	return k
}

func parseFrontmatterValues(frontLines []string) map[string]string {
	result := make(map[string]string)
	for _, line := range frontLines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := normalizeKey(parts[0])
		value := strings.TrimSpace(parts[1])
		result[key] = value
	}
	return result
}

func extractTitleFromBody(body string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	matches := re.FindStringSubmatch(body)
	if len(matches) == 2 {
		title := strings.TrimSpace(matches[1])
		if isGenericWorkRecordTitle(title) {
			return ""
		}
		return title
	}
	return ""
}

func isGenericWorkRecordTitle(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "工作记录", "工作.md", "工作", "work record", "work.md", "work":
		return true
	default:
		return false
	}
}

var folderDatePrefixPattern = regexp.MustCompile(`^(\d{4}[._-]\d{2}[._-]\d{2})([_-]?)(.*)$`)

func fallbackTitleFromFolderName(folderName string) string {
	name := strings.TrimSpace(folderName)
	if name == "" {
		return "未命名任务"
	}

	matches := folderDatePrefixPattern.FindStringSubmatch(name)
	if len(matches) == 4 {
		suffix := strings.TrimSpace(matches[3])
		if suffix != "" {
			return suffix
		}
	}
	return name
}

func stripMarkdownHeaders(body string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = normalizeSectionLine(line)
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, " ")
}

func extractWorkCoreContent(body string) string {
	workContent := truncateRunes(extractSectionContent(body, "## \u5de5\u4f5c\u5185\u5bb9", "## \u4efb\u52a1\u76ee\u6807"), 220)
	workProcess := truncateRunes(extractSectionContent(body, "## \u5de5\u4f5c\u8fc7\u7a0b", "## \u5de5\u4f5c\u65e5\u5fd7"), 260)
	progress := truncateRunes(extractSectionContent(body, "## \u5f53\u524d\u8fdb\u5c55"), 180)
	nextStep := truncateRunes(extractSectionContent(body, "## \u4e0b\u4e00\u6b65", "## \u4ea7\u51fa\u6210\u679c"), 180)

	segments := make([]string, 0, 4)
	if workContent != "" {
		segments = append(segments, "\u5de5\u4f5c\u5185\u5bb9\uff1a"+workContent)
	}
	if workProcess != "" {
		segments = append(segments, "\u5de5\u4f5c\u8fc7\u7a0b\uff1a"+workProcess)
	}
	if progress != "" {
		segments = append(segments, "\u5f53\u524d\u8fdb\u5c55\uff1a"+progress)
	}
	if nextStep != "" {
		segments = append(segments, "\u4e0b\u4e00\u6b65\uff1a"+nextStep)
	}

	if len(segments) == 0 {
		plain := stripMarkdownHeaders(body)
		if plain == "" {
			return ""
		}
		return truncateRunes(plain, 260)
	}
	return strings.Join(segments, "\n")
}

func parseTimeLoose(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseWorkRecord(filePath string) (*WorkRecordInfo, error) {
	parsed, err := readWorkRecord(filePath)
	if err != nil {
		return nil, err
	}
	return parsed.Info, nil
}

func readWorkRecord(filePath string) (*parsedWorkRecord, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	frontLines, body, hasFrontmatter := splitFrontmatter(string(data))
	values := parseFrontmatterValues(frontLines)

	info := &WorkRecordInfo{
		Status:        "active",
		ArchiveStatus: "local_active",
		Source:        "manual",
		SchemaVersion: 2,
		RawContent:    strings.TrimSpace(body),
	}

	get := func(keys ...string) string {
		for _, key := range keys {
			if val, ok := values[normalizeKey(key)]; ok {
				return strings.TrimSpace(val)
			}
		}
		return ""
	}

	info.Title = get("title")
	if info.Title == "" {
		info.Title = extractTitleFromBody(body)
	}
	if strings.TrimSpace(info.Title) == "" {
		info.Title = fallbackTitleFromFolderName(filepath.Base(filepath.Dir(filePath)))
	}

	info.Department = get("department")
	info.CreateTime = get("created")
	info.UpdateTime = get("updated")
	info.Source = get("source")
	info.Hash = get("hash")
	info.Status = get("status")
	info.ArchiveStatus = get("archive_status", "archiveStatus")
	info.ProjectPath = get("project_path", "projectPath")
	info.FolderName = get("folder_name", "folderName")

	schemaValue := get("schema_version")
	if schemaValue != "" {
		if v, err := strconv.Atoi(schemaValue); err == nil {
			info.SchemaVersion = v
		}
	}

	if strings.TrimSpace(info.Source) == "" {
		info.Source = "manual"
	}
	if strings.TrimSpace(info.Status) == "" {
		info.Status = "active"
	}
	if strings.TrimSpace(info.ArchiveStatus) == "" {
		if strings.EqualFold(info.Status, "archived") {
			info.ArchiveStatus = "local_archive"
		} else {
			info.ArchiveStatus = "local_active"
		}
	}
	if strings.TrimSpace(info.ProjectPath) == "" {
		info.ProjectPath = filepath.ToSlash(filepath.Dir(filePath))
	}
	if strings.TrimSpace(info.FolderName) == "" {
		info.FolderName = filepath.Base(filepath.Dir(filePath))
	}
	if strings.TrimSpace(info.CreateTime) == "" {
		if t, err := os.Stat(filePath); err == nil {
			info.CreateTime = t.ModTime().Format("2006-01-02")
		}
	}
	if strings.TrimSpace(info.UpdateTime) == "" {
		info.UpdateTime = info.CreateTime
	}

	info.Content = extractWorkCoreContent(body)
	if info.Content == "" {
		info.Content = truncateRunes(cleanDailyLog(body), 220)
	}

	return &parsedWorkRecord{
		Info:         info,
		FrontLines:   frontLines,
		Body:         body,
		HasFrontmatt: hasFrontmatter,
	}, nil
}

func ensureFrontmatterLines(parsed *parsedWorkRecord, folderPath string) []string {
	if parsed.HasFrontmatt && len(parsed.FrontLines) > 0 {
		return append([]string{}, parsed.FrontLines...)
	}

	info := parsed.Info
	now := time.Now().Format("2006-01-02")
	created := strings.TrimSpace(info.CreateTime)
	if created == "" {
		created = now
	}
	updated := strings.TrimSpace(info.UpdateTime)
	if updated == "" {
		updated = created
	}
	title := strings.TrimSpace(info.Title)
	if title == "" {
		title = fallbackTitleFromFolderName(filepath.Base(folderPath))
	}
	source := strings.TrimSpace(info.Source)
	if source == "" {
		source = "manual"
	}
	status := strings.TrimSpace(info.Status)
	if status == "" {
		status = "active"
	}
	archiveStatus := strings.TrimSpace(info.ArchiveStatus)
	if archiveStatus == "" {
		archiveStatus = "local_active"
	}
	projectPath := strings.TrimSpace(info.ProjectPath)
	if projectPath == "" {
		projectPath = filepath.ToSlash(folderPath)
	}
	folderName := strings.TrimSpace(info.FolderName)
	if folderName == "" {
		folderName = filepath.Base(folderPath)
	}

	return []string{
		"type: task",
		"schema_version: 3",
		fmt.Sprintf("title: %s", title),
		fmt.Sprintf("status: %s", status),
		fmt.Sprintf("created: %s", created),
		fmt.Sprintf("updated: %s", updated),
		fmt.Sprintf("source: %s", source),
		fmt.Sprintf("department: %s", info.Department),
		fmt.Sprintf("project_path: %s", projectPath),
		fmt.Sprintf("folder_name: %s", folderName),
		fmt.Sprintf("archive_status: %s", archiveStatus),
		fmt.Sprintf("hash: %s", info.Hash),
		"tags:",
		"  - 工作材料",
	}
}

func upsertFrontmatterValue(frontLines []string, aliases []string, preferredKey string, value string) []string {
	aliasMap := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		aliasMap[normalizeKey(alias)] = true
	}

	replaced := false
	for i, line := range frontLines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if aliasMap[normalizeKey(key)] {
			if value == "" {
				frontLines[i] = fmt.Sprintf("%s:", key)
			} else {
				frontLines[i] = fmt.Sprintf("%s: %s", key, value)
			}
			replaced = true
		}
	}

	if !replaced {
		if value == "" {
			frontLines = append(frontLines, fmt.Sprintf("%s:", preferredKey))
		} else {
			frontLines = append(frontLines, fmt.Sprintf("%s: %s", preferredKey, value))
		}
	}
	return frontLines
}

func writeWorkRecordFile(filePath string, frontLines []string, body string) error {
	content := fmt.Sprintf("---\n%s\n---\n%s", strings.Join(frontLines, "\n"), strings.TrimLeft(body, "\n"))
	return os.WriteFile(filePath, []byte(content), 0o644)
}

func appendArchiveInfoSection(body string, destination string, archivedAt time.Time) string {
	body = strings.TrimSpace(body)
	archiveTime := archivedAt.Format("2006-01-02 15:04:05")
	destination = filepath.ToSlash(destination)

	updateLine := func(input, prefix, value string) string {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(prefix) + `.*$`)
		line := fmt.Sprintf("%s%s", prefix, value)
		if re.MatchString(input) {
			return re.ReplaceAllString(input, line)
		}
		if strings.TrimSpace(input) == "" {
			return line
		}
		return strings.TrimRight(input, "\n") + "\n" + line
	}

	if strings.Contains(body, "## 归档记录") {
		body = updateLine(body, "- 本地状态：", "archived")
		body = updateLine(body, "- 归档位置：", destination)
		body = updateLine(body, "- 归档时间：", archiveTime)
		return body + "\n"
	}

	if body != "" {
		body += "\n\n"
	}
	body += "## 归档记录\n\n"
	body += "- 本地状态：archived\n"
	body += fmt.Sprintf("- 归档位置：%s\n", destination)
	body += fmt.Sprintf("- 归档时间：%s\n", archiveTime)
	return body
}

func markWorkRecordArchived(workRecordPath string, destination string, archivedAt time.Time) error {
	parsed, err := readWorkRecord(workRecordPath)
	if err != nil {
		return err
	}

	folderPath := filepath.Dir(workRecordPath)
	front := ensureFrontmatterLines(parsed, folderPath)
	front = upsertFrontmatterValue(front, []string{"status"}, "status", "archived")
	front = upsertFrontmatterValue(front, []string{"archive_status"}, "archive_status", "local_archive")
	front = upsertFrontmatterValue(front, []string{"updated"}, "updated", archivedAt.Format("2006-01-02"))
	front = upsertFrontmatterValue(front, []string{"project_path", "projectPath"}, "project_path", filepath.ToSlash(folderPath))
	front = upsertFrontmatterValue(front, []string{"folder_name", "folderName"}, "folder_name", filepath.Base(folderPath))

	body := appendArchiveInfoSection(parsed.Body, destination, archivedAt)
	return writeWorkRecordFile(workRecordPath, front, body)
}

func countFilesRecursively(folderPath string) int {
	count := 0
	_ = filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func normalizeScanPath(path string) string {
	return filepath.Clean(path)
}

func normalizePathKey(path string) string {
	normalized := normalizeScanPath(path)
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

func readScannedFolder(folderPath, name string) (map[string]interface{}, bool) {
	folderPath = normalizeScanPath(folderPath)
	wrPath := filepath.Join(folderPath, workRecordFileName)
	if _, err := os.Stat(wrPath); os.IsNotExist(err) {
		return nil, false
	}

	info, err := parseWorkRecord(wrPath)
	if err != nil {
		return nil, false
	}

	modified := ""
	if fi, err := os.Stat(folderPath); err == nil {
		modified = fi.ModTime().Format(time.RFC3339)
	}

	createTime := strings.TrimSpace(info.CreateTime)
	if createTime == "" {
		createTime = modified
	}

	return map[string]interface{}{
		"name":            name,
		"path":            normalizeScanPath(folderPath),
		"modified":        modified,
		"has_work_record": true,
		"department":      info.Department,
		"create_time":     createTime,
		"update_time":     info.UpdateTime,
		"source":          info.Source,
		"content":         info.Content,
		"raw_content":     info.RawContent,
		"file_count":      countFilesRecursively(folderPath),
		"hash":            info.Hash,
		"status":          info.Status,
		"archive_status":  info.ArchiveStatus,
		"schema_version":  info.SchemaVersion,
		"project_path":    info.ProjectPath,
		"folder_name":     info.FolderName,
		"title":           info.Title,
	}, true
}

func collectScannedFolders(scanPath string, recursive bool) ([]map[string]interface{}, error) {
	var folders []map[string]interface{}
	added := map[string]bool{}
	scanPath = normalizeScanPath(scanPath)

	appendFolder := func(folderPath string) {
		cleanPath := normalizeScanPath(folderPath)
		key := normalizePathKey(cleanPath)
		if added[key] {
			return
		}
		folder, ok := readScannedFolder(cleanPath, filepath.Base(cleanPath))
		if !ok {
			return
		}
		added[key] = true
		folders = append(folders, folder)
	}

	if recursive {
		err := filepath.WalkDir(scanPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if normalizePathKey(path) == normalizePathKey(scanPath) {
				return nil
			}
			if _, err := os.Stat(filepath.Join(path, workRecordFileName)); err == nil {
				appendFolder(path)
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(scanPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			appendFolder(filepath.Join(scanPath, entry.Name()))
		}
	}

	sort.Slice(folders, func(i, j int) bool {
		a, b := folders[i], folders[j]
		ta := parseTimeLoose(fmt.Sprint(a["create_time"]))
		tb := parseTimeLoose(fmt.Sprint(b["create_time"]))
		if ta.IsZero() && tb.IsZero() {
			return fmt.Sprint(a["name"]) < fmt.Sprint(b["name"])
		}
		if ta.IsZero() {
			return false
		}
		if tb.IsZero() {
			return true
		}
		return ta.After(tb)
	})

	return folders, nil
}

func handleScanWorkFolders(w http.ResponseWriter, r *http.Request) {
	scanPath := r.URL.Query().Get("scan_path")
	if scanPath == "" {
		scanPath = "~/Desktop"
	}
	scanPath = normalizeScanPath(getBaseFolder(scanPath))
	recursive := r.URL.Query().Get("recursive") == "true"

	folders, err := collectScannedFolders(scanPath, recursive)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("无法读取目录: %v", err))
		return
	}

	if folders == nil {
		folders = []map[string]interface{}{}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"scan_path": scanPath,
		"count":     len(folders),
		"folders":   folders,
	})
}

type ArchiveMoveRequest struct {
	FolderPath  string `json:"folder_path"`
	ArchivePath string `json:"archive_path"`
}

func doArchiveMove(folderPath, archivePath string) (string, error) {
	archivePath = getBaseFolder(archivePath)
	folderPath = filepath.Clean(folderPath)
	folderName := filepath.Base(folderPath)

	year := "其他"
	if len(folderName) >= 4 {
		if _, err := strconv.Atoi(folderName[:4]); err == nil {
			year = folderName[:4]
		}
	}

	destDir := filepath.Join(archivePath, year)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	destPath := filepath.Join(destDir, folderName)
	if _, err := os.Stat(destPath); err == nil {
		return "", fmt.Errorf("目标路径已存在: %s", destPath)
	}

	if err := os.Rename(folderPath, destPath); err != nil {
		return "", fmt.Errorf("移动失败: %v", err)
	}

	wrPath := filepath.Join(destPath, workRecordFileName)
	if _, err := os.Stat(wrPath); err == nil {
		_ = markWorkRecordArchived(wrPath, destPath, time.Now())
	}

	return destPath, nil
}

func handleArchiveMove(w http.ResponseWriter, r *http.Request) {
	var req ArchiveMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "无效的请求参数")
		return
	}

	destPath, err := doArchiveMove(req.FolderPath, req.ArchivePath)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"source":      req.FolderPath,
		"destination": destPath,
		"message":     fmt.Sprintf("已归档到: %s", destPath),
	})
}

type BatchMoveRequest struct {
	Items []ArchiveMoveRequest `json:"items"`
}

func handleArchiveBatchMove(w http.ResponseWriter, r *http.Request) {
	var req BatchMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "无效的请求参数")
		return
	}

	results := make([]map[string]interface{}, 0, len(req.Items))
	successCount := 0
	failCount := 0

	for _, item := range req.Items {
		destPath, err := doArchiveMove(item.FolderPath, item.ArchivePath)
		if err != nil {
			failCount++
			results = append(results, map[string]interface{}{
				"source":  item.FolderPath,
				"success": false,
				"message": err.Error(),
			})
			continue
		}

		successCount++
		results = append(results, map[string]interface{}{
			"source":      item.FolderPath,
			"destination": destPath,
			"success":     true,
			"message":     "归档成功",
		})
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"total":         len(req.Items),
		"success_count": successCount,
		"fail_count":    failCount,
		"results":       results,
	})
}

type UpdateWorkRecordRequest struct {
	FolderPath   string `json:"folder_path"`
	Department   string `json:"department"`
	Content      string `json:"content"`
	Title        string `json:"title"`
	RenameFolder bool   `json:"rename_folder"`
}

func updateMarkdownTitle(body string, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return body
	}

	normalizedBody := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(normalizedBody, "\n")
	for i := range lines {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "# ") {
			lines[i] = "# " + title
			return strings.Join(lines, "\n")
		}
	}

	trimmed := strings.TrimSpace(normalizedBody)
	if trimmed == "" {
		return "# " + title + "\n"
	}
	return "# " + title + "\n\n" + trimmed + "\n"
}

func buildRenamedFolderName(oldFolderName, title, created string) (string, error) {
	titlePart := sanitizeFolderName(title)
	if strings.TrimSpace(titlePart) == "" {
		return "", fmt.Errorf("invalid title")
	}

	if matches := folderDatePrefixPattern.FindStringSubmatch(strings.TrimSpace(oldFolderName)); len(matches) == 4 {
		sep := matches[2]
		if sep == "" {
			sep = "_"
		}
		return matches[1] + sep + titlePart, nil
	}

	prefixTime := parseTimeLoose(created)
	if prefixTime.IsZero() {
		prefixTime = time.Now()
	}
	return prefixTime.Format("2006.01.02") + "_" + titlePart, nil
}

func handleUpdateWorkRecord(w http.ResponseWriter, r *http.Request) {
	var req UpdateWorkRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	currentFolderPath := filepath.Clean(strings.TrimSpace(req.FolderPath))
	if currentFolderPath == "" {
		jsonError(w, http.StatusBadRequest, "folder_path is required")
		return
	}

	wrPath := filepath.Join(currentFolderPath, workRecordFileName)
	if _, err := os.Stat(wrPath); os.IsNotExist(err) {
		jsonError(w, http.StatusNotFound, "work record not found")
		return
	}

	parsed, err := readWorkRecord(wrPath)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to parse work record")
		return
	}

	body := parsed.Body
	if strings.TrimSpace(req.Content) != "" {
		body = strings.TrimSpace(req.Content) + "\n"
	}

	finalTitle := strings.TrimSpace(req.Title)
	if finalTitle == "" {
		finalTitle = strings.TrimSpace(parsed.Info.Title)
	}
	if finalTitle == "" {
		finalTitle = fallbackTitleFromFolderName(filepath.Base(currentFolderPath))
	}

	if strings.TrimSpace(req.Title) != "" {
		body = updateMarkdownTitle(body, req.Title)
	}

	targetFolderPath := currentFolderPath
	targetFolderName := filepath.Base(currentFolderPath)
	renamed := false
	if req.RenameFolder && strings.TrimSpace(req.Title) != "" {
		newFolderName, err := buildRenamedFolderName(filepath.Base(currentFolderPath), req.Title, parsed.Info.CreateTime)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if newFolderName != filepath.Base(currentFolderPath) {
			targetFolderPath = filepath.Join(filepath.Dir(currentFolderPath), newFolderName)
			if _, err := os.Stat(targetFolderPath); err == nil {
				jsonError(w, http.StatusConflict, fmt.Sprintf("destination already exists: %s", targetFolderPath))
				return
			}
			if err := os.Rename(currentFolderPath, targetFolderPath); err != nil {
				jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to rename folder: %v", err))
				return
			}
			renamed = true
			targetFolderName = newFolderName
		}
	}

	front := ensureFrontmatterLines(parsed, targetFolderPath)
	if strings.TrimSpace(req.Department) != "" {
		front = upsertFrontmatterValue(front, []string{"department"}, "department", strings.TrimSpace(req.Department))
	}
	front = upsertFrontmatterValue(front, []string{"schema_version"}, "schema_version", "3")
	front = upsertFrontmatterValue(front, []string{"title"}, "title", finalTitle)
	front = upsertFrontmatterValue(front, []string{"updated"}, "updated", time.Now().Format("2006-01-02"))
	front = upsertFrontmatterValue(front, []string{"project_path", "projectPath"}, "project_path", filepath.ToSlash(targetFolderPath))
	front = upsertFrontmatterValue(front, []string{"folder_name", "folderName"}, "folder_name", targetFolderName)

	targetWrPath := filepath.Join(targetFolderPath, workRecordFileName)
	if err := writeWorkRecordFile(targetWrPath, front, body); err != nil {
		if renamed {
			_ = os.Rename(targetFolderPath, currentFolderPath)
		}
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to write work record: %v", err))
		return
	}

	coreContent := extractWorkCoreContent(body)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "work record updated",
		"path":         targetFolderPath,
		"name":         targetFolderName,
		"title":        finalTitle,
		"content":      strings.TrimSpace(body),
		"core_content": coreContent,
	})
}

// GenerateHash creates a short SHA-256 hash (first 16 hex chars) from the input string.
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

// -- Daily Report Handlers --

type DailyReportAIConfig struct {
	APIURL  string `json:"api_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
}

type DailyReportItem struct {
	FolderPath string `json:"folder_path"`
	WorkRecord string `json:"work_record"`
}

type DailyReportGenerateRequest struct {
	Date  string              `json:"date"`
	Items []DailyReportItem   `json:"items"`
	AI    DailyReportAIConfig `json:"ai"`
}

type DailyReportLog struct {
	FolderPath string `json:"folder_path"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}

const dailyReportSystemPrompt = `You are an office work-log assistant.
Generate one natural, concise, formal Chinese daily log sentence from the provided real material.
Requirements:
1. Output only one paragraph without title or bullet points.
2. Keep it between 40 and 100 Chinese characters.
3. Focus on actual progress today, current status, and next step.
4. Do not fabricate people, meetings, numbers, or outcomes.
5. If information is limited, use conservative wording.`

func normalizeAIEndpoint(apiURL string) string {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return ""
	}
	apiURL = strings.TrimRight(apiURL, "/")
	if strings.Contains(apiURL, "/chat/completions") {
		return apiURL
	}
	if strings.HasSuffix(apiURL, "/v1") {
		return apiURL + "/chat/completions"
	}
	return apiURL + "/v1/chat/completions"
}

var dailyLogPrefixPattern = regexp.MustCompile(`^(?:[-*+]\s*|[0-9]+[.)]\s*)+`)

func cleanDailyLog(log string) string {
	log = strings.TrimSpace(strings.ReplaceAll(log, "\r\n", "\n"))
	lines := strings.Split(log, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts = append(parts, line)
	}

	log = strings.Join(parts, " ")
	log = strings.Trim(log, "\"'` ")
	log = dailyLogPrefixPattern.ReplaceAllString(log, "")
	return strings.TrimSpace(log)
}

func truncateRunes(input string, max int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

func normalizeSectionLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	line = strings.TrimPrefix(line, "+ ")
	line = strings.TrimPrefix(line, "> ")
	line = dailyLogPrefixPattern.ReplaceAllString(line, "")
	return strings.TrimSpace(line)
}

func extractSectionContent(body string, headers ...string) string {
	if len(headers) == 0 {
		return ""
	}

	headerSet := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		headerSet[header] = struct{}{}
	}

	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if _, ok := headerSet[strings.TrimSpace(line)]; ok {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return ""
	}

	collected := []string{}
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "## ") {
			break
		}
		line = normalizeSectionLine(line)
		if line != "" {
			collected = append(collected, line)
		}
	}
	return strings.Join(collected, " ")
}

func buildDailyReportInput(date, title, department, coreContent string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "\u672a\u547d\u540d\u4efb\u52a1"
	}
	department = strings.TrimSpace(department)
	if department == "" {
		department = "\u672a\u6307\u5b9a"
	}
	coreContent = strings.TrimSpace(coreContent)
	if coreContent == "" {
		coreContent = "\u6682\u65e0\u53ef\u63d0\u53d6\u7684\u5de5\u4f5c\u6838\u5fc3\u5185\u5bb9"
	}

	return fmt.Sprintf("\u65e5\u671f\uff1a%s\n\u4efb\u52a1\u6807\u9898\uff1a%s\n\u6240\u5c5e\u90e8\u95e8\uff1a%s\n\n\u5de5\u4f5c\u6838\u5fc3\u5185\u5bb9\uff1a\n%s", date, title, department, coreContent)
}

func fallbackDailyLog(title, coreContent string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "\u672a\u547d\u540d\u4efb\u52a1"
	}

	coreContent = cleanDailyLog(coreContent)
	if coreContent == "" {
		return truncateRunes(fmt.Sprintf("\u4eca\u65e5\u6301\u7eed\u63a8\u8fdb\u300a%s\u300b\u76f8\u5173\u5de5\u4f5c\uff0c\u5df2\u5b8c\u6210\u57fa\u7840\u68b3\u7406\uff0c\u4e0b\u4e00\u6b65\u7ee7\u7eed\u5b8c\u5584\u5e76\u5f62\u6210\u6210\u679c\u8f93\u51fa\u3002", title), 120)
	}

	compressed := truncateRunes(coreContent, 70)
	content := fmt.Sprintf("\u4eca\u65e5\u56f4\u7ed5\u300a%s\u300b\u63a8\u8fdb\uff1a%s\u3002\u540e\u7eed\u5c06\u7ee7\u7eed\u5b8c\u5584\u5e76\u5f62\u6210\u6210\u679c\u8f93\u51fa\u3002", truncateRunes(title, 20), compressed)
	return truncateRunes(content, 120)
}

func generateDailyLogWithAI(cfg DailyReportAIConfig, reportInput string) (string, error) {
	endpoint := normalizeAIEndpoint(cfg.APIURL)
	if endpoint == "" {
		return "", fmt.Errorf("empty api endpoint")
	}

	userPrompt := reportInput
	payload := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": dailyReportSystemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai api error: %s", strings.TrimSpace(string(respBody)))
	}

	var aiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		return "", err
	}
	if len(aiResp.Choices) == 0 {
		return "", fmt.Errorf("empty ai choices")
	}

	content := strings.TrimSpace(aiResp.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(aiResp.Choices[0].Text)
	}
	content = cleanDailyLog(content)
	if content == "" {
		return "", fmt.Errorf("empty ai content")
	}
	return content, nil
}

func handleGenerateDailyReport(w http.ResponseWriter, r *http.Request) {
	var req DailyReportGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Items) == 0 {
		jsonError(w, http.StatusBadRequest, "items cannot be empty")
		return
	}

	reportDate := strings.TrimSpace(req.Date)
	if reportDate == "" {
		reportDate = time.Now().Format("2006-01-02")
	}

	aiEnabled := req.AI.Enabled &&
		strings.TrimSpace(req.AI.APIURL) != "" &&
		strings.TrimSpace(req.AI.Model) != "" &&
		strings.TrimSpace(req.AI.APIKey) != ""

	logs := make([]DailyReportLog, 0, len(req.Items))
	for _, item := range req.Items {
		title := fallbackTitleFromFolderName(filepath.Base(item.FolderPath))
		department := ""
		coreContent := ""

		if strings.TrimSpace(item.WorkRecord) != "" {
			providedBody := strings.TrimSpace(item.WorkRecord)
			coreContent = extractWorkCoreContent(providedBody)
			if bodyTitle := strings.TrimSpace(extractTitleFromBody(providedBody)); bodyTitle != "" {
				title = bodyTitle
			}
		}

		if strings.TrimSpace(item.FolderPath) != "" {
			wrPath := filepath.Join(item.FolderPath, workRecordFileName)
			if parsed, err := readWorkRecord(wrPath); err == nil {
				if strings.TrimSpace(parsed.Info.Title) != "" {
					title = parsed.Info.Title
				}
				department = strings.TrimSpace(parsed.Info.Department)
				if coreContent == "" {
					coreContent = strings.TrimSpace(parsed.Info.Content)
				}
			}
		}

		if coreContent == "" {
			coreContent = truncateRunes(cleanDailyLog(item.WorkRecord), 220)
		}

		reportInput := buildDailyReportInput(reportDate, title, department, coreContent)
		content := fallbackDailyLog(title, coreContent)
		if aiEnabled {
			if generated, err := generateDailyLogWithAI(req.AI, reportInput); err == nil {
				content = generated
			}
		}

		content = truncateRunes(cleanDailyLog(content), 160)
		if content == "" {
			content = fallbackDailyLog(title, coreContent)
		}

		logs = append(logs, DailyReportLog{
			FolderPath: item.FolderPath,
			Title:      title,
			Content:    content,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"date":    reportDate,
		"logs":    logs,
	})
}
