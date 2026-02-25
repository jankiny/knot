package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"knot-backend/mail"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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

		r.Get("/archive/scan", handleScanWorkFolders)
		r.Post("/archive/move", handleArchiveMove)
		r.Post("/archive/batch-move", handleArchiveBatchMove)
		r.Post("/archive/update-work-record", handleUpdateWorkRecord)
	})

	return r
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
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
		jsonError(w, http.StatusBadRequest, "请先连接邮件服务器")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit == 0 {
		limit = 50
	}

	daysStr := r.URL.Query().Get("days")
	days, _ := strconv.Atoi(daysStr)

	mails, err := mailClient.FetchMailList(limit, days)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "data": mails})
}

func handleGetAttachments(w http.ResponseWriter, r *http.Request) {
	if mailClient == nil {
		jsonError(w, http.StatusBadRequest, "请先连接邮件服务器")
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
		jsonError(w, http.StatusBadRequest, "请先连接邮件服务器")
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
}

func getBaseFolder(basePath string) string {
	if basePath != "" {
		if basePath[:1] == "~" {
			home, _ := os.UserHomeDir()
			basePath = filepath.Join(home, basePath[1:])
		}
		if filepath.IsAbs(basePath) {
			os.MkdirAll(basePath, 0755)
			return basePath
		}
	}
	home, _ := os.UserHomeDir()
	desktop := filepath.Join(home, "Desktop")
	if _, err := os.Stat(desktop); os.IsNotExist(err) {
		desktop = filepath.Join(home, "桌面")
	}
	return desktop
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
	folderName := req.FolderName
	if folderName == "" {
		folderName = "Folder_" + strconv.FormatInt(time.Now().Unix(), 10) // fallback
	}

	folderPath := filepath.Join(baseFolder, folderName)
	os.MkdirAll(folderPath, 0755)

	contentPath := folderPath
	if req.UseSubFolder && req.SubFolderName != "" {
		contentPath = filepath.Join(folderPath, req.SubFolderName)
		os.MkdirAll(contentPath, 0755)
	}

	var mailFiles []string
	if req.SaveMailContent && req.Body != "" {
		for _, format := range req.SaveFormats {
			filename := fmt.Sprintf("%s.%s", req.MailContentFileName, format)
			if req.MailContentFileName == "" {
				filename = fmt.Sprintf("邮件正文.%s", format)
			}
			fpath := filepath.Join(contentPath, filename)
			
			// For Go PoC, write txt/html natively. 
			// If format is PDF, skip generation in Go (Handled by Electron frontend now!)
			if format == "txt" {
				txtContent := fmt.Sprintf("主题：%s\n发件人：%s\n日期：%s\n\n%s\n\n%s\n", req.Subject, req.FromAddr, req.Date, "==================================================", req.Body)
				os.WriteFile(fpath, []byte(txtContent), 0644)
				mailFiles = append(mailFiles, fpath)
			} else if format == "eml" {
				os.WriteFile(fpath, []byte(req.RawContent), 0644)
				mailFiles = append(mailFiles, fpath)
			}
		}
	}

	var downloaded []string
	if downloadAttachments && mailClient != nil {
		d, err := mailClient.DownloadAttachments(req.MailID, contentPath)
		if err == nil {
			downloaded = d
		}
	}

	// 始终生成工作记录.md（YAML frontmatter 格式）
	now := time.Now().Format("2006-01-02 15:04")
	source := req.Source
	if source == "" {
		source = "邮件"
	}
	department := req.Department // 可以为空
	wrContent := fmt.Sprintf("---\n归属部门: %s\n创建时间: %s\n来源: %s\n---\n# 工作记录\n\n> 此文件由 Knot（绳结）自动创建，用于归档和自动生成周报。\n\n<!-- 请在此记录工作过程，AI 将根据此内容生成周报 -->\n\n", department, now, source)
	wrPath := filepath.Join(folderPath, "工作记录.md")
	os.WriteFile(wrPath, []byte(wrContent), 0644)

	resp := map[string]interface{}{
		"success":      true,
		"path":         folderPath,
		"content_path": contentPath,
		"mail_files":   mailFiles,
		"work_record":  wrPath,
		"message":      fmt.Sprintf("文件夹已创建，已保存 %d 个附件", len(downloaded)),
	}
	if downloadAttachments {
		resp["attachments_downloaded"] = downloaded
	}

	jsonResponse(w, http.StatusOK, resp)
}

// -- Archive Handlers --

// WorkRecordInfo holds parsed info from 工作记录.md
type WorkRecordInfo struct {
	Department string `json:"department"`
	CreateTime string `json:"create_time"`
	Source     string `json:"source"`
	Content    string `json:"content"`
}

// parseWorkRecord parses 工作记录.md with YAML frontmatter format.
func parseWorkRecord(filePath string) (*WorkRecordInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	text := string(data)
	info := &WorkRecordInfo{}

	if !strings.HasPrefix(strings.TrimSpace(text), "---") {
		return nil, fmt.Errorf("invalid work record format: missing YAML frontmatter")
	}

	parts := strings.SplitN(text, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid work record format: incomplete frontmatter")
	}

	frontmatter := parts[1]
	scanner := bufio.NewScanner(strings.NewReader(frontmatter))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if kv := strings.SplitN(line, ":", 2); len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			switch key {
			case "归属部门":
				info.Department = val
			case "创建时间":
				info.CreateTime = val
			case "来源":
				info.Source = val
			}
		}
	}

	body := parts[2]
	// Extract content after the comment marker
	if idx := strings.Index(body, "-->"); idx != -1 {
		info.Content = strings.TrimSpace(body[idx+3:])
	} else if idx := strings.Index(body, "# 工作记录"); idx != -1 {
		info.Content = strings.TrimSpace(body[idx+len("# 工作记录"):])
	}

	return info, nil
}

