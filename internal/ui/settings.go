package ui

import (
	"context"
	"strconv"
	"strings"

	flux "github.com/xiaowumin-mark/FluxUI/ui"

	"github.com/xiaowumin-mark/LyricSync/internal/app"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	storepkg "github.com/xiaowumin-mark/LyricSync/internal/state"
)

func settingsPage(
	colors palette,
	snapshot model.Snapshot,
	urlState stringState,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	_ = urlState
	return flux.ScrollViewElement(
		flux.ColumnElement(
			amllSettingsPanel(colors, snapshot, notice, store, runtime),
			flux.VSpacerElement(12),
			lyricsSettingsPanel(colors, snapshot, notice, store, runtime),
			flux.VSpacerElement(12),
			aiSettingsPanel(colors, snapshot, notice, store, runtime),
			flux.VSpacerElement(12),
			ttmlDBSettingsPanel(colors, snapshot, notice, store, runtime),
			flux.VSpacerElement(12),
			appSettingsPanel(colors, snapshot, notice, store, runtime),
		),
		flux.ScrollVertical(true),
	)
}

func amllSettingsPanel(colors palette, snapshot model.Snapshot, notice stringState, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	cfg := snapshot.Config
	connect := func(ctx *flux.Context) {
		url := strings.TrimSpace(cfg.AMLL.URL)
		if err := runtime.ConnectAMLL(url); err != nil {
			message := "AMLL 连接失败: " + err.Error()
			notice.Set(message)
			store.AddLog(message)
			return
		}
		notice.Set("正在连接 AMLL")
	}
	disconnect := func(ctx *flux.Context) {
		if err := runtime.DisconnectAMLL(); err != nil {
			message := "AMLL 断开失败: " + err.Error()
			notice.Set(message)
			store.AddLog(message)
			return
		}
		notice.Set("AMLL 已断开")
	}
	sendSnapshot := func(ctx *flux.Context) {
		if err := runtime.SendSnapshot(); err != nil {
			message := "AMLL 快照发送失败: " + err.Error()
			notice.Set(message)
			store.AddLog(message)
			return
		}
		notice.Set("AMLL 快照已发送")
	}

	return panel(colors,
		sectionTitle(colors, "AMLL"),
		flux.VSpacerElement(12),
		settingTextField(colors, "WebSocket 地址", cfg.AMLL.URL, "ws://127.0.0.1:11444", false, func(ctx *flux.Context, value string) {
			next := cfg
			next.AMLL.URL = strings.TrimSpace(value)
			saveSettingsConfig(notice, store, runtime, next, "AMLL 地址已保存")
		}),
		flux.VSpacerElement(10),
		settingSwitch(colors, "启动后自动连接", "应用启动后自动连接 AMLL WebSocket。", cfg.AMLL.AutoConnect, func(ctx *flux.Context, checked bool) {
			next := cfg
			next.AMLL.AutoConnect = checked
			saveSettingsConfig(notice, store, runtime, next, "启动自动连接设置已保存")
		}),
		flux.VSpacerElement(8),
		settingSwitch(colors, "发送音频数据", "控制是否通过 AMLL 发送音频波形数据。", cfg.AMLL.SendAudio, func(ctx *flux.Context, checked bool) {
			if err := runtime.ToggleSendAudio(checked); err != nil {
				message := "音频发送设置保存失败: " + err.Error()
				notice.Set(message)
				store.AddLog(message)
				return
			}
			notice.Set("音频发送设置已保存")
		}),
		flux.VSpacerElement(12),
		flux.RowElement(
			flux.ExpandedElement(primaryButton(colors, "连接", connect)),
			flux.HSpacerElement(8),
			flux.ExpandedElement(secondaryButton(colors, "断开", disconnect)),
			flux.HSpacerElement(8),
			flux.ExpandedElement(secondaryButton(colors, "发送快照", sendSnapshot)),
		),
		flux.VSpacerElement(12),
		infoLine(colors, "状态", snapshot.AMLL.Status),
		infoLine(colors, "消息", clipText(snapshot.AMLL.Message, 42)),
		infoLine(colors, "最后接收", formatClock(snapshot.AMLL.LastMessageAt)),
	)
}

