package core

import (
	"testing"

	"lyricsync/pkg/model"
)

func TestSnapshotCopiesClientsAndLogs(t *testing.T) {
	state := NewState(model.DefaultConfig())
	state.AddClient(model.WSClient{ID: "a", Name: "client-a", ConnectedAt: "2026-01-01T00:00:00Z"})
	state.AddLog("info", "test", "hello")

	first := state.Snapshot()
	if len(first.Clients) != 1 {
		t.Fatalf("expected one client, got %d", len(first.Clients))
	}
	if len(first.Logs) != 1 {
		t.Fatalf("expected one log, got %d", len(first.Logs))
	}

	first.Clients[0].Name = "mutated"
	first.Logs[0].Message = "mutated"

	second := state.Snapshot()
	if second.Clients[0].Name != "client-a" {
		t.Fatalf("snapshot mutation leaked into state: %#v", second.Clients[0])
	}
	if second.Logs[0].Message != "hello" {
		t.Fatalf("log mutation leaked into state: %#v", second.Logs[0])
	}
}

func TestSubscribeReceivesEvents(t *testing.T) {
	state := NewState(model.DefaultConfig())
	events, cancel := state.Subscribe(1)
	defer cancel()

	state.SetTrack(model.Track{ID: "track", Title: "Song"})

	select {
	case event := <-events:
		if event.Type != "song_changed" {
			t.Fatalf("expected song_changed event, got %q", event.Type)
		}
	default:
		t.Fatal("expected event from subscription")
	}
}

func TestWebSocketStatsTrackClientsAndSends(t *testing.T) {
	state := NewState(model.DefaultConfig())
	state.AddWebSocketClient("/ws")
	state.AddWebSocketSend("/ws", "state", 128, false)
	state.AddWebSocketSend("/ws", "binary:audioData", 64, true)
	state.RemoveWebSocketClient("/ws")

	snapshot := state.Snapshot()
	if len(snapshot.Metrics.WebSockets) != 1 {
		t.Fatalf("expected one websocket stats entry, got %#v", snapshot.Metrics.WebSockets)
	}
	stats := snapshot.Metrics.WebSockets[0]
	if stats.Endpoint != "/ws" {
		t.Fatalf("unexpected endpoint: %#v", stats)
	}
	if stats.ActiveClients != 0 {
		t.Fatalf("expected no active clients, got %#v", stats)
	}
	if stats.MessagesSent != 2 || stats.BinaryMessagesSent != 1 || stats.BytesSent != 192 {
		t.Fatalf("unexpected counters: %#v", stats)
	}
	if stats.LastMessageType != "binary:audioData" {
		t.Fatalf("unexpected last message type: %#v", stats)
	}
	if stats.MessagesPerMinute <= 0 {
		t.Fatalf("expected non-zero message rate: %#v", stats)
	}
	if len(snapshot.Metrics.WebSocketMessages) != 2 {
		t.Fatalf("expected websocket message history, got %#v", snapshot.Metrics.WebSocketMessages)
	}
	if snapshot.Metrics.WebSocketMessages[1].MessageType != "binary:audioData" || !snapshot.Metrics.WebSocketMessages[1].Binary {
		t.Fatalf("unexpected message history: %#v", snapshot.Metrics.WebSocketMessages)
	}
	if len(snapshot.Metrics.WebSocketSeries) != 1 {
		t.Fatalf("expected one aggregated series point, got %#v", snapshot.Metrics.WebSocketSeries)
	}
	series := snapshot.Metrics.WebSocketSeries[0]
	if series.Messages != 2 || series.BinaryMessages != 1 || series.BytesSent != 192 {
		t.Fatalf("unexpected series point: %#v", series)
	}
}

func TestAPIRequestTraceAndPerformanceStats(t *testing.T) {
	state := NewState(model.DefaultConfig())
	state.AddAPIRequest(model.APIRequestTrace{
		ID:         "req-1",
		Method:     "GET",
		Path:       "/api/state",
		Status:     200,
		DurationMs: 12,
		BytesSent:  512,
		RemoteAddr: "127.0.0.1:10000",
		Time:       model.Now(),
	})
	state.AddAPIRequest(model.APIRequestTrace{
		ID:         "req-2",
		Method:     "GET",
		Path:       "/api/error",
		Status:     500,
		DurationMs: 5,
		BytesSent:  64,
		Time:       model.Now(),
	})

	snapshot := state.Snapshot()
	if len(snapshot.Metrics.API) != 2 {
		t.Fatalf("expected two API traces, got %#v", snapshot.Metrics.API)
	}
	perf := snapshot.Metrics.Performance
	if perf.HTTPRequests != 2 || perf.HTTPErrors != 1 || perf.HTTPBytesSent != 576 {
		t.Fatalf("unexpected performance counters: %#v", perf)
	}
	if perf.MemorySysBytes == 0 || perf.Goroutines == 0 {
		t.Fatalf("expected runtime counters: %#v", perf)
	}
}

func TestAudioHistoryIsCappedAndCopied(t *testing.T) {
	state := NewState(model.DefaultConfig())
	for i := 0; i < 605; i++ {
		state.SetAudioFrame(model.AudioFrame{
			Sequence:   uint64(i + 1),
			TrackID:    "track",
			PositionMs: int64(i * 50),
			DurationMs: 50,
			RMS:        0.1,
			Peak:       0.2,
		})
	}

	history := state.AudioHistory()
	if len(history) != 600 {
		t.Fatalf("expected capped audio history, got %d", len(history))
	}
	if history[0].Sequence != 6 {
		t.Fatalf("expected oldest retained frame sequence 6, got %d", history[0].Sequence)
	}

	history[0].RMS = 1
	next := state.AudioHistory()
	if next[0].RMS == 1 {
		t.Fatal("audio history mutation leaked into state")
	}

	snapshot := state.Snapshot()
	if len(snapshot.Metrics.AudioEnergy) != 600 {
		t.Fatalf("expected audio energy metrics, got %d", len(snapshot.Metrics.AudioEnergy))
	}
}
