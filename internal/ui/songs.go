package ui

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xiaowumin-mark/FluxUI/router"
	flux "github.com/xiaowumin-mark/FluxUI/ui"

	"github.com/xiaowumin-mark/LyricSync/internal/app"
	"github.com/xiaowumin-mark/LyricSync/internal/lyric"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/song"
	storepkg "github.com/xiaowumin-mark/LyricSync/internal/state"
)

type songView string

const (
	songViewRecent songView = "recent"
	songViewAll    songView = "all"
	songViewDetail songView = "detail"
	songViewCreate songView = "create"
	songViewEdit   songView = "edit"
	songViewLyrics songView = "lyrics"
)

type songForm struct {
	ID                int64
	Title             string
	Artist            string
	Album             string
	Duration          string
	FixedLyricSource  string
	LyricTTMLDB       string
	LyricQQ           string
	LyricKugou        string
	LyricNetease      string
	LyricCustomTTML   string
	LyricTTMLDBDelay  string
	LyricQQDelay      string
	LyricKugouDelay   string
	LyricNeteaseDelay string
	LyricCustomDelay  string
	LyricsPreviewFrom string
}

const lyricDocumentCacheMax = 32

type cachedLyricDocument struct {
	Document lyric.Document
	Err      error
}

var lyricDocumentCache = struct {
	sync.Mutex
	values map[string]cachedLyricDocument
}{
	values: map[string]cachedLyricDocument{},
}

type songViewState interface {
	Value() songView
	Set(songView, router.Transition)
}

type songFormState interface {
	Value() songForm
	Set(songForm)
}

type intState interface {
	Value() int
	Set(int)
}

type int64State interface {
	Value() int64
	Set(int64)
}

type songRouteState struct {
	view       songView
	selectedID int64State
	navigate   flux.NavigateFunc
}

func (s songRouteState) Value() songView {
	return s.view
}

func (s songRouteState) Set(view songView, trans router.Transition) {
	if s.navigate == nil {
		return
	}
	path := songRoutePath(view, s.selectedID.Value())
	if path == "" {
		return
	}
	s.navigate(path, flux.WithNavTransition(trans))
}

type songRouteSelectedIDState struct {
	routeID int64
	inner   int64State
}

func (s songRouteSelectedIDState) Value() int64 {
	if s.routeID > 0 {
		return s.routeID
	}
	if s.inner == nil {
		return 0
	}
	return s.inner.Value()
}

func (s songRouteSelectedIDState) Set(value int64) {
	if s.inner != nil {
		s.inner.Set(value)
	}
}

func songsPage(
	ctx *flux.Context,
	colors palette,
	snapshot model.Snapshot,
	notice stringState,
	store *storepkg.Store,
	runtime *app.Runtime,
) flux.Element {
	params := flux.UseParams(ctx)
	route := flux.UseRoute(ctx)
	navigate := flux.UseNavigate(ctx)
	currentView := songViewFromRoute(route.Name)
	routeSongID := parseSongRouteID(params.Path("id"))

	query := flux.UseState(ctx, "")
	selectedIDState := flux.UseState(ctx, int64(0))
	selectedID := songRouteSelectedIDState{routeID: routeSongID, inner: selectedIDState}
	view := songRouteState{view: currentView, selectedID: selectedID, navigate: navigate}
	deleteID := flux.UseState(ctx, int64(0))
	deleteTitle := flux.UseState(ctx, "")
	form := flux.UseState(ctx, songForm{})
	formRoute := flux.UseState(ctx, "")
	reload := flux.UseState(ctx, 0)
	loading := flux.UseState(ctx, false)
	errorText := flux.UseState(ctx, "")
	recent := flux.UseState(ctx, []song.Song{})
	allSongs := flux.UseState(ctx, []song.Song{})
	selected := flux.UseState(ctx, song.Song{})
	lyrics := flux.UseState(ctx, []song.LyricSource{})

	flux.UseEffectWithDeps(ctx, []any{store}, func() func() {
		if store == nil {
			return func() {}
		}
		events, unsubscribe := store.Subscribe(64)
		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-done:
					return
				case event, ok := <-events:
					if !ok {
						return
					}
					if event.Type == "songs_changed" {
						reload.Set(reload.Value() + 1)
					}
				}
			}
		}()
		return func() {
			close(done)
			unsubscribe()
		}
	})

	flux.UseEffectWithDeps(ctx, []any{runtime, view.Value(), query.Value(), selectedID.Value(), params.Query("rev"), reload.Value()}, func() func() {
		loadCtx, cancel := context.WithCancel(context.Background())
		currentView := view.Value()
		currentQuery := query.Value()
		currentSelectedID := selectedID.Value()
		currentFormRoute := formRoute.Value()
		nextFormRoute := songFormRouteKey(currentView, currentSelectedID, params.Query("rev"))
		loading.Set(true)
		go func() {
			defer loading.Set(false)
			if runtime == nil {
				errorText.Set("歌曲数据库不可用")
				return
			}
			recentSongs, err := runtime.RecentSongs(loadCtx, 8)
			if err != nil {
				if loadCtx.Err() == nil {
					errorText.Set("读取歌曲记录失败: " + err.Error())
				}
				return
			}
			var list []song.Song
			if currentView == songViewAll || strings.TrimSpace(currentQuery) != "" {
				list, err = runtime.SearchSongs(loadCtx, currentQuery, 200)
				if err != nil {
					if loadCtx.Err() == nil {
						errorText.Set("搜索歌曲失败: " + err.Error())
					}
					return
				}
			}
			var selectedSong song.Song
			var selectedLyrics []song.LyricSource
			if currentSelectedID > 0 && (currentView == songViewDetail || currentView == songViewEdit || currentView == songViewLyrics) {
				selectedSong, err = runtime.GetSong(loadCtx, currentSelectedID)
				if err != nil && !errors.Is(err, song.ErrNotFound) {
					if loadCtx.Err() == nil {
						errorText.Set("读取歌曲详情失败: " + err.Error())
					}
					return
				}
				selectedLyrics, err = runtime.SongLyrics(loadCtx, currentSelectedID)
				if err != nil && !errors.Is(err, song.ErrNotFound) {
					if loadCtx.Err() == nil {
						errorText.Set("读取歌词缓存失败: " + err.Error())
					}
					return
				}
			}
			if loadCtx.Err() != nil {
				return
			}
			recent.Set(recentSongs)
			if currentView == songViewAll || strings.TrimSpace(currentQuery) != "" {
				allSongs.Set(list)
			}
			selected.Set(selectedSong)
			lyrics.Set(selectedLyrics)
			switch currentView {
			case songViewCreate:
				if currentFormRoute != nextFormRoute {
					form.Set(songForm{})
					formRoute.Set(nextFormRoute)
				}
			case songViewEdit:
				if selectedSong.ID > 0 && currentFormRoute != nextFormRoute {
					form.Set(songFormFromSong(selectedSong, selectedLyrics))
					formRoute.Set(nextFormRoute)
				}
			}
			errorText.Set("")
		}()
		return cancel
	})

	content := songsPageContent(
		colors,
		snapshot,
		view,
		query,
		selectedID,
		deleteID,
		deleteTitle,
		form,
		reload,
		loading.Value(),
		errorText.Value(),
		recent.Value(),
		allSongs.Value(),
		selected.Value(),
		lyrics.Value(),
		params.Query("source"),
		notice,
		runtime,
	)
	return flux.StackElement(
		content,
		deleteSongDialog(colors, deleteID, deleteTitle, reload, view, selectedID, notice, runtime),
	)
}

