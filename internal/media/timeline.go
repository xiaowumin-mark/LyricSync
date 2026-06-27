package media

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	smtcsuite "github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const (
	timelineSeekThresholdMs = int64(2500)
	timelineStepMaxMs       = int64(5000)
	timelineUnitMaxMs       = int64(24 * 60 * 60 * 1000)
	timelineFrameInterval   = time.Second / 60
)

type timelineClock struct {
	mu       sync.Mutex
	anchors  map[string]*timelineAnchor
	activeID string
}

type timelineAnchor struct {
	sessionID      string
	signature      string
	rawPositionMs  int64
	durationMs     int64
	anchorPosition int64
	anchorAt       time.Time
	lastSeenAt     time.Time
	state          string
	rate           float64
}

func newTimelineClock() *timelineClock {
	return &timelineClock{anchors: map[string]*timelineAnchor{}}
}

func (c *timelineClock) Session(session smtcsuite.SessionInfo, active bool, now time.Time) model.Session {
	position, duration := c.update(session, active, now)
	return model.Session{
		ID:        session.SessionID,
		Name:      sessionName(session),
		AppID:     session.SourceAppUserModelID,
		Title:     session.MediaInfo.Title,
		Artist:    firstNonEmpty(session.MediaInfo.Artist, session.MediaInfo.AlbumArtist),
		Album:     session.MediaInfo.AlbumTitle,
		State:     playbackState(session.PlaybackStatus),
		Position:  position,
		Duration:  duration,
		Active:    active,
		UpdatedAt: model.FormatTime(now),
	}
}

func (c *timelineClock) Playback(session smtcsuite.SessionInfo, now time.Time) model.Playback {
	position, _ := c.update(session, true, now)
	return model.Playback{
		State:      playbackState(session.PlaybackStatus),
		Position:   position,
		Volume:     -1,
		CanControl: hasControls(session.PlaybackControls),
		UpdatedAt:  model.FormatTime(now),
	}
}

func (c *timelineClock) ActivePosition(now time.Time) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	anchor := c.anchors[c.activeID]
	if anchor == nil || anchor.state != "playing" {
		return 0, false
	}
	return anchor.positionAt(now), true
}

func (c *timelineClock) ClearActive() {
	c.mu.Lock()
	c.activeID = ""
	c.mu.Unlock()
}

func (c *timelineClock) Prune(sessions []smtcsuite.SessionInfo) {
	known := make(map[string]struct{}, len(sessions))
	for _, session := range sessions {
		if id := timelineSessionID(session); id != "" {
			known[id] = struct{}{}
		}
	}
	c.mu.Lock()
	for id := range c.anchors {
		if _, ok := known[id]; !ok {
			delete(c.anchors, id)
		}
	}
	if _, ok := known[c.activeID]; !ok {
		c.activeID = ""
	}
	c.mu.Unlock()
}

func (c *timelineClock) update(session smtcsuite.SessionInfo, active bool, now time.Time) (int64, int64) {
	if now.IsZero() {
		now = time.Now()
	}
	id := timelineSessionID(session)
	if id == "" {
		return 0, 0
	}
	rawPosition, duration := normalizedTimeline(session.TimelineInfo)
	state := playbackState(session.PlaybackStatus)
	rate := timelinePlaybackRate(session)
	signature := timelineSignature(session, duration)

	c.mu.Lock()
	defer c.mu.Unlock()
	if active {
		c.activeID = id
	}
	anchor := c.anchors[id]
	if anchor == nil || anchor.signature != signature {
		anchor = &timelineAnchor{
			sessionID:      id,
			signature:      signature,
			rawPositionMs:  rawPosition,
			durationMs:     duration,
			anchorPosition: rawPosition,
			anchorAt:       now,
			lastSeenAt:     now,
			state:          state,
			rate:           rate,
		}
		c.anchors[id] = anchor
		return anchor.positionAt(now), duration
	}

	anchor.durationMs = duration
	anchor.state = state
	anchor.rate = rate

	switch state {
	case "playing":
		c.updatePlayingAnchor(anchor, rawPosition, now)
	default:
		anchor.anchorPosition = rawPosition
		anchor.anchorAt = now
	}
	anchor.rawPositionMs = rawPosition
	anchor.lastSeenAt = now
	return anchor.positionAt(now), duration
}

