# LyricSync 阶段完成日志

本文档用于记录每个开发阶段完成后的结果、验证方式、遗留问题和下一阶段入口。

配套文档：

- [Full 版本开发文档](./lyricsync-full-development-spec.md)
- [开发周期分段文档](./lyricsync-development-roadmap.md)
- [阶段 0 架构确认](./phase-0-baseline-architecture.md)

## 记录规则

每完成一个阶段，新增一条记录，包含：

- 阶段编号和名称。
- 完成日期。
- 完成状态。
- 本阶段交付物。
- 验证方式。
- 关键决策。
- 遗留问题。
- 下一阶段入口。

## 阶段状态总览

| 阶段 | 名称 | 状态 | 完成日期 | 记录 |
| --- | --- | --- | --- | --- |
| 0 | 基线整理与架构确认 | 已完成 | 2026-06-25 | 本文档 |
| 1 | FluxUI 应用框架与导航 | 已完成 | 2026-06-25 | 本文档 |
| 2 | SMTC 会话管理与 active 会话稳定 | 已完成 | 2026-06-25 | 本文档 |
| 3 | 仪表盘播放体验 | 已完成 | 2026-06-26 | 本文档 |
| 4 | 配置、日志与基础持久化 | 已完成 | 2026-06-26 | 本文档 |
| 5 | 歌曲数据库与播放记录 | 已完成 | 2026-06-27 | 本文档 |
| 6 | 统一歌词结构与 TTML 能力 | 已完成 | 2026-06-27 | 本文档 |
| 7 | 歌词来源接入与并行搜索 | 已完成 | 2026-06-27 | 本文档 |
| 8 | 歌词任务调度与防堆积 | 已完成 | 2026-06-28 | 本文档 |
| 9 | AI 处理能力 | 已完成 | 2026-06-28 | 本文档 |
| 10 | AMLL 完整同步 | 未开始 | - | - |
| 11 | 体验打磨、测试与发布准备 | 未开始 | - | - |

## 阶段 0：基线整理与架构确认

完成日期：2026-06-25

状态：已完成。

### 本阶段目标

- 确认当前代码状态。
- 明确 Full 版本模块边界。
- 避免在 basic 版本代码上继续堆功能。

### 已完成内容

- 盘点当前包结构：
  - `internal/app`
  - `internal/config`
  - `internal/state`
  - `internal/media`
  - `internal/amll`
  - `internal/ui`
  - `internal/model`
- 确认当前应用入口和运行时启动链路。
- 确认当前 UI 仍是 basic 单页结构，阶段 1 需要改为 FluxUI 侧边导航。
- 确认当前配置路径仍使用 `LyricSyncBasic`，后续需要迁移到 `LyricSync`。
- 确认当前日志保留 200 条，Full 版本需求为 50 条。
- 确认当前测试基线可通过。
- 明确 `/ref` 使用边界：除 FluxUI 框架依赖外，参考仓库只读不直接 import 业务代码。
- 形成 Full 版本目标模块拆分。
- 形成本地数据存储方案。
- 形成运行时状态和持久化状态边界。
- 形成阶段 1 及后续阶段基础任务列表。

### 交付物

- [阶段 0 架构确认](./phase-0-baseline-architecture.md)
- 本阶段完成日志。

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

## 阶段 8：歌词任务调度与防堆积

完成日期：2026-06-28

状态：已完成。

### 本阶段目标

- 快速切歌时取消旧歌词任务。
- 防止旧任务结果覆盖新歌曲。
- 为后续 AI 清洗、翻译、音译任务预留单并发限流入口。

### 已完成内容

- Runtime 为 active track 引入歌词 revision。
- active track 变化时递增 revision，并取消旧歌词搜索任务。
- 歌词搜索任务统一使用 `context.Context` 派生取消。
- 搜索启动前加入 debounce，快速连续切歌只保留最新任务。
- 搜索保存歌词、更新配置、通知 UI、发布当前歌词前都会校验 revision。
- revision 校验同时检查当前歌曲 key 和 track ID，避免同名同艺人但 track 已切换时旧结果写入。
- 本地歌词命中也通过 revision 发布，和网络搜索使用同一套 stale result 防护。
- 新增 `runAIExclusive`，后续 AI 请求统一通过单并发限流执行，避免 AI 任务堆积。

### 交付物

- 歌词任务调度与 revision 防护：
  - `internal/app/app.go`
- 回归测试：
  - `internal/app/app_test.go`

### 验证方式

执行：

```powershell
go test ./internal/app -count=1
go test ./...
gopls check internal/app/app.go internal/app/app_test.go
```

结果：

```text
通过
```

### 关键决策

- 调度控制放在 `internal/app.Runtime`，歌词 provider 只负责遵守传入的 `context.Context`。
- latest-only 策略通过“新 revision 取消旧任务 + debounce 后再次校验 revision”实现，不引入多任务队列。
- AI 当前还没有实际请求链路，本阶段先提供单并发入口，后续 AI 清洗/翻译/音译接入时复用。

## 阶段 9：AI 处理能力

完成日期：2026-06-28

状态：已完成。

### 已完成内容

- 新增 `internal/ai` OpenAI 协议兼容客户端，支持 `/models` 和 `/chat/completions`。
- 设置页新增真实 `Fetch models` 操作，获取后持久化模型列表；有缓存模型时可下拉选择。
- 歌词搜索链路接入 AI 增强：基础歌词先保存和发布，AI 清洗/翻译/音译作为后续增强任务执行。
- AI 任务复用单并发限流，并新增独立 cancel；切歌、停止应用或 revision 变化会取消旧 AI 任务。
- AI 结果写回数据库前校验 revision、歌曲 key 和 track ID，避免旧任务覆盖当前歌曲。
- AI 失败、超时或输出不可用时只记录日志，继续使用非 AI 歌词版本。
- AI 增强后的歌词写回原来源，并标记 `lyric_sources.ai_cleaned`，避免重复处理。

### 交付物

- `internal/ai/client.go`
- `internal/ai/client_test.go`
- `internal/app/app.go`
- `internal/app/app_test.go`
- `internal/model/model.go`
- `internal/config/config.go`
- `internal/song/repository.go`
- `internal/ui/settings.go`

### 验证方式

执行：
```powershell
go test ./internal/ai -count=1
go test ./internal/app -count=1
go test ./internal/config ./internal/song ./internal/ui -count=1
go test ./...
```

结果：通过。

### 关键决策

- 不引入 OpenAI SDK，直接使用 OpenAI 协议 HTTP 接口，方便兼容第三方 OpenAI 协议服务。
- AI 不作为基础歌词发送的前置条件，避免播放时等待 AI。
- 深度思考使用 `reasoning_effort`，若兼容服务返回 400 会自动降级重试。

### 遗留问题

- 设置页历史文本存在编码损坏，本阶段只接入功能，没有整体清理设置页文案。
- AI 输出格式依赖模型遵守 JSON 指令，当前已做失败回退，后续可增加更强的 JSON 修复策略。

## 阶段 7 后续修正：歌曲去重与仪表盘性能

完成日期：2026-06-27

状态：已完成。

