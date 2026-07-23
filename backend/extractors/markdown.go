// Package extractors contains bounded, local-only document extractors.
package extractors

import (
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DocumentTypeMarkdown      = "markdown"
	DocumentTypeJournalDaily  = "journal_daily"
	DocumentTypeJournalWeekly = "journal_weekly"
)

var (
	headingPattern = regexp.MustCompile(`(?m)^\s*#\s+(.+?)\s*$`)
	datePattern    = regexp.MustCompile(`(20\d{2})[._年/-](\d{1,2})[._月/-](\d{1,2})日?`)
	compactDate    = regexp.MustCompile(`(?:^|[^\d])(20\d{2})(\d{2})(\d{2})(?:[^\d]|$)`)
	isoWeekPattern = regexp.MustCompile(`(?i)(20\d{2})[._ -]?w(?:eek)?[._ -]?(\d{1,2})`)
	cnWeekPattern  = regexp.MustCompile(`(20\d{2})年?[第._ -]?(\d{1,2})周`)
	weeklyPattern  = regexp.MustCompile(`(?i)(周报|工作周报|weekly(?:\s+report)?)`)
	dailyPattern   = regexp.MustCompile(`(?i)(日报|工作日报|daily(?:\s+report)?)`)
)

type MarkdownOptions struct {
	Journal         bool
	MaxExcerptRunes int
}

type MarkdownResult struct {
	DocumentType string
	Title        string
	TaskDate     string
	Excerpt      string
}

// ExtractMarkdown recognizes the first-stage daily and weekly journal forms.
// Ordinary Markdown is intentionally metadata-only.
func ExtractMarkdown(
	data []byte,
	relativePath string,
	options MarkdownOptions,
) MarkdownResult {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	frontmatter, body := splitFrontmatter(text)
	stem := strings.TrimSuffix(path.Base(relativePath), path.Ext(relativePath))

	title := cleanScalar(frontmatter["title"])
	if title == "" {
		if match := headingPattern.FindStringSubmatch(body); len(match) == 2 {
			title = strings.TrimSpace(match[1])
		}
	}
	if title == "" {
		title = stem
	}

	classificationText := strings.Join([]string{stem, title}, " ")
	documentType := DocumentTypeMarkdown
	switch {
	case weeklyPattern.MatchString(classificationText) ||
		isoWeekPattern.MatchString(classificationText) ||
		cnWeekPattern.MatchString(classificationText):
		documentType = DocumentTypeJournalWeekly
	case dailyPattern.MatchString(classificationText):
		documentType = DocumentTypeJournalDaily
	case options.Journal && firstDate(classificationText) != "":
		documentType = DocumentTypeJournalDaily
	}

	taskDate := firstNormalizedDate(
		frontmatter["task_date"],
		frontmatter["date"],
		frontmatter["created"],
	)
	if taskDate == "" {
		if documentType == DocumentTypeJournalWeekly {
			taskDate = firstWeekStart(classificationText)
		}
		if taskDate == "" {
			taskDate = firstDate(classificationText)
		}
	}

	excerpt := ""
	if documentType == DocumentTypeJournalDaily ||
		documentType == DocumentTypeJournalWeekly {
		excerpt = markdownExcerpt(body, options.MaxExcerptRunes)
	}

	return MarkdownResult{
		DocumentType: documentType,
		Title:        title,
		TaskDate:     taskDate,
		Excerpt:      excerpt,
	}
}

// NormalizeDate returns a stable YYYY-MM-DD value for supported local
// metadata formats. It returns an empty string for ambiguous input.
func NormalizeDate(value string) string {
	value = cleanScalar(value)
	if value == "" {
		return ""
	}
	for _, format := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006.01.02",
		"2006/01/02",
	} {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return firstDate(value)
}

func splitFrontmatter(text string) (map[string]string, string) {
	values := make(map[string]string)
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return values, text
	}

	end := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			end = index
			break
		}
	}
	if end == -1 {
		return values, text
	}
	for _, line := range lines[1:end] {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		values[key] = strings.TrimSpace(parts[1])
	}
	return values, strings.Join(lines[end+1:], "\n")
}

func firstNormalizedDate(values ...string) string {
	for _, value := range values {
		if normalized := NormalizeDate(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func firstDate(value string) string {
	if match := datePattern.FindStringSubmatch(value); len(match) == 4 {
		return validDate(match[1], match[2], match[3])
	}
	if match := compactDate.FindStringSubmatch(value); len(match) == 4 {
		return validDate(match[1], match[2], match[3])
	}
	return ""
}

func validDate(yearValue, monthValue, dayValue string) string {
	year, yearErr := strconv.Atoi(yearValue)
	month, monthErr := strconv.Atoi(monthValue)
	day, dayErr := strconv.Atoi(dayValue)
	if yearErr != nil || monthErr != nil || dayErr != nil {
		return ""
	}
	parsed := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day {
		return ""
	}
	return parsed.Format("2006-01-02")
}

func firstWeekStart(value string) string {
	var match []string
	if candidate := isoWeekPattern.FindStringSubmatch(value); len(candidate) == 3 {
		match = candidate
	} else if candidate := cnWeekPattern.FindStringSubmatch(value); len(candidate) == 3 {
		match = candidate
	}
	if len(match) != 3 {
		return ""
	}
	year, yearErr := strconv.Atoi(match[1])
	week, weekErr := strconv.Atoi(match[2])
	if yearErr != nil || weekErr != nil || week < 1 || week > 53 {
		return ""
	}

	januaryFourth := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekOneMonday := januaryFourth.AddDate(
		0,
		0,
		-int(januaryFourth.Weekday()-time.Monday+7)%7,
	)
	result := weekOneMonday.AddDate(0, 0, (week-1)*7)
	isoYear, isoWeek := result.ISOWeek()
	if isoYear != year || isoWeek != week {
		return ""
	}
	return result.Format("2006-01-02")
}

func markdownExcerpt(body string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	lines := strings.Split(body, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#>*+-0123456789.) \t")
		if line != "" {
			parts = append(parts, line)
		}
	}
	return truncateRunes(strings.Join(parts, " "), maxRunes)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}
