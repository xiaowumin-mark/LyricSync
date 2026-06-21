# LyricSync 需求整理

本文基于 [intor.md](../intor.md) 整理，用于后续拆分 Go + Wails2 桌面端实现任务。

## 1. 项目定位

LyricSync 是一个 Windows 桌面歌词同步与分发工具，核心目标是监听本机正在播放的音乐，自动获取并处理歌词，再通过 AMLL 风格 WebSocket 协议分发给客户端。

首选技术栈：

- 后端：Go
- GUI：Wails2
- 前端：Vue + JavaScript + Vuetify
- 目标平台：优先 Windows
- 桌面行为：支持窗口最小化、关闭到后台，以及任务栏/托盘入口
- 歌词统一数据结构：以 `amll-ttml` 的数据类型作为项目内部标准

## 2. 核心业务流

1. 监听 Windows SMTC 会话，获取歌曲、歌手、专辑、播放状态、进度、封面和音频数据。
2. 根据当前歌曲信息自动匹配歌词来源。
3. 将不同来源的歌词转换为项目统一的 TTML 结构。
4. 对歌词进行清洗、缓存、时间轴处理。
5. 将当前歌曲、歌词、翻译、播放状态、进度和音频数据同步到 GUI、HTTP API、WebSocket 客户端。
6. 开发者工具记录搜索、SMTC、AI、WebSocket 和 API 调试信息。

## 3. 功能模块

### 3.1 桌面外壳与 GUI

必须具备：

- Wails2 桌面窗口
- Vue + JavaScript + Vuetify 前端
- 最小化到任务栏；可选隐藏到后台
- 后台运行开关
- 主窗口显示当前歌曲、播放状态、歌词状态、连接状态
- 设置页管理端口、歌词来源、缓存路径、AI 配置、会话优先级
- 当前进度：前端已迁移到 Vuetify，并按导航、总览、会话、歌词、客户端、设置、日志等视图拆分组件；歌词页新增独立音频辅助校准组件；Wails 状态订阅、配置读写、播放控制、歌词导入/搜索、时间轴校准和 AI 动作已收拢到 `frontend/src/composables/useLyricSyncApp.js`，导航配置收拢到 `frontend/src/constants/navigation.js`。窗口默认最小化到 Windows 任务栏，可配置为隐藏到后台；隐藏后可通过 `/api/window/show` 恢复。

需要确认：

- “最小化到任务栏”是保留任务栏窗口按钮，还是隐藏到系统托盘图标。
- 关闭窗口时是直接退出，还是默认隐藏到后台。

### 3.2 音乐会话管理

依赖参考：

- `smtc-suite-go`

功能：

- 自动监听 Windows SMTC
- 获取歌曲信息、播放状态、时间轴、封面、应用来源
- 获取本机音频数据，用于后续同步给外部服务端计算动态背景
- 查看所有 SMTC 会话
- 自动选择活动会话
- 手动切换监听目标
- 会话优先级、黑名单、白名单
- 播放控制：播放、暂停、上一曲、下一曲、音量、进度跳转
- 当前进度：播放、暂停、上一曲、下一曲、进度跳转和 Windows 默认输出设备音量控制已接入；SMTC 会话已支持按优先级、白名单、黑名单和播放状态选择，前端会话页可手动选择目标会话，播放控制会优先作用到当前选中会话。

技术注意：

- 该能力 Windows 相关，且 `smtc-suite-go` 需要 cgo。
- 构建环境需要提前确认 C/C++ 编译链。
- 音频数据采集需要明确采样格式、帧大小、推送频率和带宽上限。

### 3.3 歌词获取

依赖参考：

- `AMLX-MUSIC-API`
- `amll-ttml-db`
- `amll-ttml-db` 镜像站：`https://amlldb.bikonoo.com/mirror.html`

功能：

- 自动搜索 QQ 音乐、网易云音乐、酷狗音乐、AMLL TTML DB
- 手动按歌曲名、歌手、专辑搜索
- 本地歌词目录扫描
- 本地歌词匹配
- 文件拖拽导入
- 歌词自动缓存、离线读取、自动更新
- 当前进度：手动搜索、本地 `.ttml/.lrc` 扫描、路径导入和窗口拖拽导入已接入。

优先级建议：

1. 当前歌曲自动搜索
2. 缓存命中与离线读取
3. 手动搜索与导入
4. 批量扫描和自动更新

### 3.4 歌词处理

依赖参考：

- `amll-ttml`
- `AMLX-MUSIC-API` 中的歌词清洗实现

