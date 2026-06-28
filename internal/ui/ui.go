package ui

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	_ "github.com/xiaowumin-mark/FluxUI/icons/md3"
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
		flux.WithTheme(appTheme()),
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
	path     string
}

var appPages = []pageInfo{
	{key: pageDashboard, label: "仪表盘", title: "仪表盘", subtitle: "当前监听会话的播放状态", icon: "dashboard", path: "/dashboard"},
	{key: pageSessions, label: "会话", title: "会话", subtitle: "SMTC 会话管理", icon: "devices", path: "/sessions"},
	{key: pageSongs, label: "歌曲", title: "歌曲", subtitle: "播放记录与歌词管理", icon: "library_music", path: "/songs"},
	{key: pageLogs, label: "日志", title: "日志", subtitle: "软件运行记录", icon: "article", path: "/logs"},
	{key: pageSettings, label: "设置", title: "设置", subtitle: "连接、媒体和歌词偏好", icon: "settings", path: "/settings"},
}

type palette struct {
	surface            color.NRGBA
	panel              color.NRGBA
	muted              color.NRGBA
	text               color.NRGBA
	subtle             color.NRGBA
	border             color.NRGBA
	primary            color.NRGBA
	onPrimary          color.NRGBA
	primaryContainer   color.NRGBA
	onPrimaryContainer color.NRGBA
	success            color.NRGBA
	onSuccess          color.NRGBA
	warning            color.NRGBA
	onWarning          color.NRGBA
	danger             color.NRGBA
	onDanger           color.NRGBA
	barBase            color.NRGBA
	barAccent          color.NRGBA
}

func root(ctx *flux.Context, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	colors := appPalette(flux.UseTheme(ctx))
	snapshot := useStoreSnapshot(ctx, store)
	urlState := flux.UseState(ctx, snapshot.Config.AMLL.URL)
	notice := flux.UseState(ctx, "")

	return flux.ContainerDecorationElement(
		flux.Bg(colors.surface),
		appShell(ctx, colors, snapshot, urlState, notice, store, runtime),
	)
}

