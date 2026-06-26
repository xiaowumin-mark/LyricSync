# 阶段 0：基线整理与架构确认

完成日期：2026-06-25

## 结论

阶段 0 已完成。当前项目已经从旧 basic 目录迁移到新的 `internal/...` 包结构，但仍保留若干 basic 命名和临时 UI。Full 版本不应继续在当前单页 UI 上堆功能，而应按明确模块边界推进。

本阶段未改业务代码，只完成架构确认和开发边界固化。

## 当前代码状态

当前 Go 模块：

- Go module：`lyricsync`
- Go 版本：`1.25.1`
- UI 依赖：`github.com/xiaowumin-mark/FluxUI`
- SMTC/音频依赖：`github.com/xiaowumin-mark/smtc-suite-go`
- WebSocket 依赖：`github.com/gorilla/websocket`

当前测试基线：

```powershell
go test ./...
```

结果：通过。

当前存在的核心包：

```text
internal/amll
internal/app
internal/config
internal/media
internal/model
internal/state
internal/ui
```

当前缺失的 Full 版本能力：

- 歌曲数据库。
- 歌词统一结构。
- TTML 解析、生成、压缩业务层。
- TTML DB 索引缓存和搜索。
- QQ、网易、酷狗歌词来源接入。
- 歌词任务调度和快速切歌取消策略。
- AI OpenAI 协议客户端。
- Full 版本设置模型。
- Full 版本侧边导航 UI。

## 现有模块职责

### `internal/app`

当前职责：

- 组合 `media.Service` 和 `amll.Connector`。
- 启动和停止运行时。
- 处理 AMLL 连接、断开、发送快照。
- 保存 AMLL 和 SMTC 选择相关配置。

Full 版本定位：

- 继续作为应用运行时编排层。
- 不承载歌词搜索、歌曲数据库、AI 处理等具体业务逻辑。
- 后续只调用各业务 service，并负责生命周期和上下文取消。

### `internal/config`

当前职责：

- 生成默认配置。
- 从 JSON 文件加载配置。
- 保存配置。

现状问题：

- 配置目录仍为 `LyricSyncBasic`。
- 配置模型只包含 AMLL 和 Media。
- 缺少歌词搜索、AI、关闭行为、TTML DB 等 Full 版本配置。

Full 版本定位：

- 负责配置文件读写、默认值合并和配置迁移。
- 不负责歌曲数据库和 TTML DB 索引文件。

### `internal/state`

当前职责：

- 保存运行时快照。
- 发布订阅运行时事件。
- 保存当前 track、playback、audio、sessions、AMLL 状态和 logs。

现状问题：

- 日志保留 200 条，Full 需求是 50 条。
- Store 同时承载 UI 快照和业务事件，后续需要避免放入大型持久化数据。

Full 版本定位：

- 只保存运行时状态和 UI 当前需要的轻量快照。
- 不直接保存完整歌曲库、歌词库、TTML DB 索引。
- 对封面、PCM 等大数据只保存当前播放必要数据，持久化交给专门仓储。

### `internal/media`

当前职责：

- 启动 SMTC monitor。
- 选择当前 SMTC 会话。
- 读取媒体信息、封面、播放状态、时间轴。
- 控制播放、暂停、上一首、下一首、seek。
- 启动 WASAPI loopback 音频采集。
- 生成音频波形数据。

已具备能力：

- 手动 SMTC 选择字段已存在。
- 自动选择逻辑已有稳定当前播放会话的处理。
- SMTC thumbnail 已映射到 `Track.CoverData`。
- 音频波形已有处理器。

Full 版本定位：

- 专注媒体会话、音频采集和播放控制。
- 不负责歌词搜索和歌曲数据库。
- 后续需要暴露更完整的 session 信息给会话页。

### `internal/amll`

当前职责：

- AMLL WebSocket 连接。
- AMLL V2 JSON 协议。
- BinaryV2 音频数据。
- BinaryV2 封面数据。
- 处理远端控制指令。

已具备能力：

- `setMusic`
- `progress`
- `volume`
- `paused` / `resumed`
- binary audio，magic `0x0000`
- binary cover，magic `0x0001`

Full 版本定位：

- 继续负责协议编码和连接发送。
- 后续新增 `setLyric`。
- 不负责歌词选择和歌词业务处理。

### `internal/ui`

当前职责：

- FluxUI 单页 dashboard。
- AMLL 连接控制。
- 当前播放信息。
- 音频波形。
- 简易会话选择。
- 日志展示。

现状问题：

- 标题仍为 `LyricSync Basic`。
- 还不是侧边导航结构。
- Audio 面板仍展示 Provider、Format、Frame 等开发者信息。
- 会话、歌曲、日志、设置没有独立页面。

Full 版本定位：

- 使用 FluxUI 自带侧边导航栏。
- 页面拆分为仪表盘、会话、歌曲、日志、设置。
- UI 只消费运行时 state 和 service 暴露的 view model。
- 不直接做歌词搜索、数据库读写和 AI 请求。

