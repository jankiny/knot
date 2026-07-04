package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type WorkReportScanRequest struct {
	PeriodStart string   `json:"period_start"`
	PeriodEnd   string   `json:"period_end"`
	ScanPaths   []string `json:"scan_paths"`
}

type WorkReportCandidate struct {
	WeeklyWorkItem
	LatestActivity string   `json:"latest_activity"`
	ArchiveTime    string   `json:"archive_time"`
	FilesystemTime string   `json:"filesystem_time"`
	ContentDates   []string `json:"content_dates,omitempty"`
	MatchReasons   []string `json:"match_reasons"`
}

type WorkReportGenerateRequest struct {
	ReportType  string              `json:"report_type"`
	PeriodStart string              `json:"period_start"`
	PeriodEnd   string              `json:"period_end"`
	Items       []WeeklyReportItem  `json:"items"`
	AI          DailyReportAIConfig `json:"ai"`
}

type WorkReportResult struct {
	Title      string   `json:"title"`
	Period     string   `json:"period"`
	Overview   string   `json:"overview"`
	Highlights []string `json:"highlights"`
	Details    []string `json:"details"`
	NextPlan   []string `json:"next_plan"`
	Risks      []string `json:"risks"`
	Markdown   string   `json:"markdown"`
}

const workReportSystemPrompt = `你是一个严谨的中文工作报告助手。你需要根据结构化工作记录生成正式、简洁、可直接提交的工作报告。

要求：
1. 只基于输入 JSON 写作，不要编造不存在的工作、会议、数字或成果。
2. 优先提取人工记录的关键事件、明确完成事项、当前进展和下一步。
3. 对明显模板化、只有创建或归档记录的内容保持克制。
4. 根据报告类型调整粒度：日报更具体，周报强调阶段进展，月报强调汇总和成果。
5. 输出必须是合法 JSON，不要输出 Markdown 代码块，不要解释。`

var (
	archiveTimePattern = regexp.MustCompile(`(?m)^-\s*归档时间：\s*(.+)$`)
	contentDatePattern = regexp.MustCompile(`\d{4}[-./]\d{1,2}[-./]\d{1,2}`)
)

func parseDateToken(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	value = strings.ReplaceAll(value, ".", "-")
	value = strings.ReplaceAll(value, "/", "-")
	parts := strings.Split(value, "-")
	if len(parts) == 3 {
		month, monthErr := strconv.Atoi(parts[1])
		day, dayErr := strconv.Atoi(parts[2])
		if monthErr == nil && dayErr == nil {
			value = fmt.Sprintf("%s-%02d-%02d", parts[0], month, day)
		}
	}
	return parseTimeLoose(value)
}

func timeInReportPeriod(value time.Time, start, end time.Time) bool {
	if value.IsZero() {
		return false
	}
	day := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.Local)
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, time.Local)
	return !day.Before(startDay) && !day.After(endDay)
}

func addMatchTime(times *[]time.Time, reasons *[]string, seen map[string]bool, label string, value time.Time) {
	if value.IsZero() || seen[label] {
		return
	}
	*times = append(*times, value)
	*reasons = append(*reasons, label)
	seen[label] = true
}