func lyricsSettingsPanel(colors palette, snapshot model.Snapshot, notice stringState, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	cfg := snapshot.Config
	priority := completeLyricPriority(cfg.Lyrics.SearchPriority)
	children := []flux.Element{
		sectionTitle(colors, "歌词"),
		flux.VSpacerElement(12),
		lyricPriorityList(colors, cfg, priority, notice, store, runtime),
		flux.VSpacerElement(12),
	}
	children = append(children,
		settingSelect(colors, "歌词清洗", cfg.Lyrics.CleanStrategy, cleanStrategyOptions(), func(ctx *flux.Context, value string) {
			next := cfg
			next.Lyrics.CleanStrategy = value
			saveSettingsConfig(notice, store, runtime, next, "歌词清洗策略已保存")
		}),
		flux.VSpacerElement(10),
		settingSwitch(colors, "AI 翻译", "启用后由后续 AI 流程生成歌词翻译。", cfg.Lyrics.AITranslate, func(ctx *flux.Context, checked bool) {
			next := cfg
			next.Lyrics.AITranslate = checked
			saveSettingsConfig(notice, store, runtime, next, "AI 翻译设置已保存")
		}),
		flux.VSpacerElement(8),
		settingSwitch(colors, "AI 音译", "仅针对非简体中文以及英语歌词生成。", cfg.Lyrics.AITransliterate, func(ctx *flux.Context, checked bool) {
			next := cfg
			next.Lyrics.AITransliterate = checked
			saveSettingsConfig(notice, store, runtime, next, "AI 音译设置已保存")
		}),
	)
	return panel(colors, children...)
}

func lyricPriorityList(colors palette, cfg model.Config, priority []string, notice stringState, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	items := make([]flux.Element, 0, len(priority)*2+2)
	items = append(items,
		label(colors, "搜词优先级"),
		flux.VSpacerElement(6),
	)
	for index, source := range priority {
		if index > 0 {
			items = append(items, flux.VSpacerElement(8))
		}
		source := source
		index := index
		items = append(items, flux.Key("lyric-priority-"+source,
			lyricPriorityRow(colors, cfg, priority, source, index, notice, store, runtime),
		))
	}
	return flux.ColumnElement(items...)
}

func lyricPriorityRow(
	colors palette,
	cfg model.Config,
	priority []string,
	source string,
	index int,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	move := func(direction int) func(ctx *flux.Context) {
		return func(ctx *flux.Context) {
			nextPriority := moveLyricPriority(priority, source, direction)
			if sameStringSlice(nextPriority, priority) {
				return
			}
			next := cfg
			next.Lyrics.SearchPriority = nextPriority
			saveSettingsConfig(notice, store, runtime, next, "搜词优先级已保存")
		}
	}
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.Symmetric(10, 12)).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border}),
		flux.RowElement(
			flux.FixedWidthElement(26,
				flux.TextElement(strconv.Itoa(index+1), flux.TextSize(13), flux.TextColor(colors.subtle)),
			),
			flux.HSpacerElement(8),
			flux.IconElement("sort", flux.IconSize(20), flux.IconColor(colors.subtle)),
			flux.HSpacerElement(10),
			flux.ExpandedElement(
				flux.ColumnElement(
					flux.TextElement(lyricSourceLabel(source), flux.TextSize(13), flux.TextColor(colors.text)),
					flux.VSpacerElement(2),
					flux.TextElement(lyricSourceDescription(source), flux.TextSize(11), flux.TextColor(colors.subtle)),
				),
			),
			flux.HSpacerElement(10),
			lyricPriorityIconButton(colors, "上移", "keyboard_arrow_up", index == 0, move(-1)),
			flux.HSpacerElement(4),
			lyricPriorityIconButton(colors, "下移", "keyboard_arrow_down", index == len(priority)-1, move(1)),
		),
	)
}

func lyricPriorityIconButton(colors palette, tooltip string, icon string, disabled bool, onClick func(ctx *flux.Context)) flux.Element {
	return flux.TooltipElement(
		tooltip,
		flux.IconButtonElement(
			flux.IconElement(icon, flux.IconSize(20)),
			flux.IconButtonSize(34),
			flux.IconButtonDisabled(disabled),
			flux.IconButtonForeground(colors.primary),
			flux.IconButtonOnClick(onClick),
		),
	)
}

