package ui

import (
	"strings"

	flux "github.com/xiaowumin-mark/FluxUI/ui"

	"github.com/xiaowumin-mark/LyricSync/internal/app"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const autoSessionValue = "__auto__"

func sessionsPage(colors palette, snapshot model.Snapshot, notice stringState, runtime *app.Runtime) flux.Element {
	return flux.ScrollViewElement(
		flux.ColumnElement(
			sessionOverviewPanel(colors, snapshot),
			flux.VSpacerElement(12),
			sessionSelectionPanel(colors, snapshot, notice, runtime),
		),
		flux.ScrollVertical(true),
	)
}

func sessionOverviewPanel(colors palette, snapshot model.Snapshot) flux.Element {
	return panel(colors,
		sectionTitle(colors, "当前会话"),
		flux.VSpacerElement(10),
		infoLine(colors, "模式", sessionMode(snapshot)),
		infoLine(colors, "已选择", blankAs(snapshot.Config.Media.SelectedSessionID, "自动")),
		infoLine(colors, "当前", blankAs(snapshot.Track.ID, "-")),
		infoLine(colors, "状态", selectedSessionState(snapshot)),
	)
}

func sessionSelectionPanel(
	colors palette,
	snapshot model.Snapshot,
	notice interface {
		Value() string
		Set(string)
	},
	runtime *app.Runtime,
) flux.Element {
	selectedValue := sessionSelectValue(snapshot)
	if selectedValue == "" {
		selectedValue = autoSessionValue
	}

	rows := []flux.Element{
		sessionChoiceCard(
			colors,
			selectedValue,
			autoSessionValue,
			"自动选择",
			"优先保持当前正在播放的会话",
			"",
			"auto",
			0,
			0,
			false,
			func(ctx *flux.Context) { selectSession(notice, runtime, "") },
		),
		flux.VSpacerElement(8),
	}

	if missing := missingSelectedSession(snapshot); missing != "" {
		rows = append(rows,
			sessionChoiceCard(
				colors,
				selectedValue,
				missing,
				"等待选定会话恢复",
				missing,
				"",
				"missing",
				0,
				0,
				false,
				func(ctx *flux.Context) { selectSession(notice, runtime, missing) },
			),
			flux.VSpacerElement(8),
		)
	}

	for _, session := range snapshot.Sessions {
		rows = append(rows,
			sessionCard(colors, snapshot, session, notice, runtime),
			flux.VSpacerElement(8),
		)
	}
	if len(snapshot.Sessions) == 0 {
		rows = append(rows, emptyBox(colors, "暂无媒体会话"))
	}

	return flux.ColumnElement(
		sectionTitle(colors, "监听会话"),
		flux.VSpacerElement(12),
		flux.ColumnElement(rows...),
	)
}

func sessionCard(
	colors palette,
	snapshot model.Snapshot,
	session model.Session,
	notice interface {
		Value() string
		Set(string)
	},
	runtime *app.Runtime,
) flux.Element {
	selectedValue := sessionSelectValue(snapshot)
	if selectedValue == "" {
		selectedValue = autoSessionValue
	}
	return sessionChoiceCard(
		colors,
		selectedValue,
		session.ID,
		blankAs(session.Name, session.ID),
		clipText(sessionTrackLine(session), 92),
		sessionArtistAlbum(session),
		sessionStatusLabel(session),
		session.Position,
		session.Duration,
		session.Active,
		func(ctx *flux.Context) { selectSession(notice, runtime, session.ID) },
	)
}

func sessionChoiceCard(
	colors palette,
	selectedValue string,
	value string,
	title string,
	subtitle string,
	meta string,
	status string,
	position int64,
	duration int64,
	selected bool,
	onSelect func(ctx *flux.Context),
) flux.Element {
	bg := colors.muted
	border := flux.Border{Width: 1, Color: colors.border}
	if selectedValue == value {
		bg = colors.primaryContainer
		border = flux.Border{Width: 1, Color: colors.primary}
	}
	statusColor := colors.subtle
	if status == "missing" {
		statusColor = colors.warning
	}
	if selected {
		statusColor = colors.success
	}

	content := flux.RowElement(
		flux.FixedWidthElement(
			56,
			flux.RadioGroupElement(
				selectedValue,
				[]flux.RadioItem{{Label: "", Value: value}},
				flux.RadioGroupColor(colors.primary),
				flux.RadioGroupOnChange(func(ctx *flux.Context, next string) {
					if next == value {
						onSelect(ctx)
					}
				}),
			),
		),
		flux.HSpacerElement(8),
		flux.ExpandedElement(
			flux.ColumnElement(
				flux.RowElement(
					flux.ExpandedElement(flux.TextElement(clipText(title, 74), flux.TextSize(14), flux.TextColor(colors.text))),
					flux.HSpacerElement(8),
					statusChip(colors, status, statusColor),
				),
				flux.VSpacerElement(4),
				flux.TextElement(blankAs(subtitle, "-"), flux.TextSize(12), flux.TextColor(colors.subtle)),
				sessionMetaLine(colors, meta),
				sessionProgressLine(colors, position, duration, selectedValue == value),
			),
		),
	)

	return flux.ContainerDecorationElement(
		flux.Bg(bg).WithPad(flux.All(12)).WithRad(8).WithBorder(border),
		content,
	)
}

func sessionMetaLine(colors palette, meta string) flux.Element {
	if strings.TrimSpace(meta) == "" {
		return flux.SpacerElement(0, 0)
	}
	return flux.ColumnElement(
		flux.VSpacerElement(4),
		flux.TextElement(clipText(meta, 92), flux.TextSize(11), flux.TextColor(colors.subtle)),
	)
}

func sessionProgressLine(colors palette, position int64, duration int64, selected bool) flux.Element {
	if duration <= 0 {
		return flux.SpacerElement(0, 0)
	}
	fill := colors.subtle
	if selected {
		fill = colors.primary
	}
	progress := float32(clamp01(float64(position) / float64(duration)))
	return flux.ColumnElement(
		flux.VSpacerElement(8),
		flux.ProgressBarElement(
			progress*100,
			flux.ProgressMin(0),
			flux.ProgressMax(100),
			flux.ProgressTrackColor(colors.barBase),
			flux.ProgressFillColor(fill),
		),
		flux.VSpacerElement(5),
		flux.RowElement(
			flux.TextElement(formatMillis(position), flux.TextSize(11), flux.TextColor(colors.subtle)),
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			flux.TextElement(formatMillis(duration), flux.TextSize(11), flux.TextColor(colors.subtle)),
		),
	)
}

func selectSession(
	notice interface {
		Value() string
		Set(string)
	},
	runtime *app.Runtime,
	sessionID string,
) {
	if err := runtime.SelectMediaSession(sessionID); err != nil {
		notice.Set("SMTC selection failed: " + err.Error())
		return
	}
	if sessionID == "" {
		notice.Set("SMTC auto selection enabled")
		return
	}
	notice.Set("SMTC locked to selected session")
}

func sessionSelectValue(snapshot model.Snapshot) string {
	selected := strings.TrimSpace(snapshot.Config.Media.SelectedSessionID)
	if selected == "" {
		return ""
	}
	for _, session := range snapshot.Sessions {
		if selected == session.ID || selected == session.AppID {
			return session.ID
		}
	}
	return selected
}

func missingSelectedSession(snapshot model.Snapshot) string {
	selected := strings.TrimSpace(snapshot.Config.Media.SelectedSessionID)
	if selected == "" {
		return ""
	}
	for _, session := range snapshot.Sessions {
		if selected == session.ID || selected == session.AppID {
			return ""
		}
	}
	return selected
}

func sessionTrackLine(session model.Session) string {
	title := strings.TrimSpace(session.Title)
	if title == "" {
		title = "未提供标题"
	}
	return title
}

func sessionStatusLabel(session model.Session) string {
	if session.Active {
		return "active"
	}
	if strings.TrimSpace(session.State) != "" {
		return session.State
	}
	return "idle"
}

func sessionArtistAlbum(session model.Session) string {
	artist := strings.TrimSpace(session.Artist)
	album := strings.TrimSpace(session.Album)
	switch {
	case artist != "" && album != "":
		return artist + " - " + album
	case artist != "":
		return artist
	case album != "":
		return album
	default:
		return blankAs(session.AppID, "-")
	}
}

func sessionMode(snapshot model.Snapshot) string {
	if strings.TrimSpace(snapshot.Config.Media.SelectedSessionID) != "" {
		return "手动"
	}
	if snapshot.Config.Media.AutoSelect {
		return "自动"
	}
	return "锁定"
}

func selectedSessionState(snapshot model.Snapshot) string {
	selected := strings.TrimSpace(snapshot.Config.Media.SelectedSessionID)
	if selected == "" {
		return "自动选择中"
	}
	for _, session := range snapshot.Sessions {
		if selected == session.ID || selected == session.AppID {
			if session.Active {
				return "正在监听"
			}
			return "已选择"
		}
	}
	return "等待选定会话恢复"
}