### `internal/model`

当前职责：

- 存放共享配置和运行时 DTO。

现状问题：

- `Version` 仍为 `0.1.0-basic`。
- Full 版本数据模型会明显增多，不适合全部堆到一个文件。

Full 版本定位：

- 保留跨模块共享的基础 DTO。
- 歌曲、歌词、AI、TTML DB 等领域模型优先放在对应业务包内。
- 只将 UI 和跨模块必须共享的轻量类型放入 `internal/model`。

## `/ref` 使用边界

`/ref` 下存在以下参考仓库：

- `FluxUI`
- `smtc-suite-go`
- `AMLX-MUSIC-API`
- `amll-ttml`
- `amll-player`
- `Unilyric`
- `VoxBackend`

使用原则：

- FluxUI 是 UI 框架依赖，当前通过 `go.mod` replace 到本地 `./ref/FluxUI`。
- 其他参考仓库只用于阅读协议、结构和实现思路。
- 严禁从参考仓库直接 import 业务代码。
- 严禁把参考仓库代码整段复制到项目。
- 需要的能力应在 LyricSync 自身模块内重新实现，并通过测试验证。

## Full 版本目标模块拆分

建议最终核心模块：

```text
internal/app
internal/config
internal/paths
internal/state
internal/media
internal/amll
internal/song
internal/lyric
internal/lyric/provider/ttmldb
internal/lyric/provider/qqmusic
internal/lyric/provider/netease
internal/lyric/provider/kugou
internal/ai
internal/storage
internal/ui
```

### `internal/paths`

新增模块。

职责：

- 统一管理应用目录。
- 提供 config、data、cache、cover、ttml-db index 等路径。
- 处理旧 `LyricSyncBasic` 配置迁移到 `LyricSync`。

### `internal/storage`

新增模块。

职责：

- 管理本地数据库连接和迁移。
- 提供事务边界。
- 对外不暴露具体数据库驱动细节。

建议：

- 使用 SQLite 作为本地数据库。
- 优先选择不依赖外部 C 编译环境的实现。
- 文件路径由 `internal/paths` 提供。

### `internal/song`

新增模块。

职责：

- 歌曲记录。
- 歌曲唯一性判断。
- CRUD。
- 搜索。
- 播放历史统计。
- 当前应用歌词引用。

不负责：

- 具体歌词搜索。
- AI 处理。
- AMLL 发送。

### `internal/lyric`

新增模块。

职责：

- 统一歌词结构。
- 平台歌词转换。
- 歌词可用性判断。
- 歌词优先级选择。
- TTML 解析、生成、压缩封装。
- 逐行歌词 view model。
- 歌词任务调度入口。

### `internal/lyric/provider/*`

新增模块。

职责：

- 各平台搜索和歌词获取。
- 各平台原始结构转换为统一歌词结构。
- 保留原始响应，便于调试和后续重新处理。

### `internal/ai`

新增模块。

职责：

- OpenAI 协议客户端。
- 模型列表获取。
- 歌词清洗。
- 翻译。
- 音译。
- 请求取消、超时和错误归一化。

### `internal/ui`

调整模块。

建议目录：

```text
internal/ui
internal/ui/pages
internal/ui/components
internal/ui/viewmodel
```

职责：

- 页面组合。
- FluxUI 组件。
- 将 runtime snapshot 转成 UI view model。
- 不直接访问数据库和外部网络。

## 本地数据存储方案

### 应用目录

建议目录：

```text
Config: %AppData%\LyricSync\config.json
Data:   %LocalAppData%\LyricSync\lyricsync.db
Cache:  %LocalAppData%\LyricSync\cache
Covers: %LocalAppData%\LyricSync\covers
TTML:   %LocalAppData%\LyricSync\cache\ttmldb
```

跨平台时统一通过 `internal/paths` 获取，不在业务代码中拼接系统路径。

### 配置文件

文件：

```text
config.json
```

持久化内容：

- AMLL WebSocket 地址。
- 启动后自动连接。
- 是否发送音频。
- Media 自动选择。
- 手动选择的 SMTC session ID。
- 搜词优先级。
- 软件关闭行为。
- AI 服务商地址。
- AI API Key。
- AI 模型。
- 是否深度思考。
- 歌词清洗策略。
- AI 翻译开关。
- AI 音译开关。
- TTML DB 自动更新设置。

不放入配置文件：

- 歌曲记录。
- 歌词原文。
- TTML DB 索引正文。
- 封面图片二进制。
- 音频 PCM。

### 数据库

文件：

```text
lyricsync.db
```

持久化内容：

- 歌曲表。
- 歌词表。
- 歌词来源表。
- 播放记录或播放统计。
- 当前应用歌词引用。
- AI 处理结果。
- TTML DB 索引元数据。

建议最小表：

