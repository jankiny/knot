package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"knot-backend/safepath"
)

func countFilesRecursively(folderPath string) int {
	return countFilesRecursivelyWithAuthorizer(folderPath, nil)
}

func countFilesRecursivelyWithAuthorizer(
	folderPath string,
	authorizeSensitivePath func(string) bool,
) int {
	count := 0
	_ = filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() &&
			normalizePathKey(path) != normalizePathKey(folderPath) &&
			!localPathAllowed(path, authorizeSensitivePath) {
			return filepath.SkipDir
		}
		if !d.IsDir() && localPathAllowed(path, authorizeSensitivePath) {
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
	return readScannedFolderWithAuthorizer(folderPath, name, nil)
}

func readScannedFolderWithAuthorizer(
	folderPath string,
	name string,
	authorizeSensitivePath func(string) bool,
) (map[string]interface{}, bool) {
	folderPath = normalizeScanPath(folderPath)
	if !localPathAllowed(folderPath, authorizeSensitivePath) {
		return nil, false
	}
	wrPath := filepath.Join(folderPath, workRecordFileName)
	if !localPathAllowed(wrPath, authorizeSensitivePath) {
		return nil, false
	}
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
		"type":            info.RecordType,
		"name":            name,
		"path":            normalizeScanPath(folderPath),
		"modified":        modified,
		"has_work_record": true,
		"department":      info.Department,
		"project":         info.Project,
		"create_time":     createTime,
		"update_time":     info.UpdateTime,
		"task_date":       taskDate,
		"source":          info.Source,
		"content":         info.Content,
		"raw_content":     info.RawContent,
		"file_count": countFilesRecursivelyWithAuthorizer(
			folderPath,
			authorizeSensitivePath,
		),
		"hash":              info.Hash,
		"status":            info.Status,
		"archive_status":    info.ArchiveStatus,
		"schema_version":    info.SchemaVersion,
		"project_path":      info.ProjectPath,
		"folder_name":       info.FolderName,
		"title":             info.Title,
		"sop_template_id":   info.SOPTemplateID,
		"sop_template_name": info.SOPTemplateName,
		"ai_access":         info.AIAccess,
	}, true
}

func collectScannedFolders(scanPath string, recursive bool) ([]map[string]interface{}, error) {
	return collectScannedFoldersWithAuthorizer(scanPath, recursive, nil)
}

func collectScannedFoldersWithAuthorizer(
	scanPath string,
	recursive bool,
	authorizeSensitivePath func(string) bool,
) ([]map[string]interface{}, error) {
	folders := []map[string]interface{}{}
	added := map[string]bool{}
	scanPath = normalizeScanPath(scanPath)
	if !localPathAllowed(scanPath, authorizeSensitivePath) {
		return folders, nil
	}

	appendFolder := func(folderPath string) {
		cleanPath := normalizeScanPath(folderPath)
		if !localPathAllowed(cleanPath, authorizeSensitivePath) {
			return
		}
		key := normalizePathKey(cleanPath)
		if added[key] {
			return
		}
		folder, ok := readScannedFolderWithAuthorizer(
			cleanPath,
			filepath.Base(cleanPath),
			authorizeSensitivePath,
		)
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
			if !localPathAllowed(path, authorizeSensitivePath) {
				return filepath.SkipDir
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
			folderPath := filepath.Join(scanPath, entry.Name())
			if !localPathAllowed(folderPath, authorizeSensitivePath) {
				continue
			}
			appendFolder(folderPath)
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

func localPathAllowed(
	value string,
	authorizeSensitivePath func(string) bool,
) bool {
	if !isSensitivePath(value) {
		return true
	}
	return authorizeSensitivePath != nil && authorizeSensitivePath(value)
}

func handleScanWorkFolders(w http.ResponseWriter, r *http.Request) {
	handleScanWorkFoldersWithResolver(w, r, nil)
}

func handleScanWorkFoldersWithResolver(
	w http.ResponseWriter,
	r *http.Request,
	resolver *safepath.Resolver,
) {
	scanPath := r.URL.Query().Get("scan_path")
	if scanPath == "" {
		scanPath = "~/Desktop"
	}
	scanPath = normalizeScanPath(getBaseFolder(scanPath))
	recursive := r.URL.Query().Get("recursive") == "true"

	folders, err := collectScannedFoldersWithAuthorizer(
		scanPath,
		recursive,
		localPathAuthorizer(r.Context(), resolver, false),
	)
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
	handleArchiveListWithResolver(w, r, nil)
}

func handleArchiveListWithResolver(
	w http.ResponseWriter,
	r *http.Request,
	resolver *safepath.Resolver,
) {
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

	folders, err := collectScannedFoldersWithAuthorizer(
		archivePath,
		true,
		localPathAuthorizer(r.Context(), resolver, false),
	)
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
