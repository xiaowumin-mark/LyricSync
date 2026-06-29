package amll

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
)

type ControlFunc func(command string, positionMs int64) error
type VolumeFunc func(level float64) error

const (
	progressFrameInterval = time.Second / 60
	amllPingInterval      = 30 * time.Second
	amllWriteTimeout      = 5 * time.Second
)

type Connector struct {
	store     *state.Store
	control   ControlFunc
	setVolume VolumeFunc

	mu       sync.Mutex
	cancel   context.CancelFunc
	outgoing chan outbound
}

type outbound struct {
	jsonMessage *Message
	binaryData  []byte
	messageType string
}

func NewConnector(store *state.Store) *Connector {
	return &Connector{store: store}
}

func (c *Connector) SetCommandHandlers(control ControlFunc, setVolume VolumeFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.control = control
	c.setVolume = setVolume
}

func (c *Connector) Connect(parent context.Context, rawURL string, sendAudio bool) error {
	rawURL = strings.TrimSpace(rawURL)
	if err := validateURL(rawURL); err != nil {
		return err
	}

	c.Disconnect()
	ctx, cancel := context.WithCancel(parent)
	outgoing := make(chan outbound, 128)

	c.mu.Lock()
	c.cancel = cancel
	c.outgoing = outgoing
	c.mu.Unlock()

	c.setStatus(model.AMLLConnection{
		Enabled: true,
		URL:     rawURL,
		Status:  "connecting",
		Message: "connecting",
	})
	c.store.AddLog("AMLL connecting to " + rawURL)

	go c.run(ctx, rawURL, sendAudio, outgoing)
	return nil
}

func (c *Connector) Disconnect() {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.outgoing = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	current := c.store.AMLL()
	if current.Status == "connected" || current.Status == "connecting" {
		current.Enabled = false
		current.Status = "disconnected"
		current.Message = "disconnected"
		c.setStatus(current)
	}
}

func (c *Connector) SendSnapshot() error {
	c.mu.Lock()
	outgoing := c.outgoing
	c.mu.Unlock()
	if outgoing == nil {
		return fmt.Errorf("amll: not connected")
	}
	snapshot := c.store.Snapshot()
	for _, message := range SnapshotMessages(snapshot) {
		select {
		case outgoing <- outbound{jsonMessage: &message, messageType: MessageType(message)}:
		default:
			return fmt.Errorf("amll: outgoing queue is full")
		}
	}
	if data, ok := BinaryCoverData(snapshot.Track); ok {
		select {
		case outgoing <- outbound{binaryData: data, messageType: "binary:coverData"}:
		default:
			return fmt.Errorf("amll: outgoing queue is full")
		}
	}
	return nil
}

func (c *Connector) SendLyricTTML(ttmlText string) error {
	c.mu.Lock()
	outgoing := c.outgoing
	c.mu.Unlock()
	if outgoing == nil {
		return fmt.Errorf("amll: not connected")
	}
	message := SetLyricTTML(ttmlText)
	select {
	case outgoing <- outbound{jsonMessage: &message, messageType: MessageType(message)}:
		return nil
	default:
		return fmt.Errorf("amll: outgoing queue is full")
	}
}