### 调整原因

- 部分播放器会把专辑信息拼到 SMTC 艺人字段里，导致数据库记录和下一次 SMTC 原始信息无法匹配，重复创建歌曲。
- 普通短横线 `-` 过早参与拆分会误伤正常歌名或艺人名，需要只作为兼容匹配使用。
- 仪表盘加入完整歌词窗口后，播放进度更新时不应反复扫描整份歌词或复制可见歌词切片。

### 已完成内容

- `song.Input` 清洗优先使用 `—`、`–`、`|`、斜杠等明确分隔符拆分混合元数据。
- 普通 ` - ` 只进入候选 key 兼容匹配，不作为首轮存储标准化规则。
- 歌曲数据库记录播放时会尝试标准 key、兼容拆分 key、旧版混合艺人 key，命中后统一写回标准 title/artist/album。
- 数据库启动迁移会用同一套候选规则合并旧重复记录，并同步修正展示字段。
- 仪表盘歌词窗口保持原显示效果，但范围计算改为二分定位 active 候选，再局部扩展同时 active 行。
- 仪表盘歌词渲染去掉每次播放进度刷新时的可见歌词切片复制。

### 验证方式

执行：

```powershell
go test ./...
gopls check internal/song/song.go internal/song/repository.go internal/song/repository_test.go internal/ui/dashboard.go internal/ui/dashboard_test.go
```

结果：

```text
通过
```

## 阶段 7 后续修正：首页性能回退修复

完成日期：2026-06-27

状态：已完成。

### 调整原因

- 加入完整歌词、封面传输和波形后，首页播放状态刷新路径承担了过多重复工作。
- UI 快照在高频事件下复制封面二进制，仪表盘渲染又重复计算封面 hash 和检查缓存文件。
- 音频频谱处理每帧复制频率数组和重建 history，造成持续分配。

### 已完成内容

- `Store.Snapshot()` 不再复制 `CoverData` 大字节，快照更新时间改为状态变更时维护。
- UI 订阅对 `playback_progress` 和 `audio_frame` 统一降频，避免音频帧直接驱动全量 UI 快照。
- `audio_frame` 事件 payload 去掉 PCM，避免 UI 订阅通道持有大块音频数据。
- 仪表盘封面路径按 `CoverHash` 做内存缓存，缓存命中不再 hash、不再 `os.Stat`、不再分配。
- 仪表盘歌词行渲染移除每帧 `TrimSpace`。
- 波形稳定器复用 current/target slice，重采样路径 0 分配。
- 频谱处理复用 Gaussian scratch 和环形 history，避免每帧数组复制和切片前插。
- 新增 UI 和频谱微基准，后续性能改动可量化对比。

### 验证方式

执行：

```powershell
go test ./...
gopls check internal/media/spectrum.go internal/media/spectrum_test.go internal/state/state.go internal/ui/ui.go internal/ui/dashboard.go internal/ui/dashboard_test.go
go test ./internal/media -run '^$' -bench BenchmarkSpectrumProcessorProcess -benchmem
go test ./internal/ui -run '^$' -bench 'Benchmark(DashboardLyricRange|ResampleWaveformInto|CoverCachePathCached)$' -benchmem
```

结果：

```text
通过
BenchmarkSpectrumProcessorProcess-8    151482 ns/op    261 B/op    1 allocs/op
BenchmarkDashboardLyricRange-8             87.18 ns/op   0 B/op    0 allocs/op
BenchmarkResampleWaveformInto-8          2316 ns/op      0 B/op    0 allocs/op
BenchmarkCoverCachePathCached-8            39.85 ns/op   0 B/op    0 allocs/op
```

### 关键决策

- `internal/state` 只保存运行时状态和 UI 当前需要的轻量快照。
- 歌曲、歌词、TTML DB、AI 结果等持久化数据不放入 runtime store。
- 新增 `internal/paths` 统一管理本地目录。
- 新增 `internal/storage` 管理数据库连接和迁移。
- 本地歌曲与歌词数据使用 SQLite 文件存储。
- TTML DB 索引和封面文件放入 cache/data 目录，不放入配置文件。
- UI 日志按需求只保留最新 50 条，默认不落盘。
- 歌词搜索和 AI 长任务必须从设计阶段支持 context 取消和 revision 防旧结果覆盖。

### 遗留问题

- 尚未选择具体 SQLite driver。
- 尚未实现 `internal/paths`。
- 尚未做 `LyricSyncBasic` 到 `LyricSync` 的配置迁移。
- 尚未将 UI 拆成侧边导航和多页面。
- 尚未扩展 Full 版本配置模型。

### 下一阶段入口

进入阶段 1：FluxUI 应用框架与导航。

阶段 1 首要任务：

- 建立 FluxUI 侧边导航。
- 建立五个页面路由。
- 将 UI 从 basic 单页 dashboard 拆分。
- 建立 Full 版本 UI 文案。
- 保持现有 store 订阅机制。

## 阶段 1：FluxUI 应用框架与导航

完成日期：2026-06-25

状态：已完成。

### 本阶段目标

- 建立 Full 版本 UI 骨架。
- 使用 FluxUI 自带侧边导航栏替换 basic 单页临时结构。
- 建立仪表盘、会话、歌曲、日志、设置五个主页面。
- 保留现有 store 订阅机制，继续通过状态事件驱动 UI 更新。

### 已完成内容

- 将窗口标题从 `LyricSync Basic` 更新为 `LyricSync`。
- 在 `internal/ui/ui.go` 中新增页面 key：
  - `dashboard`
  - `sessions`
  - `songs`
  - `logs`
  - `settings`
- 新增 FluxUI 应用壳：
  - 宽窗口使用 `NavigationDrawerElement`。
  - 窄窗口使用 `NavigationRailElement`。
  - 默认页面为仪表盘。
- 新增统一页面标题区，展示当前页面标题、页面说明、AMLL 状态、播放状态和最新通知。
- 新增五个主页面骨架：
  - 仪表盘：当前播放、音频波形、连接统计、歌词占位。
  - 会话：SMTC 会话选择和当前会话状态。
  - 歌曲：歌曲记录空状态和当前歌曲信息。
  - 日志：运行日志。
  - 设置：AMLL 连接、媒体、歌词、AI 设置占位。
- 继续使用 `store.Subscribe` 驱动 UI 状态更新。
- 没有引入固定时间全量刷新 UI。
- 移除仪表盘音频区的开发者字段展示，不再显示 Provider、Format、Frame。
- 用户可见页面标题和主要面板标题已切换为中文。

### 交付物

- 可切换的五个主页面。
- FluxUI 侧边导航应用骨架。
- 统一页面标题和内容区域。
- 订阅式 UI 状态更新仍然可用。

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- 阶段 1 只建立 Full 版本 UI 骨架，不提前实现歌曲数据库、歌词搜索、AI 设置等后续阶段能力。
- 页面路由先使用运行时页面 key 管理，避免在桌面端过早引入复杂路由状态。
- 侧边导航使用 FluxUI 自带 `NavigationDrawerElement`，窄窗口使用同属 FluxUI 导航组件的 `NavigationRailElement`。
- 仪表盘保留当前播放和波形的可用展示，但完整播放体验继续放到阶段 3。

