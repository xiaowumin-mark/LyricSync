package ui

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	flux "github.com/xiaowumin-mark/FluxUI/ui"

	"github.com/xiaowumin-mark/LyricSync/internal/app"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	storepkg "github.com/xiaowumin-mark/LyricSync/internal/state"
)

func Run(ctx context.Context, store *storepkg.Store, runtime *app.Runtime) error {
	_ = ctx
	return flux.RunElement(
		func(uiCtx *flux.Context) flux.Element {
			return root(uiCtx, store, runtime)
		},
		flux.Title("LyricSync"),
		flux.Size(1180, 760),
	)
}

const (
	pageDashboard = "dashboard"
	pageSessions  = "sessions"
	pageSongs     = "songs"
	pageLogs      = "logs"
	pageSettings  = "settings"
)

type stringState interface {
	Value() string
	Set(string)
}

type pageInfo struct {
	key      string
	label    string
	title    string
	subtitle string
	icon     string
}

var appPages = []pageInfo{
	{key: pageDashboard, label: "仪表盘", title: "仪表盘", subtitle: "当前监听会话的播放状态", icon: "D"},
	{key: pageSessions, label: "会话", title: "会话", subtitle: "SMTC 会话管理", icon: "S"},
	{key: pageSongs, label: "歌曲", title: "歌曲", subtitle: "播放记录与歌词管理", icon: "M"},
	{key: pageLogs, label: "日志", title: "日志", subtitle: "软件运行记录", icon: "L"},
	{key: pageSettings, label: "设置", title: "设置", subtitle: "连接、媒体和歌词偏好", icon: "G"},
}

type palette struct {
	surface   color.NRGBA
	panel     color.NRGBA
	muted     color.NRGBA
	text      color.NRGBA
	subtle    color.NRGBA
	border    color.NRGBA
	primary   color.NRGBA
	success   color.NRGBA
	warning   color.NRGBA
	danger    color.NRGBA
	barBase   color.NRGBA
	barAccent color.NRGBA
}

func root(ctx *flux.Context, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	colors := appPalette()
	snapshot := useStoreSnapshot(ctx, store)
	activePage := flux.UseState(ctx, pageDashboard)
	urlState := flux.UseState(ctx, snapshot.Config.AMLL.URL)
	notice := flux.UseState(ctx, "")

	return flux.ContainerDecorationElement(
		flux.Bg(colors.surface),
		appShell(ctx, colors, snapshot, activePage, urlState, notice, store, runtime),
	)
}

func useStoreSnapshot(ctx *flux.Context, store *storepkg.Store) model.Snapshot {
	snapshotState := flux.UseState(ctx, store.Snapshot())
	flux.UseEffectWithDeps(ctx, []any{store}, func() func() {
		events, unsubscribe := store.Subscribe(128)
		done := make(chan struct{})

		snapshotState.Set(store.Snapshot())
		go func() {
			for {
				select {
				case _, ok := <-events:
					if !ok {
						return
					}
					snapshotState.Set(store.Snapshot())
				case <-done:
					return
				}
			}
		}()

		return func() {
			close(done)
			unsubscribe()
		}
	})
	return snapshotState.Value()
}

func appShell(
	ctx *flux.Context,
	colors palette,
	snapshot model.Snapshot,
	activePage stringState,
	urlState stringState,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	page := currentPage(activePage.Value())
	compact := ctx.MaxConstraints().X < 880
	content := flux.ContainerDecorationElement(
		flux.Bg(colors.surface).WithPad(flux.All(18)),
		flux.ColumnElement(
			pageHeader(colors, snapshot, page, notice.Value()),
			flux.VSpacerElement(14),
			flux.ExpandedElement(pageBody(ctx, colors, snapshot, activePage.Value(), urlState, notice, store, runtime, compact)),
		),
	)

	return flux.RowElement(
		navRail(colors, snapshot, activePage),
		flux.ExpandedElement(content),
	)
}

