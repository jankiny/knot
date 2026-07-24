package annualsummary

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const annualSummarySystemPrompt = `你是一个严谨的中文个人年度总结大纲助手。

安全与事实约束：
1. 只能使用用户消息中 evidence JSON 的事实，不得补充外部知识或猜测。
2. 团队、部门或项目成果不能自动改写为用户个人成果；个人分工不明确时写入 missing_information。
3. 不明确的数字、排名、奖项、影响范围和个人分工必须写入 missing_information，不得作为已核实成果。
4. metadata evidence 只有标题、类型、日期和项目等元数据；excerpt 为空时不得推断正文。
5. sections 和 verified_results 中的 evidence_ids 只能引用输入中存在的 evidence_id。
6. 只生成约 1500 字正文的篇幅分配和分章节大纲，不要生成完整正文。
7. 输出必须是合法 JSON 对象，不要输出 Markdown 代码块、解释或额外字段。`

var allowedAIHosts = map[string]struct{}{
	"api.deepseek.com": {},
	"api.openai.com":   {},
}

type openAIGateway struct {
	client *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewOpenAIGateway(client *http.Client) Gateway {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &openAIGateway{client: client}
}

func (gateway *openAIGateway) Complete(
	ctx context.Context,
	config AIConfig,
	input ModelInput,
) (Completion, error) {
	if err := validateAIDestination(config.APIURL); err != nil {
		return Completion{}, err
	}
	endpoint := normalizeAIEndpoint(config.APIURL)
	if endpoint == "" {
		return Completion{}, errors.New("AI endpoint is empty")
	}
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return Completion{}, fmt.Errorf("encode annual summary evidence: %w", err)
	}
	userPrompt := `请基于以下经过本地权限校验并由用户确认的 evidence 生成个人年度总结大纲。

输出 JSON 结构：
{
  "title": "总结标题",
  "target_word_count": 1500,
  "sections": [
    {
      "heading": "章节标题",
      "word_count": 300,
      "outline": ["只写大纲要点，不写完整正文"],
      "evidence_ids": ["evidence_xxx"]
    }
  ],
  "verified_results": [
    {
      "statement": "可由证据核实的成果或进展",
      "evidence_ids": ["evidence_xxx"]
    }
  ],
  "missing_information": ["需要用户确认或补充的信息"]
}

已确认 evidence JSON：
` + string(inputJSON)

	payload := map[string]any{
		"model": config.Model,
		"messages": []chatMessage{
			{Role: "system", Content: annualSummarySystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		"temperature": 0.2,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
	if isDeepSeekConfig(config) {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Completion{}, fmt.Errorf("encode annual summary request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return Completion{}, fmt.Errorf("create annual summary request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(config.APIKey); key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}

	response, err := gateway.client.Do(request)
	if err != nil {
		return Completion{}, fmt.Errorf("request annual summary model: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(
		io.LimitReader(response.Body, (2<<20)+1),
	)
	if err != nil {
		return Completion{}, fmt.Errorf("read annual summary response: %w", err)
	}
	if len(responseBody) > 2<<20 {
		return Completion{}, errors.New("annual summary response is too large")
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return Completion{}, fmt.Errorf(
			"annual summary model returned HTTP %d",
			response.StatusCode,
		)
	}

	var providerResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     *int `json:"prompt_tokens"`
			CompletionTokens *int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(responseBody, &providerResponse); err != nil {
		return Completion{}, fmt.Errorf("decode annual summary response: %w", err)
	}
	if len(providerResponse.Choices) == 0 {
		return Completion{}, errors.New("annual summary model returned no choices")
	}
	content := strings.TrimSpace(
		providerResponse.Choices[0].Message.Content,
	)
	if content == "" {
		content = strings.TrimSpace(providerResponse.Choices[0].Text)
	}
	if content == "" {
		return Completion{}, errors.New("annual summary model returned empty content")
	}
	return Completion{
		Content:      content,
		InputTokens:  providerResponse.Usage.PromptTokens,
		OutputTokens: providerResponse.Usage.CompletionTokens,
	}, nil
}

func normalizeAIEndpoint(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	switch {
	case value == "":
		return ""
	case strings.Contains(value, "/chat/completions"):
		return value
	case strings.HasSuffix(value, "/v1"):
		return value + "/chat/completions"
	default:
		return value + "/v1/chat/completions"
	}
}

func validateAIDestination(value string) error {
	endpoint := normalizeAIEndpoint(value)
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return &ValidationError{
			Field:   "ai.api_url",
			Message: "must be a valid HTTPS URL",
		}
	}
	host := strings.ToLower(parsed.Hostname())
	_, allowed := allowedAIHosts[host]
	if parsed.Scheme != "https" ||
		parsed.User != nil ||
		(parsed.Port() != "" && parsed.Port() != "443") ||
		!allowed {
		return &ValidationError{
			Field: "ai.api_url",
			Message: "must use an approved official HTTPS endpoint " +
				"(api.deepseek.com or api.openai.com)",
		}
	}
	return nil
}

func isDeepSeekConfig(config AIConfig) bool {
	return strings.Contains(strings.ToLower(config.APIURL), "deepseek") ||
		strings.Contains(strings.ToLower(config.Model), "deepseek")
}
