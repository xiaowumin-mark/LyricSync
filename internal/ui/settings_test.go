package ui

import (
	"reflect"
	"testing"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestMoveLyricPriorityMovesByDirection(t *testing.T) {
	priority := []string{
		model.LyricSourceTTMLDB,
		model.LyricSourceQQ,
		model.LyricSourceKugou,
		model.LyricSourceNetease,
	}

	got := moveLyricPriority(priority, model.LyricSourceQQ, 1)
	want := []string{
		model.LyricSourceTTMLDB,
		model.LyricSourceKugou,
		model.LyricSourceQQ,
		model.LyricSourceNetease,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("move down = %#v, want %#v", got, want)
	}

	got = moveLyricPriority(priority, model.LyricSourceKugou, -1)
	want = []string{
		model.LyricSourceTTMLDB,
		model.LyricSourceKugou,
		model.LyricSourceQQ,
		model.LyricSourceNetease,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("move up = %#v, want %#v", got, want)
	}

	got = moveLyricPriority(priority, model.LyricSourceTTMLDB, -1)
	if !reflect.DeepEqual(got, priority) {
		t.Fatalf("move first up = %#v, want unchanged %#v", got, priority)
	}

	got = moveLyricPriority(priority, model.LyricSourceNetease, 1)
	if !reflect.DeepEqual(got, priority) {
		t.Fatalf("move last down = %#v, want unchanged %#v", got, priority)
	}
}
