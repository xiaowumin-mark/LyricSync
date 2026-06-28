package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	flux "github.com/xiaowumin-mark/FluxUI/ui"

	"github.com/xiaowumin-mark/LyricSync/internal/app"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const (
	coverArtSize          = float32(176)
	waveformBarCount      = 44
	waveformHeight        = float32(112)
	waveformContentHeight = float32(92)
	dashboardLyricAfter   = 8
	dashboardLyricHeight  = float32(520)
)

func dashboardPage(colors palette, snapshot model.Snapshot, notice stringState, runtime *app.Runtime, compact bool, waveform []float64) flux.Element {
	main := flux.ColumnElement(
		nowPlayingPanel(colors, snapshot, notice, runtime, compact),
		flux.VSpacerElement(12),
		audioWaveformPanel(colors, snapshot, waveform),
	)
	side := flux.ColumnElement(
		currentLyricPanel(colors, snapshot),
	)
	if compact {
		return flux.ScrollViewElement(
			flux.ColumnElement(
				main,
				flux.VSpacerElement(12),
				side,
			),
			flux.ScrollVertical(true),
		)
	}
	return flux.RowElement(
		flux.ExpandedElement(flux.ScrollViewElement(main, flux.ScrollVertical(true))),
		flux.HSpacerElement(12),
		flux.FixedWidthElement(360, side),
	)
}

func nowPlayingPanel(colors palette, snapshot model.Snapshot, notice stringState, runtime *app.Runtime, compact bool) flux.Element {
	details := nowPlayingDetails(colors, snapshot, notice, runtime)
	var body flux.Element
	if compact {
		body = flux.ColumnElement(
			coverArt(colors, snapshot.Track),
			flux.VSpacerElement(14),
			details,
		)
	} else {
		body = flux.RowElement(
			flux.FixedWidthElement(coverArtSize, coverArt(colors, snapshot.Track)),
			flux.HSpacerElement(18),
			flux.ExpandedElement(details),
		)
	}
	return panel(colors,
		sectionTitle(colors, "当前播放"),
		flux.VSpacerElement(12),
		body,
	)
}

func nowPlayingDetails(colors palette, snapshot model.Snapshot, notice stringState, runtime *app.Runtime) flux.Element {
	track := snapshot.Track
	playback := snapshot.Playback
	stateText := playbackStateText(playback.State)
	return flux.ColumnElement(
		flux.RowElement(
			flux.ExpandedElement(flux.TextElement(clipText(blankAs(track.Title, "等待播放"), 56), flux.TextSize(26), flux.TextColor(colors.text))),
			flux.HSpacerElement(10),
			statusChip(colors, stateText, playbackStateColor(colors, playback.State)),
		),
		flux.VSpacerElement(6),
		flux.TextElement(clipText(blankAs(track.Artist, "未知艺人"), 72), flux.TextSize(14), flux.TextColor(colors.subtle)),
		flux.VSpacerElement(3),
		flux.TextElement(clipText(blankAs(track.Album, "未知专辑"), 72), flux.TextSize(12), flux.TextColor(colors.subtle)),
		flux.VSpacerElement(18),
		playbackProgressBlock(colors, track, playback),
		flux.VSpacerElement(16),
		flux.RowElement(
			playbackControls(colors, snapshot, notice, runtime),
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			currentSongDetailButton(colors, notice, runtime),
		),
	)
}

func coverArt(colors palette, track model.Track) flux.Element {
	if path := coverCachePath(track); path != "" {
		return flux.ImageElement(
			flux.ImageSource{Path: path, Label: "封面"},
			flux.ImageWidth(coverArtSize),
			flux.ImageHeight(coverArtSize),
			flux.ImageFitMode(flux.ImageFitCover),
			flux.ImageRadius(8),
			flux.ImageBackground(colors.muted),
			flux.ImageDecoration(flux.Bg(colors.muted).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border})),
		)
	}
	return flux.FixedSizeElement(
		coverArtSize,
		coverArtSize,
		flux.ContainerDecorationElement(
			flux.Bg(colors.muted).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border}),
			flux.CenterElement(
				flux.ColumnElement(
					flux.TextElement(trackInitial(track), flux.TextSize(38), flux.TextColor(colors.primary), flux.TextAlign(flux.AlignCenter)),
					flux.VSpacerElement(6),
					flux.TextElement("暂无封面", flux.TextSize(12), flux.TextColor(colors.subtle), flux.TextAlign(flux.AlignCenter)),
				),
			),
		),
	)
}