### 遗留问题

- 歌曲页仍是骨架，播放记录、CRUD 和搜索将在阶段 5 实现。
- 设置页中的歌词、AI 配置仍是占位，完整配置模型将在阶段 4 和阶段 9 补齐。
- 会话页仍使用当前 `model.Session` 的轻量字段，动态进度和更多会话信息将在阶段 2 扩展。
- 日志保留数量仍由当前 state 实现控制，阶段 4 需要调整为最新 50 条。
- 尚未引入真实路由栈；如后续歌曲详情页需要深层跳转，可在歌曲阶段再接 FluxUI Router。

### 下一阶段入口

进入阶段 2：SMTC 会话管理与 active 会话稳定。

阶段 2 首要任务：

- 扩展 SMTC 会话模型。
- 在会话页展示每个会话的歌曲、艺人、专辑、状态和动态进度。
- 使用单选按钮作为 active 会话选择控件。
- 确认手动选择持久化和缺失会话等待行为。
- 继续防止多个 SMTC 之间频繁切换。

## 阶段 2：SMTC 会话管理与 active 会话稳定

完成日期：2026-06-25

状态：已完成。

### 本阶段目标

- 完成会话页。
- 实时展示所有 SMTC 会话信息。
- 使用单选按钮选择 active 会话。
- 保持 active 会话稳定，避免多个 SMTC 间频繁切换。
- 用户手动选择后持久化，并进入手动锁定模式。
- 手动选择的会话不存在时显示等待状态，不自动切到其他会话。
- 拆分 UI 文件。

### 已完成内容

- 扩展 `model.Session`：
  - 标题
  - 艺人
  - 专辑
  - 播放状态
  - 播放进度
  - 歌曲时长
- `media.Service` 发布完整 SMTC 会话快照。
- SMTC playback、timeline、media 变化时同步刷新会话列表。
- 500ms 媒体 ticker 同步刷新会话进度，保证会话页进度动态变化。
- 会话页选择控件从下拉框改为 FluxUI `RadioGroupElement`。
- 单选项包含：
  - 自动选择
  - 当前所有 SMTC 会话
  - 等待中的缺失手动会话
- 手动选择仍通过 `Runtime.SelectMediaSession` 保存到配置。
- 手动选择后 `AutoSelect=false`，进入手动锁定模式。
- 手动选择的会话消失时，`chooseSession` 返回 nil，不回退到其他会话。
- 会话详情卡片展示：
  - 会话名称
  - SMTC ID / AppID
  - 歌曲标题
  - 艺人和专辑
  - 播放状态
  - 动态进度条
- 根据 UI 复查结果修正主导航：
  - 宽窗口和窄窗口都统一使用 FluxUI `NavigationRailElement`。
  - 移除 `NavigationDrawerElement` 作为主侧边栏。
- 根据 UI 复查结果重做会话选择布局：
  - 不再把长会话标题放入 `RadioGroupElement` 的 label。
  - 每个会话使用独立选择卡片。
  - radio 只负责选择状态，长文本、状态和进度放在卡片内容区。
  - 会话列表从嵌套 panel 改为页面级 section + card list，减少不符合 MD3 的卡片嵌套。
- 会话页增加“等待选定会话恢复”等明确状态。
- 将会话页和会话组件从 `internal/ui/ui.go` 拆分到 `internal/ui/sessions.go`。
- 增加媒体层测试，覆盖会话快照中的播放元数据。

### 交付物

- 会话管理 UI。
- active 会话单选选择控件。
- active 会话持久化路径。
- 手动选择缺失时的等待状态。
- 扩展后的会话模型和实时会话快照。
- 拆分后的 UI 文件：
  - `internal/ui/ui.go`
  - `internal/ui/sessions.go`

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- 会话页使用 `RadioGroupElement` 作为 active 会话的主选择控件，符合阶段要求。
- `RadioGroupElement` 只承载短选择项，避免长文本破坏组件内部布局。
- 主侧边栏统一使用 `NavigationRailElement`，不使用 navigation drawer。
- 手动选择继续复用已有 `SelectedSessionID` 配置字段，避免引入重复状态。
- 手动选择缺失时不 fallback，保持用户选择的确定性。
- 自动模式仍保留“当前播放会话优先保持”的稳定策略，减少多个播放源之间的跳变。
- UI 文件拆分先从会话页开始，后续阶段再继续拆分其他页面。

### 遗留问题

- 会话页已显示动态进度，但完整播放控制体验仍在阶段 3 完成。
- `model.Session` 已满足阶段 2，会话封面等更重数据暂不放入列表，避免高频刷新负担。
- UI 仍有多个页面留在 `ui.go`，后续歌曲、日志、设置阶段可继续按页面拆分。
- 配置目录仍是阶段 0 记录的 `LyricSyncBasic`，将在配置阶段统一迁移。

### 下一阶段入口

进入阶段 3：仪表盘播放体验。

阶段 3 首要任务：

- 完成仪表盘歌曲信息布局。
- 增加封面展示。
- 增加上一首、播放/暂停、下一首控制按钮。
- 增加音量装饰控件。
- 优化波形展示。
- 增加逐行歌词区域的运行时占位和展示逻辑。

## 阶段 3：仪表盘播放体验

完成日期：2026-06-26

状态：已完成。

### 本阶段目标

- 完成用户日常使用的仪表盘主界面。
- 展示当前 active SMTC 会话的歌曲信息、封面、播放状态和播放进度。
- 增加上一首、播放/暂停、下一首控制按钮。
- 增加音量装饰控件、音频波形和当前歌词区域。
- 对空状态、暂停状态、无歌词状态做用户友好展示。

### 已完成内容

- 将仪表盘实现拆分到 `internal/ui/dashboard.go`，继续降低 `ui.go` 体积。
- 仪表盘展示当前歌曲标题、艺人、专辑、封面、播放状态和播放进度。
- 新增封面缓存渲染：将 `model.Track.CoverData` 写入用户缓存目录，再交给 FluxUI `ImageElement` 展示。
- 新增上一首、播放/暂停、下一首按钮，调用 active SMTC 的媒体控制逻辑。
- 在 `Runtime` 中新增 `ControlMedia`，供 UI 层复用现有 `media.Service.Control`。
- 新增装饰性音量滑块，仅展示当前音量，不写入真实音量控制逻辑。
- 新增固定高度、固定柱数的波形展示，并使用上一帧幅度做平滑，减少波形抖动和布局重排。
- 新增当前歌词区域，当前歌词数据管线未接入时展示空状态。
- 仪表盘不展示 Provider、format、frame、sample rate 等开发者信息。

### 交付物

- 完整仪表盘 UI。
- 可用的 SMTC 播放控制按钮。
- 封面展示与本地封面缓存。
- 稳定尺寸的音频波形图。
- 当前歌词区域的运行时占位。

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- 仪表盘只保留面向用户的播放信息，不展示音频帧、格式、Provider 等调试字段。
- 播放控制不在 UI 层重新实现，统一通过 runtime 转发到媒体服务。
- FluxUI 图片组件当前只接受文件路径，因此封面渲染通过稳定 hash 文件名写入缓存目录。
- 波形继续由订阅式 store 快照驱动，不新增 UI interval。
- 波形使用固定柱数和上一帧平滑，优先解决视觉抖动和重排问题。