功能：

- 多平台歌词结构解析
- 转换到项目统一 TTML 结构
- 删除广告、制作信息、无效空行、重复歌词
- 时间轴偏移调整
- 时间轴校准
- 自动同步
- 当前进度：毫秒级时间轴偏移已接入，会重新生成 TTML 与 AMLX；歌词页支持把任意歌词行锚定到当前播放位置，从而自动计算整体偏移并重建 TTML/AMLX；状态层保留最近音频 RMS/Peak 能量历史，歌词页可基于当前播放位置附近的能量峰值给出校准建议并一键应用。

架构要求：

- 内部模块不得直接依赖各平台的原始歌词结构。
- 平台适配层负责把外部结果转换为统一模型。
- WebSocket、HTTP API、GUI 只读取统一歌词模型。

### 3.5 AI 能力

接口要求：

- 使用 OpenAI 兼容接口格式
- 支持自定义服务商、Base URL、API Key、模型名

功能：

- AI 翻译：中文、英文、日文、韩文、多语言互译
- AI 润色
- AI 双语歌词生成
- AI 罗马音生成
- 当前进度：已接入 OpenAI 兼容 `/chat/completions` 调用，支持翻译、润色、双语歌词和罗马音，并写回统一歌词模型；AI 任务会串行排队执行，网络错误、429 和 5xx 会短退避重试，重复的同模型/同任务/同歌词输入会命中内存结果缓存。

技术注意：

- AI 调用需要可取消、可超时、可重试。
- 需要记录耗时、错误和 token/费用估算。
- 歌词内容属于用户数据，默认不应在日志中完整明文输出。

### 3.6 WebSocket Server

这是本项目主功能。

参考：

- `VoxBackend/amll-ws-client`
- `Unilyric`
- `amll-player/packages/ws-protocol`

功能：

- 提供 AMLL 风格 WebSocket 服务端
- 管理客户端连接
- `/ws`：LyricSync JSON 事件流，包含完整状态、日志和音频特征
- `/amll/ws`：AMLL v2 风格状态流，用于逐步对齐 AMLL 客户端
- 广播歌词、翻译歌词、歌曲信息、播放状态、播放进度、音频数据
- 管理在线列表、客户端名称、连接状态、延迟
- 向需要动态背景计算的外部服务端同步音频数据

协议实现建议：

- 先对齐 `amll-player/packages/ws-protocol` 的消息类型。
- 音频数据先支持动态背景计算所需的 RMS、Peak、Spectrum 特征，随后补齐原始 PCM/binary payload。
- 参考 `Unilyric` 的服务端和连接器结构。
- 将协议编码/解码做成独立包，避免和 GUI 或 SMTC 耦合。

### 3.7 API Service

HTTP API 示例：

- `GET /api/song`
- `GET /api/lyric`
- `GET /api/session`
- `GET /api/health`：返回 `ok/status/version/time/services`，便于外部服务进行运行状态探测。
- `GET/POST /api/window/show|hide|minimize`：用于本地自动化恢复、隐藏或最小化窗口。

WebSocket API 事件示例：

- `song_changed`
- `lyric_changed`
- `session_changed`
- `playback_changed`

后续 SDK：

- Go SDK
- JavaScript SDK
- Python SDK
- 当前进度：`sdk/` 下已提供 Go、JavaScript、Python 最小客户端，覆盖 HTTP API 与 WebSocket 地址/连接入口。

首版建议：

- API 先作为本机调试和自动化接口，不急于承诺稳定 SDK。

### 3.8 开发者工具

功能：

- WebSocket 监视器：连接、消息、发送频率
- API 调试：请求日志、响应日志
- 日志中心：SMTC、歌词搜索、AI、WebSocket 广播
- 性能监控：CPU、内存、网络流量、AI 耗时
- 当前进度：WebSocket 客户端列表、发送消息数、二进制帧数、字节数、每分钟频率、消息历史、基础图表、API 请求追踪、音频能量历史、运行时内存/网络计数和 Windows 进程 CPU 采样已接入。

首版建议：

- 先实现日志中心和 WebSocket 连接列表。
- 性能监控可以后置。

## 4. 本地参考库缓存

为减少反复访问 GitHub，后续可以把参考仓库浅克隆或稀疏检出到 `ref/`：

- `ref/smtc-suite-go`
- `ref/AMLX-MUSIC-API`
- `ref/amll-ttml`
- `ref/VoxBackend`
- `ref/Unilyric`
- `ref/amll-player`

建议规则：