```text
songs
lyrics
lyric_sources
play_events
settings_meta
```

### 缓存

缓存目录：

```text
cache
```

持久化内容：

- TTML DB 原始索引文件。
- TTML DB 索引构建后的辅助文件。
- AI 模型列表缓存。
- 临时下载结果。

缓存可以删除，删除后应用应能重新下载或重建。

### 封面目录

目录：

```text
covers
```

持久化内容：

- 当前播放过歌曲的封面。
- 文件名建议使用 cover hash。

数据库只保存封面 hash、MIME type、文件路径或相对路径。

### 日志

按照需求，UI 日志仅保留软件自身最新 50 条。

建议：

- UI 日志默认只保存在内存状态。
- 如后续需要诊断日志，另行设计 debug log 文件，不与用户日志混用。

## 运行时状态与持久化边界

### 运行时状态

放在 `internal/state`：

- 当前 active track。
- 当前 playback。
- 当前 audio frame 和 spectrum。
- 当前 SMTC sessions。
- 当前 AMLL 连接状态。
- 当前歌词加载状态。
- 当前逐行歌词展示状态。
- 当前搜索任务状态。
- 最新 50 条用户可见日志。

运行时状态特点：

- 可订阅。
- 可随应用退出丢失。
- 只保留 UI 当前需要的数据。
- 不存储大体积历史数据。

### 持久化状态

写入配置或数据库：

- 用户设置。
- active SMTC 手动选择。
- 歌曲记录。
- 歌词记录。
- 各平台原始歌词。
- 统一后的 TTML 歌词。
- 播放历史。
- TTML DB 索引元数据。
- 封面文件引用。

持久化状态特点：

- 重启后恢复。
- 可被 CRUD。
- 不通过 UI store 直接全量加载。

### 不持久化的数据

- PCM 音频帧。
- 波形每帧数据。
- WebSocket 临时连接对象。
- SMTC runtime session 对象。
- 已取消任务的中间结果。
- 过期搜索结果。

## 基础开发任务列表

### 阶段 1 前置任务

- 将 `LyricSync Basic` UI 文案迁移为 `LyricSync`。
- 将 `model.Version` 从 basic 版本迁移到 Full 版本语义。
- 新增 `internal/paths`。
- 将配置目录从 `LyricSyncBasic` 迁移到 `LyricSync`，保留旧配置读取兼容。
- 将日志上限从 200 调整为 50。
- 开始拆分 FluxUI 页面结构。

### 阶段 2 前置任务

- 扩展 `model.Session`，加入标题、艺人、专辑、进度、状态等会话页需要的数据。
- 明确 `SelectedSessionID` 的持久化和失效展示行为。
- 为 active SMTC 切换增加用户可见日志。

### 阶段 3 前置任务

- 设计 Dashboard view model。
- 将开发者字段从用户仪表盘移除。
- 增加当前逐行歌词 runtime state，占位支持无歌词状态。

### 阶段 4 前置任务

- 扩展 `model.Config` 或拆分配置模型。
- 增加设置项默认值。
- 增加配置保存和迁移测试。

### 阶段 5 前置任务

- 选择 SQLite driver。
- 建立 `internal/storage`。
- 定义歌曲唯一 key。
- 建立歌曲和歌词基础表。

### 阶段 6 到 8 前置任务

- 定义统一歌词结构。
- 设计歌词搜索任务状态机。
- 为 active track 引入 revision。
- 明确旧任务取消和 stale result 丢弃规则。

### 阶段 9 前置任务

- 定义 AI provider 配置。
- 定义 AI 请求超时。
- 定义 AI 结果版本和回退策略。

### 阶段 10 前置任务

- 在 `internal/amll` 中新增 `setLyric`。
- 手动 snapshot 包含当前歌词。
- 明确无歌词、歌词搜索中、歌词失败时的 AMLL 行为。

## 风险与约束

- 当前 `internal/ui` 是单文件单页结构，Full UI 会明显膨胀，阶段 1 必须拆页。
- 当前 `internal/model` 已开始聚合过多共享类型，后续领域模型应放到领域包。
- 当前配置目录仍是 `LyricSyncBasic`，必须做兼容迁移，不能直接丢用户已有配置。
- 歌词搜索和 AI 都是长任务，必须从设计阶段就绑定 context 和 revision。
- TTML DB 索引可能较大，不应直接放入运行时 store。
- `/ref` 参考仓库不能作为业务依赖直接引入。

## 验收结果

阶段 0 验收标准：

- 能清楚说明哪些数据在内存状态里，哪些数据写入磁盘。
- 后续阶段不需要再大幅调整核心目录结构。

验收结论：通过。

理由：

- 当前模块职责已梳理。
- Full 版本目标模块已确定。
- 配置、数据库、缓存、封面和日志的本地存储边界已确定。
- 运行时状态与持久化状态边界已确定。
- 基础开发任务列表已形成。
- 当前测试基线 `go test ./...` 通过。
