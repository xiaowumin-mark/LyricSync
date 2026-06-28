package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xiaowumin-mark/LyricSync/internal/lyric"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const defaultTimeout = 120 * time.Second

var ErrDisabled = errors.New("ai: disabled")

type Client struct {
	baseURL string
	apiKey  string
	model   string
	deep    bool
	vendor  string
	timeout time.Duration
	http    *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type lyricResponseLine struct {
	Index       int    `json:"index"`
	Text        string `json:"text"`
	Translation string `json:"translation"`
	Roman       string `json:"roman"`
	Delete      bool   `json:"-"`
}

func NewClient(cfg model.AIConfig) (*Client, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	key := strings.TrimSpace(cfg.APIKey)
	modelName := strings.TrimSpace(cfg.Model)
	if base == "" || key == "" || modelName == "" {
		return nil, ErrDisabled
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("ai: invalid base url")
	}
	return &Client{
		baseURL: strings.TrimRight(base, "/"),
		apiKey:  key,
		model:   modelName,
		deep:    cfg.DeepThinking,
		vendor:  aiVendor(parsed, modelName),
		timeout: timeoutFromConfig(cfg.TimeoutSeconds),
		http:    &http.Client{},
	}, nil
}

func ListModels(ctx context.Context, cfg model.AIConfig) ([]string, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	key := strings.TrimSpace(cfg.APIKey)
	if base == "" || key == "" {
		return nil, ErrDisabled
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai: list models failed: %s", statusError(resp.StatusCode, body))
	}
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	seen := map[string]struct{}{}
	for _, item := range decoded.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	sort.Strings(models)
	return models, nil
}

func (c *Client) EnhanceLyrics(ctx context.Context, doc lyric.Document, cfg model.LyricsConfig) (lyric.Document, bool, error) {
	doc = doc.Normalized()
	if !lyric.IsUsable(doc) {
		return doc, false, nil
	}
	clean := cfg.CleanStrategy == model.LyricsCleanAI
	translate := cfg.AITranslate && lacksTranslation(doc)
	romanize := cfg.AITransliterate && needsRomanization(doc)
	if !clean && !translate && !romanize {
		return doc, false, nil
	}
	input := lyricPatchInput(doc, 260)
	if strings.TrimSpace(input) == "" {
		return doc, false, nil
	}
	system := lyricPatchSystemPrompt()
	user := fmt.Sprintf(
		"%s\n任务开关：clean=%t, translate_to_simplified_chinese=%t, romanize=%t。\n歌词 JSON：\n%s",
		system, clean, translate, romanize, input,
	)
	text, err := c.chat(ctx, []Message{{Role: "system", Content: system}, {Role: "user", Content: user}})
	if err != nil {
		return doc, false, err
	}
	lines, err := parseLyricResponse(text)
	if err != nil {
		return doc, false, err
	}
	updated := applyResponseLines(doc, lines, clean, translate, romanize)
	return updated, true, nil
}

func (c *Client) chat(ctx context.Context, messages []Message) (string, error) {
	timeout := c.timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	text, err := c.chatOnce(ctx, messages, c.deep)
	if err == nil || !c.deep {
		return text, err
	}
	if !isBadRequest(err) {
		return "", err
	}
	return c.chatOnce(ctx, messages, false)
}

func timeoutFromConfig(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultTimeout
	}
	if seconds < 30 {
		seconds = 30
	}
	if seconds > 600 {
		seconds = 600
	}
	return time.Duration(seconds) * time.Second
}

func (c *Client) chatOnce(ctx context.Context, messages []Message, deep bool) (string, error) {
	body := map[string]any{
		"model":    c.model,
		"messages": messages,
	}
	applyThinkingOptions(body, c.vendor, deep)
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", httpStatusError{code: resp.StatusCode, message: statusError(resp.StatusCode, respBody)}
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("ai: empty chat completion")
	}
	return decodeContent(decoded.Choices[0].Message.Content)
}