func playbackProgressBlock(colors palette, track model.Track, playback model.Playback) flux.Element {
	progress := playbackProgress(track, playback)
	return flux.ColumnElement(
		flux.ProgressBarElement(
			progress*100,
			flux.ProgressMin(0),
			flux.ProgressMax(100),
			flux.ProgressTrackColor(colors.barBase),
			flux.ProgressFillColor(colors.primary),
		),
		flux.VSpacerElement(8),
		flux.RowElement(
			flux.TextElement(formatMillis(playback.Position), flux.TextSize(12), flux.TextColor(colors.subtle)),
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			flux.TextElement(formatMillis(track.Duration), flux.TextSize(12), flux.TextColor(colors.subtle)),
		),
	)
}

func playbackControls(colors palette, snapshot model.Snapshot, notice stringState, runtime *app.Runtime) flux.Element {
	canControl := snapshot.Playback.CanControl && !isIdleTrack(snapshot.Track)
	playCommand := "play"
	playIcon := "play_arrow"
	playTip := "播放"
	if strings.EqualFold(snapshot.Playback.State, "playing") {
		playCommand = "pause"
		playIcon = "pause"
		playTip = "暂停"
	}
	return flux.RowElement(
		mediaControlButton(colors, "上一首", "skip_previous", canControl, "previous", notice, runtime),
		flux.HSpacerElement(10),
		mediaControlButton(colors, playTip, playIcon, canControl, playCommand, notice, runtime),
		flux.HSpacerElement(10),
		mediaControlButton(colors, "下一首", "skip_next", canControl, "next", notice, runtime),
	)
}

func currentSongDetailButton(colors palette, notice stringState, runtime *app.Runtime) flux.Element {
	return flux.TooltipElement(
		"查看当前歌曲详情",
		flux.FilledTonalIconButtonElement(
			flux.IconElement("pageview", flux.IconSize(22)),
			flux.IconButtonSize(44),
			flux.IconButtonDisabled(runtime == nil),
			flux.IconButtonOnClick(func(ctx *flux.Context) {
				if runtime == nil {
					notice.Set("歌曲数据库不可用")
					return
				}
				current, err := runtime.CurrentSong(context.Background())
				if err != nil {
					notice.Set("当前歌曲还没有可查看的记录")
					return
				}
				flux.Navigate(ctx, fmt.Sprintf("/songs/%d", current.ID), flux.WithNavTransition(flux.TransitionSlideLeft))
			}),
		),
	)
}

func mediaControlButton(colors palette, tooltip string, icon string, enabled bool, command string, notice stringState, runtime *app.Runtime) flux.Element {
	return flux.TooltipElement(
		tooltip,
		flux.FilledTonalIconButtonElement(
			flux.IconElement(icon, flux.IconSize(24)),
			flux.IconButtonSize(44),
			flux.IconButtonDisabled(!enabled),
			flux.IconButtonOnClick(func(ctx *flux.Context) {
				if runtime == nil {
					notice.Set("播放器控制不可用")
					return
				}
				if err := runtime.ControlMedia(command, 0); err != nil {
					notice.Set("播放器控制失败: " + err.Error())
					return
				}
				notice.Set(mediaCommandNotice(command))
			}),
		),
	)
}

func audioWaveformPanel(colors palette, snapshot model.Snapshot, waveform []float64) flux.Element {
	return panel(colors,
		sectionTitle(colors, "音频波形"),
		flux.VSpacerElement(12),
		volumeDecoration(colors, snapshot.Playback.Volume),
		flux.VSpacerElement(16),
		waveformBars(colors, waveform, waveformEmptyText(snapshot)),
	)
}

