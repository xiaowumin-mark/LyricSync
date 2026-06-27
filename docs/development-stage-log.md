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
| 7 | 歌词来源接入与并行搜索 | 未开始 | - | - |
| 8 | 歌词任务调度与防堆积 | 未开始 | - | - |
| 9 | AI 处理能力 | 未开始 | - | - |
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
- 参考 `/ref/amll-ttml` 的结构能力，实现项目内独立 TTML 处理：
  - `ParseTTML`
  - `GenerateTTML`
  - `CompressTTML`
  - `ParseTimestamp` / `FormatTimestamp`
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

- `/ref/amll-ttml` 只作为结构和行为参考，不直接 import 参考仓库代码。
- 数据库仍保存原始歌词和统一后的 TTML 两份数据：原始歌词便于后续排查和重新清洗，TTML 作为统一处理链路与 AMLL 输出格式。
- UI 预览和仪表盘只消费统一结构导出的逐行歌词，不依赖 QQ、网易、酷狗或 TTML DB 的原始结构。
- 普通文本可预览但不判定为可同步歌词，避免没有时间轴的歌词被误发给 AMLL。
- AMLL 当前阶段优先发送 TTML 格式歌词；结构化 `lines` 输出可在完整同步阶段按需补齐。

### 遗留问题

- 当前 TTML 解析覆盖主流 `p/span`、翻译、音译和背景行场景，复杂 iTunes metadata 翻译/逐词音译映射后续接入 TTML DB 大样本时继续增强。
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