### 遗留问题

- 当前歌词区域还没有真实歌词数据源，阶段 6 到阶段 8 接入歌词管线后替换占位。
- 音量控件按本阶段要求仅作为装饰，不控制系统或播放器音量。
- 封面缓存暂未做清理策略，后续本地数据目录和缓存治理阶段统一处理。

### 下一阶段入口

进入阶段 4：配置、日志与基础持久化。

阶段 4 首要任务：

- 扩展 Full 版本设置模型。
- 完成设置页基础项。
- 将运行日志保留数量调整为 50 条。
- 继续确认配置目录和本地缓存目录的迁移策略。

## 阶段 4：配置、日志与基础持久化

完成日期：2026-06-26

状态：已完成。

### 本阶段目标

- 补齐 Full 版本设置模型。
- 完成设置页基础配置项。
- 完成日志页，并仅保留软件自身最新 50 条日志。
- 配置修改后持久化，启动时加载配置。
- 处理 basic 配置目录到 Full 配置目录的兼容迁移。

### 已完成内容

- 扩展 `model.Config`：
  - AMLL WebSocket 地址、启动后自动连接、音频发送。
  - 搜词优先级。
  - 歌词清洗策略。
  - AI 服务商地址、API Key、模型、深度思考。
  - AI 翻译、AI 音译。
  - 软件关闭行为。
  - TTML DB 索引地址、自动更新和更新间隔。
- 完成设置页：
  - AMLL 设置。
  - 歌词设置。
  - AI 设置。
  - TTML DB 设置。
  - 应用关闭行为设置。
- 设置项修改后通过 `Runtime.SaveConfig` 立即保存并同步到运行时 store。
- `ConnectAMLL` 和 `DisconnectAMLL` 不再隐式修改“启动后自动连接”，该选项改由设置页显式控制。
- 完成日志页，只展示软件自身最新 50 条日志。
- `state.Store.AddLog` 增加 50 条上限，避免运行期日志无限增长。
- 配置目录从 `LyricSyncBasic/config.json` 迁移到 `LyricSync/config.json`。
- 启动加载配置时，如果新目录不存在但旧目录存在，会读取旧配置并写入新目录。
- 修正 Full 版可见命名：
  - `model.Version` 改为 Full 版本语义。
  - 初始状态、启动日志和本地模拟会话不再显示 `LyricSync Basic`。
- 增加配置测试，覆盖：
  - Full 设置默认值。
  - 搜词优先级规范化。
  - TTML DB 自动更新显式关闭。
  - 新配置目录路径。
  - 旧配置目录迁移。
  - 缺失布尔字段使用默认值，显式关闭仍保留。

### 交付物

- 设置页：
  - `internal/ui/settings.go`
- 日志页：
  - `internal/ui/logs.go`
- Full 版本配置模型：
  - `internal/model/model.go`
- 配置加载、保存和迁移：
  - `internal/config/config.go`
- 配置测试：
  - `internal/config/config_test.go`
- 日志数量限制：
  - `internal/state/state.go`

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- 设置页采用“修改即保存”，减少单独保存按钮带来的状态不一致。
- AMLL 连接和启动自动连接解耦，手动连接不再偷偷改变启动行为。
- 日志只作为运行期 UI 信息，当前阶段不落盘。
- 旧 `LyricSyncBasic` 配置保留读取兼容，并迁移到新的 `LyricSync` 配置目录。
- `sendAudio` 和 `ttmlDb.autoUpdateIndex` 这类默认值为 `true` 的布尔项，通过加载时检测 JSON 字段是否存在来区分“缺失”和“显式关闭”。
- “获取模型”和“立即更新索引”按钮先保留 UI 入口，实际网络逻辑放到 AI 和歌词来源阶段接入。
- “最小化到系统托盘”先完成配置持久化，真实关闭行为需要后续接入 FluxUI 窗口生命周期和托盘能力。

### 遗留问题

- AI 模型列表获取尚未接入真实 OpenAI 协议接口，将在 AI 阶段实现。
- TTML DB 索引下载和更新调度尚未实现，将在歌词来源和 TTML DB 阶段实现。
- 软件关闭时最小化到系统托盘的实际行为尚未绑定。
- 歌曲数据库和播放记录尚未实现，进入阶段 5 后补齐。

### 下一阶段入口

进入阶段 5：歌曲数据库与播放记录。

阶段 5 首要任务：

- 建立本地歌曲数据库。
- 记录播放过的歌曲并做唯一去重。
- 完成歌曲页列表、搜索和基础增删改查。
- 为每首歌曲预留四个平台歌词结果存储字段。
- 接入歌词预览入口，为后续歌词搜索和处理阶段准备。

## 阶段 5：歌曲数据库与播放记录

完成日期：2026-06-27

状态：已完成。

### 本阶段目标

- 建立本地 SQLite 歌曲数据库。
- 定义歌曲唯一性规则。
- active SMTC 歌曲变化时自动记录播放历史。
- 同一首歌只保留一条记录。
- 完成歌曲页最近记录展示和全部歌曲管理。
- 支持新增、编辑、删除、搜索、查看详情。
- 支持歌词预览入口。

### 已完成内容

- 新增 `internal/paths`，统一提供本地数据目录和数据库路径：
  - Windows 下对应用户本地数据/缓存目录中的 `LyricSync/lyricsync.db`。
- 新增 `internal/song`：
  - SQLite 连接和迁移。
  - `songs` 表保存歌曲元数据、播放次数、首次播放和最近播放时间。
  - `lyric_sources` 表为 TTML DB、QQ、酷狗、网易预留歌词缓存位。
  - 播放记录 upsert。
  - 搜索、详情、新增、编辑、删除。
  - 歌词缓存读取和更新。
- 歌曲唯一性规则：
  - 规范化标题。
  - 规范化艺人分隔符。
  - 有时长时使用 5 秒 duration bucket。
  - 无时长时回退到标题、艺人、专辑组合。
- `Runtime` 接入歌曲仓库：
  - 启动时订阅 `track_changed`。
  - active SMTC 歌曲变化后自动记录。
  - 记录成功后发布 `songs_changed` 事件。
- `state.Store` 新增轻量 `Notify`，用于发布非 snapshot 持久化数据变化事件。
- `main.go` 启动时打开 SQLite 数据库，失败时应用继续启动并写入日志。
- 歌曲页替换占位实现：
  - 最近播放。
  - 当前歌曲信息。
  - 全部歌曲视图。
  - 搜索。
  - 新增歌曲。
  - 编辑歌曲。
  - 删除确认。
  - 详情页。
  - 四个平台歌词缓存编辑和预览。
- 歌曲页使用独立 `ComponentElement`，避免在主页面路由切换时改变 root hook 数量。

### 交付物

- 本地 SQLite 歌曲数据库：
  - `internal/song`
- 本地路径工具：
  - `internal/paths`
