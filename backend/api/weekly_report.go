package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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

	if err := validateReportAIConfig(req.AI); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(items) == 0 {
		jsonError(w, http.StatusBadRequest, "no work items found in selected period")
		return
	}
	report, err := generateWeeklyReportWithAI(req.AI, startText, endText, items)
	if err != nil {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("AI weekly report generation failed: %v", err))
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"period":  startText + " 至 " + endText,
		"count":   len(items),
		"items":   items,
		"report":  report,
	})
}
