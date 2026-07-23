package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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