- 自动播放记录接入：
  - `internal/app/app.go`
  - `main.go`
- 歌曲记录 UI：
  - `internal/ui/songs.go`
- 测试：
  - `internal/song/repository_test.go`
  - `internal/ui/songs_test.go`

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- 歌曲数据库不放入 `state.Snapshot`，避免高频音频和播放进度刷新时携带持久化大数据。
- 歌曲页只在 `songs_changed`、搜索条件或页面内部状态变化时读取 SQLite。
- 播放记录使用 upsert 保证同一首歌只有一条记录，重复播放只增加 `play_count` 并更新 `last_played_at`。
- 歌词缓存先以 `lyric_sources` 表预留四个平台位置，后续歌词搜索阶段直接写入同一结构。
- 歌曲页内部子页面使用本地 view state 管理，暂不引入完整路由栈。

### 遗留问题

- 歌词搜索、清洗和自动应用尚未接入，歌词缓存目前只能手动编辑。
- 当前歌词预览是逐行文本预览，统一 TTML 解析和逐行结构将在阶段 6 完成。
- 封面文件治理和歌曲封面持久化策略后续可与 AMLL 完整同步阶段一起完善。

### 下一阶段入口

进入阶段 6：统一歌词结构与 TTML 能力。

阶段 6 首要任务：

- 建立内部统一歌词结构。
- 参考 `/ref/amll-ttml` 完成 TTML 解析、生成和压缩适配。
- 歌词预览组件读取统一结构。
- 为后续多平台歌词搜索和 AMLL 歌词发送提供稳定数据模型。

## 阶段 5 后架构调整：接入 FluxUI Router

完成日期：2026-06-27

状态：已完成。

### 调整原因

- 原先主页面使用 `activePage` 本地状态和 `switch` 切换页面。
- 歌曲页使用 `songView` 本地状态管理最近播放、全部歌曲、详情、新增和编辑。
- 随着后续歌词搜索、AI 处理、歌曲详情、歌词预览等页面继续增加，继续堆本地状态会让页面流转难以维护。
- FluxUI 当前已提供 `RouterElement`、`RouteElement`、`UseNavigate`、`UseRoute`、`UseParams` 等路由能力，应尽早接入。

### 已完成内容

- 主应用壳接入 FluxUI `RouterElement`。
- 侧边 `NavigationRailElement` 不再写入 `activePage` 状态，而是根据当前路由高亮。
- 侧边导航切换改为 `NavigateReplace`，避免主页面切换不断增加返回栈。
- 默认路由为 `/dashboard`，应用启动后仍进入仪表盘。
- 建立主路由：
  - `/dashboard`
  - `/sessions`
  - `/songs`
  - `/logs`
  - `/settings`
- 建立歌曲子路由：
  - `/songs`
  - `/songs/all`
  - `/songs/new`
  - `/songs/:id`
  - `/songs/:id/edit`
- 歌曲页保留原有 CRUD 与加载逻辑，但当前视图由路由派生。
- 原有 `view.Set(...)` 调用通过路由适配器跳转，降低一次性改动范围。
- 编辑和新增路由使用 `rev` query 区分进入次数，避免复用旧表单状态。
- 新增应用级 404 页面，未知路径可返回仪表盘。

### 交付物

- Router 化主导航：
  - `internal/ui/ui.go`
- Router 化歌曲页视图流转：
  - `internal/ui/songs.go`
- 本阶段调整记录：
  - `docs/development-stage-log.md`

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- Router 只负责页面位置和跳转，不把歌曲数据库数据放进运行时 snapshot。
- 主导航使用 `NavigateReplace`，歌曲页内部跳转使用普通 `UseNavigate`，保留详情、编辑等页面的返回语义。
- 仪表盘波形平滑 hook 下沉到 `/dashboard` 路由组件内，避免主壳层因为路由变化产生 hook 顺序风险。
- 歌曲页继续使用统一组件和固定 hook 顺序，再由 `UseRoute` / `UseParams` 决定当前视图。

### 遗留问题

- 当前没有做独立的路由视觉回归测试，已通过 `go test ./...` 验证编译、单元测试和 hook 基础行为。
- 后续新增歌词搜索、AI 处理、歌词详情页时，应优先扩展 Router 路由，而不是恢复页内状态机。

## 阶段 5 后体验修正：歌曲页路由返回、列表区分与重复统计

完成日期：2026-06-27

状态：已完成。

### 调整原因

- 歌曲页返回按钮使用明确目标路由，页面增多后容易让跳转关系变乱。
- 最近播放和全部歌曲使用同一列表项样式，用户难以区分“浏览最近记录”和“管理全部歌曲”。
- FAB 放在滚动内容里，会跟随列表滚动，不符合浮动操作按钮预期。
- 歌曲唯一键包含时长，SMTC 先上报 0 时长、后上报真实时长时会产生重复歌曲记录。
- 编辑页表单没有填满内容区，视觉上像窄栏表单。

### 已完成内容

- 歌曲子页面增加统一返回图标按钮。
- 返回按钮优先调用 `NavigateBack` 返回上一个路由，没有返回栈时才走当前页面 fallback。
- 最近播放列表改为轻量行：
  - 使用历史图标。
  - 只保留标题、艺人/专辑、最近播放时间和时长。
  - 不展示编辑、删除和播放次数等管理信息。
- 全部歌曲列表保留管理型行：
  - 播放次数。
  - 时长。
  - 最近播放。
  - 查看、编辑、删除操作。
- 最近播放和全部歌曲页的 FAB 改为 `StackElement` 固定右下角，不再随滚动内容移动。
- 歌曲编辑页和歌词缓存表单改为全宽布局。
- 歌曲唯一键改为稳定的“规范化标题 + 规范化艺人”，专辑、时长和封面只作为元数据保存与更新。
- 数据库启动迁移时会合并旧规则产生的重复歌曲行：
  - 合并播放次数。
  - 保留最早首次播放时间。
  - 保留最近播放时间。
  - 回填非零时长和封面 hash。
  - 尽量保留重复行中的歌词缓存。

### 交付物

- 歌曲页 UI 修正：
  - `internal/ui/songs.go`
- 歌曲去重和旧数据合并：
  - `internal/song/song.go`
  - `internal/song/repository.go`
- 测试：
  - `internal/song/repository_test.go`

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- 路由返回使用浏览栈语义，不再把“返回某个固定页面”作为默认交互。
- 最近播放是查看入口，全部歌曲是管理入口，因此两者列表密度和操作按钮不同。
- 歌曲唯一性优先保证 SMTC 元数据变化时稳定，不再让时长和专辑影响唯一键。

### 遗留问题

- 旧数据库重复行会在下次启动打开数据库时自动合并；已经运行中的旧数据需要重启后生效。
- 还未做真实窗口截图回归，当前通过单元测试和编译验证。

## 阶段 6：统一歌词结构与 TTML 能力

完成日期：2026-06-27

状态：已完成。

### 本阶段目标

- 建立统一歌词模型，让不同平台歌词都能进入同一处理链路。
- 支持 TTML 解析、生成和压缩。
- 支持逐行歌词提取，用于歌曲预览和仪表盘当前歌词展示。
- 定义歌词可用性判断、翻译、音译和逐词时间轴字段。

