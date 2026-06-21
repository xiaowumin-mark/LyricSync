package amllws

import (
	"encoding/binary"
	"math"
	"strings"

	"lyricsync/pkg/model"
)

type Message struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value,omitempty"`
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

type CoverUpdate struct {
	Update string `json:"update"`
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
}

type LyricUpdate struct {
	Update string      `json:"update"`
	Format string      `json:"format"`
	Lines  []LyricLine `json:"lines,omitempty"`
	Data   string      `json:"data,omitempty"`
	Meta   LyricMeta   `json:"meta"`
}

type LyricMeta struct {
	Source   string `json:"source"`
	Language string `json:"language"`
}

type LyricLine struct {
	StartTime       uint64      `json:"startTime"`
	EndTime         uint64      `json:"endTime"`
	Words           []LyricWord `json:"words"`
	TranslatedLyric string      `json:"translatedLyric"`
	RomanLyric      string      `json:"romanLyric"`
	Flag            uint8       `json:"flag"`
}

type LyricWord struct {
	StartTime uint64 `json:"startTime"`
	EndTime   uint64 `json:"endTime"`
	Word      string `json:"word"`
}

type ProgressUpdate struct {
	Update   string `json:"update"`
	Progress uint64 `json:"progress"`
}

type VolumeUpdate struct {
	Update string  `json:"update"`
	Volume float64 `json:"volume"`
}

type AudioDataUpdate struct {
	Update   string         `json:"update"`
	Data     []int          `json:"data"`
	Features *AudioFeatures `json:"features,omitempty"`
}

type AudioFeatures struct {
	Sequence     uint64    `json:"sequence"`
	PositionMs   int64     `json:"positionMs"`
	SampleRate   int       `json:"sampleRate"`
	Channels     int       `json:"channels"`
	Format       string    `json:"format"`
	DurationMs   int       `json:"durationMs"`
	RMS          float64   `json:"rms"`
	Peak         float64   `json:"peak"`
	Spectrum     []float64 `json:"spectrum,omitempty"`
	ProviderMode string    `json:"providerMode"`
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

func SnapshotMessages(snapshot model.AppSnapshot) []Message {
	messages := []Message{
		SetMusic(snapshot.Track),
		SetLyric(snapshot.Lyrics),
		Progress(snapshot.Playback),
		Volume(snapshot.Playback),
	}
	if snapshot.Track.Artwork != "" {
		messages = append(messages, SetCover(snapshot.Track))
	}
	if snapshot.Playback.State == "paused" {
		messages = append(messages, state(StateUpdate{Update: "paused"}))
	} else if snapshot.Playback.State == "playing" {
		messages = append(messages, state(StateUpdate{Update: "resumed"}))
	}
	return messages
}

func EventMessages(event model.Event) []Message {
	switch event.Type {
	case "song_changed":
		if track, ok := event.Payload.(model.Track); ok {
			return []Message{SetMusic(track)}
		}
	case "lyric_changed":
		if lyrics, ok := event.Payload.(model.LyricDocument); ok {
			return []Message{SetLyric(lyrics)}
		}
	case "playback_changed":
		if playback, ok := event.Payload.(model.Playback); ok {
			messages := []Message{Progress(playback), Volume(playback)}
			if playback.State == "paused" {
				messages = append(messages, state(StateUpdate{Update: "paused"}))
			}
			if playback.State == "playing" {
				messages = append(messages, state(StateUpdate{Update: "resumed"}))
			}
			return messages
		}
	}
	return nil
}

func SetMusic(track model.Track) Message {
	artist := track.Artist
	if artist == "" {
		artist = "Unknown Artist"
	}
	return state(MusicUpdate{
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

func SetCover(track model.Track) Message {
	return state(CoverUpdate{
		Update: "setCover",
		Source: "uri",
		URL:    track.Artwork,
	})
}

func SetLyric(lyrics model.LyricDocument) Message {
	if lyrics.TTML != "" {
		return state(LyricUpdate{
			Update: "setLyric",
			Format: "ttml",
			Data:   lyrics.TTML,
			Meta: LyricMeta{
				Source:   lyrics.Source,
				Language: lyrics.Language,
			},
		})
	}

	lines := make([]LyricLine, 0, len(lyrics.Lines))
	for _, line := range lyrics.Lines {
		lines = append(lines, LyricLine{
			StartTime:       uint64(maxInt64(line.StartMs, 0)),
			EndTime:         uint64(maxInt64(line.EndMs, 0)),
			TranslatedLyric: line.Translation,
			RomanLyric:      line.Romanization,
			Words: []LyricWord{
				{
					StartTime: uint64(maxInt64(line.StartMs, 0)),
					EndTime:   uint64(maxInt64(line.EndMs, 0)),
					Word:      line.Text,
				},
			},
		})
	}
	return state(LyricUpdate{
		Update: "setLyric",
		Format: "structured",
		Lines:  lines,
		Meta: LyricMeta{
			Source:   lyrics.Source,
			Language: lyrics.Language,
		},
	})
}

func Progress(playback model.Playback) Message {
	return state(ProgressUpdate{
		Update:   "progress",
		Progress: uint64(maxInt64(playback.Position, 0)),
	})
}

func Volume(playback model.Playback) Message {
	return state(VolumeUpdate{
		Update: "volume",
		Volume: playback.Volume,
	})
}

func AudioData(frame model.AudioFrame) Message {
	return state(AudioDataUpdate{
		Update: "audioData",
		Data:   []int{},
		Features: &AudioFeatures{
			Sequence:     frame.Sequence,
			PositionMs:   frame.PositionMs,
			SampleRate:   frame.SampleRate,
			Channels:     frame.Channels,
			Format:       frame.Format,
			DurationMs:   frame.DurationMs,
			RMS:          frame.RMS,
			Peak:         frame.Peak,
			Spectrum:     frame.Spectrum,
			ProviderMode: frame.ProviderMode,
		},
	})
}

func BinaryAudioData(frame model.AudioFrame) ([]byte, bool) {
	pcm := pcmAsS16LE(frame)
	if len(pcm) == 0 {
		return nil, false
	}
	if uint64(len(pcm)) > uint64(^uint32(0)) {
		return nil, false
	}
	data := make([]byte, 2+4+len(pcm))
	binary.LittleEndian.PutUint16(data[0:2], 0)
	binary.LittleEndian.PutUint32(data[2:6], uint32(len(pcm)))
	copy(data[6:], pcm)
	return data, true
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
		value := int16(sample * 32767)
		binary.LittleEndian.PutUint16(out[i*2:], uint16(value))
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
	if width <= 0 {
		return append([]byte(nil), data...)
	}
	size := len(data) / width * width
	if size == 0 {
		return nil
	}
	return append([]byte(nil), data[:size]...)
}

func state(update interface{}) Message {
	return Message{
		Type:  "state",
		Value: update,
	}
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