func pageBody(
	ctx *flux.Context,
	colors palette,
	snapshot model.Snapshot,
	page string,
	urlState stringState,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
	compact bool,
) flux.Element {
	switch page {
	case pageSessions:
		return sessionsPage(colors, snapshot, notice, runtime)
	case pageSongs:
		return songsPage(colors, snapshot)
	case pageLogs:
		return logsPage(colors, snapshot)
	case pageSettings:
		return settingsPage(colors, snapshot, urlState, notice, store, runtime)
	default:
		return dashboardPage(colors, snapshot, notice, runtime, compact)
	}
}

func navRail(colors palette, snapshot model.Snapshot, activePage stringState) flux.Element {
	return flux.NavigationRailElement(
		activePage.Value(),
		navItems(),
		flux.NavigationRailWidth(92),
		flux.NavigationRailHeader(flux.Text("LS", flux.TextSize(16))),
		flux.NavigationRailFooter(flux.Text(strings.ToUpper(blankAs(snapshot.AMLL.Status, "off")), flux.TextSize(10))),
		flux.NavigationRailActiveColor(colors.primary),
		flux.NavigationRailInactiveColor(colors.subtle),
		flux.NavigationRailDecoration(flux.Bg(colors.panel).WithBorder(flux.Border{Width: 1, Color: colors.border})),
		flux.NavigationRailOnChange(func(ctx *flux.Context, key string) {
			activePage.Set(key)
		}),
	)
}

func navItems() []flux.ElementNavItem {
	items := make([]flux.ElementNavItem, 0, len(appPages))
	for _, page := range appPages {
		items = append(items, flux.ElementNavItem{
			Key:   page.key,
			Label: page.label,
			Icon:  flux.IconElement(page.icon),
		})
	}
	return items
}

func currentPage(key string) pageInfo {
	for _, page := range appPages {
		if page.key == key {
			return page
		}
	}
	return appPages[0]
}

func pageHeader(colors palette, snapshot model.Snapshot, page pageInfo, notice string) flux.Element {
	statusColor := statusColor(colors, snapshot.AMLL.Status)
	subtitle := fmt.Sprintf("AMLL %s | %s | %s", snapshot.AMLL.Status, snapshot.Playback.State, formatClock(snapshot.UpdatedAt))
	if strings.TrimSpace(notice) != "" {
		subtitle = notice
	}
	return flux.ContainerDecorationElement(
		flux.Bg(colors.panel).WithPad(flux.Symmetric(14, 16)).WithRad(8),
		flux.RowElement(
			flux.ColumnElement(
				flux.TextElement(page.title, flux.TextSize(24), flux.TextColor(colors.text)),
				flux.VSpacerElement(4),
				flux.TextElement(clipText(page.subtitle+" | "+subtitle, 108), flux.TextSize(12), flux.TextColor(colors.subtle)),
			),
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			statusChip(colors, strings.ToUpper(snapshot.AMLL.Status), statusColor),
		),
	)
}

func dashboardPage(colors palette, snapshot model.Snapshot, notice stringState, runtime *app.Runtime, compact bool) flux.Element {
	_ = notice
	_ = runtime
	main := flux.ColumnElement(
		mediaPanel(colors, snapshot),
		flux.VSpacerElement(12),
		audioPanel(colors, snapshot),
	)
	side := flux.ColumnElement(
		metricsPanel(colors, snapshot),
		flux.VSpacerElement(12),
		lyricsPreviewPanel(colors),
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
		flux.FixedWidthElement(330, flux.ScrollViewElement(side, flux.ScrollVertical(true))),
	)
}

func songsPage(colors palette, snapshot model.Snapshot) flux.Element {
	return flux.ScrollViewElement(
		flux.ColumnElement(
			panel(colors,
				sectionTitle(colors, "歌曲"),
				flux.VSpacerElement(12),
				emptyBox(colors, "暂无歌曲记录"),
			),
			flux.VSpacerElement(12),
			panel(colors,
				sectionTitle(colors, "当前歌曲"),
				flux.VSpacerElement(10),
				infoLine(colors, "标题", blankAs(snapshot.Track.Title, "-")),
				infoLine(colors, "艺人", blankAs(snapshot.Track.Artist, "-")),
				infoLine(colors, "专辑", blankAs(snapshot.Track.Album, "-")),
				infoLine(colors, "时长", formatMillis(snapshot.Track.Duration)),
			),
		),
		flux.ScrollVertical(true),
	)
}

