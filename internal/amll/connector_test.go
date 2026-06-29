package amll

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/xiaowumin-mark/LyricSync/internal/config"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
)

func TestConnectorSendsInitializeStateAndBinaryAudio(t *testing.T) {
	textMessages := make(chan string, 8)
	binaryMessages := make(chan []byte, 4)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if messageType == websocket.TextMessage {
				textMessages <- string(data)
			}
			if messageType == websocket.BinaryMessage {
				binaryMessages <- append([]byte(nil), data...)
			}
		}
	}))
	defer server.Close()

	store := state.New(config.Default())
	store.SetTrack(model.Track{ID: "track-1", Title: "Song", Artist: "Artist", Duration: 120000})
	store.SetAudio(model.AudioFrame{
		Sequence: 1,
		Format:   "f32le",
		PCM: []byte{
			0, 0, 0, 63,
			0, 0, 0, 191,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connector := NewConnector(store)
	if err := connector.Connect(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), true); err != nil {
		t.Fatal(err)
	}
	defer connector.Disconnect()

	if got := waitText(t, textMessages); !strings.Contains(got, `"type":"initialize"`) {
		t.Fatalf("expected initialize, got %s", got)
	}
	if got := waitText(t, textMessages); !strings.Contains(got, `"update":"setMusic"`) {
		t.Fatalf("expected setMusic, got %s", got)
	}
	data := waitBinary(t, binaryMessages)
	if len(data) != 10 {
		t.Fatalf("expected binary payload length 10, got %d", len(data))
	}
	if data[0] != 0 || data[1] != 0 || data[2] != 4 || data[3] != 0 || data[4] != 0 || data[5] != 0 {
		t.Fatalf("unexpected binary header: %#v", data[:6])
	}
}

func TestConnectorSendsBinaryCoverWithoutAudio(t *testing.T) {
	textMessages := make(chan string, 8)
	binaryMessages := make(chan []byte, 4)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if messageType == websocket.TextMessage {
				textMessages <- string(data)
			}
			if messageType == websocket.BinaryMessage {
				binaryMessages <- append([]byte(nil), data...)
			}
		}
	}))
	defer server.Close()

	store := state.New(config.Default())
	store.SetTrack(model.Track{
		ID:        "track-1",
		Title:     "Song",
		Artist:    "Artist",
		Duration:  120000,
		CoverData: []byte{0x89, 'P', 'N', 'G'},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connector := NewConnector(store)
	if err := connector.Connect(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), false); err != nil {
		t.Fatal(err)
	}
	defer connector.Disconnect()

	if got := waitText(t, textMessages); !strings.Contains(got, `"type":"initialize"`) {
		t.Fatalf("expected initialize, got %s", got)
	}
	if got := waitText(t, textMessages); !strings.Contains(got, `"update":"setMusic"`) {
		t.Fatalf("expected setMusic, got %s", got)
	}
	data := waitBinary(t, binaryMessages)
	want := []byte{1, 0, 4, 0, 0, 0, 0x89, 'P', 'N', 'G'}
	if string(data) != string(want) {
		t.Fatalf("got %#v want %#v", data, want)
	}
}

func TestConnectorDoesNotRequireRemotePong(t *testing.T) {
	textMessages := make(chan string, 16)
	closed := make(chan struct{})
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer close(closed)
		defer conn.Close()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if messageType == websocket.TextMessage {
				textMessages <- string(data)
			}
		}
	}))
	defer server.Close()

	store := state.New(config.Default())
	ctx, cancel := context.WithCancel(context.Background())
	connector := NewConnector(store)
	if err := connector.Connect(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), false); err != nil {
		t.Fatal(err)
	}
	defer connector.Disconnect()

	if got := waitText(t, textMessages); !strings.Contains(got, `"type":"initialize"`) {
		t.Fatalf("expected initialize, got %s", got)
	}
	select {
	case <-closed:
		t.Fatal("connection closed even though remote simply did not send pong")
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("connection did not close after context cancellation")
	}
}

func waitText(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for text message")
		return ""
	}
}

func waitBinary(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for binary message")
		return nil
	}
}