func volumeDecoration(colors palette, volume float64) flux.Element {
	value := clamp01(volume)
	return flux.RowElement(
		flux.FixedWidthElement(48, label(colors, "音量")),
		flux.FixedWidthElement(
			230,
			flux.SliderElement(
				float32(value*100),
				flux.SliderMin(0),
				flux.SliderMax(100),
				flux.SliderDisabled(true),
				flux.SliderTrackColor(colors.barBase),
				flux.SliderProgressColor(colors.primary),
				flux.SliderThumbColor(colors.primary),
				flux.SliderWidth(230),
			),
		),
		flux.HSpacerElement(10),
		flux.TextElement(fmt.Sprintf("%02.0f%%", value*100), flux.TextSize(12), flux.TextColor(colors.subtle)),
	)
}

func currentLyricPanel(colors palette, snapshot model.Snapshot) flux.Element {
	if isIdleTrack(snapshot.Track) {
		return panel(colors,
			sectionTitle(colors, "当前歌词"),
			flux.VSpacerElement(12),
			emptyBox(colors, "等待播放"),
		)
	}
	if snapshot.Lyrics.TrackID != "" && snapshot.Lyrics.TrackID != snapshot.Track.ID {
		return panel(colors,
			sectionTitle(colors, "当前歌词"),
			flux.VSpacerElement(12),
			emptyBox(colors, "等待歌词同步"),
		)
	}
	if len(snapshot.Lyrics.Lines) == 0 {
		return panel(colors,
			sectionTitle(colors, "当前歌词"),
			flux.VSpacerElement(12),
			emptyBox(colors, "暂无歌词"),
		)
	}
	rows := []flux.Element{
		sectionTitle(colors, "当前歌词"),
		flux.VSpacerElement(12),
	}
	start, end := dashboardLyricRange(snapshot.Lyrics.Lines, snapshot.Playback.Position)
	visibleCount := end - start
	rows = append(rows, flux.FixedHeightElement(
		dashboardLyricHeight,
		flux.ListViewElement(
			visibleCount,
			func(ctx *flux.Context, index int) flux.Element {
				actual := start + index
				if index < 0 || index >= visibleCount || actual < 0 || actual >= len(snapshot.Lyrics.Lines) {
					return flux.SpacerElement(0, 0)
				}
				line := snapshot.Lyrics.Lines[actual]
				return flux.Key(fmt.Sprintf("dashboard-lyric-%d-%d", actual, line.StartTimeMs),
					dashboardLyricLine(colors, line, lyricLineActive(line, snapshot.Playback.Position)),
				)
			},
			flux.ListVirtualized(true),
			flux.ListItemSpacing(8),
			flux.ListPadding(flux.All(2)),
			flux.ListDecoration(flux.Bg(colors.surface)),
		),
	))
	return panel(colors, rows...)
}

func dashboardLyricRange(lines []model.CurrentLyricLine, position int64) (int, int) {
	if len(lines) <= dashboardLyricAfter+2 {
		return 0, len(lines)
	}
	activeStart, activeEnd := dashboardLyricActiveRange(lines, position)
	start := activeStart - 1
	if start < 0 {
		start = 0
	}
	end := activeEnd + dashboardLyricAfter
	if end > len(lines) {
		end = len(lines)
	}
	return start, end
}

func dashboardLyricActiveRange(lines []model.CurrentLyricLine, position int64) (int, int) {
	candidate := dashboardLyricCandidateIndex(lines, position)
	if candidate < 0 {
		return 0, 1
	}
	activeStart := -1
	activeEnd := -1
	for i, checked := candidate, 0; i >= 0 && checked < 64; i, checked = i-1, checked+1 {
		if lyricLineActive(lines[i], position) {
			activeStart = i
			if activeEnd < 0 {
				activeEnd = i + 1
			}
			continue
		}
		if activeStart >= 0 && lines[i].EndTimeMs <= position {
			break
		}
	}
	for i := candidate + 1; i < len(lines); i++ {
		if lines[i].StartTimeMs > position {
			break
		}
		if !lyricLineActive(lines[i], position) {
			continue
		}
		if activeStart < 0 {
			activeStart = i
		}
		activeEnd = i + 1
	}
	if activeStart >= 0 {
		return activeStart, activeEnd
	}
	return candidate, candidate + 1
}