func (c *Connector) run(ctx context.Context, rawURL string, sendAudio bool, outgoing <-chan outbound) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, response, err := dialer.DialContext(ctx, rawURL, nil)
	if err != nil {
		message := err.Error()
		if response != nil {
			message = fmt.Sprintf("%s (HTTP %d)", message, response.StatusCode)
		}
		c.finishWithError(rawURL, message)
		c.clearRuntime(outgoing)
		return
	}
	defer conn.Close()

	c.setStatus(model.AMLLConnection{
		Enabled:     true,
		URL:         rawURL,
		Status:      "connected",
		Message:     "connected",
		ConnectedAt: model.Now(),
	})
	c.store.AddLog("AMLL connected")

	var writeMu sync.Mutex
	sendJSON := func(message Message, messageType string) error {
		data, err := json.Marshal(message)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(amllWriteTimeout))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
		c.recordSend(messageType, len(data), false)
		return nil
	}
	sendBinary := func(data []byte, messageType string) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(amllWriteTimeout))
		if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			return err
		}
		c.recordSend(messageType, len(data), true)
		return nil
	}

	if err := sendJSON(Initialize(), "initialize"); err != nil {
		c.finishWithError(rawURL, err.Error())
		return
	}
	c.sendSnapshot(sendJSON, sendBinary, sendAudio)

	events, cancelEvents := c.store.Subscribe(128)
	defer cancelEvents()

	readDone := make(chan error, 1)
	go c.readLoop(ctx, conn, readDone, sendJSON)

	ping := time.NewTicker(amllPingInterval)
	defer ping.Stop()
	progress := time.NewTicker(progressFrameInterval)
	defer progress.Stop()
	var lastProgress uint64
	var hasLastProgress bool

	defer func() {
		c.clearRuntime(outgoing)
		if ctx.Err() == nil {
			current := c.store.AMLL()
			current.Status = "disconnected"
			current.Message = "server closed connection"
			c.setStatus(current)
			c.store.AddLog("AMLL disconnected")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client disconnect"), time.Now().Add(time.Second))
			return
		case err := <-readDone:
			if err != nil && ctx.Err() == nil {
				c.finishWithError(rawURL, err.Error())
			}
			return
		case event := <-events:
			if err := c.sendEvent(event, sendJSON, sendBinary, sendAudio); err != nil {
				c.finishWithError(rawURL, err.Error())
				return
			}
		case message, ok := <-outgoing:
			if !ok {
				return
			}
			if message.jsonMessage != nil {
				if err := sendJSON(*message.jsonMessage, message.messageType); err != nil {
					c.finishWithError(rawURL, err.Error())
					return
				}
			}
			if len(message.binaryData) > 0 {
				if err := sendBinary(message.binaryData, message.messageType); err != nil {
					c.finishWithError(rawURL, err.Error())
					return
				}
			}
		case <-ping.C:
			if err := sendJSON(Ping(), "ping"); err != nil {
				c.finishWithError(rawURL, err.Error())
				return
			}
		case <-progress.C:
			playback := c.store.Playback()
			if !strings.EqualFold(playback.State, "playing") {
				hasLastProgress = false
				continue
			}
			delayMs := c.store.LyricDelayMs()
			currentProgress := uint64(maxInt64(playback.Position-delayMs, 0))
			if hasLastProgress && currentProgress == lastProgress {
				continue
			}
			message := ProgressWithLyricDelay(playback, delayMs)
			if err := sendJSON(message, MessageType(message)); err != nil {
				c.finishWithError(rawURL, err.Error())
				return
			}
			lastProgress = currentProgress
			hasLastProgress = true
		}
	}
}

func (c *Connector) sendSnapshot(sendJSON func(Message, string) error, sendBinary func([]byte, string) error, sendAudio bool) {
	snapshot := c.store.Snapshot()
	for _, message := range SnapshotMessages(snapshot) {
		_ = sendJSON(message, MessageType(message))
	}
	if data, ok := BinaryCoverData(snapshot.Track); ok {
		_ = sendBinary(data, "binary:coverData")
	}
	if sendAudio {
		if data, ok := BinaryAudioData(snapshot.Audio); ok {
			_ = sendBinary(data, "binary:audioData")
		}
	}
}

