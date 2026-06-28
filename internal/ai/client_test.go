package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/lyric"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "z-model"}, {"id": "a-model"}, {"id": "z-model"}},
		})
	}))
	defer server.Close()

	models, err := ListModels(context.Background(), model.AIConfig{BaseURL: server.URL, APIKey: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "a-model" || models[1] != "z-model" {
		t.Fatalf("models = %#v", models)
	}
}

func TestEnhanceLyricsFallsBackWithoutReasoningEffort(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if calls == 1 {
			if _, ok := body["reasoning_effort"]; !ok {
				t.Fatal("expected first request to include reasoning_effort")
			}
			http.Error(w, `{"error":{"message":"unknown field"}}`, http.StatusBadRequest)
			return
		}
		if _, ok := body["reasoning_effort"]; ok {
			t.Fatal("expected retry to omit reasoning_effort")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{
					"content": `{"lines":[{"index":0,"text":"Hello world","translation":"你好，世界","roman":""}]}`,
				},
			}},
		})
	}))
	defer server.Close()

	client, err := NewClient(model.AIConfig{
		BaseURL:      server.URL,
		APIKey:       "token",
		Model:        "test-model",
		DeepThinking: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	doc := lyric.Document{Lines: []lyric.Line{{
		StartTimeMs: 0,
		EndTimeMs:   1000,
		Words:       []lyric.Word{{StartTimeMs: 0, EndTimeMs: 1000, Text: "hello world"}},
	}}}
	got, changed, err := client.EnhanceLyrics(context.Background(), doc, model.LyricsConfig{
		CleanStrategy: model.LyricsCleanAI,
		AITranslate:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed lyrics")
	}
	if got.Lines[0].Text() != "Hello world" || got.Lines[0].TranslatedLyric != "你好，世界" {
		t.Fatalf("enhanced line = %#v", got.Lines[0])
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestEnhanceLyricsOmitsReasoningEffortByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["reasoning_effort"]; ok {
			t.Fatal("expected default request to omit reasoning_effort")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{
					"content": `+ 1 trans | 你好，世界`,
				},
			}},
		})
	}))
	defer server.Close()

	client, err := NewClient(model.AIConfig{
		BaseURL: server.URL,
		APIKey:  "token",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	doc := lyric.Document{Lines: []lyric.Line{{
		StartTimeMs: 0,
		EndTimeMs:   1000,
		Words:       []lyric.Word{{StartTimeMs: 0, EndTimeMs: 1000, Text: "hello world"}},
	}}}
	got, changed, err := client.EnhanceLyrics(context.Background(), doc, model.LyricsConfig{AITranslate: true})
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Lines[0].TranslatedLyric != "你好，世界" {
		t.Fatalf("enhanced line = %#v changed=%t", got.Lines[0], changed)
	}
}

func TestDeepSeekDisablesThinkingByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		thinking, ok := body["thinking"].(map[string]any)
		if !ok || thinking["type"] != "disabled" {
			t.Fatalf("thinking = %#v, want disabled", body["thinking"])
		}
		if _, ok := body["reasoning_effort"]; ok {
			t.Fatal("expected default DeepSeek request to omit reasoning_effort")
		}
		if _, ok := body["temperature"]; !ok {
			t.Fatal("expected disabled thinking request to keep temperature")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": `+ 1 trans | 你好，世界`},
			}},
		})
	}))
	defer server.Close()

	client, err := NewClient(model.AIConfig{
		BaseURL: server.URL,
		APIKey:  "token",
		Model:   "deepseek-v4-0324",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, changed, err := client.EnhanceLyrics(context.Background(), testLyricDocument(), model.LyricsConfig{AITranslate: true})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed lyrics")
	}
}

func TestDeepSeekEnablesThinkingWhenConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		thinking, ok := body["thinking"].(map[string]any)
		if !ok || thinking["type"] != "enabled" {
			t.Fatalf("thinking = %#v, want enabled", body["thinking"])
		}
		if body["reasoning_effort"] != "high" {
			t.Fatalf("reasoning_effort = %#v, want high", body["reasoning_effort"])
		}
		if _, ok := body["temperature"]; ok {
			t.Fatal("expected thinking request to omit temperature")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": `+ 1 trans | 你好，世界`},
			}},
		})
	}))
	defer server.Close()

	client, err := NewClient(model.AIConfig{
		BaseURL:      server.URL,
		APIKey:       "token",
		Model:        "deepseek-v4-0324",
		DeepThinking: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, changed, err := client.EnhanceLyrics(context.Background(), testLyricDocument(), model.LyricsConfig{AITranslate: true})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed lyrics")
	}
}

