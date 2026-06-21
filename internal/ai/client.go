package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lyricsync/pkg/model"
)

const (
	TaskTranslate = "translate"
	TaskPolish    = "polish"
	TaskBilingual = "bilingual"
	TaskRomanize  = "romanize"
)

type Client struct {
	cfg    model.AIConfig
	client *http.Client
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.client = httpClient
		}
	}
}

func NewClient(cfg model.AIConfig, options ...Option) *Client {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &Client{
		cfg: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}
	for _, option := range options {
		option(client)
	}
	return client
}

type LineResult struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

func (c *Client) Process(ctx context.Context, request model.AITaskRequest, doc model.LyricDocument) (model.LyricDocument, error) {
	if !c.cfg.Enabled {
		return model.LyricDocument{}, fmt.Errorf("ai: disabled")
	}
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return model.LyricDocument{}, fmt.Errorf("ai: base URL is required")
	}
	if strings.TrimSpace(c.cfg.Model) == "" {
		return model.LyricDocument{}, fmt.Errorf("ai: model is required")
	}
	if len(doc.Lines) == 0 {
		return model.LyricDocument{}, fmt.Errorf("ai: no lyric lines to process")
	}

	task := normalizeTask(request.Task)
	if task == "" {
		return model.LyricDocument{}, fmt.Errorf("ai: unsupported task %q", request.Task)
	}

	results, err := c.requestLineResults(ctx, task, request, doc)
	if err != nil {
		return model.LyricDocument{}, err
	}
	if len(results) == 0 {
		return model.LyricDocument{}, fmt.Errorf("ai: empty model result")
	}

	next := doc
	next.Lines = append([]model.LyricLine(nil), doc.Lines...)
	for _, result := range results {
		if result.Index < 0 || result.Index >= len(next.Lines) {
			continue
		}
		text := strings.TrimSpace(result.Text)
		if text == "" {
			continue
		}
		switch task {
		case TaskTranslate, TaskBilingual:
			next.Lines[result.Index].Translation = text
		case TaskPolish:
			next.Lines[result.Index].Text = text
		case TaskRomanize:
			next.Lines[result.Index].Romanization = text
		}
	}
	next.Translated = hasLineTranslations(next.Lines)
	next.LastUpdated = model.Now()
	return next, nil
}

func (c *Client) requestLineResults(ctx context.Context, task string, request model.AITaskRequest, doc model.LyricDocument) ([]LineResult, error) {
	payload := chatCompletionRequest{
		Model: c.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt(task, request)},
			{Role: "user", Content: userPrompt(task, request, doc)},
		},
		Temperature: temperatureForTask(task),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		results, err := c.doLineResultRequest(ctx, body)
		if err == nil {
			return results, nil
		}
		lastErr = err
		if !isRetryableAIError(err) || attempt == 3 {
			break
		}
		if err := sleepWithContext(ctx, time.Duration(attempt)*250*time.Millisecond); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) doLineResultRequest(ctx context.Context, body []byte) ([]LineResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, completionURL(c.cfg.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, retryableError{err: fmt.Errorf("ai: request failed: %w", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("ai: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("ai: provider returned %s", resp.Status)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return nil, retryableError{err: err}
		}
		return nil, err
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return nil, fmt.Errorf("ai: decode response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("ai: response has no choices")
	}
	content := completion.Choices[0].Message.Content
	results, err := parseLineResults(content)
	if err != nil {
		return nil, err
	}
	return results, nil
}

type retryableError struct {
	err error
}

func (e retryableError) Error() string {
	return e.err.Error()
}

func (e retryableError) Unwrap() error {
	return e.err
}

func isRetryableAIError(err error) bool {
	_, ok := err.(retryableError)
	return ok
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func normalizeTask(task string) string {
	switch strings.ToLower(strings.TrimSpace(task)) {
	case TaskTranslate, "translation":
		return TaskTranslate
	case TaskPolish, "rewrite":
		return TaskPolish
	case TaskBilingual:
		return TaskBilingual
	case TaskRomanize, "romanization":
		return TaskRomanize
	default:
		return ""
	}
}

func systemPrompt(task string, request model.AITaskRequest) string {
	target := strings.TrimSpace(request.TargetLanguage)
	if target == "" {
		target = "the target language requested by the user"
	}
	switch task {
	case TaskTranslate:
		return "You translate song lyrics line by line. Preserve meaning, tone, line order, and timing alignment. Return only JSON."
	case TaskPolish:
		tone := strings.TrimSpace(request.Tone)
		if tone == "" {
			tone = "natural, singable, and readable"
		}
		return "You polish song lyrics line by line. Keep the original language and timing alignment. Tone: " + tone + ". Return only JSON."
	case TaskBilingual:
		return "You generate bilingual song lyrics by translating each line to " + target + ". Preserve line order and timing alignment. Return only JSON."
	case TaskRomanize:
		return "You romanize song lyrics line by line. Return readable Latin-script romanization only. Preserve line order and timing alignment. Return only JSON."
	default:
		return "Process song lyrics line by line and return only JSON."
	}
}

func userPrompt(task string, request model.AITaskRequest, doc model.LyricDocument) string {
	type promptLine struct {
		Index       int    `json:"index"`
		StartMs     int64  `json:"startMs"`
		EndMs       int64  `json:"endMs"`
		Text        string `json:"text"`
		Translation string `json:"translation,omitempty"`
	}
	lines := make([]promptLine, 0, len(doc.Lines))
	for i, line := range doc.Lines {
		lines = append(lines, promptLine{
			Index:       i,
			StartMs:     line.StartMs,
			EndMs:       line.EndMs,
			Text:        line.Text,
			Translation: line.Translation,
		})
	}
	payload := map[string]interface{}{
		"task":           task,
		"targetLanguage": request.TargetLanguage,
		"tone":           request.Tone,
		"requiredOutput": "JSON array only, each item: {\"index\": number, \"text\": string}",
		"lines":          lines,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func temperatureForTask(task string) float64 {
	if task == TaskPolish {
		return 0.4
	}
	return 0.2
}

func completionURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	parsed, err := url.Parse(baseURL)
	if err == nil && strings.HasSuffix(parsed.Path, "/chat/completions") {
		return parsed.String()
	}
	return baseURL + "/chat/completions"
}

func parseLineResults(content string) ([]LineResult, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var results []LineResult
	if err := json.Unmarshal([]byte(content), &results); err == nil {
		return results, nil
	}
	var wrapped struct {
		Lines []LineResult `json:"lines"`
	}
	if err := json.Unmarshal([]byte(content), &wrapped); err == nil && len(wrapped.Lines) > 0 {
		return wrapped.Lines, nil
	}
	return nil, fmt.Errorf("ai: expected JSON array line results")
}

func hasLineTranslations(lines []model.LyricLine) bool {
	for _, line := range lines {
		if strings.TrimSpace(line.Translation) != "" {
			return true
		}
	}
	return false
}
