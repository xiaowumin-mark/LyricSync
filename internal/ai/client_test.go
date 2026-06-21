package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lyricsync/pkg/model"
)

func TestProcessTranslateAppliesLineResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing bearer token: %s", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": `[{"index":0,"text":"你好"},{"index":1,"text":"世界"}]`,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(model.AIConfig{
		Enabled: true,
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 5,
	})
	doc, err := client.Process(context.Background(), model.AITaskRequest{
		Task:           TaskTranslate,
		TargetLanguage: "中文",
	}, model.LyricDocument{
		Lines: []model.LyricLine{
			{StartMs: 0, EndMs: 1000, Text: "hello"},
			{StartMs: 1000, EndMs: 2000, Text: "world"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Lines[0].Translation != "你好" || doc.Lines[1].Translation != "世界" {
		t.Fatalf("unexpected translated lines: %#v", doc.Lines)
	}
	if !doc.Translated {
		t.Fatal("expected translated flag")
	}
}

func TestProcessRejectsDisabledAI(t *testing.T) {
	client := NewClient(model.AIConfig{})
	_, err := client.Process(context.Background(), model.AITaskRequest{Task: TaskTranslate}, model.LyricDocument{
		Lines: []model.LyricLine{{Text: "hello"}},
	})
	if err == nil {
		t.Fatal("expected disabled AI error")
	}
}

func TestProcessRetriesTransientProviderFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": `[{"index":0,"text":"你好"}]`,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(model.AIConfig{
		Enabled: true,
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
		Timeout: 5,
	})
	doc, err := client.Process(context.Background(), model.AITaskRequest{Task: TaskTranslate}, model.LyricDocument{
		Lines: []model.LyricLine{{StartMs: 0, EndMs: 1000, Text: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if doc.Lines[0].Translation != "你好" {
		t.Fatalf("unexpected translation: %#v", doc.Lines[0])
	}
}
