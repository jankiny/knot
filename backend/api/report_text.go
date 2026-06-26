package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

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