func useStoreSnapshot(ctx *flux.Context, store *storepkg.Store) model.Snapshot {
	snapshotState := flux.UseState(ctx, store.Snapshot())
	flux.UseEffectWithDeps(ctx, []any{store}, func() func() {
		events, unsubscribe := store.Subscribe(128)
		done := make(chan struct{})

		snapshotState.Set(store.Snapshot())
		go func() {
			lastRealtimeUI := time.Time{}
			for {
				select {
				case event, ok := <-events:
					if !ok {
						return
					}
					if event.Type == "playback_progress" || event.Type == "audio_frame" {
						now := time.Now()
						if !lastRealtimeUI.IsZero() && now.Sub(lastRealtimeUI) < 100*time.Millisecond {
							continue
						}
						lastRealtimeUI = now
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
	urlState stringState,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	currentPath := flux.CurrentPath(ctx)
	page := currentPageForPath(currentPath)
	compact := ctx.MaxConstraints().X < 880
	content := flux.ContainerDecorationElement(
		flux.Bg(colors.surface).WithPad(flux.All(18)),
		flux.ColumnElement(
			pageHeader(colors, snapshot, page, notice.Value()),
			flux.VSpacerElement(14),
			flux.ExpandedElement(appRouter(colors, snapshot, urlState, notice, store, runtime, compact)),
		),
	)

	return flux.RowElement(
		navRail(colors, snapshot, page.key),
		flux.ExpandedElement(content),
	)
}

func appRouter(
	colors palette,
	snapshot model.Snapshot,
	urlState stringState,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
	compact bool,
) flux.Element {
	return flux.RouterElement(
		flux.RouteElement("/dashboard", func(routeCtx *flux.Context) flux.Element {
			waveform := useStableWaveform(routeCtx, snapshot.Audio.Spectrum, waveformBarCount)
			return dashboardPage(colors, snapshot, notice, runtime, compact, waveform)
		}, flux.RouteName(pageDashboard), flux.RouteTitle("仪表盘")),
		flux.RouteElement("/sessions", func(routeCtx *flux.Context) flux.Element {
			return sessionsPage(colors, snapshot, notice, runtime)
		}, flux.RouteName(pageSessions), flux.RouteTitle("会话")),
		flux.RouteElement("/songs", func(routeCtx *flux.Context) flux.Element {
			return songsPage(routeCtx, colors, snapshot, notice, store, runtime)
		}, flux.RouteName(pageSongs), flux.RouteTitle("歌曲")),
		flux.RouteElement("/songs/all", func(routeCtx *flux.Context) flux.Element {
			return songsPage(routeCtx, colors, snapshot, notice, store, runtime)
		}, flux.RouteName("songs-all"), flux.RouteTitle("全部歌曲")),
		flux.RouteElement("/songs/new", func(routeCtx *flux.Context) flux.Element {
			return songsPage(routeCtx, colors, snapshot, notice, store, runtime)
		}, flux.RouteName("songs-new"), flux.RouteTitle("新增歌曲")),
		flux.RouteElement("/songs/:id/edit", func(routeCtx *flux.Context) flux.Element {
			return songsPage(routeCtx, colors, snapshot, notice, store, runtime)
		}, flux.RouteName("song-edit"), flux.RouteTitle("编辑歌曲")),
		flux.RouteElement("/songs/:id/lyrics", func(routeCtx *flux.Context) flux.Element {
			return songsPage(routeCtx, colors, snapshot, notice, store, runtime)
		}, flux.RouteName("song-lyrics"), flux.RouteTitle("歌词")),
		flux.RouteElement("/songs/:id", func(routeCtx *flux.Context) flux.Element {
			return songsPage(routeCtx, colors, snapshot, notice, store, runtime)
		}, flux.RouteName("song-detail"), flux.RouteTitle("歌曲详情")),
		flux.RouteElement("/logs", func(routeCtx *flux.Context) flux.Element {
			return logsPage(colors, snapshot)
		}, flux.RouteName(pageLogs), flux.RouteTitle("日志")),
		flux.RouteElement("/settings", func(routeCtx *flux.Context) flux.Element {
			return settingsPage(colors, snapshot, urlState, notice, store, runtime)
		}, flux.RouteName(pageSettings), flux.RouteTitle("设置")),
	).With(
		flux.RouterNotFoundElement(appNotFoundPage(colors)),
	)
}

func navRail(colors palette, snapshot model.Snapshot, activePage string) flux.Element {
	return flux.NavigationRailElement(
		activePage,
		navItems(),
		flux.NavigationRailWidth(92),
		flux.NavigationRailHeader(flux.Text("LS", flux.TextSize(16))),
		flux.NavigationRailFooter(flux.Text(strings.ToUpper(blankAs(snapshot.AMLL.Status, "off")), flux.TextSize(10))),
		flux.NavigationRailActiveColor(colors.primary),
		flux.NavigationRailInactiveColor(colors.subtle),
		flux.NavigationRailDecoration(flux.Bg(colors.muted).WithBorder(flux.Border{Width: 1, Color: colors.border})),
		flux.NavigationRailOnChange(func(ctx *flux.Context, key string) {
			if path := pagePath(key); path != "" {
				flux.NavigateReplace(ctx, path, flux.WithNavTransition(flux.TransitionFade))
			}
		}),
	)
}

func navItems() []flux.ElementNavItem {
	items := make([]flux.ElementNavItem, 0, len(appPages))
	for _, page := range appPages {
		items = append(items, flux.ElementNavItem{
			Key:   page.key,
			Label: page.label,
			Icon:  flux.IconElement(page.icon, flux.IconSize(22)),
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

func currentPageForPath(path string) pageInfo {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return currentPage(pageDashboard)
	}
	for _, page := range appPages {
		if path == page.path || strings.HasPrefix(path, page.path+"/") {
			return page
		}
	}
	return currentPage(pageDashboard)
}

func pagePath(key string) string {
	for _, page := range appPages {
		if page.key == key {
			return page.path
		}
	}
	return ""
}

func appNotFoundPage(colors palette) flux.Component {
	return func(ctx *flux.Context) flux.Element {
		return flux.CenterElement(
			flux.ColumnElement(
				flux.IconElement("error", flux.IconSize(32), flux.IconColor(colors.danger)),
				flux.VSpacerElement(10),
				flux.TextElement("页面不存在", flux.TextSize(18), flux.TextColor(colors.text), flux.TextAlign(flux.AlignCenter)),
				flux.VSpacerElement(8),
				flux.TextElement(blankAs(flux.CurrentPath(ctx), "-"), flux.TextSize(12), flux.TextColor(colors.subtle), flux.TextAlign(flux.AlignCenter)),
				flux.VSpacerElement(14),
				secondaryButton(colors, "返回仪表盘", func(ctx *flux.Context) {
					flux.NavigateReplace(ctx, "/dashboard", flux.WithNavTransition(flux.TransitionSlideRight))
				}),
			),
		)
	}
}

func pageHeader(colors palette, snapshot model.Snapshot, page pageInfo, notice string) flux.Element {
	statusColor := statusColor(colors, snapshot.AMLL.Status)
	subtitle := fmt.Sprintf("AMLL %s | %s | %s", snapshot.AMLL.Status, snapshot.Playback.State, formatClock(snapshot.UpdatedAt))
	if strings.TrimSpace(notice) != "" {
		subtitle = notice
	}
	return flux.FilledCardElement(
		flux.RowElement(
			flux.ColumnElement(
				flux.TextElement(page.title, flux.TextSize(24), flux.TextColor(colors.text)),
				flux.VSpacerElement(4),
				flux.TextElement(clipText(page.subtitle+" | "+subtitle, 108), flux.TextSize(12), flux.TextColor(colors.subtle)),
			),
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			statusChip(colors, strings.ToUpper(snapshot.AMLL.Status), statusColor),
		),
		flux.CardPadding(flux.Symmetric(14, 16)),
		flux.CardRadius(8),
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

func panel(colors palette, children ...flux.Element) flux.Element {
	return flux.OutlinedCardElement(
		flux.ColumnElement(children...),
		flux.CardPadding(flux.All(14)),
		flux.CardRadius(8),
	)
}

func sectionTitle(colors palette, text string) flux.Element {
	return flux.TextElement(text, flux.TextSize(16), flux.TextColor(colors.text))
}

func label(colors palette, text string) flux.Element {
	return flux.TextElement(text, flux.TextSize(12), flux.TextColor(colors.subtle))
}

func primaryButton(colors palette, text string, onClick func(ctx *flux.Context)) flux.Element {
	return flux.FilledButtonElement(
		flux.TextElement(text),
		flux.ButtonPadding(flux.Symmetric(8, 12)),
		flux.OnClick(onClick),
	)
}

func secondaryButton(colors palette, text string, onClick func(ctx *flux.Context)) flux.Element {
	return flux.FilledTonalButtonElement(
		flux.TextElement(text),
		flux.ButtonPadding(flux.Symmetric(8, 12)),
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
		flux.TextElement(text, flux.TextSize(11), flux.TextColor(statusForeground(colors, bg))),
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

func appTheme() *flux.Theme {
	return flux.NewTheme(flux.LightColors())
}

func appPalette(th *flux.Theme) palette {
	if th == nil {
		th = appTheme()
	}
	cs := th.Colors
	return palette{
		surface:            cs.SurfaceContainerLowest,
		panel:              cs.Surface,
		muted:              cs.SurfaceContainerHigh,
		text:               cs.OnSurface,
		subtle:             cs.OnSurfaceVariant,
		border:             cs.OutlineVariant,
		primary:            cs.Primary,
		onPrimary:          cs.OnPrimary,
		primaryContainer:   cs.PrimaryContainer,
		onPrimaryContainer: cs.OnPrimaryContainer,
		success:            cs.Success,
		onSuccess:          cs.OnSuccess,
		warning:            cs.Warning,
		onWarning:          cs.OnWarning,
		danger:             cs.Error,
		onDanger:           cs.OnError,
		barBase:            cs.SurfaceContainerHighest,
		barAccent:          cs.Primary,
	}
}

func statusForeground(colors palette, bg color.NRGBA) color.NRGBA {
	switch bg {
	case colors.success:
		return colors.onSuccess
	case colors.warning:
		return colors.onWarning
	case colors.danger:
		return colors.onDanger
	case colors.primary:
		return colors.onPrimary
	case colors.primaryContainer:
		return colors.onPrimaryContainer
	default:
		return colors.panel
	}
}
