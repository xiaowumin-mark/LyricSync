package amllws

import (
	"bytes"
	"testing"

	"lyricsync/pkg/model"
)

func TestSnapshotMessagesIncludeCoreAMLLUpdates(t *testing.T) {
	snapshot := model.AppSnapshot{
		Track: model.Track{
			ID:       "track-1",
			Title:    "Song",
			Artist:   "Artist",
			Album:    "Album",
			Duration: 123000,
		},
		Playback: model.Playback{
			State:    "playing",
			Position: 4567,
			Volume:   0.8,
		},
		Lyrics: model.LyricDocument{
			Source:   "test",
			Format:   "ttml",
			Language: "zh-CN",
			TTML:     "<tt></tt>",
			Lines: []model.LyricLine{
				{StartMs: 1000, EndMs: 2000, Text: "hello", Translation: "你好"},
			},
		},
	}

	messages := SnapshotMessages(snapshot)
	updates := map[string]bool{}
	for _, message := range messages {
		if message.Type != "state" {
			continue
		}
		switch value := message.Value.(type) {
		case MusicUpdate:
			updates[value.Update] = true
			if value.MusicName != "Song" {
				t.Fatalf("expected music title Song, got %q", value.MusicName)
			}
		case LyricUpdate:
			updates[value.Update] = true
			if value.Format != "ttml" || value.Data == "" {
				t.Fatalf("unexpected lyric payload: %#v", value)
			}
		case ProgressUpdate:
			updates[value.Update] = true
		case VolumeUpdate:
			updates[value.Update] = true
		case StateUpdate:
			updates[value.Update] = true
		}
	}

	for _, key := range []string{"setMusic", "setLyric", "progress", "volume", "resumed"} {
		if !updates[key] {
			t.Fatalf("missing AMLL update %q in %#v", key, updates)
		}
	}
}

func TestEventMessagesForPlayback(t *testing.T) {
	messages := EventMessages(model.Event{
		Type: "playback_changed",
		Payload: model.Playback{
			State:    "paused",
			Position: 9000,
			Volume:   0.42,
		},
	})

	if len(messages) != 3 {
		t.Fatalf("expected progress, volume and paused messages, got %d", len(messages))
	}
	if messages[0].Value.(ProgressUpdate).Progress != 9000 {
		t.Fatalf("unexpected progress payload: %#v", messages[0].Value)
	}
	if messages[1].Value.(VolumeUpdate).Volume != 0.42 {
		t.Fatalf("unexpected volume payload: %#v", messages[1].Value)
	}
	if messages[2].Value.(StateUpdate).Update != "paused" {
		t.Fatalf("unexpected state payload: %#v", messages[2].Value)
	}
}

func TestBinaryAudioDataEmptyPCM(t *testing.T) {
	data, ok := BinaryAudioData(model.AudioFrame{})
	if ok {
		t.Fatalf("expected empty PCM to be skipped, got %#v", data)
	}
	if data != nil {
		t.Fatalf("expected nil payload for empty PCM, got %#v", data)
	}
}

func TestBinaryAudioDataEncodesAMLLV2AudioPayload(t *testing.T) {
	data, ok := BinaryAudioData(model.AudioFrame{PCM: []byte{1, 2, 3}})
	if !ok {
		t.Fatal("expected binary audio payload")
	}

	want := []byte{0, 0, 3, 0, 0, 0, 1, 2, 3}
	if !bytes.Equal(data, want) {
		t.Fatalf("unexpected binary payload: got %#v want %#v", data, want)
	}
}

func TestBinaryAudioDataConvertsFloat32PCMToS16LE(t *testing.T) {
	data, ok := BinaryAudioData(model.AudioFrame{
		Format: "f32le",
		PCM: []byte{
			0, 0, 0, 0,
			0, 0, 0, 63,
			0, 0, 128, 191,
			0, 0, 0, 64,
		},
	})
	if !ok {
		t.Fatal("expected converted binary audio payload")
	}

	want := []byte{0, 0, 8, 0, 0, 0, 0, 0, 255, 63, 1, 128, 255, 127}
	if !bytes.Equal(data, want) {
		t.Fatalf("unexpected converted binary payload: got %#v want %#v", data, want)
	}
}