func songViewFromRoute(name string) songView {
	switch name {
	case "songs-all":
		return songViewAll
	case "songs-new":
		return songViewCreate
	case "song-detail":
		return songViewDetail
	case "song-edit":
		return songViewEdit
	case "song-lyrics":
		return songViewLyrics
	default:
		return songViewRecent
	}
}

func parseSongRouteID(value string) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}

func songRoutePath(view songView, selectedID int64) string {
	switch view {
	case songViewAll:
		return "/songs/all"
	case songViewCreate:
		return fmt.Sprintf("/songs/new?rev=%d", time.Now().UnixNano())
	case songViewDetail:
		if selectedID > 0 {
			return fmt.Sprintf("/songs/%d", selectedID)
		}
		return "/songs/all"
	case songViewEdit:
		if selectedID > 0 {
			return fmt.Sprintf("/songs/%d/edit?rev=%d", selectedID, time.Now().UnixNano())
		}
		return "/songs/all"
	case songViewLyrics:
		if selectedID > 0 {
			return fmt.Sprintf("/songs/%d/lyrics", selectedID)
		}
		return "/songs/all"
	default:
		return "/songs"
	}
}

func songFormRouteKey(view songView, selectedID int64, revision string) string {
	switch view {
	case songViewCreate:
		return "create:" + revision
	case songViewEdit:
		return fmt.Sprintf("edit:%d:%s", selectedID, revision)
	default:
		return ""
	}
}

func songsPageContent(
	colors palette,
	snapshot model.Snapshot,
	view songViewState,
	query stringState,
	selectedID int64State,
	deleteID int64State,
	deleteTitle stringState,
	form songFormState,
	reload intState,
	loading bool,
	errorText string,
	recent []song.Song,
	allSongs []song.Song,
	selected song.Song,
	lyrics []song.LyricSource,
	lyricSource string,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	switch view.Value() {
	case songViewAll:
		return allSongsView(colors, view, query, selectedID, deleteID, deleteTitle, form, reload, loading, errorText, allSongs, notice, runtime)
	case songViewDetail:
		return songDetailView(colors, selected, lyrics, view, selectedID, deleteID, deleteTitle, form, notice, runtime)
	case songViewCreate:
		return songFormView(colors, "新增歌曲", true, form, view, selectedID, reload, notice, runtime)
	case songViewEdit:
		return songFormView(colors, "编辑歌曲", false, form, view, selectedID, reload, notice, runtime)
	case songViewLyrics:
		return songLyricsFullView(colors, selected, lyrics, view, selectedID, lyricSource)
	default:
		return recentSongsView(colors, snapshot, view, selectedID, form, recent, loading, errorText, notice, runtime)
	}
}

func recentSongsView(
	colors palette,
	snapshot model.Snapshot,
	view songViewState,
	selectedID int64State,
	form songFormState,
	recent []song.Song,
	loading bool,
	errorText string,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	children := []flux.Element{
		currentTrackRecordPanel(colors, snapshot),
		flux.VSpacerElement(12),
		panel(colors,
			sectionTitle(colors, "最近播放"),
			flux.VSpacerElement(10),
			songStatusLine(colors, loading, errorText),
			recentSongList(colors, recent, selectedID, view),
		),
		flux.VSpacerElement(88),
	}
	return songPageWithFixedFAB(
		flux.ScrollViewElement(flux.ColumnElement(children...), flux.ScrollVertical(true)),
		flux.ExtendedFloatingActionButtonElement(
			flux.IconElement("library_music"),
			flux.TextElement("全部歌曲"),
			flux.FloatingActionButtonOnClick(func(ctx *flux.Context) {
				view.Set(songViewAll, router.TransitionSlideLeft)
			}),
		),
	)
}

func currentTrackRecordPanel(colors palette, snapshot model.Snapshot) flux.Element {
	return panel(colors,
		sectionTitle(colors, "当前歌曲"),
		flux.VSpacerElement(10),
		infoLine(colors, "标题", blankAs(snapshot.Track.Title, "-")),
		infoLine(colors, "艺人", blankAs(snapshot.Track.Artist, "-")),
		infoLine(colors, "专辑", blankAs(snapshot.Track.Album, "-")),
		infoLine(colors, "时长", formatMillis(snapshot.Track.Duration)),
		infoLine(colors, "状态", playbackStateText(snapshot.Playback.State)),
	)
}

func recentSongList(
	colors palette,
	songs []song.Song,
	selectedID int64State,
	view songViewState,
) flux.Element {
	if len(songs) == 0 {
		return emptyBox(colors, "暂无播放记录")
	}
	children := make([]flux.Element, 0, len(songs)*2)
	for index, item := range songs {
		if index > 0 {
			children = append(children, flux.VSpacerElement(8))
		}
		item := item
		children = append(children, flux.Key(fmt.Sprintf("recent-song-%d", item.ID),
			recentSongRow(colors, item, selectedID, view),
		))
	}
	return flux.ColumnElement(children...)
}