- `ref/` 只作为阅读和接口确认用途，不直接作为业务代码目录。
- 需要确认许可证后再复用代码。
- 对体积大的仓库优先使用 shallow clone 或 sparse checkout。
- `amll-ttml-db` 可能很大，优先使用镜像站或只缓存索引/协议文档。

## 5. 推荐目录结构

```text
LyricSync/
  app/
    app.go
  frontend/
  internal/
    ai/
    api/
    config/
    lyric/
    smtc/
    store/
    tray/
    websocket/
  pkg/
    amllws/
    model/
  docs/
  ref/
```

说明：

- `internal/smtc`：封装 Windows SMTC 和会话选择。
- `internal/lyric`：歌词搜索、适配、清洗、缓存。
- `pkg/model`：项目内部统一数据模型，优先贴合 `amll-ttml`。
- `pkg/amllws`：AMLL WebSocket 协议编码/解码，可供 API 和测试复用。
- `internal/tray`：窗口最小化、托盘、后台运行行为。

## 6. 迭代建议

### M0：项目骨架

- 初始化 Wails2 + Go 工程
- 配置前后端开发命令
- 实现主窗口、设置页骨架
- 实现最小化/关闭行为
- 加入配置、日志、错误处理基础设施

### M1：SMTC 当前歌曲

- 接入 `smtc-suite-go`
- 显示当前播放信息
- 显示播放状态和进度
- 支持多会话列表和手动切换
- 当前进度：Windows+cgo 原生 SMTC 监听已接入，失败时回退模拟 provider。

### M2：歌词获取与缓存

- 接入至少一个在线歌词源
- 接入 `amll-ttml` 统一模型
- 实现缓存命中、失败降级、手动搜索
- 当前进度：已接入 `amll-ttml`、`AMLX-MUSIC-API`、本地 `.ttml/.lrc` 扫描、歌词缓存、自动搜索和手动搜索。

### M3：AMLL WebSocket 服务端

- 对齐协议消息格式
- 广播当前歌曲、歌词、播放状态、进度、音频数据
- 显示客户端连接列表和延迟
- 建立音频数据推送的基础格式和限流策略
- 当前进度：`/ws` JSON 事件流与 `/amll/ws` AMLL v2 风格状态流已接入；WASAPI loopback 音频特征已接入，AMLL binary PCM 已按 v2 `OnAudioData` 格式接入。

### M4：本地歌词与高级处理

- 本地目录扫描
- 文件导入
- 时间轴偏移与校准
- 歌词清洗规则可配置
- 当前进度：本地目录扫描、`.ttml/.lrc` 匹配、拖拽导入、路径导入、毫秒偏移、播放位置锚点校准和音频能量辅助自动校准已接入。

### M5：AI 能力

- OpenAI 兼容配置
- 翻译、双语、罗马音
- 任务队列、超时、日志、缓存
- 当前进度：OpenAI 兼容配置、翻译、润色、双语歌词和罗马音已接入；任务串行队列、瞬时失败重试和内存结果缓存已接入。

### M6：开发者工具和开放 API

- HTTP API
- 调试面板
- WebSocket 消息监视器
- 后续 SDK 设计
- 当前进度：HTTP API、日志面板、客户端列表、WebSocket 计数监视器、消息历史、基础图表、API 请求跟踪、音频能量历史、基础性能计数和 SDK 骨架已接入。

### M7：交付打包

- Windows 可执行文件
- 版本归档
- SHA256 校验
- GitHub Actions 构建
- 当前进度：`scripts/package-windows.ps1` 和 Windows Release workflow 已接入；签名和安装器元数据待补齐。

## 7. 主要风险与待确认问题

- 窗口隐藏、任务栏和系统托盘的最终交互细节需要产品侧确认。
- `smtc-suite-go` 依赖 cgo，Windows 构建链需要提前打通。
- AMLL WebSocket 协议需要以 `amll-player/packages/ws-protocol` 为准。
- `Unilyric` 参考程度最高，但代码复用前需要检查许可证和边界。
- 歌词来源可能存在网络不稳定、接口变动、版权和限流问题。
- AI 服务商配置需要兼容 OpenAI 格式，但不同供应商的流式响应和错误码可能不一致。
- 音频数据同步已确认需要；当前先同时提供 RMS/Peak/Spectrum 特征、最近能量历史和 AMLL binary PCM，并已复用 RMS/Peak 历史辅助歌词校准；后续按外部动态背景服务端的带宽与延迟要求决定是否增加压缩帧或降频策略。
