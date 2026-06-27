package amll

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestStateMessageJSONMatchesAMLLV2Shape(t *testing.T) {
	data, err := json.Marshal(SetMusic(model.Track{
		ID:       "track-1",
		Title:    "Song",
		Artist:   "Artist",
		Album:    "Album",
		Duration: 123000,
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"state","value":{"update":"setMusic","musicId":"track-1","musicName":"Song","albumId":"Album","albumName":"Album","artists":[{"id":"Artist","name":"Artist"}],"duration":123000}}`
	if string(data) != want {
		t.Fatalf("unexpected JSON:\n%s\nwant:\n%s", data, want)
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
		t.Fatal("expected binary audio data")
	}
	want := []byte{0, 0, 8, 0, 0, 0, 0, 0, 255, 63, 1, 128, 255, 127}
	if !bytes.Equal(data, want) {
		t.Fatalf("got %#v want %#v", data, want)
	}
}

func TestBinaryCoverDataUsesCoverMagic(t *testing.T) {
	data, ok := BinaryCoverData(model.Track{CoverData: []byte{0x89, 'P', 'N', 'G'}})
	if !ok {
		t.Fatal("expected binary cover data")
	}
	want := []byte{1, 0, 4, 0, 0, 0, 0x89, 'P', 'N', 'G'}
	if !bytes.Equal(data, want) {
		t.Fatalf("got %#v want %#v", data, want)
	}
}

func TestBinaryCoverDataSkipsEmptyCover(t *testing.T) {
	if data, ok := BinaryCoverData(model.Track{}); ok || data != nil {
		t.Fatalf("expected empty cover to be skipped, got %#v", data)
	}
}

func TestSetLyricTTMLJSONMatchesAMLLV2Shape(t *testing.T) {
	data, err := json.Marshal(SetLyricTTML("<tt></tt>"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"state","value":{"update":"setLyric","format":"ttml","data":"\u003ctt\u003e\u003c/tt\u003e"}}`
	if string(data) != want {
		t.Fatalf("unexpected JSON:\n%s\nwant:\n%s", data, want)
	}
}

func TestSnapshotMessagesIncludeCurrentLyric(t *testing.T) {
	messages := SnapshotMessages(model.Snapshot{
		Track: model.Track{ID: "track-1"},
		Lyrics: model.CurrentLyrics{
			TrackID: "track-1",
			TTML:    "<tt></tt>",
		},
	})
	found := false
	for _, message := range messages {
		if MessageType(message) == "state:setLyric" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected snapshot messages to include setLyric, got %#v", messages)
	}
}
