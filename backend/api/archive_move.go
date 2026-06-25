package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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