func recentSongRow(colors palette, item song.Song, selectedID int64State, view songViewState) flux.Element {
	openDetail := func(ctx *flux.Context) {
		selectedID.Set(item.ID)
		view.Set(songViewDetail, router.TransitionSlideLeft)
	}
	return flux.ContainerDecorationElement(
		flux.Bg(colors.panel).WithPad(flux.All(10)).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border}),
		flux.RowElement(
			flux.FixedWidthElement(
				34,
				flux.CenterElement(flux.IconElement("history", flux.IconSize(20), flux.IconColor(colors.subtle))),
			),
			flux.HSpacerElement(8),
			flux.ExpandedElement(
				flux.ColumnElement(
					flux.TextElement(clipText(blankAs(item.Title, "未命名歌曲"), 64), flux.TextSize(14), flux.TextColor(colors.text)),
					flux.VSpacerElement(3),
					flux.TextElement(clipText(songArtistAlbum(item), 72), flux.TextSize(12), flux.TextColor(colors.subtle)),
				),
			),
			flux.HSpacerElement(10),
			flux.ColumnElement(
				flux.TextElement(formatSongTime(item.LastPlayedAt), flux.TextSize(11), flux.TextColor(colors.subtle), flux.TextAlign(flux.AlignEnd)),
				flux.VSpacerElement(4),
				flux.TextElement(formatMillis(item.DurationMs), flux.TextSize(11), flux.TextColor(colors.subtle), flux.TextAlign(flux.AlignEnd)),
			),
			flux.HSpacerElement(4),
			songIconButton(colors, "查看详情", "chevron_right", false, openDetail),
		),
	)
}

func allSongsView(
	colors palette,
	view songViewState,
	query stringState,
	selectedID int64State,
	deleteID int64State,
	deleteTitle stringState,
	form songFormState,
	reload intState,
	loading bool,
	errorText string,
	songs []song.Song,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	return flux.StackElement(
		flux.ScrollViewElement(
			flux.ColumnElement(
				panel(colors,
					flux.RowElement(
						songBackButton(colors, "返回", func(ctx *flux.Context) {
							query.Set("")
							selectedID.Set(0)
							view.Set(songViewRecent, router.TransitionSlideRight)
						}),
						flux.HSpacerElement(8),
						sectionTitle(colors, "全部歌曲"),
					),
					flux.VSpacerElement(12),
					flux.FillWidthElement(flux.OutlinedTextFieldElement(
						query.Value(),
						flux.InputPlaceholder("搜索标题、艺人或专辑"),
						flux.InputSingleLine(true),
						flux.InputOnChange(func(ctx *flux.Context, value string) {
							query.Set(value)
						}),
					)),
					flux.VSpacerElement(12),
					songStatusLine(colors, loading, errorText),
					flux.VSpacerElement(8),
					allSongsVirtualList(colors, songs, selectedID, deleteID, deleteTitle, form, view, notice, runtime),
				),
				flux.VSpacerElement(88),
			),
			flux.ScrollVertical(true),
		),
		fixedFABLayer(
			flux.ExtendedFloatingActionButtonElement(
				flux.IconElement("add"),
				flux.TextElement("新增歌曲"),
				flux.FloatingActionButtonOnClick(func(ctx *flux.Context) {
					form.Set(songForm{})
					selectedID.Set(0)
					view.Set(songViewCreate, router.TransitionSlideLeft)
					reload.Set(reload.Value() + 1)
				}),
			),
		),
	)
}

func allSongsVirtualList(
	colors palette,
	songs []song.Song,
	selectedID int64State,
	deleteID int64State,
	deleteTitle stringState,
	form songFormState,
	view songViewState,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	if len(songs) == 0 {
		return emptyBox(colors, "没有匹配的歌曲")
	}
	items := append([]song.Song(nil), songs...)
	return flux.FixedHeightElement(
		560,
		flux.ListViewElement(
			len(items),
			func(ctx *flux.Context, index int) flux.Element {
				if index < 0 || index >= len(items) {
					return flux.SpacerElement(0, 0)
				}
				item := items[index]
				return flux.Key(fmt.Sprintf("song-row-%d", item.ID),
					songListRow(colors, item, selectedID, deleteID, deleteTitle, form, view, notice, runtime),
				)
			},
			flux.ListVirtualized(true),
			flux.ListItemSpacing(8),
			flux.ListPadding(flux.All(2)),
			flux.ListDecoration(flux.Bg(colors.surface)),
		),
	)
}

func songPageWithFixedFAB(content flux.Element, fab flux.Element) flux.Element {
	return flux.StackElement(
		content,
		fixedFABLayer(fab),
	)
}

func fixedFABLayer(fab flux.Element) flux.Element {
	return flux.FillElement(
		flux.ColumnElement(
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			flux.RowElement(
				flux.ExpandedElement(flux.SpacerElement(0, 0)),
				flux.PaddingElement(flux.Insets{Right: 14, Bottom: 14}, fab),
			),
		),
	)
}

