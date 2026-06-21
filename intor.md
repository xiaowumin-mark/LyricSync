# LyricSync 功能列表

## 1. Music Session Management（音乐会话管理）

> [smtc-suite-go](https://github.com/xiaowumin-mark/smtc-suite-go) 这个是一个go库，提供了smtc监听和获取本地电脑音频流的接口 需要cgo

### SMTC监听

* 自动监听 Windows SMTC
* 获取歌曲信息
* 获取播放状态
* 获取时间轴信息
* 获取专辑封面
* 获取应用来源

### 多会话管理

* 查看所有SMTC会话
* 一键切换监听目标
* 自动选择活动会话
* 会话优先级配置
* 会话黑名单
* 会话白名单

### 播放控制

* 播放
* 暂停
* 上一曲
* 下一曲
* 音量控制
* 进度跳转

---

# 2. Lyrics Acquisition（歌词获取）

> [AMLX-MUSIC-API](https://github.com/xiaowumin-mark/AMLX-MUSIC-API) 这个库提供了歌曲搜索，歌词搜索，等等的接口 支持 QQ音乐 酷狗音乐 网易云音乐

### 自动搜索

支持：

* QQ音乐
* 网易云音乐
* 酷狗音乐
* amll-ttml-db https://github.com/amll-dev/amll-ttml-db 这是一个由社区维护的歌词站，里面有高质量的ttml歌词，有qq音乐 酷狗音乐 网易云音乐的平台用户制作的歌词，也有apple music的 
因为github访问可能比较困难，他也有镜像站 https://amlldb.bikonoo.com/mirror.html 可以直接获取对应的歌词


### 手动搜索

支持：

* 歌曲名搜索
* 歌手搜索
* 专辑搜索

### 本地歌词

支持：

* 自动扫描目录
* 本地歌词匹配
* 文件拖拽导入

### 歌词缓存

* 自动缓存
* 离线读取
* 自动更新

---

# 3. Lyrics Processing（歌词处理）

> [amll-ttml](https://github.com/xiaowumin-mark/amll-ttml) 这个库实现了ttml的解析，和压缩，注意！！
> 本项目的所有歌词格式都使用本库的数据类型，上面提到的多平台搜索歌词功能，最后搜索的歌词类型也要转换成本数据类型

### 格式解析

支持：从多个平台的封装api库中解析每个库的结构，并且转换成本项目通用的结构


### 歌词清洗 在 https://github.com/xiaowumin-mark/AMLX-MUSIC-API 中有实现对应的功能

* 删除广告
* 删除制作信息
* 删除无效空行
* 删除重复歌词

### 时间轴处理

* 偏移调整
* 时间轴校准
* 自动同步

---

# 4. AI Features（AI能力） 此能力使用openai官方的接口格式，可自定义服务商和apikey

### AI翻译

支持：

* 中文
* 英文
* 日文
* 韩文
* 多语言互译

### AI润色

* 提高可读性
* 调整语气
* 优化歌词表达

### AI双语歌词

生成：

```text
原文
翻译
```

### AI罗马音

例如：

```text
ありがとう
↓
arigatou
```


---


# 5. WebSocket Server（歌词分发） 这也是本软件的主要功能

ws格式是AMLL的自定义格式

https://github.com/xiaowumin-mark/VoxBackend/tree/main/amll-ws-client 这个库中实现了amll风格的WebSocket客户端 ，需要连接服务端，来向服务端同步进度和歌词以及音频数据

https://github.com/apoint123/Unilyric 内有客户端和服务端的实现

https://github.com/amll-dev/amll-player/tree/main/packages/ws-protocol 这个是ws协议

### WebSocket服务

* 服务端客户端连接

### 实时广播

推送：

* 歌词
* 翻译歌词
* 歌曲信息
* 播放状态

### 客户端管理

* 在线列表
* 客户端名称
* 连接状态
* 延迟监控


---

# 6. API Service（开放接口）

### HTTP API

例如：

```http
GET /api/song
GET /api/lyric
GET /api/session
```

### WebSocket API

例如：

```text
song_changed
lyric_changed
session_changed
playback_changed
```

### SDK支持

未来：

* Go SDK
* JavaScript SDK
* Python SDK


# 7. Developer Tools（开发者工具）

### WebSocket监视器

显示：

* 当前连接
* 当前消息
* 发送频率

### API调试

* 请求日志
* 响应日志

### 日志中心

记录：

* SMTC事件
* 歌词搜索
* AI调用
* WebSocket广播

### 性能监控

显示：

* CPU
* 内存
* 网络流量
* AI耗时


其中unilyric的参考程度最高，因为里面的amll连接器功能已经和本项目很相似，但是我们的软件的功能更加强大