### 已完成内容

- 新增 `internal/lyric` 领域模块：
  - `Document`、`Line`、`Word`、`Metadata` 统一歌词模型。
  - `TranslatedLyric`、`RomanLyric`、`Word.RomanText` 字段。
  - `IsBackground`、`IsDuet`、`IgnoreSync` 等 TTML 可表达行属性。
  - 逐词 `StartTimeMs` / `EndTimeMs` 时间轴。
- 通过 `go get github.com/xiaowumin-mark/amll-ttml` 接入官方 TTML 库，本项目只保留内部模型适配层：
  - `ParseTTML` 调用 `amllttml.ParseLyric`，再转换为 `internal/lyric.Document`。
  - `GenerateTTML` 将 `internal/lyric.Document` 转换为 `amllttml.TTMLLyric`，再调用 `amllttml.ExportTTMLText`。
  - `CompressTTML` 使用官方库解析和紧凑导出。
  - `ParseTimestamp` / `FormatTimestamp` 仅用于 LRC、预览和内部时间格式化，不参与 TTML XML 解析或生成。
- 增加 LRC/普通文本入口：
  - LRC 可转换为统一逐行结构。
  - 普通文本可用于预览，但因没有时间轴不会被判定为可直接同步歌词。
- 定义歌词可用性判断：
  - 至少存在原文歌词行。
  - 至少存在可解析时间轴。
  - 转换为统一结构后没有解析错误。
- 歌词缓存写入接入统一结构：
  - `song.Repository.SetLyric` 保存歌词时会自动生成紧凑 TTML。
  - 自动维护 `available`、`has_translation`、`has_transliteration`。
  - 数据库打开迁移时会尝试补齐旧歌词缓存的 TTML 和可用性标记。
- 歌曲页歌词预览改为读取统一结构：
  - 不再直接把平台原始歌词文本按行展示。
  - LRC 预览会隐藏时间戳，只显示用户可读原文、翻译、音译。
  - QQ 翻译占位 `//` 会被清理为空翻译。
- 仪表盘当前歌词接入运行时状态：
  - `state.Snapshot` 新增轻量当前歌词行。
  - active 歌曲命中本地歌词缓存时发布逐行歌词。
  - idle 或不可记录歌曲会清空当前歌词，避免显示上一首内容。
- AMLL 歌词发送出口接入统一结构：
  - 新增 `setLyric` + `format:"ttml"` 消息。
  - `lyrics_changed` 事件可触发歌词发送。
  - `SendSnapshot` 会携带当前已加载的 TTML 歌词，避免先加载歌词、后连接 AMLL 时漏发。

### 交付物

- 统一歌词模型与 TTML 能力：
  - `internal/lyric`
- 歌词缓存规范化：
  - `internal/song/repository.go`
- 当前歌词运行时状态：
  - `internal/model/model.go`
  - `internal/state/state.go`
  - `internal/app/app.go`
- AMLL 歌词消息出口：
  - `internal/amll/protocol.go`
  - `internal/amll/connector.go`
- UI 预览和仪表盘当前歌词：
  - `internal/ui/songs.go`
  - `internal/ui/dashboard.go`
- 测试：
  - `internal/lyric/lyric_test.go`
  - `internal/song/repository_test.go`
  - `internal/amll/protocol_test.go`
  - `internal/ui/songs_test.go`

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- TTML 解析、生成和压缩不在项目内自写 XML 逻辑，统一委托 `github.com/xiaowumin-mark/amll-ttml` 官方依赖。
- `/ref/amll-ttml` 只作为结构和行为参考；实际依赖通过 Go module 引入，不直接 import `/ref` 目录代码。
- 数据库仍保存原始歌词和统一后的 TTML 两份数据：原始歌词便于后续排查和重新清洗，TTML 作为统一处理链路与 AMLL 输出格式。
- UI 预览和仪表盘只消费统一结构导出的逐行歌词，不依赖 QQ、网易、酷狗或 TTML DB 的原始结构。
- 普通文本可预览但不判定为可同步歌词，避免没有时间轴的歌词被误发给 AMLL。
- AMLL 当前阶段优先发送 TTML 格式歌词；结构化 `lines` 输出可在完整同步阶段按需补齐。

### 遗留问题

- TTML 正确性跟随 `amll-ttml` 官方库；后续如果遇到兼容性问题，应优先升级依赖或调整适配层字段映射，不在项目内重新实现 TTML XML 解析器。
- 多平台原始歌词结构转换尚未实现，将在阶段 7 接入 QQ、网易、酷狗、TTML DB 搜索时补齐。
- AI 清洗、翻译、音译和任务取消策略尚未接入，将在阶段 8 到阶段 9 完成。

### 下一阶段入口

进入阶段 7：歌词来源接入与并行搜索。

阶段 7 首要任务：

- 接入 TTML DB 索引和歌词获取。
- 接入 AMLX-MUSIC-API 的 QQ、网易、酷狗歌词搜索。
- 将各平台结果先转换为 `internal/lyric.Document`。
- 按用户设置的搜词优先级选择第一份可用歌词。
- 将四个平台结果写入当前 `lyric_sources` 缓存结构。

## 后续阶段记录模板

````markdown
## 阶段 N：阶段名称

完成日期：YYYY-MM-DD

状态：已完成。

### 本阶段目标

- ...

### 已完成内容

- ...

### 交付物

- ...

### 验证方式

```powershell
go test ./...
```

结果：

```text
通过 / 未通过
```

### 关键决策

- ...

### 遗留问题

- ...

### 下一阶段入口

- ...
````
## 阶段 7：歌词来源接入与并行搜索

完成日期：2026-06-27

状态：已完成。

### 本阶段目标

- 接入 TTML DB、QQ 音乐、网易云音乐、酷狗音乐。
- 歌曲变化后并行搜索多个来源。
- 根据用户设置的搜词优先级选择第一份可用歌词。
- 本地已有可用歌词时优先使用本地，不等待网络搜索。

### 已完成内容

- 新增歌词搜索服务：
  - `internal/lyric/SearchService`
  - `internal/lyric/Provider`
  - `internal/lyric/ProviderResult`
- 通过 `go get github.com/xiaowumin-mark/AMLX-MUSIC-API` 接入三平台：
  - QQ 音乐
  - 网易云音乐
  - 酷狗音乐
- 三平台搜索不再手写平台 HTTP API，统一使用 `AMLX-MUSIC-API` 的 provider。
- 新增 AMLX 统一模型到 LyricSync 内部歌词结构的转换，覆盖行级歌词、翻译、音译和逐词/音节时间轴。
- QQ 音乐翻译行 `//` 会在内部统一结构里归一为空翻译。
- 新增 TTML DB 本地索引缓存、自动更新、手动更新和本地搜索。
- Runtime 接入自动搜词流程：本地优先，未命中再并行搜索，保存各来源结果后按用户优先级应用。
- 新增歌词搜索任务取消，快速切歌时取消上一首歌的搜索任务，避免旧结果覆盖新歌曲。
- 歌曲仓库补齐 `source_track_id` 保存和当前应用歌词来源写入能力。

