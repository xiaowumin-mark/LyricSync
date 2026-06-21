package amllclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"lyricsync/internal/core"
	"lyricsync/pkg/amllws"
	"lyricsync/pkg/model"
)

const endpointName = "amll-client"

type ControlFunc func(command string, positionMs int64) error
type VolumeFunc func(level float64) error

type Connector struct {
	state     *core.State
	control   ControlFunc
	setVolume VolumeFunc

	mu       sync.Mutex
	cancel   context.CancelFunc
	outgoing chan outgoingMessage
}

type outgoingMessage struct {
	jsonMessage *amllws.Message
	binaryData  []byte
	messageType string
}

func NewConnector(state *core.State) *Connector {
	return &Connector{state: state}
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
	outgoing := make(chan outgoingMessage, 128)

	c.mu.Lock()
	c.cancel = cancel
	c.outgoing = outgoing
	c.mu.Unlock()

	c.setStatus(model.AMLLConnection{
		Enabled: true,
		URL:     rawURL,
		Status:  "connecting",
		Message: "connecting to AMLL WebSocket server",
	})
	c.state.SetService("amll-client", "starting", "connecting to "+rawURL)

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
	current := c.state.AMLLConnection()
	if current.Status == "connected" || current.Status == "connecting" {
		current.Enabled = false
		current.Status = "disconnected"
		current.Message = "disconnected"
		c.setStatus(current)
	}
}

func (c *Connector) SendLyrics() error {
	c.mu.Lock()
	outgoing := c.outgoing
	c.mu.Unlock()
	if outgoing == nil {
		return fmt.Errorf("amll: not connected")
	}
	msg := amllws.SetLyric(c.state.Snapshot().Lyrics)
	select {
	case outgoing <- outgoingMessage{jsonMessage: &msg, messageType: "state:setLyric"}:
		return nil
	default:
		return fmt.Errorf("amll: outgoing queue is full")
	}
}

func (c *Connector) run(ctx context.Context, rawURL string, sendAudio bool, outgoing <-chan outgoingMessage) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, response, err := dialer.DialContext(ctx, rawURL, nil)
	if err != nil {
		message := err.Error()
		if response != nil {
			message = fmt.Sprintf("%s (HTTP %d)", message, response.StatusCode)
		}
		c.setStatus(model.AMLLConnection{
			Enabled: true,
			URL:     rawURL,
			Status:  "error",
			Message: message,
		})
		c.state.SetService("amll-client", "error", message)
		c.clearRuntime(outgoing)
		return
	}
	defer conn.Close()

	connectedAt := model.Now()
	c.setStatus(model.AMLLConnection{
		Enabled:     true,
		URL:         rawURL,
		Status:      "connected",
		Message:     "connected",
		ConnectedAt: connectedAt,
	})
	c.state.SetService("amll-client", "running", "connected to "+rawURL)
	c.state.AddWebSocketClient(endpointName)
	defer func() {
		c.state.RemoveWebSocketClient(endpointName)
		c.clearRuntime(outgoing)
		if ctx.Err() == nil {
			current := c.state.AMLLConnection()
			current.Status = "disconnected"
			current.Message = "server closed connection"
			c.setStatus(current)
			c.state.SetService("amll-client", "stopped", "AMLL client disconnected")
		}
	}()

	var writeMu sync.Mutex
	sendJSON := func(message amllws.Message, messageType string) error {
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
		c.recordSend(messageType, len(data), false)
		return nil
	}
	sendBinary := func(data []byte, messageType string) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			return err
		}
		c.recordSend(messageType, len(data), true)
		return nil
	}

	if err := sendJSON(amllws.Initialize(), "initialize"); err != nil {
		c.finishWithError(rawURL, err)
		return
	}
	c.sendSnapshot(sendJSON, sendBinary, sendAudio)

	events, cancelEvents := c.state.Subscribe(128)
	defer cancelEvents()

	readDone := make(chan error, 1)
	go c.readLoop(ctx, conn, readDone, sendJSON)

	ping := time.NewTicker(5 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client disconnect"), time.Now().Add(time.Second))
			return
		case err := <-readDone:
			if err != nil && ctx.Err() == nil {
				c.finishWithError(rawURL, err)
			}
			return
		case event := <-events:
			if err := c.sendEvent(event, sendJSON, sendBinary, sendAudio); err != nil {
				c.finishWithError(rawURL, err)
				return
			}
		case message := <-outgoing:
			if message.jsonMessage != nil {
				if err := sendJSON(*message.jsonMessage, message.messageType); err != nil {
					c.finishWithError(rawURL, err)
					return
				}
			}
			if len(message.binaryData) > 0 {
				if err := sendBinary(message.binaryData, message.messageType); err != nil {
					c.finishWithError(rawURL, err)
					return
				}
			}
		case <-ping.C:
			if err := sendJSON(amllws.Ping(), "ping"); err != nil {
				c.finishWithError(rawURL, err)
				return
			}
		}
	}
}