func aiSettingsPanel(colors palette, snapshot model.Snapshot, notice stringState, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	cfg := snapshot.Config
	modelControl := flux.VSpacerElement(0)
	if len(cfg.AI.Models) > 0 {
		modelControl = flux.ColumnElement(
			flux.VSpacerElement(10),
			settingSelect(colors, "AI 模型", cfg.AI.Model, aiModelOptions(cfg.AI.Models, cfg.AI.Model), func(ctx *flux.Context, value string) {
				next := cfg
				next.AI.Model = strings.TrimSpace(value)
				saveSettingsConfig(notice, store, runtime, next, "AI 模型已保存")
			}),
		)
	}
	return panel(colors,
		sectionTitle(colors, "AI"),
		modelControl,
		flux.VSpacerElement(8),
		secondaryButton(colors, "获取模型", func(ctx *flux.Context) {
			notice.Set("正在获取 AI 模型")
			go func() {
				models, err := runtime.FetchAIModels(context.Background())
				if err != nil {
					message := "AI 模型获取失败: " + err.Error()
					notice.Set(message)
					store.AddLog(message)
					return
				}
				notice.Set("AI 模型已更新: " + strconv.Itoa(len(models)))
			}()
		}),
		flux.VSpacerElement(10),
		settingSelect(colors, "AI 超时时间", strconv.Itoa(aiTimeoutSeconds(cfg.AI.TimeoutSeconds)), aiTimeoutOptions(), func(ctx *flux.Context, value string) {
			seconds, err := strconv.Atoi(value)
			if err != nil {
				return
			}
			next := cfg
			next.AI.TimeoutSeconds = seconds
			saveSettingsConfig(notice, store, runtime, next, "AI 超时时间已保存")
		}),
		flux.VSpacerElement(12),
		settingTextField(colors, "服务商地址", cfg.AI.BaseURL, "https://api.openai.com/v1", false, func(ctx *flux.Context, value string) {
			next := cfg
			next.AI.BaseURL = strings.TrimSpace(value)
			saveSettingsConfig(notice, store, runtime, next, "AI 服务商地址已保存")
		}),
		flux.VSpacerElement(10),
		settingTextField(colors, "API Key", cfg.AI.APIKey, "sk-...", true, func(ctx *flux.Context, value string) {
			next := cfg
			next.AI.APIKey = strings.TrimSpace(value)
			saveSettingsConfig(notice, store, runtime, next, "AI API Key 已保存")
		}),
		flux.VSpacerElement(10),
		settingSwitch(colors, "深度思考", "启用后由支持该能力的 AI 模型使用更强推理模式。", cfg.AI.DeepThinking, func(ctx *flux.Context, checked bool) {
			next := cfg
			next.AI.DeepThinking = checked
			saveSettingsConfig(notice, store, runtime, next, "AI 深度思考设置已保存")
		}),
	)
}

func ttmlDBSettingsPanel(colors palette, snapshot model.Snapshot, notice stringState, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	cfg := snapshot.Config
	interval := cfg.TTMLDB.UpdateIntervalHours
	if interval <= 0 {
		interval = 24
	}
	return panel(colors,
		sectionTitle(colors, "TTML DB"),
		flux.VSpacerElement(12),
		settingSwitch(colors, "自动更新索引", "默认每天更新一次 TTML DB 索引。", cfg.TTMLDB.AutoUpdateIndex, func(ctx *flux.Context, checked bool) {
			next := cfg
			next.TTMLDB.AutoUpdateIndex = checked
			saveSettingsConfig(notice, store, runtime, next, "TTML DB 自动更新设置已保存")
		}),
		flux.VSpacerElement(10),
		settingSelect(colors, "更新间隔", strconv.Itoa(interval), ttmlIntervalOptions(), func(ctx *flux.Context, value string) {
			hours, err := strconv.Atoi(value)
			if err != nil || hours <= 0 {
				return
			}
			next := cfg
			next.TTMLDB.UpdateIntervalHours = hours
			saveSettingsConfig(notice, store, runtime, next, "TTML DB 更新间隔已保存")
		}),
		flux.VSpacerElement(10),
		settingTextField(colors, "索引地址", cfg.TTMLDB.IndexURL, "https://amlldb.bikonoo.com/metadata/raw-lyrics-index.jsonl", false, func(ctx *flux.Context, value string) {
			next := cfg
			next.TTMLDB.IndexURL = strings.TrimSpace(value)
			saveSettingsConfig(notice, store, runtime, next, "TTML DB 索引地址已保存")
		}),
		flux.VSpacerElement(12),
		secondaryButton(colors, "立即更新索引", func(ctx *flux.Context) {
			notice.Set("TTML DB 索引正在更新")
			go func() {
				if err := runtime.UpdateTTMLDBIndex(context.Background()); err != nil {
					message := "TTML DB 索引更新失败: " + err.Error()
					notice.Set(message)
					store.AddLog(message)
					return
				}
				notice.Set("TTML DB 索引已更新")
			}()
		}),
	)
}

