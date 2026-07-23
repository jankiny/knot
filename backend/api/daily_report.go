package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

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

func validateReportAIConfig(cfg DailyReportAIConfig) error {
	if !cfg.Enabled {
		return fmt.Errorf("AI report generation is required")
	}
	if strings.TrimSpace(cfg.APIURL) == "" {
		return fmt.Errorf("AI API URL is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("AI model is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("AI API key is required")
	}
	return nil
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

	if err := validateReportAIConfig(req.AI); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	logs := make([]DailyReportLog, 0, len(req.Items))
	for _, item := range req.Items {
		if isAIRestrictedFolderPath(item.FolderPath) {
			jsonError(w, http.StatusForbidden, "敏感路径不允许用于 AI 报告")
			return
		}
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
		content, err := generateDailyLogWithAI(req.AI, reportInput)
		if err != nil {
			jsonError(w, http.StatusBadGateway, fmt.Sprintf("AI daily report generation failed: %v", err))
			return
		}

		content = truncateRunes(cleanDailyLog(content), 160)
		if content == "" {
			jsonError(w, http.StatusBadGateway, "AI daily report generation returned empty content")
			return
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
