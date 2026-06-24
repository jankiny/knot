package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// -- Daily Report Handlers --

type DailyReportAIConfig struct {
	APIURL  string `json:"api_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

func isDeepSeekConfig(cfg DailyReportAIConfig) bool {
	apiURL := strings.ToLower(strings.TrimSpace(cfg.APIURL))
	model := strings.ToLower(strings.TrimSpace(cfg.Model))
	return strings.Contains(apiURL, "deepseek") || strings.Contains(model, "deepseek")
}

func callChatCompletion(cfg DailyReportAIConfig, messages []chatMessage, jsonMode bool) (string, error) {
	endpoint := normalizeAIEndpoint(cfg.APIURL)
	if endpoint == "" {
		return "", fmt.Errorf("empty api endpoint")
	}

	payload := map[string]interface{}{
		"model":       cfg.Model,
		"messages":    messages,
		"temperature": 0.2,
	}
	if isDeepSeekConfig(cfg) {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
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
	if content == "" {
		return "", fmt.Errorf("empty ai content")
	}
	return content, nil
}

func generateDailyLogWithAI(cfg DailyReportAIConfig, reportInput string) (string, error) {
	content, err := callChatCompletion(cfg, []chatMessage{
		{Role: "system", Content: dailyReportSystemPrompt},
		{Role: "user", Content: reportInput},
	}, false)
	if err != nil {
		return "", err
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

type WeeklyReportItem struct {
	FolderPath string `json:"folder_path"`
	WorkRecord string `json:"work_record"`
}

type WeeklyReportGenerateRequest struct {
	PeriodStart string              `json:"period_start"`
	PeriodEnd   string              `json:"period_end"`
	Items       []WeeklyReportItem  `json:"items"`
	AI          DailyReportAIConfig `json:"ai"`
}

type WeeklyWorkItem struct {
	FolderPath    string   `json:"folder_path"`
	Title         string   `json:"title"`
	Department    string   `json:"department"`
	Project       string   `json:"project"`
	TaskDate      string   `json:"task_date"`
	Created       string   `json:"created"`
	Updated       string   `json:"updated"`
	Status        string   `json:"status"`
	ArchiveStatus string   `json:"archive_status"`
	Source        string   `json:"source"`
	CoreContent   string   `json:"core_content"`
	WorkContent   string   `json:"work_content"`
	WorkProcess   string   `json:"work_process"`
	Progress      string   `json:"progress"`
	NextStep      string   `json:"next_step"`
	ArchiveRecord string   `json:"archive_record"`
	Tags          []string `json:"tags,omitempty"`
}

type WeeklyReportResult struct {
	Title         string `json:"title"`
	Period        string `json:"period"`
	Overview      string `json:"overview"`
	CompletedWork []struct {
		Project    string `json:"project"`
		Department string `json:"department"`
		Content    string `json:"content"`
		Result     string `json:"result"`
	} `json:"completed_work"`
	InProgressWork []struct {
		Project    string `json:"project"`
		Department string `json:"department"`
		Content    string `json:"content"`
		NextStep   string `json:"next_step"`
	} `json:"in_progress_work"`
	NextWeekPlan         []string `json:"next_week_plan"`
	RisksOrSupportNeeded []string `json:"risks_or_support_needed"`
	Markdown             string   `json:"markdown"`
}

const weeklyReportSystemPrompt = `你是一个严谨的中文工作周报助手。你需要根据结构化工作记录生成正式、简洁、可直接提交的周报。

要求：
1. 只基于输入 JSON 写作，不要编造不存在的工作。
2. 优先提取人工记录的关键事件、明确完成事项、当前进展和下一步。
3. 对明显模板化内容保持克制，不要夸大。
4. 如果项目只有创建、归档或资料整理记录，应保守表述。
5. 输出必须是合法 JSON，不要输出 Markdown 代码块，不要解释。`

func parseReportDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	return time.Parse("2006-01-02", value)
}

func dateStringInRange(value string, start, end time.Time) bool {
	t := parseTimeLoose(value)
	if t.IsZero() {
		return false
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, time.Local)
	return !day.Before(startDay) && !day.After(endDay)
}

func workItemInPeriod(item WeeklyWorkItem, start, end time.Time) bool {
	return dateStringInRange(item.TaskDate, start, end) ||
		dateStringInRange(item.Created, start, end) ||
		dateStringInRange(item.Updated, start, end)
}

func extractTags(frontLines []string) []string {
	tags := []string{}
	inTags := false
	for _, line := range frontLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "tags:" {
			inTags = true
			continue
		}
		if inTags {
			if strings.HasPrefix(trimmed, "- ") {
				tag := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				if tag != "" {
					tags = append(tags, tag)
				}
				continue
			}
			if strings.Contains(trimmed, ":") {
				break
			}
		}
	}
	return tags
}

func weeklyItemFromParsed(folderPath string, parsed *parsedWorkRecord) WeeklyWorkItem {
	body := parsed.Body
	info := parsed.Info
	return WeeklyWorkItem{
		FolderPath:    filepath.ToSlash(folderPath),
		Title:         info.Title,
		Department:    info.Department,
		Project:       info.Project,
		TaskDate:      info.TaskDate,
		Created:       info.CreateTime,
		Updated:       info.UpdateTime,
		Status:        info.Status,
		ArchiveStatus: info.ArchiveStatus,
		Source:        info.Source,
		CoreContent:   info.Content,
		WorkContent:   truncateRunes(extractSectionContent(body, "## 工作内容", "## 任务目标"), 500),
		WorkProcess:   truncateRunes(extractSectionContent(body, "## 工作过程", "## 工作日志"), 800),
		Progress:      truncateRunes(extractSectionContent(body, "## 当前进展"), 500),
		NextStep:      truncateRunes(extractSectionContent(body, "## 下一步", "## 产出成果"), 500),
		ArchiveRecord: truncateRunes(extractSectionContent(body, "## 归档记录"), 300),
		Tags:          extractTags(parsed.FrontLines),
	}
}

func collectWeeklyWorkItems(req WeeklyReportGenerateRequest, start, end time.Time) []WeeklyWorkItem {
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
				TaskDate:      start.Format("2006-01-02"),
				CreateTime:    start.Format("2006-01-02"),
				UpdateTime:    end.Format("2006-01-02"),
				FolderName:    filepath.Base(folderPath),
				ProjectPath:   filepath.ToSlash(folderPath),
				SchemaVersion: 3,
			}
			parsed = &parsedWorkRecord{Info: info, FrontLines: frontLines, Body: body, HasFrontmatt: hasFrontmatter}
		}

		if parsed == nil {
			continue
		}

		weeklyItem := weeklyItemFromParsed(folderPath, parsed)
		if !workItemInPeriod(weeklyItem, start, end) {
			continue
		}

		key := normalizePathKey(weeklyItem.FolderPath)
		if key == "" {
			key = weeklyItem.Title + "|" + weeklyItem.TaskDate
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, weeklyItem)
	}

	sort.Slice(items, func(i, j int) bool {
		ti := parseTimeLoose(items[i].TaskDate)
		tj := parseTimeLoose(items[j].TaskDate)
		if ti.IsZero() && tj.IsZero() {
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

func buildWeeklyReportUserPrompt(start, end string, items []WeeklyWorkItem) string {
	raw, _ := json.MarshalIndent(items, "", "  ")
	return fmt.Sprintf(`请根据以下工作记录生成周报。

输出 JSON 格式：
{
  "title": "YYYY年第X周工作周报",
  "period": "YYYY-MM-DD 至 YYYY-MM-DD",
  "overview": "本周总体工作概述，2-4句话",
  "completed_work": [
    {
      "project": "项目名称",
      "department": "部门",
      "content": "本周完成的具体工作",
      "result": "形成的成果或当前状态"
    }
  ],
  "in_progress_work": [
    {
      "project": "项目名称",
      "department": "部门",
      "content": "正在推进的事项",
      "next_step": "下一步安排"
    }
  ],
  "next_week_plan": [
    "下周计划事项"
  ],
  "risks_or_support_needed": [
    "风险或需协调事项，没有则为空数组"
  ],
  "markdown": "完整 Markdown 周报正文"
}

周报周期：
%s 至 %s

工作记录 JSON：
%s`, start, end, string(raw))
}

func stripJSONCodeFence(input string) string {
	text := strings.TrimSpace(input)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}

func buildFallbackWeeklyReport(start, end string, items []WeeklyWorkItem) WeeklyReportResult {
	title := fmt.Sprintf("%s 至 %s 工作周报", start, end)
	lines := []string{
		fmt.Sprintf("# %s", title),
		"",
		"## 本周工作概述",
	}
	if len(items) == 0 {
		lines = append(lines, "本周暂无可提取的工作记录。")
	} else {
		lines = append(lines, fmt.Sprintf("本周共整理 %d 项工作记录，主要围绕已选任务推进相关工作。", len(items)))
	}
	lines = append(lines, "", "## 本周完成与推进")

	nextSteps := []string{}
	for _, item := range items {
		name := strings.TrimSpace(item.Title)
		if name == "" {
			name = filepath.Base(item.FolderPath)
		}
		content := strings.TrimSpace(item.WorkProcess)
		if content == "" {
			content = strings.TrimSpace(item.Progress)
		}
		if content == "" {
			content = strings.TrimSpace(item.CoreContent)
		}
		if content == "" {
			content = "完成相关资料整理与任务推进。"
		}
		lines = append(lines, fmt.Sprintf("- %s：%s", name, truncateRunes(cleanDailyLog(content), 140)))
		if strings.TrimSpace(item.NextStep) != "" {
			nextSteps = append(nextSteps, fmt.Sprintf("%s：%s", name, truncateRunes(cleanDailyLog(item.NextStep), 100)))
		}
	}

	lines = append(lines, "", "## 下周计划")
	if len(nextSteps) == 0 {
		lines = append(lines, "- 根据任务进展继续完善相关工作。")
	} else {
		for _, step := range nextSteps {
			lines = append(lines, "- "+step)
		}
	}

	return WeeklyReportResult{
		Title:                title,
		Period:               start + " 至 " + end,
		Overview:             fmt.Sprintf("本周共整理 %d 项工作记录。", len(items)),
		NextWeekPlan:         nextSteps,
		RisksOrSupportNeeded: []string{},
		Markdown:             strings.Join(lines, "\n"),
	}
}

func generateWeeklyReportWithAI(cfg DailyReportAIConfig, start, end string, items []WeeklyWorkItem) (WeeklyReportResult, error) {
	content, err := callChatCompletion(cfg, []chatMessage{
		{Role: "system", Content: weeklyReportSystemPrompt},
		{Role: "user", Content: buildWeeklyReportUserPrompt(start, end, items)},
	}, true)
	if err != nil {
		return WeeklyReportResult{}, err
	}

	var result WeeklyReportResult
	if err := json.Unmarshal([]byte(stripJSONCodeFence(content)), &result); err != nil {
		return WeeklyReportResult{}, err
	}
	if strings.TrimSpace(result.Markdown) == "" {
		return WeeklyReportResult{}, fmt.Errorf("empty weekly markdown")
	}
	if strings.TrimSpace(result.Period) == "" {
		result.Period = start + " 至 " + end
	}
	if strings.TrimSpace(result.Title) == "" {
		result.Title = fmt.Sprintf("%s 至 %s 工作周报", start, end)
	}
	return result, nil
}

func handleGenerateWeeklyReport(w http.ResponseWriter, r *http.Request) {
	var req WeeklyReportGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Items) == 0 {
		jsonError(w, http.StatusBadRequest, "items cannot be empty")
		return
	}

	start, err := parseReportDate(req.PeriodStart)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "period_start is required")
		return
	}
	end, err := parseReportDate(req.PeriodEnd)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "period_end is required")
		return
	}
	if end.Before(start) {
		jsonError(w, http.StatusBadRequest, "period_end cannot be earlier than period_start")
		return
	}

	startText := start.Format("2006-01-02")
	endText := end.Format("2006-01-02")
	items := collectWeeklyWorkItems(req, start, end)
	report := buildFallbackWeeklyReport(startText, endText, items)

	aiEnabled := req.AI.Enabled &&
		strings.TrimSpace(req.AI.APIURL) != "" &&
		strings.TrimSpace(req.AI.Model) != "" &&
		strings.TrimSpace(req.AI.APIKey) != ""
	if aiEnabled && len(items) > 0 {
		if generated, err := generateWeeklyReportWithAI(req.AI, startText, endText, items); err == nil {
			report = generated
		}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"period":  startText + " 至 " + endText,
		"count":   len(items),
		"items":   items,
		"report":  report,
	})
}
