package ui

import (
	flux "github.com/xiaowumin-mark/FluxUI/ui"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func logsPage(colors palette, snapshot model.Snapshot) flux.Element {
	logs := snapshot.Logs
	if len(logs) > 50 {
		logs = logs[len(logs)-50:]
	}
	items := make([]flux.Element, 0, len(logs)*2)
	for _, entry := range logs {
		items = append(items,
			flux.ContainerDecorationElement(
				flux.Bg(colors.muted).WithPad(flux.Symmetric(8, 10)).WithRad(8),
				flux.TextElement(clipText(entry, 96), flux.TextSize(12), flux.TextColor(colors.text)),
			),
			flux.VSpacerElement(6),
		)
	}
	if len(items) == 0 {
		items = append(items, emptyBox(colors, "暂无日志"))
	}
	return flux.ScrollViewElement(
		flux.ColumnElement(
			panel(colors,
				sectionTitle(colors, "运行日志"),
				flux.VSpacerElement(6),
				flux.TextElement("仅保留软件自身最新 50 条日志。", flux.TextSize(12), flux.TextColor(colors.subtle)),
				flux.VSpacerElement(12),
				flux.ColumnElement(items...),
			),
		),
		flux.ScrollVertical(true),
	)
}