func logsPage(colors palette, snapshot model.Snapshot) flux.Element {
	return flux.ScrollViewElement(
		flux.ColumnElement(logsPanel(colors, snapshot)),
		flux.ScrollVertical(true),
	)
}

func settingsPage(
	colors palette,
	snapshot model.Snapshot,
	urlState stringState,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	return flux.ScrollViewElement(
		flux.ColumnElement(
			connectionPanel(colors, snapshot, urlState, notice, store, runtime),
			flux.VSpacerElement(12),
			panel(colors,
				sectionTitle(colors, "媒体"),
				flux.VSpacerElement(10),
				infoLine(colors, "SMTC", sessionMode(snapshot)),
				infoLine(colors, "音频数据", yesNo(snapshot.Config.AMLL.SendAudio)),
			),
			flux.VSpacerElement(12),
			panel(colors,
				sectionTitle(colors, "歌词"),
				flux.VSpacerElement(10),
				emptyBox(colors, "暂无歌词设置"),
			),
			flux.VSpacerElement(12),
			panel(colors,
				sectionTitle(colors, "AI"),
				flux.VSpacerElement(10),
				emptyBox(colors, "暂无 AI 设置"),
			),
		),
		flux.ScrollVertical(true),
	)
}

func connectionPanel(
	colors palette,
	snapshot model.Snapshot,
	urlState interface {
		Value() string
		Set(string)
	},
	notice interface {
		Value() string
		Set(string)
	},
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	connect := func(ctx *flux.Context) {
		url := strings.TrimSpace(urlState.Value())
		if err := runtime.ConnectAMLL(url); err != nil {
			message := "Connect failed: " + err.Error()
			notice.Set(message)
			store.AddLog(message)
			return
		}
		notice.Set("Connecting AMLL server")
	}
	disconnect := func(ctx *flux.Context) {
		if err := runtime.DisconnectAMLL(); err != nil {
			message := "Disconnect failed: " + err.Error()
			notice.Set(message)
			store.AddLog(message)
			return
		}
		notice.Set("AMLL disconnected")
	}
	sendSnapshot := func(ctx *flux.Context) {
		if err := runtime.SendSnapshot(); err != nil {
			message := "Snapshot failed: " + err.Error()
			notice.Set(message)
			store.AddLog(message)
			return
		}
		notice.Set("Snapshot sent")
	}

	return panel(colors,
		sectionTitle(colors, "AMLL 连接"),
		flux.VSpacerElement(10),
		label(colors, "WebSocket 地址"),
		flux.VSpacerElement(6),
		flux.TextFieldElement(
			urlState.Value(),
			flux.InputPlaceholder("ws://127.0.0.1:11444"),
			flux.InputSingleLine(true),
			flux.InputPadding(flux.Symmetric(10, 12)),
			flux.InputRadius(8),
			flux.InputBorder(colors.border),
			flux.InputBorderFocus(colors.primary),
			flux.InputBackground(colors.panel),
			flux.InputForeground(colors.text),
			flux.InputTextSize(13),
			flux.InputOnChange(func(ctx *flux.Context, value string) {
				urlState.Set(value)
			}),
		),
		flux.VSpacerElement(10),
		flux.RowElement(
			flux.ExpandedElement(primaryButton(colors, "连接", connect)),
			flux.HSpacerElement(8),
			flux.ExpandedElement(secondaryButton(colors, "断开", disconnect)),
		),
		flux.VSpacerElement(8),
		flux.RowElement(
			flux.ExpandedElement(secondaryButton(colors, "发送快照", sendSnapshot)),
			flux.HSpacerElement(8),
			flux.ExpandedElement(audioSwitch(colors, snapshot.Config.AMLL.SendAudio, notice, store, runtime)),
		),
		flux.VSpacerElement(12),
		infoLine(colors, "状态", snapshot.AMLL.Status),
		infoLine(colors, "消息", clipText(snapshot.AMLL.Message, 42)),
		infoLine(colors, "最后接收", formatClock(snapshot.AMLL.LastMessageAt)),
	)
}

