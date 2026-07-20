package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// -- SOP Template Helpers --

type SOPTemplateFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type SOPTemplate struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	Description    string            `json:"description"`
	Folders        []string          `json:"folders"`
	Files          []SOPTemplateFile `json:"files,omitempty"`
	AllowedSources []string          `json:"allowed_sources,omitempty"`
	Builtin        bool              `json:"builtin"`
	Path           string            `json:"path,omitempty"`
}

const photoProjectSOPTemplateID = "photo-project"

func defaultSOPTemplates() []SOPTemplate {
	return []SOPTemplate{
		{
			ID:          defaultSOPTemplateID,
			Name:        "通用任务",
			Version:     "1.0.0",
			Description: "默认工作材料目录结构",
			Folders: []string{
				taskSourceDirName,
				taskProcessDirName,
				taskOutputDirName,
			},
			Builtin: true,
		},
		{
			ID:          "learning-notes",
			Name:        "学习笔记",
			Version:     "1.0.0",
			Description: "用于课程、培训、阅读资料的学习整理",
			Folders: []string{
				"00_学习资料",
				"01_学习笔记",
				"02_课后作业",
			},
			Files: []SOPTemplateFile{
				{
					Path: "01_学习笔记/学习笔记.md",
					Content: `# 学习笔记

## 核心概念

## 重点摘录

## 我的理解

## 待复习问题
`,
				},
				{
					Path: "02_课后作业/作业记录.md",
					Content: `# 作业记录

## 作业要求

## 完成过程

## 提交结果
`,
				},
			},
			Builtin: true,
		},
		{
			ID:          photoProjectSOPTemplateID,
			Name:        "照片项目",
			Version:     "1.0.0",
			Description: "用于 Lightroom Classic 原片、Photoshop 主文件和发布文件管理；仅支持快速创建",
			Folders: []string{
				"00_Originals",
				"10_Masters",
				"20_Exports/Web",
				"20_Exports/Social",
				"20_Exports/Print",
				"20_Exports/Delivery",
			},
			AllowedSources: []string{"manual"},
			Builtin:        true,
		},
	}
}

func sopTemplateSupportsSource(tpl SOPTemplate, source string) bool {
	if len(tpl.AllowedSources) == 0 {
		return true
	}
	source = strings.ToLower(strings.TrimSpace(source))
	for _, allowed := range tpl.AllowedSources {
		if strings.ToLower(strings.TrimSpace(allowed)) == source {
			return true
		}
	}
	return false
}

func sopTemplateRoots() []string {
	roots := []string{}
	if cfg, err := os.UserConfigDir(); err == nil && strings.TrimSpace(cfg) != "" {
		userRoot := filepath.Join(cfg, "Knot", "sop-templates")
		_ = os.MkdirAll(userRoot, 0o755)
		roots = append(roots, userRoot)
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		roots = append(roots, filepath.Join(cwd, "templates", "sop"))
	}
	return roots
}

func writeSeedSOPTemplate(root string, tpl SOPTemplate) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(tpl.ID) == "" {
		return
	}
	dir := filepath.Join(root, sanitizeFolderName(tpl.ID))
	sopPath := filepath.Join(dir, "sop.json")
	if _, err := os.Stat(sopPath); err == nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tpl.Path = ""
	raw, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(sopPath, append(raw, '\n'), 0o644)
}

func ensureSeedSOPTemplates() {
	roots := sopTemplateRoots()
	if len(roots) == 0 {
		return
	}
	userRoot := roots[0]
	for _, tpl := range defaultSOPTemplates() {
		writeSeedSOPTemplate(userRoot, tpl)
	}
}

func isSeedSOPTemplate(id string) bool {
	for _, tpl := range defaultSOPTemplates() {
		if tpl.ID == id {
			return true
		}
	}
	return false
}

func loadExternalSOPTemplates() []SOPTemplate {
	templates := []SOPTemplate{}
	for _, root := range sopTemplateRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			raw, err := os.ReadFile(filepath.Join(dir, "sop.json"))
			if err != nil {
				continue
			}
			var tpl SOPTemplate
			if err := json.Unmarshal(raw, &tpl); err != nil {
				continue
			}
			tpl.ID = strings.TrimSpace(tpl.ID)
			tpl.Name = strings.TrimSpace(tpl.Name)
			if tpl.ID == "" || tpl.Name == "" {
				continue
			}
			tpl.Builtin = isSeedSOPTemplate(tpl.ID)
			tpl.Path = dir
			templates = append(templates, tpl)
		}
	}
	return templates
}

func getSOPTemplates() []SOPTemplate {
	ensureSeedSOPTemplates()
	templates := []SOPTemplate{}
	seen := map[string]bool{}
	for _, tpl := range loadExternalSOPTemplates() {
		if seen[tpl.ID] {
			continue
		}
		seen[tpl.ID] = true
		templates = append(templates, tpl)
	}
	if len(templates) == 0 {
		templates = defaultSOPTemplates()
	}
	return templates
}

func findSOPTemplate(id string) SOPTemplate {
	id = strings.TrimSpace(id)
	if id == "" {
		id = defaultSOPTemplateID
	}
	for _, tpl := range getSOPTemplates() {
		if tpl.ID == id {
			return tpl
		}
	}
	return defaultSOPTemplates()[0]
}

func safeTemplateRelativePath(name string) (string, bool) {
	name = filepath.Clean(strings.TrimSpace(name))
	if name == "." || name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "..") {
		return "", false
	}
	return name, true
}

func createTaskStructure(folderPath string, tpl SOPTemplate) error {
	dirs := tpl.Folders
	if len(dirs) == 0 {
		dirs = defaultSOPTemplates()[0].Folders
	}
	for _, d := range dirs {
		rel, ok := safeTemplateRelativePath(d)
		if !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Join(folderPath, rel), 0o755); err != nil {
			return err
		}
	}
	for _, file := range tpl.Files {
		rel, ok := safeTemplateRelativePath(file.Path)
		if !ok {
			continue
		}
		filePath := filepath.Join(folderPath, rel)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filePath, []byte(file.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func handleListSOPTemplates(w http.ResponseWriter, r *http.Request) {
	templates := getSOPTemplates()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"templates": templates,
		"roots":     sopTemplateRoots(),
	})
}

// GenerateHash creates a short SHA-256 hash (first 16 hex chars) from the input string.