func songListRow(
	colors palette,
	item song.Song,
	selectedID int64State,
	deleteID int64State,
	deleteTitle stringState,
	form songFormState,
	view songViewState,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	openDetail := func(ctx *flux.Context) {
		selectedID.Set(item.ID)
		if view != nil {
			view.Set(songViewDetail, router.TransitionSlideLeft)
		}
	}
	edit := func(ctx *flux.Context) {
		beginEditSong(runtime, form, notice, item)
		selectedID.Set(item.ID)
		if view != nil {
			view.Set(songViewEdit, router.TransitionSlideLeft)
		}
	}
	delete := func(ctx *flux.Context) {
		if deleteID == nil || deleteTitle == nil {
			return
		}
		deleteID.Set(item.ID)
		deleteTitle.Set(item.Title)
	}
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.All(12)).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border}),
		flux.RowElement(
			flux.ExpandedElement(
				flux.ColumnElement(
					flux.TextElement(clipText(blankAs(item.Title, "未命名歌曲"), 72), flux.TextSize(14), flux.TextColor(colors.text)),
					flux.VSpacerElement(4),
					flux.TextElement(clipText(songArtistAlbum(item), 92), flux.TextSize(12), flux.TextColor(colors.subtle)),
					flux.VSpacerElement(8),
					flux.RowElement(
						statusChip(colors, fmt.Sprintf("%d 次", item.PlayCount), colors.primaryContainer),
						flux.HSpacerElement(6),
						statusChip(colors, formatMillis(item.DurationMs), colors.barBase),
						flux.HSpacerElement(6),
						statusChip(colors, formatSongTime(item.LastPlayedAt), colors.barBase),
					),
				),
			),
			flux.HSpacerElement(10),
			songIconButton(colors, "查看详情", "visibility", false, openDetail),
			flux.HSpacerElement(4),
			songIconButton(colors, "编辑", "edit", false, edit),
			flux.HSpacerElement(4),
			songIconButton(colors, "删除", "delete", deleteID == nil, delete),
		),
	)
}

func songDetailView(
	colors palette,
	selected song.Song,
	lyrics []song.LyricSource,
	view songViewState,
	selectedID int64State,
	deleteID int64State,
	deleteTitle stringState,
	form songFormState,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	if selected.ID == 0 {
		return flux.ScrollViewElement(
			flux.ColumnElement(
				flux.FillWidthElement(panel(colors,
					flux.RowElement(
						songBackButton(colors, "返回", func(ctx *flux.Context) {
							view.Set(songViewAll, router.TransitionSlideRight)
						}),
						flux.HSpacerElement(8),
						sectionTitle(colors, "歌曲详情"),
					),
					flux.VSpacerElement(12),
					emptyBox(colors, "请选择一首歌曲"),
				)),
			),
			flux.ScrollVertical(true),
		)
	}
	return flux.ScrollViewElement(
		flux.ColumnElement(
			flux.FillWidthElement(panel(colors,
				flux.RowElement(
					songBackButton(colors, "返回", func(ctx *flux.Context) {
						selectedID.Set(0)
						view.Set(songViewAll, router.TransitionSlideRight)
					}),
					flux.HSpacerElement(8),
					sectionTitle(colors, "歌曲详情"),
					flux.ExpandedElement(flux.SpacerElement(0, 0)),
					songIconButton(colors, "编辑", "edit", false, func(ctx *flux.Context) {
						beginEditSong(runtime, form, notice, selected)
						view.Set(songViewEdit, router.TransitionSlideLeft)
					}),
					flux.HSpacerElement(4),
					songIconButton(colors, "删除", "delete", false, func(ctx *flux.Context) {
						deleteID.Set(selected.ID)
						deleteTitle.Set(selected.Title)
					}),
				),
				flux.VSpacerElement(12),
				infoLine(colors, "标题", selected.Title),
				infoLine(colors, "艺人", selected.Artist),
				infoLine(colors, "专辑", selected.Album),
				infoLine(colors, "时长", formatMillis(selected.DurationMs)),
				infoLine(colors, "播放次数", strconv.Itoa(selected.PlayCount)),
				infoLine(colors, "首次播放", formatSongTime(selected.FirstPlayedAt)),
				infoLine(colors, "最近播放", formatSongTime(selected.LastPlayedAt)),
			)),
			flux.VSpacerElement(12),
			flux.FillWidthElement(lyricPreviewPanel(colors, selected, lyrics, view, selectedID)),
		),
		flux.ScrollVertical(true),
	)
}

