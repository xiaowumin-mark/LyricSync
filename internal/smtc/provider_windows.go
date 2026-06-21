//go:build windows && cgo

package smtc

import "lyricsync/internal/core"

func newProvider(state *core.State) Provider {
	return &fallbackProvider{
		primary:  NewNativeProvider(state),
		fallback: NewSimulator(state),
		state:    state,
	}
}