func (c *Connector) sendSnapshot(sendJSON func(amllws.Message, string) error, sendBinary func([]byte, string) error, sendAudio bool) {
	snapshot := c.state.Snapshot()
	for _, message := range amllws.SnapshotMessages(snapshot) {
		_ = sendJSON(message, amllMessageType(message))
	}
	if sendAudio {
		if data, ok := amllws.BinaryAudioData(snapshot.Audio); ok {
			_ = sendBinary(data, "binary:audioData")
		}
	}
}

func (c *Connector) sendEvent(event model.Event, sendJSON func(amllws.Message, string) error, sendBinary func([]byte, string) error, sendAudio bool) error {
	for _, message := range amllws.EventMessages(event) {
		if err := sendJSON(message, amllMessageType(message)); err != nil {
			return err
		}
	}
	if event.Type == "audio_frame" && sendAudio {
		if frame, ok := event.Payload.(model.AudioFrame); ok {
			if data, ok := amllws.BinaryAudioData(frame); ok {
				if err := sendBinary(data, "binary:audioData"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Connector) readLoop(ctx context.Context, conn *websocket.Conn, done chan<- error, sendJSON func(amllws.Message, string) error) {
	for {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
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
			c.state.AddLog("warn", "amll-client", err.Error())
		}
	}
}

func (c *Connector) handleRemoteMessage(data []byte, sendJSON func(amllws.Message, string) error) error {
	var envelope struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("amll: decode remote message: %w", err)
	}
	current := c.state.AMLLConnection()
	current.LastMessageAt = model.Now()
	if envelope.Type == "ping" {
		c.setStatus(current)
		return sendJSON(amllws.Pong(), "pong")
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
		return fmt.Errorf("amll: decode command: %w", err)
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
		c.state.AddLog("info", "amll-client", "unsupported remote command: "+command)
	}
	return nil
}

func (c *Connector) recordSend(messageType string, bytesSent int, binary bool) {
	current := c.state.AMLLConnection()
	current.LastMessageAt = model.Now()
	current.MessagesSent++
	if binary {
		current.BinaryMessagesSent++
	}
	if bytesSent > 0 {
		current.BytesSent += uint64(bytesSent)
	}
	c.setStatus(current)
	c.state.AddWebSocketSend(endpointName, messageType, bytesSent, binary)
}

func (c *Connector) finishWithError(rawURL string, err error) {
	c.setStatus(model.AMLLConnection{
		Enabled: true,
		URL:     rawURL,
		Status:  "error",
		Message: err.Error(),
	})
	c.state.SetService("amll-client", "error", err.Error())
}

func (c *Connector) clearRuntime(outgoing <-chan outgoingMessage) {
	c.mu.Lock()
	if c.outgoing == outgoing {
		c.outgoing = nil
		c.cancel = nil
	}
	c.mu.Unlock()
}

func (c *Connector) setStatus(connection model.AMLLConnection) {
	if connection.URL == "" {
		connection.URL = c.state.Config().AMLL.URL
	}
	c.state.SetAMLLConnection(connection)
}

func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("amll: invalid WebSocket URL: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("amll: WebSocket URL must start with ws:// or wss://")
	}
	if parsed.Host == "" {
		return fmt.Errorf("amll: WebSocket URL host is required")
	}
	return nil
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