func songFormView(
	colors palette,
	title string,
	create bool,
	form songFormState,
	view songViewState,
	selectedID int64State,
	reload intState,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	current := form.Value()
	return flux.ScrollViewElement(
		flux.ColumnElement(
			flux.FillWidthElement(panel(colors,
				flux.RowElement(
					songBackButton(colors, "返回", func(ctx *flux.Context) {
						if selectedID.Value() > 0 {
							view.Set(songViewDetail, router.TransitionSlideRight)
							return
						}
						view.Set(songViewAll, router.TransitionSlideRight)
					}),
					flux.HSpacerElement(8),
					sectionTitle(colors, title),
				),
				flux.VSpacerElement(12),
				songFormField(colors, "标题", current.Title, "歌曲标题", func(ctx *flux.Context, value string) {
					next := form.Value()
					next.Title = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				songFormField(colors, "艺人", current.Artist, "艺人", func(ctx *flux.Context, value string) {
					next := form.Value()
					next.Artist = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				songFormField(colors, "专辑", current.Album, "专辑", func(ctx *flux.Context, value string) {
					next := form.Value()
					next.Album = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				songFormField(colors, "时长", current.Duration, "03:45 或 225", func(ctx *flux.Context, value string) {
					next := form.Value()
					next.Duration = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				songFixedSourceField(colors, current.FixedLyricSource, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.FixedLyricSource = value
					form.Set(next)
				}),
				flux.VSpacerElement(12),
				flux.RowElement(
					primaryButton(colors, "保存", func(ctx *flux.Context) {
						saveSongForm(ctx, runtime, form, view, selectedID, reload, notice, create)
					}),
					flux.HSpacerElement(8),
					secondaryButton(colors, "取消", func(ctx *flux.Context) {
						songNavigateBack(ctx, func(ctx *flux.Context) {
							if selectedID.Value() > 0 {
								view.Set(songViewDetail, router.TransitionSlideRight)
								return
							}
							view.Set(songViewAll, router.TransitionSlideRight)
						})
					}),
				),
			)),
			flux.VSpacerElement(12),
			flux.FillWidthElement(panel(colors,
				sectionTitle(colors, "歌词缓存"),
				flux.VSpacerElement(10),
				lyricFormField(colors, "TTML DB", current.LyricTTMLDB, current.LyricTTMLDBDelay, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricTTMLDB = value
					form.Set(next)
				}, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricTTMLDBDelay = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				lyricFormField(colors, "QQ 音乐", current.LyricQQ, current.LyricQQDelay, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricQQ = value
					form.Set(next)
				}, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricQQDelay = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				lyricFormField(colors, "酷狗音乐", current.LyricKugou, current.LyricKugouDelay, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricKugou = value
					form.Set(next)
				}, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricKugouDelay = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				lyricFormField(colors, "网易云音乐", current.LyricNetease, current.LyricNeteaseDelay, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricNetease = value
					form.Set(next)
				}, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricNeteaseDelay = value
					form.Set(next)
				}),
				flux.VSpacerElement(10),
				lyricFormField(colors, "自定义 TTML", current.LyricCustomTTML, current.LyricCustomDelay, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricCustomTTML = value
					form.Set(next)
				}, func(ctx *flux.Context, value string) {
					next := form.Value()
					next.LyricCustomDelay = value
					form.Set(next)
				}),
			)),
		),
		flux.ScrollVertical(true),
	)
}

func songFixedSourceField(colors palette, value string, onChange func(ctx *flux.Context, value string)) flux.Element {
	options := append([]flux.SelectOptionItem[string]{{Label: "自动选择", Value: ""}}, lyricSourceOptions()...)
	return flux.ColumnElement(
		label(colors, "固定歌词搜索结果"),
		flux.VSpacerElement(6),
		flux.SelectElement[string](
			value,
			options,
			flux.SelectMaxHeight[string](240),
			flux.SelectOnChange[string](onChange),
		),
	)
}

func songFormField(colors palette, title, value, placeholder string, onChange func(ctx *flux.Context, value string)) flux.Element {
	return flux.ColumnElement(
		label(colors, title),
		flux.VSpacerElement(6),
		flux.FillWidthElement(flux.OutlinedTextFieldElement(
			value,
			flux.InputPlaceholder(placeholder),
			flux.InputSingleLine(true),
			flux.InputOnChange(onChange),
		)),
	)
}

func lyricFormField(
	colors palette,
	title string,
	value string,
	delay string,
	onChange func(ctx *flux.Context, value string),
	onDelayChange func(ctx *flux.Context, value string),
) flux.Element {
	return flux.ColumnElement(
		flux.RowElement(
			flux.ExpandedElement(label(colors, title)),
			flux.FixedWidthElement(118,
				flux.OutlinedTextFieldElement(
					delay,
					flux.InputPlaceholder("延迟 ms"),
					flux.InputSingleLine(true),
					flux.InputOnChange(onDelayChange),
				),
			),
		),
		flux.VSpacerElement(6),
		flux.FillWidthElement(flux.FixedHeightElement(
			96,
			flux.OutlinedTextFieldElement(
				value,
				flux.InputPlaceholder("粘贴歌词文本，后续搜索阶段会自动填充"),
				flux.InputSingleLine(false),
				flux.InputOnChange(onChange),
			),
		)),
	)
}

func lyricPreviewPanel(colors palette, selected song.Song, lyrics []song.LyricSource, view songViewState, selectedID int64State) flux.Element {
	children := []flux.Element{
		flux.RowElement(
			sectionTitle(colors, "歌词预览"),
			flux.ExpandedElement(flux.SpacerElement(0, 0)),
			secondaryButton(colors, "查看已应用", func(ctx *flux.Context) {
				if selected.ID <= 0 {
					return
				}
				selectedID.Set(selected.ID)
				view.Set(songViewLyrics, router.TransitionSlideLeft)
			}),
		),
		flux.VSpacerElement(12),
	}
	ordered := orderedLyricSources(lyrics)
	if len(ordered) == 0 {
		children = append(children, emptyBox(colors, "暂无歌词缓存"))
		return panel(colors, children...)
	}
	for index, lyric := range ordered {
		if index > 0 {
			children = append(children, flux.VSpacerElement(8))
		}
		children = append(children, lyricPreviewCard(colors, selected, lyric, selectedID))
	}
	return panel(colors, children...)
}

func orderedLyricSources(lyrics []song.LyricSource) []song.LyricSource {
	bySource := make(map[string]song.LyricSource, len(lyrics))
	for _, item := range lyrics {
		bySource[item.Source] = item
	}
	out := make([]song.LyricSource, 0, len(song.LyricSources))
	for _, source := range song.LyricSources {
		item := bySource[source]
		item.Source = source
		out = append(out, item)
	}
	return out
}

func lyricPreviewCard(colors palette, selected song.Song, lyric song.LyricSource, selectedID int64State) flux.Element {
	content := lyricContent(lyric)
	status := "暂无"
	statusColor := colors.subtle
	hasContent := strings.TrimSpace(content) != ""
	if hasContent {
		status = "可预览"
		statusColor = colors.primary
	}
	openFull := func(ctx *flux.Context) {
		if selected.ID <= 0 || !hasContent {
			return
		}
		selectedID.Set(selected.ID)
		flux.Navigate(ctx, fmt.Sprintf("/songs/%d/lyrics?source=%s", selected.ID, url.QueryEscape(lyric.Source)), flux.WithNavTransition(router.TransitionSlideLeft))
	}
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.All(12)).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border}),
		flux.ColumnElement(
			flux.RowElement(
				flux.TextElement(lyricSourceLabel(lyric.Source), flux.TextSize(13), flux.TextColor(colors.text)),
				flux.ExpandedElement(flux.SpacerElement(0, 0)),
				statusChip(colors, formatLyricDelayChip(lyric.DelayMs), colors.barBase),
				flux.HSpacerElement(6),
				statusChip(colors, status, statusColor),
			),
			flux.VSpacerElement(8),
			flux.TextElement(lyricPreviewText(content), flux.TextSize(12), flux.TextColor(colors.subtle)),
			flux.VSpacerElement(8),
			flux.RowElement(
				flux.ExpandedElement(flux.SpacerElement(0, 0)),
				secondaryButton(colors, "查看全部歌词", openFull),
			),
		),
	)
}