func (c *timelineClock) updatePlayingAnchor(anchor *timelineAnchor, rawPosition int64, now time.Time) {
	expected := anchor.positionAt(now)
	delta := rawPosition - expected
	rawDelta := rawPosition - anchor.rawPositionMs
	if rawPosition == anchor.rawPositionMs {
		return
	}
	if rawPosition < anchor.rawPositionMs-timelineSeekThresholdMs || absInt64(delta) > timelineSeekThresholdMs {
		anchor.anchorPosition = rawPosition
		anchor.anchorAt = now
		return
	}
	if isQuantizedTimelineStep(anchor.rawPositionMs, rawPosition, rawDelta) {
		if absInt64(delta) <= 150 {
			return
		}
		anchor.anchorPosition = rawPosition
		anchor.anchorAt = midpointTime(anchor.lastSeenAt, now)
		return
	}
	anchor.anchorPosition = rawPosition
	anchor.anchorAt = now
}

func (a *timelineAnchor) positionAt(now time.Time) int64 {
	position := a.anchorPosition
	if a.state == "playing" {
		rate := a.rate
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			rate = 1
		}
		elapsed := now.Sub(a.anchorAt)
		if elapsed > 0 {
			position += int64(float64(elapsed.Milliseconds()) * rate)
		}
	}
	if position < 0 {
		position = 0
	}
	if a.durationMs > 0 && position > a.durationMs {
		return a.durationMs
	}
	return position
}

func normalizedTimeline(info smtcsuite.TimelineInfo) (int64, int64) {
	duration := normalizedTimelineEndpoint(info.EndTime)
	position := normalizedTimelinePosition(info.Position, duration)
	if position < 0 {
		position = 0
	}
	if duration > 0 && position > duration {
		position = duration
	}
	return position, duration
}

func normalizedTimelineEndpoint(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	if ms := value.Milliseconds(); ms > 0 {
		return ms
	}
	raw := int64(value)
	if raw <= 0 {
		return 0
	}
	if raw <= timelineUnitMaxMs/1000 {
		return raw * 1000
	}
	if raw <= timelineUnitMaxMs {
		return raw
	}
	return 0
}

func normalizedTimelinePosition(value time.Duration, durationMs int64) int64 {
	if value <= 0 {
		return 0
	}
	if ms := value.Milliseconds(); ms > 0 {
		return ms
	}
	raw := int64(value)
	if raw <= 0 {
		return 0
	}
	if durationMs > 0 {
		if raw <= durationMs/1000+2 {
			return raw * 1000
		}
		if raw <= durationMs+1000 {
			return raw
		}
		return 0
	}
	return normalizedTimelineEndpoint(value)
}

func timelinePlaybackRate(session smtcsuite.SessionInfo) float64 {
	rate := session.TimelineInfo.PlaybackRate
	if rate <= 0 {
		rate = session.PlaybackRate
	}
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 1
	}
	return rate
}

func timelineSessionID(session smtcsuite.SessionInfo) string {
	if strings.TrimSpace(session.SessionID) != "" {
		return session.SessionID
	}
	return strings.TrimSpace(session.SourceAppUserModelID)
}

func timelineSignature(session smtcsuite.SessionInfo, durationMs int64) string {
	return strings.Join([]string{
		timelineSessionID(session),
		strings.TrimSpace(session.SourceAppUserModelID),
		strings.TrimSpace(session.MediaInfo.Title),
		strings.TrimSpace(firstNonEmpty(session.MediaInfo.Artist, session.MediaInfo.AlbumArtist)),
		strings.TrimSpace(session.MediaInfo.AlbumTitle),
		strconv.FormatInt(durationMs, 10),
	}, "\x00")
}

func isQuantizedTimelineStep(previous, current, delta int64) bool {
	if delta <= 0 || delta > timelineStepMaxMs {
		return false
	}
	return previous%1000 == 0 && current%1000 == 0
}

func midpointTime(a, b time.Time) time.Time {
	if a.IsZero() || b.Before(a) {
		return b
	}
	return a.Add(b.Sub(a) / 2)
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
