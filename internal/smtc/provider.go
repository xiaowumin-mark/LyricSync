package smtc

import (
	"context"

	"lyricsync/internal/core"
)

type Provider interface {
	Start(ctx context.Context) error
}

func NewProvider(state *core.State) Provider {
	return newProvider(state)
}

type fallbackProvider struct {
	primary  Provider
	fallback Provider
	state    *core.State
}

func (p *fallbackProvider) Start(ctx context.Context) error {
	if err := p.primary.Start(ctx); err != nil {
		p.state.AddLog("warn", "smtc", "native SMTC provider unavailable: "+err.Error())
		p.state.SetService("smtc", "degraded", "native provider unavailable; using simulator")
		return p.fallback.Start(ctx)
	}
	return nil
}