func songLyricsFullView(colors palette, selected song.Song, lyrics []song.LyricSource, view songViewState, selectedID int64State, lyricSource string) flux.Element {
	if selected.ID == 0 {
		return flux.ScrollViewElement(
			flux.FillWidthElement(panel(colors,
				flux.RowElement(
					songBackButton(colors, "返回", func(ctx *flux.Context) {
						view.Set(songViewAll, router.TransitionSlideRight)
					}),
					flux.HSpacerElement(8),
					sectionTitle(colors, "歌词"),
				),
				flux.VSpacerElement(12),
				emptyBox(colors, "请选择一首歌曲"),
			)),
			flux.ScrollVertical(true),
		)
	}
	selectedLyric, ok := preferredLyricForDisplay(selected, lyrics, lyricSource)
	if !ok {
		return flux.ScrollViewElement(
			flux.FillWidthElement(panel(colors,
				flux.RowElement(
					songBackButton(colors, "返回", func(ctx *flux.Context) {
						selectedID.Set(selected.ID)
						view.Set(songViewDetail, router.TransitionSlideRight)
					}),
					flux.HSpacerElement(8),
					sectionTitle(colors, "歌词"),
				),
				flux.VSpacerElement(12),
				emptyBox(colors, "暂无可查看的歌词"),
			)),
			flux.ScrollVertical(true),
		)
	}
	content := lyricContent(selectedLyric)
	document, err := cachedLyricDocumentFor(content)
	children := []flux.Element{
		flux.FillWidthElement(panel(colors,
			flux.RowElement(
				songBackButton(colors, "返回", func(ctx *flux.Context) {
					selectedID.Set(selected.ID)
					view.Set(songViewDetail, router.TransitionSlideRight)
				}),
				flux.HSpacerElement(8),
				sectionTitle(colors, "歌词"),
				flux.ExpandedElement(flux.SpacerElement(0, 0)),
				statusChip(colors, formatLyricDelayChip(selectedLyric.DelayMs), colors.barBase),
				flux.HSpacerElement(6),
				statusChip(colors, lyricSourceLabel(selectedLyric.Source), colors.primaryContainer),
			),
			flux.VSpacerElement(10),
			infoLine(colors, "歌曲", selected.Title),
			infoLine(colors, "艺人", selected.Artist),
		)),
		flux.VSpacerElement(12),
	}
	if err != nil {
		children = append(children, flux.FillWidthElement(panel(colors, emptyBox(colors, "歌词解析失败: "+err.Error()))))
		return flux.ScrollViewElement(flux.ColumnElement(children...), flux.ScrollVertical(true))
	}
	if len(document.Metadata) > 0 {
		children = append(children, flux.FillWidthElement(ttmlMetadataPanel(colors, document.Metadata)), flux.VSpacerElement(12))
	}
	children = append(children, flux.FillWidthElement(fullLyricLinesPanel(colors, document)))
	return flux.ScrollViewElement(
		flux.ColumnElement(children...),
		flux.ScrollVertical(true),
	)
}

func preferredLyricForDisplay(selected song.Song, lyrics []song.LyricSource, explicitSource string) (song.LyricSource, bool) {
	explicitSource = strings.TrimSpace(explicitSource)
	if explicitSource != "" {
		for _, item := range lyrics {
			if item.Source == explicitSource && strings.TrimSpace(lyricContent(item)) != "" {
				return item, true
			}
		}
		return song.LyricSource{}, false
	}
	priority := []string{}
	if strings.TrimSpace(selected.FixedLyricSource) != "" {
		priority = append(priority, selected.FixedLyricSource)
	}
	priority = append(priority, selected.AppliedLyricSource, song.SourceTTMLDB, song.SourceQQ, song.SourceKugou, song.SourceNetease, song.SourceCustom)
	bySource := map[string]song.LyricSource{}
	for _, item := range lyrics {
		if strings.TrimSpace(lyricContent(item)) == "" {
			continue
		}
		bySource[item.Source] = item
	}
	for _, source := range priority {
		if item, ok := bySource[source]; ok {
			return item, true
		}
	}
	for _, item := range bySource {
		return item, true
	}
	return song.LyricSource{}, false
}

func ttmlMetadataPanel(colors palette, metadata []lyric.Metadata) flux.Element {
	rows := []flux.Element{
		sectionTitle(colors, "TTML 元数据"),
		flux.VSpacerElement(10),
	}
	for _, item := range metadata {
		rows = append(rows, infoLine(colors, item.Key, strings.Join(item.Values, ", ")))
	}
	return panel(colors, rows...)
}

func fullLyricLinesPanel(colors palette, document lyric.Document) flux.Element {
	document = document.Normalized()
	rows := []flux.Element{
		sectionTitle(colors, "全文歌词"),
		flux.VSpacerElement(10),
	}
	if len(document.Lines) == 0 {
		rows = append(rows, emptyBox(colors, "暂无歌词行"))
		return panel(colors, rows...)
	}
	lines := append([]lyric.Line(nil), document.Lines...)
	rows = append(rows,
		flux.FixedHeightElement(
			520,
			flux.ListViewElement(
				len(lines),
				func(ctx *flux.Context, index int) flux.Element {
					if index < 0 || index >= len(lines) {
						return flux.SpacerElement(0, 0)
					}
					return flux.Key(fmt.Sprintf("full-lyric-line-%d-%d", index, lines[index].StartTimeMs),
						fullLyricLineCard(colors, lines[index]),
					)
				},
				flux.ListVirtualized(true),
				flux.ListItemSpacing(8),
				flux.ListPadding(flux.All(2)),
				flux.ListDecoration(flux.Bg(colors.surface)),
			),
		),
	)
	return panel(colors, rows...)
}