### 交付物

- 多平台歌词搜索服务：
  - `internal/lyric/search_service.go`
  - `internal/lyric/provider_amlx.go`
  - `internal/lyric/provider_ttmldb.go`
  - `internal/lyric/source.go`
- TTML DB 索引缓存：
  - `internal/paths/paths.go`
  - `internal/lyric/provider_ttmldb.go`
- 歌词来源存储增强：
  - `internal/song/repository.go`
- 自动搜索与应用流程：
  - `internal/app/app.go`
- 设置页手动更新索引：
  - `internal/ui/settings.go`
- 测试：
  - `internal/lyric/source_search_test.go`

### 验证方式

执行：
```powershell
go test ./...
```

结果：
```text
通过
```

### 关键决策

- QQ、网易、酷狗不在本项目内手写平台 API，直接使用 `AMLX-MUSIC-API` 依赖。
- `/ref/AMLX-MUSIC-API` 继续只作为参考，不直接 import 参考目录代码。
- TTML DB 不属于 `AMLX-MUSIC-API`，因此保留本项目内独立的索引缓存和搜索逻辑。
- 搜索并发执行，选择结果严格按设置优先级，而不是按网络返回先后顺序。
- 本地缓存可用时不启动网络阻塞路径，保证切歌响应和 AMLL 歌词发布速度。

### 遗留问题

- 平台接口可能受网络、风控或版权状态影响，失败时会保留日志并回退到其它可用来源。
- TTML DB 匹配目前使用标题、艺人、专辑的本地评分，后续可结合更多平台 ID 提高命中率。
- AI 清洗、AI 翻译和 AI 音译仍留到后续 AI 阶段实现。

### 下一阶段入口

进入阶段 8：歌词任务调度与防堆积。

阶段 8 首要任务：
- 进一步完善歌词搜索任务队列和 revision 保护。
- 处理更复杂的快速切歌、AI 长任务和手动应用歌词场景。
- 增强任务状态日志和 UI 可见反馈。

## 阶段 7 后续修正：歌词匹配、TTML 依赖与歌词展示

完成日期：2026-06-27

状态：已完成。

### 调整原因

- 部分播放器在未播放时会通过 SMTC 上报等待文案，不能把这类文本当作歌曲写入数据库。
- 部分播放器会把艺人、专辑或标题混写到同一字段，需要做保守拆分和修正。
- TTML 解析和生成不应由本项目自写，必须使用 `amll-ttml` 官方库。
- TTML DB 匹配过宽，随机字符串也可能命中，需要降低误匹配风险。
- 用户需要自定义 TTML 歌词、固定歌词来源、完整歌词预览和仪表盘完整歌词展示。

### 已完成内容

- 接入 `github.com/xiaowumin-mark/amll-ttml` 作为直接依赖，删除项目内自写 TTML XML 解析和生成逻辑。
- `internal/lyric/ttml.go` 改为官方库薄适配层：
  - `ParseTTML` 使用 `amllttml.ParseLyric`。
  - `GenerateTTML` 使用 `amllttml.ExportTTMLText`。
  - `CompressTTML` 使用官方库解析后再紧凑导出。
- 新增 SMTC 歌曲可记录性判断：标题和艺人必须同时存在，并过滤 waiting、loading、unknown、no media 等占位文本。
- 增加混合元数据拆分逻辑，对 `Artist - Title`、`Artist - Album` 等常见形式做保守修正。
- 歌曲数据库新增 `fixed_lyric_source`，支持用户固定使用某一个歌词来源，取消自动匹配。
- 歌词来源新增 `custom`，每首歌都有自定义 TTML 歌词槽位，并参与优先级兜底。
- 歌曲编辑页支持固定歌词来源选择和自定义 TTML 粘贴保存。
- 歌词选择逻辑支持固定来源；未固定时按设置优先级选择，并把 `custom` 放在最后兜底。
- 收紧 TTML DB 匹配阈值，并过滤“只有 TTML DB 命中、QQ/酷狗/网易全部无可用结果”的可疑结果。
- 仪表盘歌词区域改为完整逐行渲染，支持普通行、背景行、对唱、翻译和音译展示。
- 仪表盘新增当前歌曲详情按钮，可直接跳转当前歌曲数据库详情页。
- 歌曲详情页歌词预览新增“查看全部歌词”，并新增全文歌词路由 `/songs/:id/lyrics`。
- 全文歌词页显示 TTML metadata、每行起止时间、背景行/对唱标记、翻译和音译。

### 交付物

- TTML 官方库适配：
  - `internal/lyric/ttml.go`
  - `go.mod`
  - `go.sum`
- 歌词搜索和匹配增强：
  - `internal/lyric/search_service.go`
  - `internal/lyric/provider_ttmldb.go`
- 歌曲模型和数据库迁移：
  - `internal/song/song.go`
  - `internal/song/repository.go`
- 自动记录与歌词应用：
  - `internal/app/app.go`
  - `internal/model/model.go`
  - `internal/config/config.go`
- UI 调整：
  - `internal/ui/dashboard.go`
  - `internal/ui/songs.go`
  - `internal/ui/ui.go`
- 测试：
  - `internal/song/repository_test.go`

### 验证方式

执行：

```powershell
go test ./...
```

结果：

```text
通过
```

### 关键决策

- TTML 的规范兼容性由 `amll-ttml` 官方库负责，本项目不再维护自写 TTML XML parser/writer。
- 项目内部仍保留 `Document` 作为统一歌词结构，用于 UI、歌词来源转换、可用性判断和 AMLL 输出前的数据整理。
- 自定义歌词只接受 TTML，避免没有时间轴的文本被误用于 AMLL 同步。
- TTML DB 不单独作为低置信度歌曲识别依据，至少需要第三方平台搜索结果参与校验，减少占位 SMTC 文案误命中。

### 遗留问题

- 自定义歌词当前支持粘贴 TTML 字符串，文件选择导入可以后续接 FluxUI 文件对话框补齐。
- SMTC 混合字段拆分只能做保守启发式处理，后续可结合平台搜索结果进一步回填更准确的标题、艺人和专辑。

## 阶段 7 后续修正：歌词性能、五来源预览与来源延迟

完成日期：2026-06-27

状态：已完成。

### 调整原因

- 仪表盘完整歌词渲染后，播放进度高频更新会导致歌词计算和 UI 重绘压力偏高。
- 歌曲详情页只预览单一歌词来源，不能直接比较 TTML DB、QQ、酷狗、网易和自定义歌词。
- 固定歌词来源和全局歌词优先级都需要包含自定义歌词，避免用户自填 TTML 无法参与正常选择链路。
- 不同来源歌词可能与音频有固定时间差，需要为每份歌词提供独立延迟设置，但不能修改歌词文件本身。

### 已完成内容

