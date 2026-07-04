package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type ArchiveAISearchRequest struct {
	Query        string              `json:"query"`
	ArchivePaths []string            `json:"archive_paths"`
	Limit        int                 `json:"limit"`
	AI           DailyReportAIConfig `json:"ai"`
}

type ArchiveAISearchCandidate struct {
	Path       string `json:"path"`
	Title      string `json:"title"`
	FolderName string `json:"folder_name"`
	Department string `json:"department"`
	Project    string `json:"project"`
	TaskDate   string `json:"task_date"`
	Content    string `json:"content"`
	Score      int    `json:"score"`
}

type ArchiveAISearchMatch struct {
	Path            string   `json:"path"`
	Title           string   `json:"title"`
	FolderName      string   `json:"folder_name"`
	Department      string   `json:"department"`
	Project         string   `json:"project"`
	TaskDate        string   `json:"task_date"`
	Confidence      float64  `json:"confidence"`
	Reason          string   `json:"reason"`
	MatchedKeywords []string `json:"matched_keywords"`
}

type ArchiveAISearchResult struct {
	Matches        []ArchiveAISearchMatch `json:"matches"`
	SuggestedQuery string                 `json:"suggested_query"`
}

const archiveAISearchSystemPrompt = `You are Knot's archive retrieval assistant.
You receive a user query and a list of candidate archived work folders.
Select the archived folders that are most likely relevant.
Rules:
1. Do not invent paths. Every returned path must exactly match one candidate path.
2. Prefer concrete evidence from title, date, department, project, folder name, and work record summary.
3. If nothing is relevant, return an empty matches array.
4. Return strict JSON only, without Markdown code fences.

JSON schema:
{
  "matches": [
    {
      "path": "candidate path",
      "title": "candidate title",
      "confidence": 0.0,
      "reason": "short Chinese explanation",
      "matched_keywords": ["keyword"]
    }
  ],
  "suggested_query": "optional Chinese suggestion"
}`

func archiveSearchTerms(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}

	replacer := strings.NewReplacer(
		"　", " ",
		",", " ",
		"，", " ",
		".", " ",
		"。", " ",
		";", " ",
		"；", " ",
		":", " ",
		"：", " ",
		"/", " ",
		"\\", " ",
		"_", " ",
		"-", " ",
		"(", " ",
		")", " ",
		"（", " ",
		"）", " ",
	)
	parts := strings.Fields(replacer.Replace(query))
	seen := map[string]bool{}
	terms := []string{}
	for _, part := range append([]string{query}, parts...) {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		terms = append(terms, part)
	}
	return terms
}

func scoreArchiveCandidate(query string, folder map[string]interface{}) int {
	terms := archiveSearchTerms(query)
	if len(terms) == 0 {
		return 0
	}

	fields := []struct {
		value  string
		weight int
	}{
		{fmt.Sprint(folder["title"]), 5},
		{fmt.Sprint(folder["folder_name"]), 4},
		{fmt.Sprint(folder["name"]), 4},
		{fmt.Sprint(folder["department"]), 3},
		{fmt.Sprint(folder["project"]), 3},
		{fmt.Sprint(folder["task_date"]), 2},
		{fmt.Sprint(folder["content"]), 2},
		{fmt.Sprint(folder["raw_content"]), 1},
		{fmt.Sprint(folder["path"]), 1},
	}

	score := 0
	for _, field := range fields {
		text := strings.ToLower(field.value)
		if text == "" {
			continue
		}
		for _, term := range terms {
			if strings.Contains(text, term) {
				score += field.weight
			}
		}
	}
	return score
}

func archiveCandidateFromFolder(query string, folder map[string]interface{}) ArchiveAISearchCandidate {
	content := strings.TrimSpace(fmt.Sprint(folder["content"]))
	if content == "" {
		content = strings.TrimSpace(fmt.Sprint(folder["raw_content"]))
	}
	return ArchiveAISearchCandidate{
		Path:       fmt.Sprint(folder["path"]),
		Title:      fmt.Sprint(folder["title"]),
		FolderName: fmt.Sprint(folder["folder_name"]),
		Department: fmt.Sprint(folder["department"]),
		Project:    fmt.Sprint(folder["project"]),
		TaskDate:   fmt.Sprint(folder["task_date"]),
		Content:    truncateRunes(content, 900),
		Score:      scoreArchiveCandidate(query, folder),
	}
}

