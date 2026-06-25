package api

import (
	"encoding/json"
	"fmt"
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
)

// -- Archive Handlers --

// WorkRecordInfo holds parsed info from 工作记录.md
type WorkRecordInfo struct {
	Title           string `json:"title"`
	Department      string `json:"department"`
	Project         string `json:"project"`
	CreateTime      string `json:"create_time"`
	UpdateTime      string `json:"update_time"`
	TaskDate        string `json:"task_date"`
	Source          string `json:"source"`
	Content         string `json:"content"`
	RawContent      string `json:"raw_content"`
	Hash            string `json:"hash"`
	Status          string `json:"status"`
	ArchiveStatus   string `json:"archive_status"`
	SchemaVersion   int    `json:"schema_version"`
	ProjectPath     string `json:"project_path"`
	FolderName      string `json:"folder_name"`
	SOPTemplateID   string `json:"sop_template_id"`
	SOPTemplateName string `json:"sop_template_name"`
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
	info.Project = get("project")
	info.CreateTime = get("created")
	info.UpdateTime = get("updated")
	info.TaskDate = get("task_date", "taskDate", "marked_time", "markedTime")
	info.Source = get("source")
	info.Hash = get("hash")
	info.Status = get("status")
	info.ArchiveStatus = get("archive_status", "archiveStatus")
	info.ProjectPath = get("project_path", "projectPath")
	info.FolderName = get("folder_name", "folderName")
	info.SOPTemplateID = get("sop_template_id", "sopTemplateId")
	info.SOPTemplateName = get("sop_template_name", "sopTemplateName")

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
	if strings.TrimSpace(info.TaskDate) == "" {
		info.TaskDate = info.CreateTime
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
	taskDate := strings.TrimSpace(info.TaskDate)
	if taskDate == "" {
		taskDate = created
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
		fmt.Sprintf("task_date: %s", taskDate),
		fmt.Sprintf("source: %s", source),
		fmt.Sprintf("department: %s", info.Department),
		fmt.Sprintf("project: %s", info.Project),
		fmt.Sprintf("sop_template_id: %s", info.SOPTemplateID),
		fmt.Sprintf("sop_template_name: %s", info.SOPTemplateName),
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

func appendRestoreInfoSection(body string, destination string, restoredAt time.Time) string {
	body = strings.TrimSpace(body)
	if body != "" {
		body += "\n\n"
	}
	body += "## 恢复记录\n\n"
	body += fmt.Sprintf("- 恢复位置：%s\n", filepath.ToSlash(destination))
	body += fmt.Sprintf("- 恢复时间：%s\n", restoredAt.Format("2006-01-02 15:04:05"))
	return body + "\n"
}

func markWorkRecordRestored(workRecordPath string, destination string, restoredAt time.Time) error {
	parsed, err := readWorkRecord(workRecordPath)
	if err != nil {
		return err
	}

	folderPath := filepath.Dir(workRecordPath)
	front := ensureFrontmatterLines(parsed, folderPath)
	front = upsertFrontmatterValue(front, []string{"status"}, "status", "active")
	front = upsertFrontmatterValue(front, []string{"archive_status"}, "archive_status", "local_active")
	front = upsertFrontmatterValue(front, []string{"updated"}, "updated", restoredAt.Format("2006-01-02"))
	front = upsertFrontmatterValue(front, []string{"project_path", "projectPath"}, "project_path", filepath.ToSlash(folderPath))
	front = upsertFrontmatterValue(front, []string{"folder_name", "folderName"}, "folder_name", filepath.Base(folderPath))

	body := appendRestoreInfoSection(parsed.Body, destination, restoredAt)
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
	taskDate := strings.TrimSpace(info.TaskDate)
	if taskDate == "" {
		taskDate = createTime
	}

	return map[string]interface{}{
		"name":              name,
		"path":              normalizeScanPath(folderPath),
		"modified":          modified,
		"has_work_record":   true,
		"department":        info.Department,
		"project":           info.Project,
		"create_time":       createTime,
		"update_time":       info.UpdateTime,
		"task_date":         taskDate,
		"source":            info.Source,
		"content":           info.Content,
		"raw_content":       info.RawContent,
		"file_count":        countFilesRecursively(folderPath),
		"hash":              info.Hash,
		"status":            info.Status,
		"archive_status":    info.ArchiveStatus,
		"schema_version":    info.SchemaVersion,
		"project_path":      info.ProjectPath,
		"folder_name":       info.FolderName,
		"title":             info.Title,
		"sop_template_id":   info.SOPTemplateID,
		"sop_template_name": info.SOPTemplateName,
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
		ta := parseTimeLoose(fmt.Sprint(a["task_date"]))
		tb := parseTimeLoose(fmt.Sprint(b["task_date"]))
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

func handleArchiveList(w http.ResponseWriter, r *http.Request) {
	archivePath := strings.TrimSpace(r.URL.Query().Get("archive_path"))
	if archivePath == "" {
		jsonError(w, http.StatusBadRequest, "archive_path is required")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}
	if pageSize > 200 {
		pageSize = 200
	}

	keyword := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("keyword")))
	year := strings.TrimSpace(r.URL.Query().Get("year"))
	archivePath = normalizeScanPath(getBaseFolder(archivePath))

	folders, err := collectScannedFolders(archivePath, true)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("无法读取归档目录: %v", err))
		return
	}

	filtered := make([]map[string]interface{}, 0, len(folders))
	years := map[string]bool{}
	for _, folder := range folders {
		taskDate := fmt.Sprint(folder["task_date"])
		if len(taskDate) >= 4 {
			years[taskDate[:4]] = true
		}
		if year != "" && (len(taskDate) < 4 || taskDate[:4] != year) {
			continue
		}
		if keyword != "" {
			searchText := strings.ToLower(strings.Join([]string{
				fmt.Sprint(folder["title"]),
				fmt.Sprint(folder["name"]),
				fmt.Sprint(folder["content"]),
				fmt.Sprint(folder["department"]),
				fmt.Sprint(folder["project"]),
			}, " "))
			if !strings.Contains(searchText, keyword) {
				continue
			}
		}
		filtered = append(filtered, folder)
	}

	yearList := make([]string, 0, len(years))
	for y := range years {
		yearList = append(yearList, y)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(yearList)))

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"count":     len(filtered[start:end]),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  end < total,
		"years":     yearList,
		"folders":   filtered[start:end],
	})
}