func audioSwitch(
	colors palette,
	checked bool,
	notice interface {
		Value() string
		Set(string)
	},
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.Symmetric(8, 10)).WithRad(8),
		flux.RowElement(
			flux.SwitchElement(
				checked,
				flux.SwitchOnChange(func(ctx *flux.Context, enabled bool) {
					if err := runtime.ToggleSendAudio(enabled); err != nil {
						message := "Audio toggle failed: " + err.Error()
						notice.Set(message)
						store.AddLog(message)
						return
					}
					if enabled {
						notice.Set("Audio sending enabled")
					} else {
						notice.Set("Audio sending disabled")
					}
				}),
			),
			flux.HSpacerElement(8),
			flux.TextElement("音频", flux.TextSize(13), flux.TextColor(colors.text)),
		),
	)
}

func metricsPanel(colors palette, snapshot model.Snapshot) flux.Element {
	return panel(colors,
		sectionTitle(colors, "连接统计"),
		flux.VSpacerElement(10),
		metricTile(colors, "文本消息", fmt.Sprintf("%d", snapshot.AMLL.MessagesSent), colors.primary),
		flux.VSpacerElement(8),
		metricTile(colors, "二进制消息", fmt.Sprintf("%d", snapshot.AMLL.BinaryMessagesSent), colors.success),
		flux.VSpacerElement(8),
		metricTile(colors, "已发送", formatBytes(snapshot.AMLL.BytesSent), colors.warning),
	)
}

func mediaPanel(colors palette, snapshot model.Snapshot) flux.Element {
	progress := playbackProgress(snapshot.Track, snapshot.Playback)
	return panel(colors,
		sectionTitle(colors, "当前播放"),
		flux.VSpacerElement(12),
		flux.TextElement(clipText(blankAs(snapshot.Track.Title, "暂无歌曲"), 68), flux.TextSize(22), flux.TextColor(colors.text)),
		flux.VSpacerElement(4),
		flux.TextElement(clipText(blankAs(snapshot.Track.Artist, "未知艺人"), 82), flux.TextSize(13), flux.TextColor(colors.subtle)),
		flux.VSpacerElement(12),
		flux.ProgressBarElement(
			progress*100,
			flux.ProgressMin(0),
			flux.ProgressMax(100),
			flux.ProgressTrackColor(colors.barBase),
			flux.ProgressFillColor(colors.primary),
		),
		flux.VSpacerElement(8),
		flux.RowElement(
			flux.TextElement(formatMillis(snapshot.Playback.Position), flux.TextSize(12), flux.TextColor(colors.subtle)),
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			flux.TextElement(formatMillis(snapshot.Track.Duration), flux.TextSize(12), flux.TextColor(colors.subtle)),
		),
		flux.VSpacerElement(12),
		flux.RowElement(
			infoTile(colors, "状态", snapshot.Playback.State),
			flux.HSpacerElement(8),
			infoTile(colors, "来源", clipText(snapshot.Track.SourceApp, 20)),
			flux.HSpacerElement(8),
			infoTile(colors, "控制", yesNo(snapshot.Playback.CanControl)),
		),
	)
}

func audioPanel(colors palette, snapshot model.Snapshot) flux.Element {
	rms := clamp01(snapshot.Audio.RMS)
	peak := clamp01(snapshot.Audio.Peak)
	volume := clamp01(snapshot.Playback.Volume)
	return panel(colors,
		sectionTitle(colors, "音频波形"),
		flux.VSpacerElement(12),
		levelRow(colors, "音量", volume, colors.primary),
		flux.VSpacerElement(8),
		levelRow(colors, "电平", rms, colors.success),
		flux.VSpacerElement(8),
		levelRow(colors, "峰值", peak, colors.warning),
		flux.VSpacerElement(14),
		spectrumBars(colors, snapshot.Audio.Spectrum),
	)
}