func collectArchiveSearchCandidates(query string, archivePaths []string, maxCandidates int) ([]ArchiveAISearchCandidate, error) {
	seen := map[string]bool{}
	candidates := []ArchiveAISearchCandidate{}

	for _, archivePath := range archivePaths {
		archivePath = strings.TrimSpace(archivePath)
		if archivePath == "" {
			continue
		}
		folders, err := collectScannedFolders(normalizeScanPath(getBaseFolder(archivePath)), true)
		if err != nil {
			return nil, err
		}
		for _, folder := range folders {
			candidate := archiveCandidateFromFolder(query, folder)
			if strings.TrimSpace(candidate.Path) == "" {
				continue
			}
			key := normalizePathKey(candidate.Path)
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, candidate)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].TaskDate > candidates[j].TaskDate
		}
		return candidates[i].Score > candidates[j].Score
	})

	positive := []ArchiveAISearchCandidate{}
	for _, candidate := range candidates {
		if candidate.Score > 0 {
			positive = append(positive, candidate)
		}
	}
	if len(positive) > 0 {
		candidates = positive
	}

	if maxCandidates <= 0 {
		maxCandidates = 80
	}
	if maxCandidates > 120 {
		maxCandidates = 120
	}
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}
	return candidates, nil
}

func buildArchiveAISearchUserPrompt(query string, candidates []ArchiveAISearchCandidate, limit int) string {
	raw, _ := json.MarshalIndent(candidates, "", "  ")
	return fmt.Sprintf(`用户要查找的历史资料：
%s

最多返回 %d 个结果。

候选归档项目 JSON：
%s`, query, limit, string(raw))
}

func generateArchiveAISearch(cfg DailyReportAIConfig, query string, candidates []ArchiveAISearchCandidate, limit int) (ArchiveAISearchResult, error) {
	content, err := callChatCompletion(cfg, []chatMessage{
		{Role: "system", Content: archiveAISearchSystemPrompt},
		{Role: "user", Content: buildArchiveAISearchUserPrompt(query, candidates, limit)},
	}, true)
	if err != nil {
		return ArchiveAISearchResult{}, err
	}

	var result ArchiveAISearchResult
	if err := json.Unmarshal([]byte(stripJSONCodeFence(content)), &result); err != nil {
		return ArchiveAISearchResult{}, err
	}
	return result, nil
}

func validateArchiveAISearchResult(result ArchiveAISearchResult, candidates []ArchiveAISearchCandidate, limit int) ArchiveAISearchResult {
	byPath := map[string]ArchiveAISearchCandidate{}
	for _, candidate := range candidates {
		byPath[normalizePathKey(candidate.Path)] = candidate
	}

	matches := []ArchiveAISearchMatch{}
	seen := map[string]bool{}
	for _, match := range result.Matches {
		key := normalizePathKey(match.Path)
		candidate, ok := byPath[key]
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		if strings.TrimSpace(match.Title) == "" {
			match.Title = candidate.Title
		}
		match.Path = candidate.Path
		match.FolderName = candidate.FolderName
		match.Department = candidate.Department
		match.Project = candidate.Project
		match.TaskDate = candidate.TaskDate
		if match.Confidence < 0 {
			match.Confidence = 0
		}
		if match.Confidence > 1 {
			match.Confidence = 1
		}
		matches = append(matches, match)
		if limit > 0 && len(matches) >= limit {
			break
		}
	}
	result.Matches = matches
	return result
}

func handleArchiveAISearch(w http.ResponseWriter, r *http.Request) {
	var req ArchiveAISearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		jsonError(w, http.StatusBadRequest, "query is required")
		return
	}
	if len(req.ArchivePaths) == 0 {
		jsonError(w, http.StatusBadRequest, "archive_paths cannot be empty")
		return
	}
	if err := validateReportAIConfig(req.AI); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 6
	}
	if limit > 12 {
		limit = 12
	}

	candidates, err := collectArchiveSearchCandidates(query, req.ArchivePaths, 80)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("failed to scan archive paths: %v", err))
		return
	}
	if len(candidates) == 0 {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success":         true,
			"query":           query,
			"candidate_count": 0,
			"matches":         []ArchiveAISearchMatch{},
			"suggested_query": "没有找到可供 AI 判断的归档项目，请检查归档目录或换一个关键词。",
		})
		return
	}

	result, err := generateArchiveAISearch(req.AI, query, candidates, limit)
	if err != nil {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("AI archive search failed: %v", err))
		return
	}
	result = validateArchiveAISearchResult(result, candidates, limit)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"query":           query,
		"candidate_count": len(candidates),
		"matches":         result.Matches,
		"suggested_query": result.SuggestedQuery,
	})
}