type ArchiveMoveRequest struct {
	FolderPath    string `json:"folder_path"`
	ArchivePath   string `json:"archive_path"`
	UseYearFolder *bool  `json:"use_year_folder"`
}

func doArchiveMove(folderPath, archivePath string, useYearFolder bool) (string, error) {
	archivePath = getBaseFolder(archivePath)
	folderPath = filepath.Clean(folderPath)
	folderName := filepath.Base(folderPath)

	year := "其他"
	if len(folderName) >= 4 {
		if _, err := strconv.Atoi(folderName[:4]); err == nil {
			year = folderName[:4]
		}
	}

	destDir := archivePath
	if useYearFolder {
		destDir = filepath.Join(archivePath, year)
	}
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

	useYearFolder := true
	if req.UseYearFolder != nil {
		useYearFolder = *req.UseYearFolder
	}

	destPath, err := doArchiveMove(req.FolderPath, req.ArchivePath, useYearFolder)
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
		useYearFolder := true
		if item.UseYearFolder != nil {
			useYearFolder = *item.UseYearFolder
		}
		destPath, err := doArchiveMove(item.FolderPath, item.ArchivePath, useYearFolder)
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

type ArchiveRestoreRequest struct {
	FolderPath  string `json:"folder_path"`
	RestorePath string `json:"restore_path"`
	NewName     string `json:"new_name"`
}

func uniqueRestorePath(dir, name string) string {
	base := sanitizeFolderName(name)
	if base == "" {
		base = "restored_task"
	}
	candidate := filepath.Join(dir, base)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 1; i <= 99; i++ {
		next := filepath.Join(dir, fmt.Sprintf("%s_恢复%d", base, i))
		if _, err := os.Stat(next); os.IsNotExist(err) {
			return next
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s_%d", base, time.Now().Unix()))
}

func handleArchiveRestore(w http.ResponseWriter, r *http.Request) {
	var req ArchiveRestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "无效的请求参数")
		return
	}

	source := filepath.Clean(strings.TrimSpace(req.FolderPath))
	if source == "" {
		jsonError(w, http.StatusBadRequest, "folder_path is required")
		return
	}
	restoreRoot := getBaseFolder(req.RestorePath)
	if err := os.MkdirAll(restoreRoot, 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("创建恢复目录失败: %v", err))
		return
	}

	name := strings.TrimSpace(req.NewName)
	if name == "" {
		name = filepath.Base(source)
	}
	destPath := uniqueRestorePath(restoreRoot, name)
	if err := os.Rename(source, destPath); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("恢复失败: %v", err))
		return
	}

	wrPath := filepath.Join(destPath, workRecordFileName)
	if _, err := os.Stat(wrPath); err == nil {
		_ = markWorkRecordRestored(wrPath, destPath, time.Now())
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"source":      source,
		"destination": destPath,
		"message":     fmt.Sprintf("已恢复到当前工作: %s", destPath),
	})
}

type UpdateWorkRecordRequest struct {
	FolderPath   string `json:"folder_path"`
	Department   string `json:"department"`
	Project      string `json:"project"`
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
		newFolderName, err := buildRenamedFolderName(filepath.Base(currentFolderPath), req.Title, parsed.Info.TaskDate)
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
		front = upsertFrontmatterValue(front, []string{"project"}, "project", "")
	}
	if strings.TrimSpace(req.Project) != "" {
		front = upsertFrontmatterValue(front, []string{"project"}, "project", strings.TrimSpace(req.Project))
		front = upsertFrontmatterValue(front, []string{"department"}, "department", "")
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