func dashboardLyricCandidateIndex(lines []model.CurrentLyricLine, position int64) int {
	return sort.Search(len(lines), func(i int) bool {
		return lines[i].StartTimeMs > position
	}) - 1
}

func dashboardLyricLine(colors palette, line model.CurrentLyricLine, active bool) flux.Element {
	align := flux.AlignStart
	if line.Duet {
		align = flux.AlignEnd
	}
	textColor := colors.subtle
	subColor := colors.subtle
	bg := colors.surface
	if active {
		textColor = colors.primary
		subColor = colors.text
		bg = colors.primaryContainer
	}
	size := float32(15)
	if line.Background {
		size = 12
	}
	children := []flux.Element{
		flux.TextElement(line.Text, flux.TextSize(size), flux.TextColor(textColor), flux.TextAlign(align)),
	}
	if line.Translation != "" {
		children = append(children, flux.VSpacerElement(5), flux.TextElement(line.Translation, flux.TextSize(12), flux.TextColor(subColor), flux.TextAlign(align)))
	}
	if line.Roman != "" {
		children = append(children, flux.VSpacerElement(4), flux.TextElement(line.Roman, flux.TextSize(11), flux.TextColor(subColor), flux.TextAlign(align)))
	}
	return flux.ContainerDecorationElement(
		flux.Bg(bg).WithPad(flux.All(10)).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border}),
		flux.ColumnElement(children...),
	)
}

func lyricLineActive(line model.CurrentLyricLine, position int64) bool {
	return position >= line.StartTimeMs && (line.EndTimeMs <= line.StartTimeMs || position < line.EndTimeMs)
}

func useStableWaveform(ctx *flux.Context, values []float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	ref := flux.UseRef(ctx, stableWaveformState{
		current: make([]float64, count),
		target:  make([]float64, count),
	})
	state := ref.Current
	if len(state.current) != count {
		state.current = make([]float64, count)
	}
	if len(state.target) != count {
		state.target = make([]float64, count)
	}
	resampleWaveformInto(values, state.target)
	for i := 0; i < count; i++ {
		alpha := 0.24
		if state.target[i] > state.current[i] {
			alpha = 0.42
		}
		state.current[i] = state.current[i] + (state.target[i]-state.current[i])*alpha
		if state.current[i] < 0.01 {
			state.current[i] = 0
		}
	}
	ref.Current = state
	return state.current
}

type stableWaveformState struct {
	current []float64
	target  []float64
}

func resampleWaveformInto(values []float64, result []float64) {
	count := len(result)
	if count == 0 {
		return
	}
	if len(values) == 0 {
		for i := range result {
			result[i] = 0
		}
		return
	}
	for i := 0; i < count; i++ {
		start := int(float64(i) * float64(len(values)) / float64(count))
		end := int(float64(i+1) * float64(len(values)) / float64(count))
		if end <= start {
			end = start + 1
		}
		if end > len(values) {
			end = len(values)
		}
		var sum float64
		for j := start; j < end; j++ {
			sum += clamp01(values[j])
		}
		value := sum / float64(end-start)
		result[i] = clamp01(value * 1.18)
	}
}

func waveformBars(colors palette, values []float64, emptyText string) flux.Element {
	if !hasWaveform(values) {
		return flux.FixedHeightElement(waveformHeight, emptyBox(colors, emptyText))
	}
	peak := waveformPeak(values)
	children := make([]flux.Element, 0, len(values)*2)
	for i, value := range values {
		if i > 0 {
			children = append(children, flux.HSpacerElement(3))
		}
		value = clamp01(value)
		barHeight := float32(8 + value*float64(waveformContentHeight-18))
		top := (waveformContentHeight - barHeight) / 2
		barColor := colors.barAccent
		if i%6 == 0 {
			barColor = colors.primary
		}
		barColor = waveformAlpha(barColor, value, peak)
		children = append(children,
			flux.ExpandedElement(
				flux.FixedHeightElement(
					waveformContentHeight,
					flux.ColumnElement(
						flux.SpacerElement(0, top),
						flux.ContainerDecorationElement(
							flux.Bg(barColor).WithRad(999),
							flux.FixedHeightElement(barHeight, flux.SpacerElement(0, 0)),
						),
						flux.SpacerElement(0, top),
					),
				),
			),
		)
	}
	return flux.FixedHeightElement(
		waveformHeight,
		flux.ContainerDecorationElement(
			flux.Bg(colors.muted).WithPad(flux.All(10)).WithRad(8),
			flux.RowElement(children...),
		),
	)
}

