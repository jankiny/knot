package api

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WorkRecordInfo holds parsed info from 工作记录.md
type WorkRecordInfo struct {
	RecordType      string `json:"type"`
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
	AIAccess        string `json:"ai_access"`
}

type parsedWorkRecord struct {
	Info         *WorkRecordInfo
	FrontLines   []string
	Body         string
	RawFile      []byte
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
		RecordType:    "task",
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

	info.RecordType = get("type")
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
	info.AIAccess = get("ai_access", "aiAccess")

	schemaValue := get("schema_version")
	if schemaValue != "" {
		if v, err := strconv.Atoi(schemaValue); err == nil {
			info.SchemaVersion = v
		}
	}

	if strings.TrimSpace(info.Source) == "" {
		info.Source = "manual"
	}
	if strings.TrimSpace(info.RecordType) == "" {
		info.RecordType = "task"
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
		RawFile:      data,
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
		fmt.Sprintf("type: %s", info.RecordType),
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