func appSettingsPanel(colors palette, snapshot model.Snapshot, notice stringState, store *storepkg.Store, runtime *app.Runtime) flux.Element {
	cfg := snapshot.Config
	return panel(colors,
		sectionTitle(colors, "应用"),
		flux.VSpacerElement(12),
		settingSelect(colors, "关闭行为", cfg.App.CloseBehavior, closeBehaviorOptions(), func(ctx *flux.Context, value string) {
			next := cfg
			next.App.CloseBehavior = value
			saveSettingsConfig(notice, store, runtime, next, "关闭行为已保存")
		}),
	)
}

func settingTextField(colors palette, title string, value string, placeholder string, password bool, onChange func(ctx *flux.Context, value string)) flux.Element {
	return flux.ColumnElement(
		label(colors, title),
		flux.VSpacerElement(6),
		flux.OutlinedTextFieldElement(
			value,
			flux.InputPlaceholder(placeholder),
			flux.InputSingleLine(true),
			flux.InputPassword(password),
			flux.InputOnChange(onChange),
		),
	)
}

func settingSwitch(colors palette, title string, subtitle string, checked bool, onChange func(ctx *flux.Context, checked bool)) flux.Element {
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.All(12)).WithRad(8),
		flux.RowElement(
			flux.ExpandedElement(
				flux.ColumnElement(
					flux.TextElement(title, flux.TextSize(13), flux.TextColor(colors.text)),
					flux.VSpacerElement(3),
					flux.TextElement(subtitle, flux.TextSize(11), flux.TextColor(colors.subtle)),
				),
			),
			flux.HSpacerElement(12),
			flux.SwitchElement(checked, flux.SwitchOnChange(onChange)),
		),
	)
}

func settingSelect(colors palette, title string, value string, options []flux.SelectOptionItem[string], onChange func(ctx *flux.Context, value string)) flux.Element {
	return flux.ColumnElement(
		label(colors, title),
		flux.VSpacerElement(6),
		flux.SelectElement[string](
			value,
			options,
			flux.SelectMaxHeight[string](220),
			flux.SelectOnChange[string](onChange),
		),
	)
}

func saveSettingsConfig(notice stringState, store *storepkg.Store, runtime *app.Runtime, cfg model.Config, success string) {
	if runtime == nil {
		notice.Set("设置保存失败: runtime 不可用")
		return
	}
	if err := runtime.SaveConfig(cfg); err != nil {
		message := "设置保存失败: " + err.Error()
		notice.Set(message)
		if store != nil {
			store.AddLog(message)
		}
		return
	}
	notice.Set(success)
}

func lyricSourceOptions() []flux.SelectOptionItem[string] {
	return []flux.SelectOptionItem[string]{
		{Label: "TTML DB", Value: model.LyricSourceTTMLDB},
		{Label: "QQ 音乐", Value: model.LyricSourceQQ},
		{Label: "酷狗音乐", Value: model.LyricSourceKugou},
		{Label: "网易云音乐", Value: model.LyricSourceNetease},
		{Label: "自定义歌词", Value: model.LyricSourceCustom},
	}
}

func lyricSourceLabel(source string) string {
	switch source {
	case model.LyricSourceTTMLDB:
		return "TTML DB"
	case model.LyricSourceQQ:
		return "QQ 音乐"
	case model.LyricSourceKugou:
		return "酷狗音乐"
	case model.LyricSourceNetease:
		return "网易云音乐"
	case model.LyricSourceCustom:
		return "自定义歌词"
	default:
		return source
	}
}