func handleScanWorkFolders(w http.ResponseWriter, r *http.Request) {
	scanPath := r.URL.Query().Get("scan_path")
	if scanPath == "" {
		scanPath = "~/Desktop"
	}
	scanPath = getBaseFolder(scanPath)

	var folders []map[string]interface{}

	entries, err := os.ReadDir(scanPath)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("无法读取目录: %v", err))
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folderPath := filepath.Join(scanPath, entry.Name())
		wrPath := filepath.Join(folderPath, "工作记录.md")
		if _, err := os.Stat(wrPath); os.IsNotExist(err) {
			continue
		}

		info, err := parseWorkRecord(wrPath)
		if err != nil {
			continue
		}

		// Get folder modification time
		var modTime string
		if fi, err := entry.Info(); err == nil {
			modTime = fi.ModTime().Format("2006-01-02T15:04:05")
		}

		// Calculate folder size (file count)
		fileCount := 0
		filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				fileCount++
			}
			return nil
		})

		folders = append(folders, map[string]interface{}{
			"name":            entry.Name(),
			"path":            folderPath,
			"modified":        modTime,
			"has_work_record": true,
			"department":      info.Department,
			"create_time":     info.CreateTime,
			"source":          info.Source,
			"content":         info.Content,
			"file_count":      fileCount,
		})
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
	// Resolve archive path
	archivePath = getBaseFolder(archivePath)

	// Extract year from folder name (e.g. "2026.02.25_xxx" -> "2026")
	folderName := filepath.Base(folderPath)
	year := "其他"
	if len(folderName) >= 4 {
		if _, err := strconv.Atoi(folderName[:4]); err == nil {
			year = folderName[:4]
		}
	}

	destDir := filepath.Join(archivePath, year)
	os.MkdirAll(destDir, 0755)

	destPath := filepath.Join(destDir, folderName)
	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		return "", fmt.Errorf("目标路径已存在: %s", destPath)
	}

	if err := os.Rename(folderPath, destPath); err != nil {
		return "", fmt.Errorf("移动失败: %v", err)
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

	var results []map[string]interface{}
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
		} else {
			successCount++
			results = append(results, map[string]interface{}{
				"source":      item.FolderPath,
				"destination": destPath,
				"success":     true,
				"message":     "归档成功",
			})
		}
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
	FolderPath string `json:"folder_path"`
	Department string `json:"department"`
	Content    string `json:"content"`
}

func handleUpdateWorkRecord(w http.ResponseWriter, r *http.Request) {
	var req UpdateWorkRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "无效的请求参数")
		return
	}

	wrPath := filepath.Join(req.FolderPath, "工作记录.md")
	if _, err := os.Stat(wrPath); os.IsNotExist(err) {
		jsonError(w, http.StatusNotFound, "工作记录.md 不存在")
		return
	}

	// Parse existing record to preserve create_time and source
	existing, err := parseWorkRecord(wrPath)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "解析工作记录失败")
		return
	}

	// Apply updates
	if req.Department != "" {
		existing.Department = req.Department
	}
	if req.Content != "" {
		existing.Content = req.Content
	}

	source := existing.Source
	if source == "" {
		source = "邮件"
	}

	// Rewrite in new YAML frontmatter format
	newContent := fmt.Sprintf("---\n归属部门: %s\n创建时间: %s\n来源: %s\n---\n# 工作记录\n\n> 此文件由 Knot（绳结）自动创建，用于归档和自动生成周报。\n\n<!-- 请在此记录工作过程，AI 将根据此内容生成周报 -->\n\n%s\n", existing.Department, existing.CreateTime, source, existing.Content)

	if err := os.WriteFile(wrPath, []byte(newContent), 0644); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("写入失败: %v", err))
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "工作记录已更新",
	})
}