func lyricPatchInput(doc lyric.Document, limit int) string {
	if limit <= 0 || limit > len(doc.Lines) {
		limit = len(doc.Lines)
	}
	type patchLine struct {
		Text  string `json:"text"`
		Trans string `json:"trans"`
		Roman string `json:"roman"`
	}
	input := make(map[string]patchLine, limit)
	count := 0
	for i, line := range doc.Lines {
		text := strings.TrimSpace(line.Text())
		if text == "" {
			continue
		}
		input[strconv.Itoa(i+1)] = patchLine{
			Text:  text,
			Trans: strings.TrimSpace(line.TranslatedLyric),
			Roman: strings.TrimSpace(line.RomanLyric),
		}
		count++
		if count >= limit {
			break
		}
	}
	if len(input) == 0 {
		return ""
	}
	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseLyricResponse(text string) ([]lyricResponseLine, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	// 如果是"---" 表示没有任何修改
	if text == "---" {
		return []lyricResponseLine{}, nil
	}

	if strings.Contains(text, "\n- ") || strings.HasPrefix(text, "- ") || strings.Contains(text, "\n+ ") || strings.HasPrefix(text, "+ ") {
		return parsePatchResponse(text)
	}
	var object struct {
		Lines []lyricResponseLine `json:"lines"`
	}
	if err := json.Unmarshal([]byte(text), &object); err == nil && len(object.Lines) > 0 {
		return object.Lines, nil
	}
	var lines []lyricResponseLine
	if err := json.Unmarshal([]byte(text), &lines); err != nil {
		return nil, err
	}
	return lines, nil
}

func parsePatchResponse(text string) ([]lyricResponseLine, error) {
	rows := strings.Split(text, "\n")
	out := make([]lyricResponseLine, 0, len(rows))
	for _, row := range rows {
		row = strings.TrimSpace(row)
		if row == "" || strings.HasPrefix(row, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(row, "-"):
			items, err := parseDeletePatch(strings.TrimSpace(strings.TrimPrefix(row, "-")))
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		case strings.HasPrefix(row, "+"):
			item, err := parseUpdatePatch(strings.TrimSpace(strings.TrimPrefix(row, "+")))
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ai: empty lyric patch")
	}
	return out, nil
}

func parseDeletePatch(value string) ([]lyricResponseLine, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("ai: empty delete patch")
	}
	startText, endText, ranged := strings.Cut(value, "-")
	start, err := parseLineNumber(startText)
	if err != nil {
		return nil, err
	}
	end := start
	if ranged {
		end, err = parseLineNumber(endText)
		if err != nil {
			return nil, err
		}
	}
	if end < start {
		start, end = end, start
	}
	out := make([]lyricResponseLine, 0, end-start+1)
	for index := start; index <= end; index++ {
		out = append(out, lyricResponseLine{Index: index - 1, Delete: true})
	}
	return out, nil
}

func parseUpdatePatch(value string) (lyricResponseLine, error) {
	head, raw, ok := strings.Cut(value, "|")
	if !ok {
		return parseQuotedUpdatePatch(value)
	}
	fields := strings.Fields(strings.TrimSpace(head))
	if len(fields) != 2 {
		return lyricResponseLine{}, fmt.Errorf("ai: invalid update patch %q", value)
	}
	lineNumber, err := parseLineNumber(fields[0])
	if err != nil {
		return lyricResponseLine{}, err
	}
	field := strings.ToLower(strings.TrimSpace(fields[1]))
	return patchFieldValue(lineNumber-1, field, strings.TrimSpace(raw))
}

func parseQuotedUpdatePatch(value string) (lyricResponseLine, error) {
	fields := strings.Fields(value)
	if len(fields) < 3 {
		return lyricResponseLine{}, fmt.Errorf("ai: invalid update patch %q", value)
	}
	lineNumber, err := parseLineNumber(fields[0])
	if err != nil {
		return lyricResponseLine{}, err
	}
	field := strings.ToLower(strings.TrimSpace(fields[1]))
	raw := strings.TrimSpace(strings.TrimPrefix(value, fields[0]))
	raw = strings.TrimSpace(strings.TrimPrefix(raw, fields[1]))
	return patchFieldValue(lineNumber-1, field, unquotePatchValue(raw))
}

func patchFieldValue(index int, field string, value string) (lyricResponseLine, error) {
	item := lyricResponseLine{Index: index}
	switch field {
	case "text":
		item.Text = value
	case "trans", "translation":
		item.Translation = value
	case "roman", "romanization":
		item.Roman = value
	default:
		return lyricResponseLine{}, fmt.Errorf("ai: unknown patch field %q", field)
	}
	return item, nil
}

func parseLineNumber(value string) (int, error) {
	value = strings.TrimSpace(value)
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("ai: invalid line number %q", value)
	}
	return number, nil
}