func logsPanel(colors palette, snapshot model.Snapshot) flux.Element {
	logs := snapshot.Logs
	if len(logs) > 80 {
		logs = logs[len(logs)-80:]
	}
	items := make([]flux.Element, 0, len(logs))
	for _, entry := range logs {
		items = append(items,
			flux.ContainerDecorationElement(
				flux.Bg(colors.muted).WithPad(flux.Symmetric(7, 9)).WithRad(6),
				flux.TextElement(clipText(entry, 72), flux.TextSize(11), flux.TextColor(colors.text)),
			),
			flux.VSpacerElement(6),
		)
	}
	if len(items) == 0 {
		items = append(items, emptyBox(colors, "暂无日志"))
	}
	return panel(colors,
		sectionTitle(colors, "日志"),
		flux.VSpacerElement(10),
		flux.FixedHeightElement(
			280,
			flux.ScrollViewElement(
				flux.ColumnElement(items...),
				flux.ScrollVertical(true),
				flux.ScrollAutoToEndKey(len(logs)),
			),
		),
	)
}

func lyricsPreviewPanel(colors palette) flux.Element {
	return panel(colors,
		sectionTitle(colors, "歌词"),
		flux.VSpacerElement(12),
		emptyBox(colors, "暂无歌词"),
	)
}

func panel(colors palette, children ...flux.Element) flux.Element {
	return flux.ContainerDecorationElement(
		flux.Bg(colors.panel).WithPad(flux.All(14)).WithRad(8),
		flux.ColumnElement(children...),
	)
}

func sectionTitle(colors palette, text string) flux.Element {
	return flux.TextElement(text, flux.TextSize(16), flux.TextColor(colors.text))
}

func label(colors palette, text string) flux.Element {
	return flux.TextElement(text, flux.TextSize(12), flux.TextColor(colors.subtle))
}

func primaryButton(colors palette, text string, onClick func(ctx *flux.Context)) flux.Element {
	return flux.ButtonElement(
		flux.TextElement(text),
		flux.ButtonPadding(flux.Symmetric(8, 12)),
		flux.ButtonRadius(8),
		flux.ButtonBackground(colors.primary),
		flux.ButtonForeground(flux.NRGBA(255, 255, 255, 255)),
		flux.OnClick(onClick),
	)
}

func secondaryButton(colors palette, text string, onClick func(ctx *flux.Context)) flux.Element {
	return flux.ButtonElement(
		flux.TextElement(text),
		flux.ButtonPadding(flux.Symmetric(8, 12)),
		flux.ButtonRadius(8),
		flux.ButtonBackground(colors.muted),
		flux.ButtonForeground(colors.text),
		flux.OnClick(onClick),
	)
}

func metricTile(colors palette, labelText, value string, accent color.NRGBA) flux.Element {
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.All(12)).WithRad(8),
		flux.RowElement(
			flux.ColumnElement(
				label(colors, labelText),
				flux.VSpacerElement(4),
				flux.TextElement(value, flux.TextSize(21), flux.TextColor(accent)),
			),
		),
	)
}

func infoTile(colors palette, labelText, value string) flux.Element {
	return flux.ExpandedElement(
		flux.ContainerDecorationElement(
			flux.Bg(colors.muted).WithPad(flux.All(10)).WithRad(8),
			flux.ColumnElement(
				label(colors, labelText),
				flux.VSpacerElement(4),
				flux.TextElement(clipText(blankAs(value, "-"), 28), flux.TextSize(13), flux.TextColor(colors.text)),
			),
		),
	)
}

func infoLine(colors palette, name, value string) flux.Element {
	return flux.PaddingElement(
		flux.Insets{Top: 4},
		flux.RowElement(
			flux.FixedWidthElement(78, label(colors, name)),
			flux.ExpandedElement(flux.TextElement(blankAs(value, "-"), flux.TextSize(12), flux.TextColor(colors.text))),
		),
	)
}

