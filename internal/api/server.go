package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"lyricsync/internal/core"
	"lyricsync/pkg/amllws"
	"lyricsync/pkg/model"
)

type Server struct {
	state    *core.State
	server   *http.Server
	upgrader websocket.Upgrader
	window   WindowController
	mu       sync.Mutex
}

type WindowController interface {
	ShowWindow()
	HideWindow()
	MinimizeWindow()
}

type wsEnvelope struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	SentAt  string      `json:"sentAt"`
}

func (s *Server) SetWindowController(window WindowController) {
	s.mu.Lock()
	s.window = window
	s.mu.Unlock()
}

func NewServer(state *core.State) *Server {
	return &Server{
		state: state,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *Server) Start(ctx context.Context, cfg model.ServerConfig) error {
	if !cfg.Enabled {
		s.state.SetService("api", "disabled", "HTTP/WebSocket API disabled")
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.traceHTTP(s.handleHealth))
	mux.HandleFunc("/api/state", s.traceHTTP(s.handleState))
	mux.HandleFunc("/api/song", s.traceHTTP(s.handleSong))
	mux.HandleFunc("/api/lyric", s.traceHTTP(s.handleLyric))
	mux.HandleFunc("/api/session", s.traceHTTP(s.handleSession))
	mux.HandleFunc("/api/clients", s.traceHTTP(s.handleClients))
	mux.HandleFunc("/api/logs", s.traceHTTP(s.handleLogs))
	mux.HandleFunc("/api/config", s.traceHTTP(s.handleConfig))
	mux.HandleFunc("/api/metrics", s.traceHTTP(s.handleMetrics))
	mux.HandleFunc("/api/window/show", s.traceHTTP(s.handleWindowShow))
	mux.HandleFunc("/api/window/hide", s.traceHTTP(s.handleWindowHide))
	mux.HandleFunc("/api/window/minimize", s.traceHTTP(s.handleWindowMinimize))
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/amll/ws", s.handleAMLLWebSocket)

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	server := &http.Server{
		Addr:              addr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.server = server

	go func() {
		s.state.SetService("api", "running", "listening on http://"+addr)
		s.state.AddLog("info", "api", "HTTP and WebSocket server listening on http://"+addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.state.SetService("api", "error", err.Error())
			s.state.AddLog("error", "api", err.Error())
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Stop(shutdownCtx)
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.mu.Unlock()

	if server == nil {
		return nil
	}
	s.state.SetService("api", "stopping", "shutting down HTTP/WebSocket API")
	err := server.Shutdown(ctx)
	if err != nil {
		s.state.SetService("api", "error", err.Error())
		return err
	}
	s.state.SetService("api", "stopped", "HTTP/WebSocket API stopped")
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	snapshot := s.state.Snapshot()
	ok, status := healthStatus(snapshot.Services)
	writeJSON(w, http.StatusOK, model.HealthResponse{
		OK:       ok,
		Status:   status,
		Version:  model.AppVersion,
		Time:     model.FormatTime(time.Now()),
		Services: snapshot.Services,
	})
}

func healthStatus(services []model.ServiceStatus) (bool, string) {
	status := "ok"
	for _, service := range services {
		switch service.Status {
		case "error":
			return false, "error"
		case "degraded":
			status = "degraded"
		}
	}
	return true, status
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot())
}

func (s *Server) handleSong(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot().Track)
}

func (s *Server) handleLyric(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot().Lyrics)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot().Sessions)
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot().Clients)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot().Logs)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Config())
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot().Metrics)
}

func (s *Server) handleWindowShow(w http.ResponseWriter, r *http.Request) {
	s.handleWindowAction(w, "show", func(window WindowController) {
		window.ShowWindow()
	})
}

func (s *Server) handleWindowHide(w http.ResponseWriter, r *http.Request) {
	s.handleWindowAction(w, "hide", func(window WindowController) {
		window.HideWindow()
	})
}

func (s *Server) handleWindowMinimize(w http.ResponseWriter, r *http.Request) {
	s.handleWindowAction(w, "minimize", func(window WindowController) {
		window.MinimizeWindow()
	})
}