func lyricSourceDescription(source string) string {
	switch source {
	case model.LyricSourceTTMLDB:
		return "优先使用本地索引匹配的 TTML 歌词"
	case model.LyricSourceQQ:
		return "QQ 音乐歌词来源"
	case model.LyricSourceKugou:
		return "酷狗音乐歌词来源"
	case model.LyricSourceNetease:
		return "网易云音乐歌词来源"
	case model.LyricSourceCustom:
		return "歌曲数据库中的自定义 TTML 歌词"
	default:
		return "歌词来源"
	}
}

func cleanStrategyOptions() []flux.SelectOptionItem[string] {
	return []flux.SelectOptionItem[string]{
		{Label: "不处理", Value: model.LyricsCleanNone},
		{Label: "软件处理", Value: model.LyricsCleanSoftware},
		{Label: "AI 处理", Value: model.LyricsCleanAI},
	}
}

func closeBehaviorOptions() []flux.SelectOptionItem[string] {
	return []flux.SelectOptionItem[string]{
		{Label: "直接退出", Value: model.CloseBehaviorExit},
		{Label: "最小化到系统托盘", Value: model.CloseBehaviorTray},
	}
}

func ttmlIntervalOptions() []flux.SelectOptionItem[string] {
	return []flux.SelectOptionItem[string]{
		{Label: "每 6 小时", Value: "6"},
		{Label: "每 12 小时", Value: "12"},
		{Label: "每天", Value: "24"},
		{Label: "每 2 天", Value: "48"},
	}
}

func aiModelOptions(models []string, current string) []flux.SelectOptionItem[string] {
	seen := map[string]struct{}{}
	out := make([]flux.SelectOptionItem[string], 0, len(models)+1)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, flux.SelectOptionItem[string]{Label: value, Value: value})
	}
	add(current)
	for _, modelName := range models {
		add(modelName)
	}
	return out
}

func aiTimeoutOptions() []flux.SelectOptionItem[string] {
	return []flux.SelectOptionItem[string]{
		{Label: "60 秒", Value: "60"},
		{Label: "120 秒", Value: "120"},
		{Label: "180 秒", Value: "180"},
		{Label: "300 秒", Value: "300"},
		{Label: "600 秒", Value: "600"},
	}
}

func aiTimeoutSeconds(seconds int) int {
	if seconds <= 0 {
		return 120
	}
	if seconds < 30 {
		return 30
	}
	if seconds > 600 {
		return 600
	}
	return seconds
}

func completeLyricPriority(values []string) []string {
	defaults := []string{
		model.LyricSourceTTMLDB,
		model.LyricSourceQQ,
		model.LyricSourceKugou,
		model.LyricSourceNetease,
		model.LyricSourceCustom,
	}
	allowed := map[string]struct{}{
		model.LyricSourceTTMLDB:  {},
		model.LyricSourceQQ:      {},
		model.LyricSourceKugou:   {},
		model.LyricSourceNetease: {},
		model.LyricSourceCustom:  {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(defaults))
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range defaults {
		if _, ok := seen[value]; ok {
			continue
		}
		out = append(out, value)
	}
	return out
}

func setLyricPrioritySlot(priority []string, index int, value string) []string {
	next := append([]string(nil), completeLyricPriority(priority)...)
	if index < 0 || index >= len(next) {
		return next
	}
	currentIndex := -1
	for i, item := range next {
		if item == value {
			currentIndex = i
			break
		}
	}
	if currentIndex >= 0 {
		next[index], next[currentIndex] = next[currentIndex], next[index]
		return next
	}
	next[index] = value
	return completeLyricPriority(next)
}

func moveLyricPriority(priority []string, source string, direction int) []string {
	next := completeLyricPriority(priority)
	if source == "" || direction == 0 {
		return next
	}
	from := -1
	for i, value := range next {
		if value == source {
			from = i
			break
		}
	}
	to := from + direction
	if from < 0 || to < 0 || to >= len(next) {
		return next
	}
	next[from], next[to] = next[to], next[from]
	return next
}

func sameStringSlice(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
