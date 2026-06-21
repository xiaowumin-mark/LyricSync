//go:build !windows || !cgo

package smtc

import "lyricsync/internal/core"

func newProvider(state *core.State) Provider {
	return NewSimulator(state)
}
