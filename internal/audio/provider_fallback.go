//go:build !windows || !cgo

package audio

import "lyricsync/internal/core"

func newProvider(state *core.State) Provider {
	return NewSimulator(state)
}