func (s *Server) handleWindowAction(w http.ResponseWriter, action string, run func(WindowController)) {
	s.mu.Lock()
	window := s.window
	s.mu.Unlock()
	if window == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":     false,
			"action": action,
			"error":  "window controller is not ready",
		})
		return
	}
	run(window)
	s.state.AddLog("info", "window", action+" requested through HTTP API")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"action": action,
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.state.AddLog("warn", "websocket", "upgrade failed: "+err.Error())
		return
	}
	defer conn.Close()

	now := time.Now()
	client := model.WSClient{
		ID:          fmt.Sprintf("client-%d", now.UnixNano()),
		Name:        "AMLL Client",
		RemoteAddr:  r.RemoteAddr,
		ConnectedAt: model.FormatTime(now),
		LastSeenAt:  model.FormatTime(now),
	}
	s.state.AddClient(client)
	s.state.AddWebSocketClient("/ws")
	s.state.AddLog("info", "websocket", "client connected: "+client.RemoteAddr)
	defer func() {
		s.state.RemoveClient(client.ID)
		s.state.RemoveWebSocketClient("/ws")
		s.state.AddLog("info", "websocket", "client disconnected: "+client.RemoteAddr)
	}()

	events, cancel := s.state.Subscribe(64)
	defer cancel()

	var writeMu sync.Mutex
	write := func(message wsEnvelope) error {
		data, err := json.Marshal(message)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
		s.state.AddWebSocketSend("/ws", message.Type, len(data), false)
		return nil
	}

	if err := write(wsEnvelope{Type: "hello", Payload: map[string]string{
		"name":    "LyricSync",
		"version": model.AppVersion,
	}, SentAt: model.Now()}); err != nil {
		return
	}
	if err := write(wsEnvelope{Type: "state", Payload: s.state.Snapshot(), SentAt: model.Now()}); err != nil {
		return
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			var msg map[string]interface{}
			_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			client.LastSeenAt = model.Now()
			if name, ok := msg["name"].(string); ok && name != "" {
				client.Name = name
			}
			if sent, ok := msg["sentAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, sent); err == nil {
					client.LatencyMs = time.Since(t).Milliseconds()
				}
			}
			s.state.UpdateClient(client)
			if msg["type"] == "ping" {
				_ = write(wsEnvelope{Type: "pong", Payload: map[string]string{"time": model.Now()}, SentAt: model.Now()})
			}
		}
	}()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-readDone:
			return
		case event := <-events:
			if err := write(wsEnvelope{Type: event.Type, Payload: event.Payload, SentAt: event.Time}); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := write(wsEnvelope{Type: "heartbeat", Payload: map[string]string{"time": model.Now()}, SentAt: model.Now()}); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleAMLLWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.state.AddLog("warn", "amll-ws", "upgrade failed: "+err.Error())
		return
	}
	defer conn.Close()

	now := time.Now()
	client := model.WSClient{
		ID:          fmt.Sprintf("amll-%d", now.UnixNano()),
		Name:        "AMLL v2 Client",
		RemoteAddr:  r.RemoteAddr,
		ConnectedAt: model.FormatTime(now),
		LastSeenAt:  model.FormatTime(now),
	}
	s.state.AddClient(client)
	s.state.AddWebSocketClient("/amll/ws")
	s.state.AddLog("info", "amll-ws", "client connected: "+client.RemoteAddr)
	defer func() {
		s.state.RemoveClient(client.ID)
		s.state.RemoveWebSocketClient("/amll/ws")
		s.state.AddLog("info", "amll-ws", "client disconnected: "+client.RemoteAddr)
	}()

	events, cancel := s.state.Subscribe(128)
	defer cancel()

	var writeMu sync.Mutex
	writeJSONMessage := func(message amllws.Message) error {
		data, err := json.Marshal(message)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
		s.state.AddWebSocketSend("/amll/ws", amllMessageType(message), len(data), false)
		return nil
	}
	writeBinaryMessage := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			return err
		}
		s.state.AddWebSocketSend("/amll/ws", "binary:audioData", len(data), true)
		return nil
	}

	if err := writeJSONMessage(amllws.Initialize()); err != nil {
		return
	}
	snapshot := s.state.Snapshot()
	for _, message := range amllws.SnapshotMessages(snapshot) {
		if err := writeJSONMessage(message); err != nil {
			return
		}
	}
	if data, ok := amllws.BinaryAudioData(snapshot.Audio); ok {
		if err := writeBinaryMessage(data); err != nil {
			return
		}
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			var msg map[string]interface{}
			_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			client.LastSeenAt = model.Now()
			s.state.UpdateClient(client)
			if msg["type"] == "ping" {
				_ = writeJSONMessage(amllws.Pong())
			}
		}
	}()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-readDone:
			return
		case event := <-events:
			for _, message := range amllws.EventMessages(event) {
				if err := writeJSONMessage(message); err != nil {
					return
				}
			}
			if event.Type == "audio_frame" {
				if frame, ok := event.Payload.(model.AudioFrame); ok {
					if data, ok := amllws.BinaryAudioData(frame); ok {
						if err := writeBinaryMessage(data); err != nil {
							return
						}
					}
				}
			}
		case <-heartbeat.C:
			if err := writeJSONMessage(amllws.Ping()); err != nil {
				return
			}
		}
	}
}

func amllMessageType(message amllws.Message) string {
	if message.Type != "state" {
		return message.Type
	}
	data, err := json.Marshal(message.Value)
	if err != nil {
		return "state"
	}
	var payload struct {
		Update string `json:"update"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Update == "" {
		return "state"
	}
	return "state:" + payload.Update
}

func (s *Server) traceHTTP(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(recorder, r)
		s.state.AddAPIRequest(model.APIRequestTrace{
			ID:         fmt.Sprintf("api-%d", start.UnixNano()),
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     recorder.status,
			DurationMs: time.Since(start).Milliseconds(),
			BytesSent:  recorder.bytes,
			RemoteAddr: r.RemoteAddr,
			Time:       model.FormatTime(start),
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