func (c *Connector) sendEvent(event model.Event, sendJSON func(Message, string) error, sendBinary func([]byte, string) error, sendAudio bool) error {
	for _, message := range EventMessagesWithLyricDelay(event, c.store.LyricDelayMs()) {
		if err := sendJSON(message, MessageType(message)); err != nil {
			return err
		}
	}
	if event.Type == "audio_frame" && sendAudio {
		if frame, ok := event.Payload.(model.AudioFrame); ok {
			if data, ok := BinaryAudioData(frame); ok {
				if err := sendBinary(data, "binary:audioData"); err != nil {
					return err
				}
			}
		}
	}
	if event.Type == "track_changed" {
		if track, ok := event.Payload.(model.Track); ok {
			if data, ok := BinaryCoverData(track); ok {
				if err := sendBinary(data, "binary:coverData"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Connector) readLoop(ctx context.Context, conn *websocket.Conn, done chan<- error, sendJSON func(Message, string) error) {
	// Many AMLL-compatible local receivers do not implement application-level
	// ping/pong. Do not use a read deadline as a heartbeat requirement; writes
	// or an actual socket close will still surface connection failures.
	_ = conn.SetReadDeadline(time.Time{})
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				done <- nil
				return
			}
			done <- err
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		if err := c.handleRemoteMessage(data, sendJSON); err != nil {
			c.store.AddLog("AMLL command ignored: " + err.Error())
		}
	}
}

func (c *Connector) handleRemoteMessage(data []byte, sendJSON func(Message, string) error) error {
	var envelope struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode message: %w", err)
	}
	current := c.store.AMLL()
	current.LastMessageAt = model.Now()
	if envelope.Type == "ping" {
		c.setStatus(current)
		return sendJSON(Pong(), "pong")
	}
	if envelope.Type == "pong" {
		current.Message = "pong received"
		c.setStatus(current)
		return nil
	}
	if envelope.Type != "command" {
		c.setStatus(current)
		return nil
	}

	var command struct {
		Command  string   `json:"command"`
		Progress *uint64  `json:"progress,omitempty"`
		Volume   *float64 `json:"volume,omitempty"`
	}
	if err := json.Unmarshal(envelope.Value, &command); err != nil {
		return fmt.Errorf("decode command: %w", err)
	}
	return c.handleCommand(command.Command, command.Progress, command.Volume)
}

func (c *Connector) handleCommand(command string, progress *uint64, volume *float64) error {
	normalized := strings.ToLower(strings.TrimSpace(command))
	c.mu.Lock()
	control := c.control
	setVolume := c.setVolume
	c.mu.Unlock()

	switch normalized {
	case "pause":
		if control != nil {
			return control("pause", 0)
		}
	case "resume", "play":
		if control != nil {
			return control("play", 0)
		}
	case "forwardsong", "next":
		if control != nil {
			return control("next", 0)
		}
	case "backwardsong", "previous":
		if control != nil {
			return control("previous", 0)
		}
	case "seekplayprogress", "seek":
		if control != nil && progress != nil {
			return control("seek", int64(*progress))
		}
	case "setvolume":
		if setVolume != nil && volume != nil {
			return setVolume(*volume)
		}
	default:
		c.store.AddLog("AMLL unsupported command: " + command)
	}
	return nil
}

func (c *Connector) recordSend(messageType string, bytesSent int, binary bool) {
	current := c.store.AMLL()
	current.LastMessageAt = model.Now()
	current.MessagesSent++
	if binary {
		current.BinaryMessagesSent++
	}
	if bytesSent > 0 {
		current.BytesSent += uint64(bytesSent)
	}
	c.setStatus(current)
}

func (c *Connector) finishWithError(rawURL, message string) {
	c.setStatus(model.AMLLConnection{
		Enabled: true,
		URL:     rawURL,
		Status:  "error",
		Message: message,
	})
	c.store.AddLog("AMLL error: " + message)
}

func (c *Connector) clearRuntime(outgoing <-chan outbound) {
	c.mu.Lock()
	if c.outgoing == outgoing {
		c.outgoing = nil
		c.cancel = nil
	}
	c.mu.Unlock()
}

func (c *Connector) setStatus(connection model.AMLLConnection) {
	if connection.URL == "" {
		connection.URL = c.store.Config().AMLL.URL
	}
	c.store.SetAMLL(connection)
}

func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("WebSocket URL must start with ws:// or wss://")
	}
	if parsed.Host == "" {
		return fmt.Errorf("WebSocket URL host is required")
	}
	return nil
}