func fullLyricLineCard(colors palette, line lyric.Line) flux.Element {
	text := strings.TrimSpace(line.Text())
	if text == "" {
		text = "-"
	}
	align := flux.AlignStart
	if line.IsDuet {
		align = flux.AlignEnd
	}
	textSize := float32(14)
	if line.IsBackground {
		textSize = 12
	}
	badges := []flux.Element{
		statusChip(colors, formatMillis(line.StartTimeMs)+" - "+formatMillis(line.EndTimeMs), colors.barBase),
	}
	if line.IsBackground {
		badges = append(badges, flux.HSpacerElement(6), statusChip(colors, "背景", colors.barBase))
	}
	if line.IsDuet {
		badges = append(badges, flux.HSpacerElement(6), statusChip(colors, "对唱", colors.primaryContainer))
	}
	children := []flux.Element{
		flux.RowElement(badges...),
		flux.VSpacerElement(8),
		flux.TextElement(text, flux.TextSize(textSize), flux.TextColor(colors.text), flux.TextAlign(align)),
	}
	if strings.TrimSpace(line.TranslatedLyric) != "" {
		children = append(children, flux.VSpacerElement(5), flux.TextElement(strings.TrimSpace(line.TranslatedLyric), flux.TextSize(12), flux.TextColor(colors.subtle), flux.TextAlign(align)))
	}
	if strings.TrimSpace(line.RomanLyric) != "" {
		children = append(children, flux.VSpacerElement(4), flux.TextElement(strings.TrimSpace(line.RomanLyric), flux.TextSize(11), flux.TextColor(colors.subtle), flux.TextAlign(align)))
	}
	return flux.ContainerDecorationElement(
		flux.Bg(colors.muted).WithPad(flux.All(12)).WithRad(8).WithBorder(flux.Border{Width: 1, Color: colors.border}),
		flux.ColumnElement(children...),
	)
}

func deleteSongDialog(
	colors palette,
	deleteID int64State,
	deleteTitle stringState,
	reload intState,
	view songViewState,
	selectedID int64State,
	notice stringState,
	runtime *app.Runtime,
) flux.Element {
	open := deleteID.Value() > 0
	return flux.DialogElement(
		open,
		flux.TextElement("删除后会同时移除这首歌的本地歌词缓存。", flux.TextColor(colors.text)),
		flux.DialogTitle("删除歌曲"),
		flux.DialogWidth(360),
		flux.DialogConfirmText("删除"),
		flux.DialogCancelText("取消"),
		flux.DialogOnOpenChange(func(ctx *flux.Context, opened bool) {
			if !opened {
				deleteID.Set(0)
				deleteTitle.Set("")
			}
		}),
		flux.DialogOnCancel(func(ctx *flux.Context) {
			deleteID.Set(0)
			deleteTitle.Set("")
		}),
		flux.DialogOnConfirm(func(ctx *flux.Context) {
			id := deleteID.Value()
			if id <= 0 || runtime == nil {
				return
			}
			if err := runtime.DeleteSong(context.Background(), id); err != nil {
				notice.Set("删除歌曲失败: " + err.Error())
				return
			}
			notice.Set("已删除歌曲: " + deleteTitle.Value())
			deleteID.Set(0)
			deleteTitle.Set("")
			selectedID.Set(0)
			view.Set(songViewAll, router.TransitionSlideRight)
			reload.Set(reload.Value() + 1)
		}),
	)
}

func songStatusLine(colors palette, loading bool, errorText string) flux.Element {
	if strings.TrimSpace(errorText) != "" {
		return flux.ColumnElement(
			emptyBox(colors, errorText),
			flux.VSpacerElement(10),
		)
	}
	if loading {
		return flux.ColumnElement(
			emptyBox(colors, "正在读取歌曲记录"),
			flux.VSpacerElement(10),
		)
	}
	return flux.SpacerElement(0, 0)
}

func songIconButton(colors palette, tooltip string, icon string, disabled bool, onClick func(ctx *flux.Context)) flux.Element {
	return flux.TooltipElement(
		tooltip,
		flux.IconButtonElement(
			flux.IconElement(icon, flux.IconSize(20)),
			flux.IconButtonSize(36),
			flux.IconButtonDisabled(disabled),
			flux.IconButtonForeground(colors.primary),
			flux.IconButtonOnClick(onClick),
		),
	)
}

func songBackButton(colors palette, tooltip string, fallback func(ctx *flux.Context)) flux.Element {
	return songIconButton(colors, tooltip, "arrow_back", false, func(ctx *flux.Context) {
		songNavigateBack(ctx, fallback)
	})
}

func songNavigateBack(ctx *flux.Context, fallback func(ctx *flux.Context)) {
	if flux.CanGoBack(ctx) {
		// FluxUI reverses transitions for navPop, so pass the forward slide here
		// to get a visual slide-right back animation.
		flux.NavigateBack(ctx, flux.WithNavTransition(router.TransitionSlideLeft))
		return
	}
	if fallback != nil {
		fallback(ctx)
	}
}

func beginEditSong(runtime *app.Runtime, form songFormState, notice stringState, item song.Song) {
	var lyrics []song.LyricSource
	if runtime != nil {
		got, err := runtime.SongLyrics(context.Background(), item.ID)
		if err == nil {
			lyrics = got
		} else {
			notice.Set("读取歌词缓存失败: " + err.Error())
		}
	}
	form.Set(songFormFromSong(item, lyrics))
}

func saveSongForm(
	ctx *flux.Context,
	runtime *app.Runtime,
	formState songFormState,
	view songViewState,
	selectedID int64State,
	reload intState,
	notice stringState,
	create bool,
) {
	if runtime == nil {
		notice.Set("歌曲数据库不可用")
		return
	}
	current := formState.Value()
	input, err := songInputFromForm(current)
	if err != nil {
		notice.Set(err.Error())
		return
	}
	var saved song.Song
	if create {
		saved, err = runtime.CreateSong(context.Background(), input)
	} else {
		saved, err = runtime.UpdateSong(context.Background(), current.ID, input)
	}
	if err != nil {
		notice.Set("保存歌曲失败: " + err.Error())
		return
	}
	if err := saveFormLyrics(runtime, saved.ID, current); err != nil {
		notice.Set("歌曲已保存，但歌词缓存保存失败: " + err.Error())
	} else {
		notice.Set("歌曲已保存")
	}
	selectedID.Set(saved.ID)
	reload.Set(reload.Value() + 1)
	// 这里不需要返回
	//view.Set(songViewDetail, router.TransitionSlideRight)
}

