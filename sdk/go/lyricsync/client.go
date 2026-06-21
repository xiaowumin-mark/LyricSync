package lyricsync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"lyricsync/pkg/model"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	dialer     *websocket.Dialer
}

func New(baseURL string, options ...Option) *Client {
	client := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		dialer: websocket.DefaultDialer,
	}
	if client.baseURL == "" {
		client.baseURL = "http://127.0.0.1:41917"
	}
	for _, option := range options {
		option(client)
	}
	return client
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func WithDialer(dialer *websocket.Dialer) Option {
	return func(client *Client) {
		if dialer != nil {
			client.dialer = dialer
		}
	}
}

func (c *Client) Health(ctx context.Context) (model.HealthResponse, error) {
	var result model.HealthResponse
	err := c.getJSON(ctx, "/api/health", &result)
	return result, err
}

func (c *Client) State(ctx context.Context) (model.AppSnapshot, error) {
	var result model.AppSnapshot
	err := c.getJSON(ctx, "/api/state", &result)
	return result, err
}

func (c *Client) Song(ctx context.Context) (model.Track, error) {
	var result model.Track
	err := c.getJSON(ctx, "/api/song", &result)
	return result, err
}

func (c *Client) Lyric(ctx context.Context) (model.LyricDocument, error) {
	var result model.LyricDocument
	err := c.getJSON(ctx, "/api/lyric", &result)
	return result, err
}

func (c *Client) Sessions(ctx context.Context) ([]model.Session, error) {
	var result []model.Session
	err := c.getJSON(ctx, "/api/session", &result)
	return result, err
}

func (c *Client) Clients(ctx context.Context) ([]model.WSClient, error) {
	var result []model.WSClient
	err := c.getJSON(ctx, "/api/clients", &result)
	return result, err
}

func (c *Client) Logs(ctx context.Context) ([]model.LogEntry, error) {
	var result []model.LogEntry
	err := c.getJSON(ctx, "/api/logs", &result)
	return result, err
}

func (c *Client) Config(ctx context.Context) (model.AppConfig, error) {
	var result model.AppConfig
	err := c.getJSON(ctx, "/api/config", &result)
	return result, err
}

func (c *Client) Metrics(ctx context.Context) (model.AppMetrics, error) {
	var result model.AppMetrics
	err := c.getJSON(ctx, "/api/metrics", &result)
	return result, err
}

func (c *Client) ConnectEvents(ctx context.Context) (*websocket.Conn, *http.Response, error) {
	return c.connect(ctx, "/ws")
}

func (c *Client) ConnectAMLL(ctx context.Context) (*websocket.Conn, *http.Response, error) {
	return c.connect(ctx, "/amll/ws")
}

func (c *Client) getJSON(ctx context.Context, path string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lyricsync: GET %s returned %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) connect(ctx context.Context, path string) (*websocket.Conn, *http.Response, error) {
	wsURL, err := c.webSocketURL(path)
	if err != nil {
		return nil, nil, err
	}
	return c.dialer.DialContext(ctx, wsURL, nil)
}

func (c *Client) webSocketURL(path string) (string, error) {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("lyricsync: unsupported URL scheme %q", parsed.Scheme)
	}
	parsed.Path = path
	parsed.RawQuery = ""
	return parsed.String(), nil
}
