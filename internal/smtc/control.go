package smtc

import (
	"fmt"
	"time"

	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/control"

	"lyricsync/internal/core"
)

type Controller struct {
	state *core.State
}

func NewController(state *core.State) *Controller {
	return &Controller{state: state}
}

func (c *Controller) Command(command string, positionMs int64) error {
	sessionID := c.state.Snapshot().Track.ID
	if sessionID == "idle" || sessionID == "demo-track" {
		sessionID = ""
	}
	ctrl, err := control.New(sessionID)
	if err != nil {
		return err
	}
	defer ctrl.Close()

	switch command {
	case "play":
		err = ctrl.Play()
	case "pause":
		err = ctrl.Pause()
	case "toggle":
		err = ctrl.TogglePlayPause()
	case "next":
		err = ctrl.Next()
	case "previous":
		err = ctrl.Previous()
	case "seek":
		if positionMs < 0 {
			positionMs = 0
		}
		err = ctrl.Seek(time.Duration(positionMs) * time.Millisecond)
	default:
		err = fmt.Errorf("smtc: unsupported command %q", command)
	}

	if err != nil {
		c.state.AddLog("warn", "smtc-control", command+" failed: "+err.Error())
		return err
	}
	c.state.AddLog("info", "smtc-control", command+" command sent")
	return nil
}