func extractArchiveTime(body string) string {
	matches := archiveTimePattern.FindStringSubmatch(strings.ReplaceAll(body, "\r\n", "\n"))
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func extractContentDates(body string) []string {
	matches := contentDatePattern.FindAllString(body, -1)
	seen := map[string]bool{}
	dates := make([]string, 0, len(matches))
	for _, match := range matches {
		t := parseDateToken(match)
		if t.IsZero() {
			continue
		}
		value := t.Format("2006-01-02")
		if seen[value] {
			continue
		}
		seen[value] = true
		dates = append(dates, value)
	}
	sort.Strings(dates)
	return dates
}

func buildWorkReportCandidate(folderPath string, parsed *parsedWorkRecord, start, end time.Time) (WorkReportCandidate, bool) {
	item := weeklyItemFromParsed(folderPath, parsed)
	item.FolderPath = filepath.ToSlash(folderPath)

	reasons := []string{}
	matchedTimes := []time.Time{}
	seenReasons := map[string]bool{}

	if t := parseTimeLoose(item.TaskDate); timeInReportPeriod(t, start, end) {
		addMatchTime(&matchedTimes, &reasons, seenReasons, "标记时间在范围内", t)
	}
	if t := parseTimeLoose(item.Updated); timeInReportPeriod(t, start, end) {
		addMatchTime(&matchedTimes, &reasons, seenReasons, "工作记录更新时间在范围内", t)
	}

	archiveTime := extractArchiveTime(parsed.Body)
	if t := parseTimeLoose(archiveTime); timeInReportPeriod(t, start, end) {
		addMatchTime(&matchedTimes, &reasons, seenReasons, "归档时间在范围内", t)
	}

	filesystemTime := ""
	if stat, err := os.Stat(filepath.Join(folderPath, workRecordFileName)); err == nil {
		filesystemTime = stat.ModTime().Format(time.RFC3339)
		if timeInReportPeriod(stat.ModTime(), start, end) {
			addMatchTime(&matchedTimes, &reasons, seenReasons, "工作记录文件最近修改", stat.ModTime())
		}
	} else if stat, err := os.Stat(folderPath); err == nil {
		filesystemTime = stat.ModTime().Format(time.RFC3339)
		if timeInReportPeriod(stat.ModTime(), start, end) {
			addMatchTime(&matchedTimes, &reasons, seenReasons, "任务目录最近修改", stat.ModTime())
		}
	}

	contentDates := extractContentDates(parsed.Body)
	for _, date := range contentDates {
		if t := parseTimeLoose(date); timeInReportPeriod(t, start, end) {
			addMatchTime(&matchedTimes, &reasons, seenReasons, "正文日期在范围内", t)
		}
	}

	if len(reasons) == 0 {
		return WorkReportCandidate{}, false
	}

	latest := matchedTimes[0]
	for _, t := range matchedTimes[1:] {
		if t.After(latest) {
			latest = t
		}
	}

	return WorkReportCandidate{
		WeeklyWorkItem: item,
		LatestActivity: latest.Format("2006-01-02"),
		ArchiveTime:    archiveTime,
		FilesystemTime: filesystemTime,
		ContentDates:   contentDates,
		MatchReasons:   reasons,
	}, true
}

func collectWorkReportCandidates(scanPaths []string, start, end time.Time) ([]WorkReportCandidate, int, error) {
	candidates := []WorkReportCandidate{}
	seen := map[string]bool{}
	scannedCount := 0

	for _, scanPath := range scanPaths {
		scanPath = strings.TrimSpace(scanPath)
		if scanPath == "" {
			continue
		}
		folders, err := collectScannedFolders(normalizeScanPath(getBaseFolder(scanPath)), true)
		if err != nil {
			return nil, scannedCount, err
		}
		scannedCount += len(folders)

		for _, folder := range folders {
			folderPath := strings.TrimSpace(fmt.Sprint(folder["path"]))
			if folderPath == "" {
				continue
			}
			key := normalizePathKey(folderPath)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true

			parsed, err := readWorkRecord(filepath.Join(folderPath, workRecordFileName))
			if err != nil {
				continue
			}
			candidate, ok := buildWorkReportCandidate(folderPath, parsed, start, end)
			if ok {
				candidates = append(candidates, candidate)
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		ti := parseTimeLoose(candidates[i].LatestActivity)
		tj := parseTimeLoose(candidates[j].LatestActivity)
		if ti.Equal(tj) {
			return candidates[i].Title < candidates[j].Title
		}
		return ti.After(tj)
	})

	return candidates, scannedCount, nil
}

func collectWorkReportItems(req WorkReportGenerateRequest) []WeeklyWorkItem {
	items := make([]WeeklyWorkItem, 0, len(req.Items))
	seen := map[string]bool{}

	for _, item := range req.Items {
		folderPath := strings.TrimSpace(item.FolderPath)
		var parsed *parsedWorkRecord
		if folderPath != "" {
			if p, err := readWorkRecord(filepath.Join(folderPath, workRecordFileName)); err == nil {
				parsed = p
			}
		}
		if parsed == nil && strings.TrimSpace(item.WorkRecord) != "" {
			frontLines, body, hasFrontmatter := splitFrontmatter(item.WorkRecord)
			info := &WorkRecordInfo{
				Title:         extractTitleFromBody(body),
				Content:       extractWorkCoreContent(body),
				RawContent:    strings.TrimSpace(body),
				Status:        "active",
				Source:        "manual",
				TaskDate:      req.PeriodStart,
				CreateTime:    req.PeriodStart,
				UpdateTime:    req.PeriodEnd,
				FolderName:    filepath.Base(folderPath),
				ProjectPath:   filepath.ToSlash(folderPath),
				SchemaVersion: 3,
			}
			parsed = &parsedWorkRecord{Info: info, FrontLines: frontLines, Body: body, HasFrontmatt: hasFrontmatter}
		}
		if parsed == nil {
			continue
		}

		workItem := weeklyItemFromParsed(folderPath, parsed)
		key := normalizePathKey(workItem.FolderPath)
		if key == "" {
			key = workItem.Title + "|" + workItem.TaskDate
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, workItem)
	}

	sort.Slice(items, func(i, j int) bool {
		ti := parseTimeLoose(items[i].TaskDate)
		tj := parseTimeLoose(items[j].TaskDate)
		if ti.Equal(tj) {
			return items[i].Title < items[j].Title
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.Before(tj)
	})

	return items
}

func normalizeReportType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "daily", "day", "日报":
		return "daily"
	case "monthly", "month", "月报":
		return "monthly"
	case "custom", "自定义":
		return "custom"
	default:
		return "weekly"
	}
}

func reportTypeLabel(reportType string) string {
	switch normalizeReportType(reportType) {
	case "daily":
		return "日报"
	case "monthly":
		return "月报"
	case "custom":
		return "工作报告"
	default:
		return "周报"
	}
}

func buildWorkReportUserPrompt(reportType, start, end string, items []WeeklyWorkItem) string {
	raw, _ := json.MarshalIndent(items, "", "  ")
	label := reportTypeLabel(reportType)
	return fmt.Sprintf(`请根据以下工作记录生成%s。

输出 JSON 格式：
{
  "title": "报告标题",
  "period": "YYYY-MM-DD 至 YYYY-MM-DD",
  "overview": "整体概述，2-4句话",
  "highlights": ["重点成果或关键进展"],
  "details": ["具体工作事项"],
  "next_plan": ["后续计划"],
  "risks": ["风险或需协调事项，没有则为空数组"],
  "markdown": "完整 Markdown 报告正文"
}

报告类型：%s
报告周期：%s 至 %s

工作记录 JSON：
%s`, label, label, start, end, string(raw))
}

func generateWorkReportWithAI(cfg DailyReportAIConfig, reportType, start, end string, items []WeeklyWorkItem) (WorkReportResult, error) {
	content, err := callChatCompletion(cfg, []chatMessage{
		{Role: "system", Content: workReportSystemPrompt},
		{Role: "user", Content: buildWorkReportUserPrompt(reportType, start, end, items)},
	}, true)
	if err != nil {
		return WorkReportResult{}, err
	}

	var result WorkReportResult
	if err := json.Unmarshal([]byte(stripJSONCodeFence(content)), &result); err != nil {
		return WorkReportResult{}, err
	}
	if strings.TrimSpace(result.Markdown) == "" {
		return WorkReportResult{}, fmt.Errorf("empty report markdown")
	}
	if strings.TrimSpace(result.Period) == "" {
		result.Period = start + " 至 " + end
	}
	if strings.TrimSpace(result.Title) == "" {
		result.Title = fmt.Sprintf("%s 至 %s %s", start, end, reportTypeLabel(reportType))
	}
	return result, nil
}

func parseReportPeriod(startText, endText string) (time.Time, time.Time, bool) {
	start, err := parseReportDate(startText)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	end, err := parseReportDate(endText)
	if err != nil || end.Before(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func handleScanWorkReport(w http.ResponseWriter, r *http.Request) {
	var req WorkReportScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	start, end, ok := parseReportPeriod(req.PeriodStart, req.PeriodEnd)
	if !ok {
		jsonError(w, http.StatusBadRequest, "valid period_start and period_end are required")
		return
	}
	if len(req.ScanPaths) == 0 {
		jsonError(w, http.StatusBadRequest, "scan_paths cannot be empty")
		return
	}

	candidates, scannedCount, err := collectWorkReportCandidates(req.ScanPaths, start, end)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("scan report tasks failed: %v", err))
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"period":        start.Format("2006-01-02") + " 至 " + end.Format("2006-01-02"),
		"scanned_count": scannedCount,
		"count":         len(candidates),
		"items":         candidates,
	})
}

func handleGenerateWorkReport(w http.ResponseWriter, r *http.Request) {
	var req WorkReportGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	start, end, ok := parseReportPeriod(req.PeriodStart, req.PeriodEnd)
	if !ok {
		jsonError(w, http.StatusBadRequest, "valid period_start and period_end are required")
		return
	}
	if len(req.Items) == 0 {
		jsonError(w, http.StatusBadRequest, "items cannot be empty")
		return
	}
	if err := validateReportAIConfig(req.AI); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	items := collectWorkReportItems(req)
	if len(items) == 0 {
		jsonError(w, http.StatusBadRequest, "no readable work items found")
		return
	}

	startText := start.Format("2006-01-02")
	endText := end.Format("2006-01-02")
	report, err := generateWorkReportWithAI(req.AI, normalizeReportType(req.ReportType), startText, endText, items)
	if err != nil {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("AI work report generation failed: %v", err))
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"period":  startText + " 至 " + endText,
		"type":    normalizeReportType(req.ReportType),
		"count":   len(items),
		"items":   items,
		"report":  report,
	})
}
