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
		model.LyricSourceCustom,
	}

	got := moveLyricPriority(priority, model.LyricSourceQQ, 1)
	want := []string{
		model.LyricSourceTTMLDB,
		model.LyricSourceKugou,
		model.LyricSourceQQ,
		model.LyricSourceNetease,
		model.LyricSourceCustom,
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
		model.LyricSourceCustom,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("move up = %#v, want %#v", got, want)
	}

	got = moveLyricPriority(priority, model.LyricSourceTTMLDB, -1)
	if !reflect.DeepEqual(got, priority) {
		t.Fatalf("move first up = %#v, want unchanged %#v", got, priority)
	}

	got = moveLyricPriority(priority, model.LyricSourceCustom, 1)
	if !reflect.DeepEqual(got, priority) {
		t.Fatalf("move last down = %#v, want unchanged %#v", got, priority)
	}
}

func TestAIApplyWaitSecondsClamp(t *testing.T) {
	if got := aiApplyWaitSeconds(-1); got != 0 {
		t.Fatalf("negative wait = %d, want 0", got)
	}
	if got := aiApplyWaitSeconds(3); got != 3 {
		t.Fatalf("wait = %d, want 3", got)
	}
	if got := aiApplyWaitSeconds(99); got != 15 {
		t.Fatalf("large wait = %d, want 15", got)
	}
}