func unquotePatchValue(value string) string {
	value = strings.TrimSpace(value)
	if unquoted, err := strconv.Unquote(value); err == nil {
		return strings.TrimSpace(unquoted)
	}
	return strings.Trim(value, `"`)
}

func lyricPatchSystemPrompt() string {
	return strings.Join([]string{
		"你是一个用于音乐播放器的歌词同步增强系统（Lyric Patch Engine）。",
		"",
		"你的任务是：对用户提供的编号歌词数据进行清洗、翻译和拼音标注，并输出 patch 操作。",
		"",
		"【核心规则】",
		"1. 你只能处理用户提供的歌词编号数据。",
		"- 行号必须保持不变",
		"- 不允许新增行",
		"- 不允许修改行号",
		"2. 每一行可能包含以下字段：",
		"- text：原始歌词",
		"- trans：中文翻译（可能为空）",
		"- roman：拼音/音译（可能为空）",
		"3. 你的任务包括：",
		"- 清洗无效歌词内容",
		"- 生成或更新翻译（trans）",
		"- 生成或更新拼音（roman）",
		"",
		"【清洗规则】",
		"必须删除以下内容（不作为歌词）：",
		"- 歌曲标题",
		"- 歌手名称",
		"- 作词 / 作曲 / 编曲 / 制作人信息",
		"- 版权信息",
		"- 平台水印",
		"- 上传者信息",
		"- 空行",
		"只保留“可演唱的歌词内容”。",
		"",
		"【翻译规则】",
		"trans 字段：",
		"- 翻译为自然、流畅的简体中文",
		"- 必须符合歌词意境，而不是逐字翻译",
		"- 保持情绪、隐喻和艺术性",
		"- 不解释歌词，只做文学化表达",
		"- 如果歌词已经是中文，则不需要翻译",
		"- 如果歌词是其他语言，则翻译为中文",
		"- 不允许使用拼音或音译作为翻译",
		"- 如果歌词是繁体中文，则翻译为简体中文",
		"- 如果你觉得原翻译不准确或不自然，可以修改翻译",
		"",
		"【拼音规则】",
		"roman 字段：",
		"- 仅处理 非中文且非英文 的歌词（如日语、韩语等）",
		"- 使用标准中文拼音风格音译（不带声调）",
		"- 不允许对中文或英文进行拼音转换",
		"- 如果已经有拼音，则不需要修改",
		"- 如果你觉得原拼音不准确或不自然，可以修改拼音",
		"",
		"【输入格式】",
		"输入为 JSON：",
		"{\"1\":{\"text\":\"...\",\"trans\":\"...\",\"roman\":\"...\"}}",
		"说明：key 为行号，value 为歌词数据，字段可能为空。",
		"",
		"【输出格式（非常重要）】",
		"你必须只输出 patch 指令，禁止任何额外内容。",
		"支持三种操作：",
		"删除行：- N",
		"删除范围：- A-B",
		"修改字段：+ N text | ...",
		"修改字段：+ N trans | ...",
		"修改字段：+ N roman | ...",
		"",
		"【强制格式规则】",
		"- 使用 | 分隔字段和值",
		"- 不允许使用引号",
		"- 不允许 JSON 输出",
		"- 不允许 Markdown",
		"- 不允许解释文本",
		"- 不允许添加任何多余字符",
		"",
		"【字段规则】",
		"- 每条 patch 只能修改一个字段",
		"- text / trans / roman 必须分开写",
		"- 操作之间无顺序依赖",
		"- 只输出发生变化的内容",
		"- 如果没有任何修改，输出空内容",
		"",
		"【输出示例】",
		"+ 1 text | hello world",
		"+ 1 trans | 你好 世界",
		"+ 1 roman | ni hao shi jie",
		"- 5",
		"- 8-12",
		"注意：输出必须严格遵守以上规则，否则将无法被解析。",
		"注意：中文歌词不需要翻译！！",
		"如果你认为没有任何修改需要输出，请直接输出字符串'---'。",
	}, "\n")
}

