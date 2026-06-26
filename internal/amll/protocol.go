package amll

import (
	"encoding/binary"
	"math"
	"strings"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const (
	binaryHeaderSize     = 6
	binaryMagicAudioData = 0
	binaryMagicCoverData = 1
)

type Message struct {
	Type  string `json:"type"`
	Value any    `json:"value,omitempty"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MusicInfo struct {
	MusicID   string   `json:"musicId"`
	MusicName string   `json:"musicName"`
	AlbumID   string   `json:"albumId"`
	AlbumName string   `json:"albumName"`
	Artists   []Artist `json:"artists"`
	Duration  uint64   `json:"duration"`
}

type StateUpdate struct {
	Update string `json:"update"`
}

type MusicUpdate struct {
	Update string `json:"update"`
	MusicInfo
}

type ProgressUpdate struct {
	Update   string `json:"update"`
	Progress uint64 `json:"progress"`
}

type VolumeUpdate struct {
	Update string  `json:"update"`
	Volume float64 `json:"volume"`
}

func Initialize() Message {
	return Message{Type: "initialize"}
}

func Ping() Message {
	return Message{Type: "ping"}
}

func Pong() Message {
	return Message{Type: "pong"}
}

func SnapshotMessages(snapshot model.Snapshot) []Message {
	messages := []Message{
		SetMusic(snapshot.Track),
		Progress(snapshot.Playback),
		Volume(snapshot.Playback),
	}
	switch snapshot.Playback.State {
	case "playing":
		messages = append(messages, stateMessage(StateUpdate{Update: "resumed"}))
	case "paused", "stopped":
		messages = append(messages, stateMessage(StateUpdate{Update: "paused"}))
	}
	return messages
}

func EventMessages(event model.Event) []Message {
	switch event.Type {
	case "track_changed":
		if track, ok := event.Payload.(model.Track); ok {
			return []Message{SetMusic(track)}
		}
	case "playback_changed":
		if playback, ok := event.Payload.(model.Playback); ok {
			messages := []Message{Progress(playback), Volume(playback)}
			switch playback.State {
			case "playing":
				messages = append(messages, stateMessage(StateUpdate{Update: "resumed"}))
			case "paused", "stopped":
				messages = append(messages, stateMessage(StateUpdate{Update: "paused"}))
			}
			return messages
		}
	}
	return nil
}

func SetMusic(track model.Track) Message {
	artist := strings.TrimSpace(track.Artist)
	if artist == "" {
		artist = "Unknown Artist"
	}
	return stateMessage(MusicUpdate{
		Update: "setMusic",
		MusicInfo: MusicInfo{
			MusicID:   track.ID,
			MusicName: track.Title,
			AlbumID:   track.Album,
			AlbumName: track.Album,
			Artists:   []Artist{{ID: artist, Name: artist}},
			Duration:  uint64(maxInt64(track.Duration, 0)),
		},
	})
}

func Progress(playback model.Playback) Message {
	return stateMessage(ProgressUpdate{
		Update:   "progress",
		Progress: uint64(maxInt64(playback.Position, 0)),
	})
}

func Volume(playback model.Playback) Message {
	return stateMessage(VolumeUpdate{
		Update: "volume",
		Volume: math.Max(0, math.Min(1, playback.Volume)),
	})
}

func BinaryAudioData(frame model.AudioFrame) ([]byte, bool) {
	pcm := pcmAsS16LE(frame)
	return binaryData(binaryMagicAudioData, pcm)
}

func BinaryCoverData(track model.Track) ([]byte, bool) {
	return binaryData(binaryMagicCoverData, track.CoverData)
}

func MessageType(message Message) string {
	if message.Type != "state" {
		return message.Type
	}
	switch value := message.Value.(type) {
	case MusicUpdate:
		return "state:" + value.Update
	case ProgressUpdate:
		return "state:" + value.Update
	case VolumeUpdate:
		return "state:" + value.Update
	case StateUpdate:
		return "state:" + value.Update
	default:
		return "state"
	}
}

func pcmAsS16LE(frame model.AudioFrame) []byte {
	if len(frame.PCM) == 0 {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(frame.Format)) {
	case "f32le", "float32":
		return f32LEToS16LE(frame.PCM)
	case "s16le", "int16":
		return alignedCopy(frame.PCM, 2)
	case "s24le", "int24":
		return s24LEToS16LE(frame.PCM)
	case "s32le", "int32":
		return s32LEToS16LE(frame.PCM)
	default:
		return append([]byte(nil), frame.PCM...)
	}
}

func f32LEToS16LE(data []byte) []byte {
	count := len(data) / 4
	if count == 0 {
		return nil
	}
	out := make([]byte, count*2)
	for i := 0; i < count; i++ {
		sample := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:])))
		if sample > 1 {
			sample = 1
		}
		if sample < -1 {
			sample = -1
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(sample*32767)))
	}
	return out
}

func s24LEToS16LE(data []byte) []byte {
	count := len(data) / 3
	if count == 0 {
		return nil
	}
	out := make([]byte, count*2)
	for i := 0; i < count; i++ {
		offset := i * 3
		value := int32(data[offset]) | int32(data[offset+1])<<8 | int32(data[offset+2])<<16
		if value&0x800000 != 0 {
			value |= ^0xffffff
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(value>>8)))
	}
	return out
}

func s32LEToS16LE(data []byte) []byte {
	count := len(data) / 4
	if count == 0 {
		return nil
	}
	out := make([]byte, count*2)
	for i := 0; i < count; i++ {
		value := int32(binary.LittleEndian.Uint32(data[i*4:]))
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(value>>16)))
	}
	return out
}

func alignedCopy(data []byte, width int) []byte {
	size := len(data) / width * width
	if size == 0 {
		return nil
	}
	return append([]byte(nil), data[:size]...)
}

func binaryData(magic uint16, payload []byte) ([]byte, bool) {
	if len(payload) == 0 {
		return nil, false
	}
	if uint64(len(payload)) > uint64(^uint32(0)) {
		return nil, false
	}
	data := make([]byte, binaryHeaderSize+len(payload))
	binary.LittleEndian.PutUint16(data[0:2], magic)
	binary.LittleEndian.PutUint32(data[2:6], uint32(len(payload)))
	copy(data[binaryHeaderSize:], payload)
	return data, true
}

func stateMessage(update any) Message {
	return Message{Type: "state", Value: update}
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