func saveFormLyrics(runtime *app.Runtime, songID int64, form songForm) error {
	values := map[string]string{
		song.SourceTTMLDB:  form.LyricTTMLDB,
		song.SourceQQ:      form.LyricQQ,
		song.SourceKugou:   form.LyricKugou,
		song.SourceNetease: form.LyricNetease,
	}
	for source, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := runtime.SetSongLyric(context.Background(), songID, source, value, ""); err != nil {
			return err
		}
	}
	if strings.TrimSpace(form.LyricCustomTTML) != "" {
		if err := runtime.SetSongLyric(context.Background(), songID, song.SourceCustom, "", form.LyricCustomTTML); err != nil {
			return err
		}
	}
	for source, value := range lyricDelayInputs(form) {
		delay, err := parseLyricDelayInput(value)
		if err != nil {
			return fmt.Errorf("%s 歌词延迟无效: %w", lyricSourceLabel(source), err)
		}
		if err := runtime.SetSongLyricDelay(context.Background(), songID, source, delay); err != nil {
			return err
		}
	}
	return nil
}

func lyricDelayInputs(form songForm) map[string]string {
	return map[string]string{
		song.SourceTTMLDB:  form.LyricTTMLDBDelay,
		song.SourceQQ:      form.LyricQQDelay,
		song.SourceKugou:   form.LyricKugouDelay,
		song.SourceNetease: form.LyricNeteaseDelay,
		song.SourceCustom:  form.LyricCustomDelay,
	}
}

func songInputFromForm(form songForm) (song.Input, error) {
	duration, err := parseDurationInput(form.Duration)
	if err != nil {
		return song.Input{}, err
	}
	input := song.Input{
		Title:            strings.TrimSpace(form.Title),
		Artist:           strings.TrimSpace(form.Artist),
		Album:            strings.TrimSpace(form.Album),
		DurationMs:       duration,
		FixedLyricSource: strings.TrimSpace(form.FixedLyricSource),
	}
	if input.Title == "" {
		return song.Input{}, fmt.Errorf("标题不能为空")
	}
	return input, nil
}

func songFormFromSong(item song.Song, lyrics []song.LyricSource) songForm {
	form := songForm{
		ID:               item.ID,
		Title:            item.Title,
		Artist:           item.Artist,
		Album:            item.Album,
		Duration:         formatDurationInput(item.DurationMs),
		FixedLyricSource: item.FixedLyricSource,
	}
	for _, lyric := range lyrics {
		value := lyricEditContent(lyric)
		switch lyric.Source {
		case song.SourceTTMLDB:
			form.LyricTTMLDB = value
			form.LyricTTMLDBDelay = formatLyricDelayInput(lyric.DelayMs)
		case song.SourceQQ:
			form.LyricQQ = value
			form.LyricQQDelay = formatLyricDelayInput(lyric.DelayMs)
		case song.SourceKugou:
			form.LyricKugou = value
			form.LyricKugouDelay = formatLyricDelayInput(lyric.DelayMs)
		case song.SourceNetease:
			form.LyricNetease = value
			form.LyricNeteaseDelay = formatLyricDelayInput(lyric.DelayMs)
		case song.SourceCustom:
			form.LyricCustomTTML = value
			form.LyricCustomDelay = formatLyricDelayInput(lyric.DelayMs)
		}
	}
	return form
}

func parseDurationInput(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return 0, fmt.Errorf("时长格式应为 mm:ss 或 hh:mm:ss")
		}
		var total int64
		for _, part := range parts {
			n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || n < 0 {
				return 0, fmt.Errorf("时长格式应为 mm:ss 或秒数")
			}
			total = total*60 + n
		}
		return total * 1000, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("时长格式应为 mm:ss 或秒数")
	}
	if n > 10000 {
		return n, nil
	}
	return n * 1000, nil
}

func formatDurationInput(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return formatMillis(ms)
}

func parseLyricDelayInput(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	delay, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("请输入整数毫秒")
	}
	return delay, nil
}

func formatLyricDelayInput(ms int64) string {
	if ms == 0 {
		return ""
	}
	return strconv.FormatInt(ms, 10)
}

func formatLyricDelayChip(ms int64) string {
	if ms == 0 {
		return "0 ms"
	}
	if ms > 0 {
		return fmt.Sprintf("+%d ms", ms)
	}
	return fmt.Sprintf("%d ms", ms)
}

func lyricContent(lyric song.LyricSource) string {
	if strings.TrimSpace(lyric.TTMLLyric) != "" {
		return lyric.TTMLLyric
	}
	return lyric.RawLyric
}

func lyricEditContent(lyric song.LyricSource) string {
	if strings.TrimSpace(lyric.RawLyric) != "" {
		return lyric.RawLyric
	}
	return lyric.TTMLLyric
}

func lyricPreviewText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "暂无歌词"
	}
	document, err := cachedLyricDocumentFor(value)
	if err == nil {
		if preview := lyric.PreviewText(document, 8); strings.TrimSpace(preview) != "" {
			return preview
		}
	}
	document = lyric.ParsePlainText(value)
	if preview := lyric.PreviewText(document, 8); strings.TrimSpace(preview) != "" {
		return preview
	}
	return "暂无可预览内容"
}

func cachedLyricDocumentFor(value string) (lyric.Document, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return lyric.Document{}, fmt.Errorf("lyric content is empty")
	}
	key := lyricDocumentCacheKey(value)
	lyricDocumentCache.Lock()
	if cached, ok := lyricDocumentCache.values[key]; ok {
		lyricDocumentCache.Unlock()
		return cached.Document, cached.Err
	}
	lyricDocumentCache.Unlock()

	document, err := lyric.Parse(value)
	lyricDocumentCache.Lock()
	if len(lyricDocumentCache.values) >= lyricDocumentCacheMax {
		lyricDocumentCache.values = map[string]cachedLyricDocument{}
	}
	lyricDocumentCache.values[key] = cachedLyricDocument{Document: document, Err: err}
	lyricDocumentCache.Unlock()
	return document, err
}

func lyricDocumentCacheKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%d:%x", len(value), sum)
}

func songArtistAlbum(item song.Song) string {
	artist := strings.TrimSpace(item.Artist)
	album := strings.TrimSpace(item.Album)
	switch {
	case artist != "" && album != "":
		return artist + " - " + album
	case artist != "":
		return artist
	case album != "":
		return album
	default:
		return "未知艺人"
	}
}

func formatSongTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}