- UI 状态订阅对播放进度更新做降频处理，减少 dashboard 跟随 60fps 进度重绘的压力。
- AMLL WebSocket 的实时进度仍保持 60fps 发送，不受 UI 降频影响。
- 仪表盘歌词区域改为只渲染当前行附近窗口，保留当前上下文展示，避免整首歌在高频进度下反复计算和布局。
- 全文歌词页改为 FluxUI 虚拟列表，并固定列表高度，打开长歌词时减少一次性布局成本。
- 歌曲详情页歌词预览固定展示五个来源：`ttml-db`、`qq`、`kugou`、`netease`、`custom`。
- 全文歌词路由支持 `source` 查询参数，可分别查看每个歌词来源的完整内容。
- 全局歌词优先级默认加入 `custom`，设置页优先级拖拽、标签和校验逻辑同步支持自定义歌词。
- 每个歌词来源新增 `delay_ms` 字段，数据库迁移会为旧库补齐默认值。
- 歌曲编辑页为五个歌词来源都提供延迟毫秒输入，支持正数和负数。
- AMLL 发送播放进度时按当前应用歌词来源的 `delay_ms` 做偏移；该偏移只影响 WebSocket 同步进度，不修改 TTML 内容，也不影响仪表盘本地高亮。
- 增加歌词解析缓存，减少详情页、预览页和全文页重复解析同一份 TTML 的开销。

### 交付物

- 仪表盘性能优化：
  - `internal/ui/dashboard.go`
  - `internal/state/state.go`
- 五来源歌词预览与全文路由：
  - `internal/ui/songs.go`
  - `internal/ui/ui.go`
- 歌词来源延迟模型、存储和应用：
  - `internal/model/model.go`
  - `internal/song/song.go`
  - `internal/song/repository.go`
  - `internal/app/app.go`
  - `internal/amll/protocol.go`
  - `internal/amll/connector.go`
- 设置页自定义歌词优先级：
  - `internal/config/config.go`
  - `internal/ui/settings.go`
- 测试：
  - `internal/amll/protocol_test.go`
  - `internal/song/repository_test.go`
  - `internal/ui/songs_test.go`
  - `internal/config/config_test.go`
  - `internal/ui/settings_test.go`

### 验证方式

执行：

```powershell
go test ./...
gopls check internal/ui/songs.go internal/ui/dashboard.go internal/app/app.go internal/amll/protocol.go internal/song/repository.go internal/config/config.go internal/ui/settings.go internal/state/state.go
```

结果：

```text
通过
```

### 关键决策

- UI 不再跟随 AMLL 发送频率刷新；AMLL 同步和本地视觉刷新分离。
- 来源延迟采用“偏移实时进度”的方式实现，避免破坏用户原始歌词和平台原始数据。
- 自定义歌词作为第五个来源参与优先级和固定来源选择，但仍要求是 TTML，保证后续 AMLL 输出链路稳定。

### 遗留问题

- 自定义歌词文件选择导入仍待接入 FluxUI 文件对话框；当前继续支持直接粘贴 TTML。
- 若后续长歌词仍出现卡顿，可进一步把详情页五来源预览改成懒解析，仅在展开或进入全文页时解析。

## 阶段 7 后续修正：三平台原始歌词解析与当前歌词刷新

完成日期：2026-06-27

状态：已完成。

### 调整原因

- QQ 音乐、酷狗音乐、网易云音乐返回的 raw 歌词格式不完全一致，不能只按普通 LRC 处理。
- 网易云存在逐字格式和普通 LRC fallback；酷狗 KRC 内可能带内嵌翻译和音译。
- 用户设置固定歌词来源后，当前仪表盘和 AMLL WebSocket 应立即应用新的来源。
- 仪表盘歌词不应继续展示大窗口，应只显示当前 active 歌词附近的少量上下文。

### 已完成内容

- 新增平台 raw 歌词解析入口，自动识别并解析：
  - 酷狗 KRC：`[start,duration]` 行、`<offset,duration,0>` 逐字时间轴、`[language:...]` 内嵌翻译和音译。
  - QQ QRC：`[start,duration]` 行、`text(start,duration)` 逐字时间轴、背景和声行识别。
  - 网易 YRC：`[start,duration]` 行、`(start,duration,0)` 逐字时间轴。
  - 普通 LRC fallback。
- `Parse` 和 `NormalizeContent` 统一接入平台 raw 解析，歌曲数据库保存 raw 歌词时会生成正确 TTML。
- AMLX provider 转换时优先用 raw 重建平台歌词结构，再叠加 AMLX 返回的翻译和音译。
- 固定歌词来源变更后，如果歌曲是当前播放歌曲，会重新发布当前歌词到 store；AMLL connector 通过 `lyrics_changed` 重新发送 `setLyric`。
- 仪表盘歌词区域改为显示当前 active 最上方歌词的上一行、同时 active 的歌词行和下一行。
- 全文歌词页继续使用 FluxUI 虚拟列表。

### 交付物

- 平台 raw 解析：
  - `internal/lyric/platform_parse.go`
  - `internal/lyric/parse.go`
  - `internal/lyric/provider_amlx.go`
- 固定来源刷新：
  - `internal/app/app.go`
- 仪表盘歌词范围：
  - `internal/ui/dashboard.go`
- 测试：
  - `internal/lyric/platform_parse_test.go`
  - `internal/lyric/source_search_test.go`

### 验证方式

执行：

```powershell
go test ./...
gopls check internal/lyric/platform_parse.go internal/lyric/parse.go internal/lyric/provider_amlx.go internal/lyric/source_search_test.go internal/app/app.go internal/ui/dashboard.go internal/ui/songs.go
```

结果：

```text
通过
```

### 关键决策

- 不修改 `AMLX-MUSIC-API` 依赖源码；在 LyricSync 内部把平台 raw 格式转换到统一歌词结构。
- KRC/QRC/YRC 解析只服务于统一模型和 TTML 输出，避免 UI 直接依赖平台原始结构。
- 固定来源刷新走现有 `lyrics_changed` 事件，不新增独立 WS 发送路径。

## 阶段 7 后续修正：仪表盘歌词窗口与全部歌曲虚拟列表

完成日期：2026-06-27

状态：已完成。

### 调整原因

- 仪表盘歌词需要保留 active 上方一行，active 下方保持后续歌词窗口，而不是只显示三行。
- 仪表盘歌词区域不应通过外层滚动容器查看整首歌。
- 全部歌曲列表可能增长，需要避免一次性构建所有歌曲 item。
- 固定歌词来源选项需要稳定包含用户自定义歌词。

### 已完成内容

- 仪表盘歌词窗口调整为：active 最上方歌词的上一行、所有同时 active 的歌词行、后续 8 行。
- 仪表盘歌词区域改用 FluxUI `ListViewElement` 渲染固定窗口，桌面侧栏移除外层歌词滚动。
- 全部歌曲页面列表改为 FluxUI `ListViewElement`，只构建可见列表项。
- 固定歌词搜索结果下拉改为复用统一歌词来源选项，包含 `custom` 自定义歌词。
- 新增仪表盘歌词窗口范围测试，覆盖普通 active 和多行同时 active。

### 验证方式

执行：

```powershell
go test ./...
gopls check internal/ui/dashboard.go internal/ui/dashboard_test.go internal/ui/songs.go internal/ui/settings.go
```

结果：

```text
通过
```
