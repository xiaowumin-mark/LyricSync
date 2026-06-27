package lyric

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultUserAgent = "LyricSync/0.2"

func getJSON(ctx context.Context, client *http.Client, rawURL string, params url.Values, headers map[string]string, target any) error {
	body, err := getText(ctx, client, rawURL, params, headers)
	if err != nil {
		return err
	}
	body = strings.TrimSpace(stripJSONP(body))
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func getText(ctx context.Context, client *http.Client, rawURL string, params url.Values, headers map[string]string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if len(params) > 0 {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return "", err
		}
		query := parsed.Query()
		for key, values := range params {
			for _, value := range values {
				query.Add(key, value)
			}
		}
		parsed.RawQuery = query.Encode()
		rawURL = parsed.String()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	applyHeaders(req, headers)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	return html.UnescapeString(string(data)), nil
}

func postJSON(ctx context.Context, client *http.Client, rawURL string, payload any, headers map[string]string, target any) error {
	if client == nil {
		client = http.DefaultClient
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if headers == nil {
		headers = map[string]string{}
	}
	headers["Content-Type"] = "application/json"
	applyHeaders(req, headers)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal([]byte(stripJSONP(string(body))), target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func applyHeaders(req *http.Request, headers map[string]string) {
	req.Header.Set("User-Agent", defaultUserAgent)
	for key, value := range headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
}

func stripJSONP(input string) string {
	input = strings.TrimSpace(input)
	if input == "" || strings.HasPrefix(input, "{") || strings.HasPrefix(input, "[") {
		return input
	}
	start := strings.IndexByte(input, '(')
	end := strings.LastIndexByte(input, ')')
	if start >= 0 && end > start {
		return strings.TrimSpace(input[start+1 : end])
	}
	return input
}
