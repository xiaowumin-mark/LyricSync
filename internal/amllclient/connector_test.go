package amllclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"lyricsync/internal/core"
	"lyricsync/pkg/model"
)

func TestConnectorConnectsToExternalWebSocketAndSendsAudio(t *testing.T) {
	textMessages := make(chan string, 8)
	binaryMessages := make(chan []byte, 4)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			switch messageType {
			case websocket.TextMessage:
				textMessages <- string(data)
			case websocket.BinaryMessage:
				binaryMessages <- append([]byte(nil), data...)
			}
		}
	}))
	defer server.Close()

	cfg := model.DefaultConfig()
	state := core.NewState(cfg)
	state.SetTrack(model.Track{ID: "track-1", Title: "Song", Artist: "Artist", Duration: 90000})
	state.SetAudioFrame(model.AudioFrame{
		Sequence:   1,
		Format:     "f32le",
		SampleRate: 48000,
		Channels:   2,
		PCM: []byte{
			0, 0, 0, 63,
			0, 0, 0, 191,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connector := NewConnector(state)
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	if err := connector.Connect(ctx, url, true); err != nil {
		t.Fatalf("connect AMLL websocket: %v", err)
	}
	defer connector.Disconnect()

	if got := waitTextMessage(t, textMessages); !strings.Contains(got, `"type":"initialize"`) {
		t.Fatalf("expected initialize message, got %s", got)
	}
	if got := waitTextMessage(t, textMessages); !strings.Contains(got, `"update":"setMusic"`) {
		t.Fatalf("expected setMusic message, got %s", got)
	}
	data := waitBinaryMessage(t, binaryMessages)
	if len(data) != 10 {
		t.Fatalf("expected AMLL binary audio payload length 10, got %d: %#v", len(data), data)
	}
	if data[0] != 0 || data[1] != 0 || data[2] != 4 || data[3] != 0 || data[4] != 0 || data[5] != 0 {
		t.Fatalf("unexpected AMLL binary audio header: %#v", data[:6])
	}
}

func waitTextMessage(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case message := <-ch:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for text websocket message")
		return ""
	}
}

func waitBinaryMessage(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case message := <-ch:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for binary websocket message")
		return nil
	}
}
