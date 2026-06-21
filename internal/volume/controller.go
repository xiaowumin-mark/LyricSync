package volume

import (
	"fmt"
	"math"

	"lyricsync/internal/core"
	"lyricsync/pkg/model"
)

type Controller struct {
	state *core.State
}

func NewController(state *core.State) *Controller {
	return &Controller{state: state}
}

func (c *Controller) Set(level float64) error {
	level = math.Max(0, math.Min(1, level))
	if err := setSystemVolume(level); err != nil {
		c.state.AddLog("warn", "volume", fmt.Sprintf("set volume failed: %v", err))
		return err
	}
	playback := c.state.Snapshot().Playback
	playback.Volume = level
	playback.UpdatedAt = model.Now()
	c.state.SetPlayback(playback)
	c.state.AddLog("info", "volume", fmt.Sprintf("system volume set to %d%%", int(math.Round(level*100))))
	return nil
}

func (c *Controller) Refresh() {
	level, err := getSystemVolume()
	if err != nil {
		c.state.AddLog("warn", "volume", "read system volume failed: "+err.Error())
		return
	}
	playback := c.state.Snapshot().Playback
	playback.Volume = level
	playback.UpdatedAt = model.Now()
	c.state.SetPlayback(playback)
}