func levelRow(colors palette, title string, value float64, accent color.NRGBA) flux.Element {
	return flux.RowElement(
		flux.FixedWidthElement(44, label(colors, title)),
		flux.ExpandedElement(
			flux.ProgressBarElement(
				float32(value*100),
				flux.ProgressMin(0),
				flux.ProgressMax(100),
				flux.ProgressTrackColor(colors.barBase),
				flux.ProgressFillColor(accent),
			),
		),
		flux.HSpacerElement(8),
		flux.FixedWidthElement(44, flux.TextElement(fmt.Sprintf("%02.0f%%", value*100), flux.TextSize(12), flux.TextColor(colors.subtle))),
	)
}

func spectrumBars(colors palette, values []float64) flux.Element {
	const height = float32(92)
	if len(values) == 0 {
		return flux.FixedHeightElement(height, emptyBox(colors, "No audio data"))
	}
	maxBars := 36
	if len(values) < maxBars {
		maxBars = len(values)
	}
	children := make([]flux.Element, 0, maxBars*2)
	for i := 0; i < maxBars; i++ {
		if i > 0 {
			children = append(children, flux.HSpacerElement(3))
		}
		value := clamp01(values[i])
		barHeight := float32(8 + value*float64(height-10))
		barColor := colors.barAccent
		if i%5 == 0 {
			barColor = colors.primary
		}
		children = append(children,
			flux.ExpandedElement(
				flux.FixedHeightElement(
					height,
					flux.ColumnElement(
						flux.SpacerElement(0, height-barHeight),
						flux.ContainerDecorationElement(
							flux.Bg(barColor).WithRad(4),
							flux.FixedHeightElement(barHeight, flux.SpacerElement(0, 0)),
						),
					),
				),
			),
		)
	}
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.All(10)).WithRad(8),
		flux.RowElement(children...),
	)
}

func emptyBox(colors palette, text string) flux.Element {
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.All(12)).WithRad(8),
		flux.TextElement(text, flux.TextSize(12), flux.TextColor(colors.subtle)),
	)
}

func statusChip(colors palette, text string, bg color.NRGBA) flux.Element {
	return flux.ContainerDecorationElement(
		flux.Bg(bg).WithPad(flux.Symmetric(5, 9)).WithRad(999),
		flux.TextElement(text, flux.TextSize(11), flux.TextColor(flux.NRGBA(255, 255, 255, 255))),
	)
}

func playbackProgress(track model.Track, playback model.Playback) float32 {
	if track.Duration <= 0 {
		return 0
	}
	value := float64(playback.Position) / float64(track.Duration)
	return float32(clamp01(value))
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return "00:00"
	}
	total := ms / 1000
	minutes := total / 60
	seconds := total % 60
	if minutes >= 60 {
		hours := minutes / 60
		minutes = minutes % 60
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func formatClock(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return t.Local().Format("15:04:05")
}

func formatBytes(value uint64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	kb := float64(value) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1f KB", kb)
	}
	return fmt.Sprintf("%.1f MB", kb/1024)
}

func blankAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func clipText(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if maxLen <= 3 || len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen-3]) + "..."
}

func clamp01(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func statusColor(colors palette, status string) color.NRGBA {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "connected":
		return colors.success
	case "connecting":
		return colors.warning
	case "error":
		return colors.danger
	default:
		return colors.subtle
	}
}

func appPalette() palette {
	return palette{
		surface:   flux.NRGBA(238, 241, 245, 255),
		panel:     flux.NRGBA(255, 255, 255, 255),
		muted:     flux.NRGBA(244, 247, 250, 255),
		text:      flux.NRGBA(23, 31, 42, 255),
		subtle:    flux.NRGBA(91, 104, 120, 255),
		border:    flux.NRGBA(205, 213, 224, 255),
		primary:   flux.NRGBA(15, 118, 110, 255),
		success:   flux.NRGBA(22, 163, 74, 255),
		warning:   flux.NRGBA(217, 119, 6, 255),
		danger:    flux.NRGBA(220, 38, 38, 255),
		barBase:   flux.NRGBA(226, 232, 240, 255),
		barAccent: flux.NRGBA(37, 99, 235, 255),
	}
}