func applyResponseLines(doc lyric.Document, lines []lyricResponseLine, clean bool, translate bool, romanize bool) lyric.Document {
	for _, item := range lines {
		if item.Index < 0 || item.Index >= len(doc.Lines) {
			continue
		}
		line := &doc.Lines[item.Index]
		if item.Delete && clean {
			line.Words = nil
			line.TranslatedLyric = ""
			line.RomanLyric = ""
			continue
		}
		if clean {
			text := strings.TrimSpace(item.Text)
			if text != "" {
				line.Words = []lyric.Word{{StartTimeMs: line.StartTimeMs, EndTimeMs: line.EndTimeMs, Text: text}}
			}
		}
		if translate {
			if value := strings.TrimSpace(item.Translation); value != "" {
				line.TranslatedLyric = value
			}
		}
		if romanize {
			if value := strings.TrimSpace(item.Roman); value != "" {
				line.RomanLyric = value
			}
		}
	}
	return doc.Normalized()
}

func needsRomanization(doc lyric.Document) bool {
	for _, line := range doc.Lines {
		text := line.Text()
		if strings.TrimSpace(text) == "" || strings.TrimSpace(line.RomanLyric) != "" {
			continue
		}
		for _, r := range text {
			if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
				return true
			}
			if r > unicode.MaxASCII && !unicode.Is(unicode.Han, r) && unicode.IsLetter(r) {
				return true
			}
		}
	}
	return false
}

func lacksTranslation(doc lyric.Document) bool {
	for _, line := range doc.Lines {
		if strings.TrimSpace(line.Text()) == "" {
			continue
		}
		if strings.TrimSpace(line.TranslatedLyric) == "" {
			return true
		}
	}
	return false
}

type httpStatusError struct {
	code    int
	message string
}

func (e httpStatusError) Error() string {
	return e.message
}

func isBadRequest(err error) bool {
	var status httpStatusError
	return errors.As(err, &status) && status.code == http.StatusBadRequest
}

