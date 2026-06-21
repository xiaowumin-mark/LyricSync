package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lyricsync/internal/core"
	"lyricsync/pkg/model"
)

func TestHandleHealthReturnsStatus(t *testing.T) {
	state := core.NewState(model.DefaultConfig())
	state.SetService("api", "running", "listening")
	server := NewServer(state)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	server.handleHealth(recorder, request)

	var response model.HealthResponse
	decodeJSON(t, recorder.Body.Bytes(), &response)
	if !response.OK {
		t.Fatalf("expected ok health response: %#v", response)
	}
	if response.Status != "ok" {
		t.Fatalf("expected status ok, got %q", response.Status)
	}
	if response.Version != model.AppVersion {
		t.Fatalf("expected version %q, got %q", model.AppVersion, response.Version)
	}
}

func TestHealthStatusReportsErrors(t *testing.T) {
	ok, status := healthStatus([]model.ServiceStatus{{Name: "api", Status: "error"}})
	if ok || status != "error" {
		t.Fatalf("expected error health, got ok=%v status=%q", ok, status)
	}
}

func TestWindowActionRequiresController(t *testing.T) {
	server := NewServer(core.NewState(model.DefaultConfig()))
	recorder := httptest.NewRecorder()
	server.handleWindowShow(recorder, httptest.NewRequest(http.MethodPost, "/api/window/show", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}

func TestWindowActionInvokesController(t *testing.T) {
	server := NewServer(core.NewState(model.DefaultConfig()))
	window := &fakeWindowController{}
	server.SetWindowController(window)

	recorder := httptest.NewRecorder()
	server.handleWindowShow(recorder, httptest.NewRequest(http.MethodPost, "/api/window/show", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if window.shown != 1 {
		t.Fatalf("expected show call, got %d", window.shown)
	}
}

type fakeWindowController struct {
	shown     int
	hidden    int
	minimized int
}

func (c *fakeWindowController) ShowWindow() {
	c.shown++
}

func (c *fakeWindowController) HideWindow() {
	c.hidden++
}

func (c *fakeWindowController) MinimizeWindow() {
	c.minimized++
}

func decodeJSON(t *testing.T, data []byte, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode response: %v\n%s", err, string(data))
	}
}