func waveformPeak(values []float64) float64 {
	var peak float64
	for _, value := range values {
		if value = clamp01(value); value > peak {
			peak = value
		}
	}
	return peak
}

func waveformAlpha(col color.NRGBA, value float64, peak float64) color.NRGBA {
	const minAlpha = 105
	relative := clamp01(value)
	if peak > 0.015 {
		relative = clamp01(value / peak)
	}
	boosted := 1 - (1-relative)*(1-relative)
	alpha := minAlpha + int(boosted*float64(255-minAlpha))
	if alpha > 255 {
		alpha = 255
	}
	col.A = uint8(alpha)
	return col
}

func hasWaveform(values []float64) bool {
	for _, value := range values {
		if value > 0.015 {
			return true
		}
	}
	return false
}

func waveformEmptyText(snapshot model.Snapshot) string {
	if isIdleTrack(snapshot.Track) {
		return "等待播放"
	}
	if strings.EqualFold(snapshot.Playback.State, "paused") {
		return "已暂停"
	}
	return "等待音频波形"
}

func playbackStateText(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "playing":
		return "播放中"
	case "paused":
		return "已暂停"
	case "stopped":
		return "已停止"
	default:
		return blankAs(state, "未知")
	}
}

func playbackStateColor(colors palette, state string) color.NRGBA {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "playing":
		return colors.success
	case "paused":
		return colors.warning
	case "stopped":
		return colors.subtle
	default:
		return colors.subtle
	}
}

func mediaCommandNotice(command string) string {
	switch command {
	case "previous":
		return "已发送上一首"
	case "pause":
		return "已发送暂停"
	case "play":
		return "已发送播放"
	case "next":
		return "已发送下一首"
	default:
		return "已发送播放器控制"
	}
}

func isIdleTrack(track model.Track) bool {
	id := strings.TrimSpace(track.ID)
	return id == "" || id == "idle"
}

func trackInitial(track model.Track) string {
	for _, source := range []string{track.Title, track.Artist, track.Album} {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		runes := []rune(source)
		if len(runes) > 0 {
			return strings.ToUpper(string(runes[0]))
		}
	}
	return "LS"
}

func coverCachePath(track model.Track) string {
	if len(track.CoverData) == 0 {
		return ""
	}
	cacheKey := strings.TrimSpace(track.CoverHash)
	if cacheKey == "" {
		cacheKey = coverCacheKey(track)
	}
	if path := cachedCoverPath(cacheKey); path != "" {
		return path
	}
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "LyricSync", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	sum := sha256.Sum256(track.CoverData)
	name := hex.EncodeToString(sum[:]) + coverExtension(track.CoverMimeType, track.CoverData)
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		storeCoverPath(cacheKey, path)
		return path
	}
	if err := os.WriteFile(path, track.CoverData, 0o644); err != nil {
		return ""
	}
	storeCoverPath(cacheKey, path)
	return path
}

var dashboardCoverCache = struct {
	sync.RWMutex
	paths map[string]string
}{paths: map[string]string{}}

func coverCacheKey(track model.Track) string {
	sum := sha256.Sum256(track.CoverData)
	return hex.EncodeToString(sum[:]) + "|" + track.CoverMimeType + "|" + fmt.Sprint(len(track.CoverData))
}

func cachedCoverPath(key string) string {
	dashboardCoverCache.RLock()
	defer dashboardCoverCache.RUnlock()
	return dashboardCoverCache.paths[key]
}

func storeCoverPath(key string, path string) {
	dashboardCoverCache.Lock()
	dashboardCoverCache.paths[key] = path
	dashboardCoverCache.Unlock()
}

func coverExtension(mime string, data []byte) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png"
	}
	if len(data) >= 3 && string(data[:3]) == "\xff\xd8\xff" {
		return ".jpg"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return ".gif"
	}
	return ".img"
}