func TestApplyThinkingOptionsByVendor(t *testing.T) {
	tests := []struct {
		name      string
		vendor    string
		deep      bool
		wantKey   string
		wantValue any
		absentKey string
	}{
		{name: "deepseek off", vendor: "deepseek", wantKey: "thinking", wantValue: "disabled", absentKey: "reasoning_effort"},
		{name: "deepseek on", vendor: "deepseek", deep: true, wantKey: "reasoning_effort", wantValue: "high", absentKey: "temperature"},
		{name: "zhipu off", vendor: "zhipu", wantKey: "thinking", wantValue: "disabled"},
		{name: "kimi on", vendor: "kimi", deep: true, wantKey: "thinking", wantValue: "enabled"},
		{name: "qwen on", vendor: "qwen", deep: true, wantKey: "enable_thinking", wantValue: true},
		{name: "qwen off", vendor: "qwen", wantKey: "enable_thinking", wantValue: false},
		{name: "gemini on", vendor: "gemini", deep: true, wantKey: "thinkingBudget", wantValue: 8192},
		{name: "baidu off", vendor: "baidu", wantKey: "thinking_budget", wantValue: 0},
		{name: "siliconflow on", vendor: "siliconflow", deep: true, wantKey: "thinking_budget", wantValue: 8192},
		{name: "openai on", vendor: "openai", deep: true, wantKey: "reasoning_effort", wantValue: "medium"},
		{name: "unknown off", vendor: "", wantKey: "temperature", wantValue: 0.2},
		{name: "unknown on", vendor: "", deep: true, wantKey: "reasoning_effort", wantValue: "medium"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]any{}
			applyThinkingOptions(body, tt.vendor, tt.deep)
			got, ok := body[tt.wantKey]
			if !ok {
				t.Fatalf("missing key %q in %#v", tt.wantKey, body)
			}
			if tt.wantKey == "thinking" {
				thinking, ok := got.(map[string]string)
				if !ok || thinking["type"] != tt.wantValue {
					t.Fatalf("thinking = %#v, want %v", got, tt.wantValue)
				}
			} else if got != tt.wantValue {
				t.Fatalf("%s = %#v, want %#v", tt.wantKey, got, tt.wantValue)
			}
			if tt.absentKey != "" {
				if _, ok := body[tt.absentKey]; ok {
					t.Fatalf("unexpected key %q in %#v", tt.absentKey, body)
				}
			}
		})
	}
}

func TestAIVendorDetection(t *testing.T) {
	tests := []struct {
		baseURL string
		model   string
		want    string
	}{
		{baseURL: "https://api.deepseek.com/v1", model: "deepseek-chat", want: "deepseek"},
		{baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", model: "qwen-plus", want: "qwen"},
		{baseURL: "https://open.bigmodel.cn/api/paas/v4", model: "glm-4.5", want: "zhipu"},
		{baseURL: "https://api.moonshot.cn/v1", model: "kimi-k2", want: "kimi"},
		{baseURL: "https://generativelanguage.googleapis.com/v1beta/openai", model: "gemini-2.5-pro", want: "gemini"},
		{baseURL: "https://qianfan.baidubce.com/v2", model: "ernie-x1", want: "baidu"},
		{baseURL: "https://api.hunyuan.cloud.tencent.com/v1", model: "hunyuan-t1", want: "tencent"},
		{baseURL: "https://api.mistral.ai/v1", model: "magistral-medium", want: "mistral"},
		{baseURL: "https://api.x.ai/v1", model: "grok-4", want: "xai"},
		{baseURL: "https://api.groq.com/openai/v1", model: "openai/gpt-oss-120b", want: "groq"},
		{baseURL: "https://openrouter.ai/api/v1", model: "openai/gpt-oss-120b", want: "openrouter"},
		{baseURL: "https://api.siliconflow.cn/v1", model: "Qwen/Qwen3", want: "siliconflow"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			parsed, err := url.Parse(tt.baseURL)
			if err != nil {
				t.Fatal(err)
			}
			if got := aiVendor(parsed, tt.model); got != tt.want {
				t.Fatalf("aiVendor(%q, %q) = %q, want %q", tt.baseURL, tt.model, got, tt.want)
			}
		})
	}
}