func statusError(code int, body []byte) string {
	var decoded struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && strings.TrimSpace(decoded.Error.Message) != "" {
		return fmt.Sprintf("%d %s", code, strings.TrimSpace(decoded.Error.Message))
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 240 {
		text = text[:240]
	}
	if text == "" {
		text = http.StatusText(code)
	}
	return fmt.Sprintf("%d %s", code, text)
}

func applyThinkingOptions(body map[string]any, vendor string, deep bool) {
	switch vendor {
	case "deepseek":
		if deep {
			body["thinking"] = map[string]string{"type": "enabled"}
			body["reasoning_effort"] = "high"
			return
		}
		body["thinking"] = map[string]string{"type": "disabled"}
		body["temperature"] = 0.2
	case "zhipu", "kimi":
		if deep {
			body["thinking"] = map[string]string{"type": "enabled"}
			return
		}
		body["thinking"] = map[string]string{"type": "disabled"}
		body["temperature"] = 0.2
	case "qwen":
		body["enable_thinking"] = deep
		if !deep {
			body["temperature"] = 0.2
		}
	case "gemini":
		if deep {
			body["thinkingBudget"] = 8192
			return
		}
		body["thinkingBudget"] = 0
		body["temperature"] = 0.2
	case "baidu", "siliconflow":
		if deep {
			body["thinking_budget"] = 8192
			return
		}
		body["thinking_budget"] = 0
		body["temperature"] = 0.2
	case "openai", "azure-openai", "tencent", "mistral", "xai", "groq", "openrouter":
		if deep {
			body["reasoning_effort"] = "medium"
			return
		}
		body["temperature"] = 0.2
	default:
		if deep {
			body["reasoning_effort"] = "medium"
			return
		}
		body["temperature"] = 0.2
	}
}

func aiVendor(base *url.URL, modelName string) string {
	host := strings.ToLower(base.Host)
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if strings.Contains(host, "openrouter") {
		return "openrouter"
	}
	if strings.Contains(host, "siliconflow") || strings.Contains(host, "silicon") {
		return "siliconflow"
	}
	if strings.Contains(host, "groq") {
		return "groq"
	}
	if strings.Contains(host, "deepseek.com") {
		return "deepseek"
	}
	if strings.Contains(host, "openai.azure.com") {
		return "azure-openai"
	}
	if strings.Contains(host, "openai.com") {
		return "openai"
	}
	if strings.Contains(host, "dashscope") || strings.Contains(host, "aliyun") {
		return "qwen"
	}
	if strings.Contains(host, "bigmodel") || strings.Contains(host, "z.ai") || strings.Contains(host, "zhipu") {
		return "zhipu"
	}
	if strings.Contains(host, "moonshot") || strings.Contains(host, "kimi") {
		return "kimi"
	}
	if strings.Contains(host, "googleapis.com") || strings.Contains(host, "generativelanguage") {
		return "gemini"
	}
	if strings.Contains(host, "qianfan") || strings.Contains(host, "baidu") {
		return "baidu"
	}
	if strings.Contains(host, "hunyuan") || strings.Contains(host, "tencent") {
		return "tencent"
	}
	if strings.Contains(host, "mistral") {
		return "mistral"
	}
	if strings.Contains(host, "x.ai") || strings.Contains(host, "xai") {
		return "xai"
	}
	if strings.HasPrefix(modelName, "deepseek-v4-") || strings.HasPrefix(modelName, "deepseek-reasoner") {
		return "deepseek"
	}
	if strings.HasPrefix(modelName, "gpt-") || strings.HasPrefix(modelName, "o1") || strings.HasPrefix(modelName, "o3") || strings.HasPrefix(modelName, "o4") {
		return "openai"
	}
	if strings.HasPrefix(modelName, "qwen") || strings.HasPrefix(modelName, "qwq") {
		return "qwen"
	}
	if strings.HasPrefix(modelName, "glm-") {
		return "zhipu"
	}
	if strings.HasPrefix(modelName, "kimi") {
		return "kimi"
	}
	if strings.HasPrefix(modelName, "gemini") {
		return "gemini"
	}
	if strings.HasPrefix(modelName, "ernie") {
		return "baidu"
	}
	if strings.HasPrefix(modelName, "hunyuan") {
		return "tencent"
	}
	if strings.HasPrefix(modelName, "mistral") || strings.HasPrefix(modelName, "magistral") {
		return "mistral"
	}
	if strings.HasPrefix(modelName, "grok") {
		return "xai"
	}
	return ""
}

func decodeContent(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text), nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, part := range parts {
		if strings.TrimSpace(part.Text) != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(strings.TrimSpace(part.Text))
		}
	}
	return strings.TrimSpace(b.String()), nil
}