func TestParsePatchResponse(t *testing.T) {
	lines, err := parseLyricResponse(strings.Join([]string{
		"- 1-2",
		`+ 3 text | Singable lyric`,
		`+ 3 trans | Elegant translation`,
		`+ 3 roman | zhong wen shi luo ma yin`,
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Fatalf("patch lines = %d, want 5", len(lines))
	}
	if !lines[0].Delete || lines[0].Index != 0 || !lines[1].Delete || lines[1].Index != 1 {
		t.Fatalf("delete patches = %#v", lines[:2])
	}
	if lines[2].Index != 2 || lines[2].Text != "Singable lyric" {
		t.Fatalf("text patch = %#v", lines[2])
	}
	if lines[3].Translation != "Elegant translation" || lines[4].Roman != "zhong wen shi luo ma yin" {
		t.Fatalf("aux patches = %#v", lines[3:])
	}
}

func TestParseQuotedPatchResponseForCompatibility(t *testing.T) {
	lines, err := parseLyricResponse(`+ 1 trans "Elegant translation"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Translation != "Elegant translation" {
		t.Fatalf("patch lines = %#v", lines)
	}
}

func TestLyricPatchInputUsesJSONByLineNumber(t *testing.T) {
	input := lyricPatchInput(lyric.Document{Lines: []lyric.Line{{
		StartTimeMs:     0,
		EndTimeMs:       1000,
		Words:           []lyric.Word{{StartTimeMs: 0, EndTimeMs: 1000, Text: "hello world"}},
		TranslatedLyric: "你好，世界",
		RomanLyric:      "ni hao shi jie",
	}, {
		StartTimeMs: 1000,
		EndTimeMs:   2000,
		Words:       []lyric.Word{{StartTimeMs: 1000, EndTimeMs: 2000, Text: "second line"}},
	}}}, 10)
	var decoded map[string]struct {
		Text  string `json:"text"`
		Trans string `json:"trans"`
		Roman string `json:"roman"`
	}
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["1"].Text != "hello world" || decoded["1"].Trans != "你好，世界" || decoded["1"].Roman != "ni hao shi jie" {
		t.Fatalf("line 1 = %#v", decoded["1"])
	}
	if decoded["2"].Text != "second line" || decoded["2"].Trans != "" || decoded["2"].Roman != "" {
		t.Fatalf("line 2 = %#v", decoded["2"])
	}
}

func testLyricDocument() lyric.Document {
	return lyric.Document{Lines: []lyric.Line{{
		StartTimeMs: 0,
		EndTimeMs:   1000,
		Words:       []lyric.Word{{StartTimeMs: 0, EndTimeMs: 1000, Text: "hello world"}},
	}}}
}

func TestTimeoutFromConfig(t *testing.T) {
	tests := []struct {
		in   int
		want time.Duration
	}{
		{in: 0, want: 120 * time.Second},
		{in: 5, want: 30 * time.Second},
		{in: 180, want: 180 * time.Second},
		{in: 999, want: 600 * time.Second},
	}
	for _, tt := range tests {
		if got := timeoutFromConfig(tt.in); got != tt.want {
			t.Fatalf("timeoutFromConfig(%d) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestApplyPatchDeletesOnlyWhenCleaning(t *testing.T) {
	doc := lyric.Document{Lines: []lyric.Line{
		{StartTimeMs: 0, EndTimeMs: 1000, Words: []lyric.Word{{StartTimeMs: 0, EndTimeMs: 1000, Text: "Composer: Demo"}}},
		{StartTimeMs: 1000, EndTimeMs: 2000, Words: []lyric.Word{{StartTimeMs: 1000, EndTimeMs: 2000, Text: "Actual lyric"}}},
	}}
	patch := []lyricResponseLine{
		{Index: 0, Delete: true},
		{Index: 1, Translation: "An elegant line"},
	}
	withoutClean := applyResponseLines(doc, patch, false, true, false)
	if len(withoutClean.Lines) != 2 {
		t.Fatalf("delete should be ignored without clean, got %d lines", len(withoutClean.Lines))
	}
	withClean := applyResponseLines(doc, patch, true, true, false)
	if len(withClean.Lines) != 1 || withClean.Lines[0].Text() != "Actual lyric" {
		t.Fatalf("cleaned lines = %#v", withClean.Lines)
	}
	if withClean.Lines[0].TranslatedLyric != "An elegant line" {
		t.Fatalf("translation = %q", withClean.Lines[0].TranslatedLyric)
	}
}
