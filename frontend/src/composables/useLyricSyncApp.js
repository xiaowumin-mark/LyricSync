import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  AlignLyrics,
  ApplyLyricCandidate,
  AutoAlignLyrics,
  ConfigPath,
  ConnectAMLL,
  ControlPlayback,
  DisconnectAMLL,
  GetConfig,
  GetState,
  HideWindow,
  ImportLyrics,
  MinimizeWindow,
  OffsetLyrics,
  Quit,
  RestartServices,
  RunAITask,
  SaveConfig,
  SearchLyricCandidates,
  SelectSession,
  SendLyricsToAMLL,
  SetVolume,
  SuggestLyricAlignment
} from '../../wailsjs/go/main/App'
import { EventsOn, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'

export function useLyricSyncApp() {
  const activeTab = ref('overview')
  const state = ref(null)
  const config = ref(null)
  const configPath = ref('')
  const saving = ref(false)
  const statusText = ref('启动中')
  const errorText = ref('')
  const manualTitle = ref('')
  const manualArtist = ref('')
  const manualAlbum = ref('')
  const importPath = ref('')
  const lyricOffsetMs = ref(0)
  const calibrationSuggestion = ref(null)
  const amllURL = ref('ws://127.0.0.1:11444')
  const aiTask = ref('translate')
  const aiTargetLanguage = ref('中文')
  const aiTone = ref('自然，适合演唱')
  const unsubscribe = []
  let refreshTimer = null

  const snapshot = computed(() => state.value || emptyState())
  const currentTrack = computed(() => snapshot.value.track || {})
  const playback = computed(() => snapshot.value.playback || {})
  const lyrics = computed(() => snapshot.value.lyrics || { lines: [] })
  const lyricCandidates = computed(() => snapshot.value.lyricCandidates || [])
  const amll = computed(() => snapshot.value.amll || emptyState().amll)
  const services = computed(() => snapshot.value.services || [])
  const clients = computed(() => snapshot.value.clients || [])
  const logs = computed(() => [...(snapshot.value.logs || [])].reverse().slice(0, 120))
  const audio = computed(() => snapshot.value.audio || {})
  const webSocketStats = computed(() => snapshot.value.metrics?.webSockets || [])
  const webSocketMessages = computed(() => snapshot.value.metrics?.webSocketMessages || [])
  const webSocketSeries = computed(() => snapshot.value.metrics?.webSocketSeries || [])
  const apiRequests = computed(() => snapshot.value.metrics?.api || [])
  const performance = computed(() => snapshot.value.metrics?.performance || {})
  const progress = computed(() => {
    const duration = Number(currentTrack.value.durationMs || 0)
    const position = Number(playback.value.positionMs || 0)
    if (!duration) return 0
    return Math.min(100, Math.max(0, (position / duration) * 100))
  })
  const currentLine = computed(() => {
    const position = Number(playback.value.positionMs || 0)
    return (lyrics.value.lines || []).find((line) => position >= line.startMs && position < line.endMs)
  })
  const serverConfig = computed(() => snapshot.value.config?.server || config.value?.server || {})
  const apiBase = computed(() => `http://${serverConfig.value.host || '127.0.0.1'}:${serverConfig.value.port || 41917}`)
  const wsURL = computed(() => `ws://${serverConfig.value.host || '127.0.0.1'}:${serverConfig.value.port || 41917}/ws`)
  const amllWsURL = computed(() => `ws://${serverConfig.value.host || '127.0.0.1'}:${serverConfig.value.port || 41917}/amll/ws`)

  onMounted(async () => {
    await load()
    unsubscribe.push(EventsOn('lyricsync:state', (next) => {
      state.value = next
    }))
    unsubscribe.push(EventsOn('lyricsync:event', (event) => {
      if (event?.type === 'audio_frame' && state.value) {
        state.value.audio = event.payload
      }
    }))
    OnFileDrop((_x, _y, paths) => {
      const lyricPath = (paths || []).find((path) => /\.(ttml|lrc)$/i.test(path))
      if (lyricPath) {
        activeTab.value = 'lyrics'
        importPath.value = lyricPath
        importLyricsFromPath(lyricPath)
      }
    }, false)
    refreshTimer = window.setInterval(loadState, 2500)
  })

  onUnmounted(() => {
    unsubscribe.forEach((off) => off && off())
    OnFileDropOff()
    if (refreshTimer) window.clearInterval(refreshTimer)
  })

  async function load() {
    try {
      await Promise.all([loadState(), loadConfig(), loadConfigPath()])
      statusText.value = '运行中'
    } catch (error) {
      errorText.value = String(error)
      statusText.value = '异常'
    }
  }

  async function loadState() {
    state.value = await GetState()
  }

  async function loadConfig() {
    config.value = await GetConfig()
    if (config.value?.amll?.url) {
      amllURL.value = config.value.amll.url
    }
  }

  async function loadConfigPath() {
    configPath.value = await ConfigPath()
  }

  async function saveConfig() {
    saving.value = true
    errorText.value = ''
    try {
      await SaveConfig(JSON.parse(JSON.stringify(config.value)))
      await loadState()
      statusText.value = '配置已保存'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function restartServices() {
    saving.value = true
    errorText.value = ''
    try {
      await RestartServices()
      await loadState()
      statusText.value = '服务已重启'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function selectSession(sessionID) {
    saving.value = true
    errorText.value = ''
    try {
      await SelectSession(sessionID)
      await Promise.all([loadState(), loadConfig()])
      statusText.value = '会话已选择'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function searchLyricsManual() {
    saving.value = true
    errorText.value = ''
    try {
      const title = manualTitle.value || currentTrack.value.title || ''
      const artist = manualArtist.value || currentTrack.value.artist || ''
      const album = manualAlbum.value || currentTrack.value.album || ''
      const candidates = await SearchLyricCandidates(title, artist, album)
      if (state.value) {
        state.value.lyricCandidates = candidates
      }
      statusText.value = `找到 ${candidates.length || 0} 个歌词候选`
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function applyLyricCandidate(candidate) {
    saving.value = true
    errorText.value = ''
    try {
      const doc = await ApplyLyricCandidate(candidate)
      if (state.value) {
        state.value.lyrics = doc
      }
      statusText.value = `已使用 ${candidate.source || '候选'} 歌词`
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function importLyricsFromPath(path = importPath.value) {
    saving.value = true
    errorText.value = ''
    try {
      const doc = await ImportLyrics(path)
      if (state.value) {
        state.value.lyrics = doc
      }
      statusText.value = '歌词已导入'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function offsetLyrics() {
    saving.value = true
    errorText.value = ''
    try {
      const doc = await OffsetLyrics(Number(lyricOffsetMs.value || 0))
      if (state.value) {
        state.value.lyrics = doc
      }
      statusText.value = '歌词时间轴已调整'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function alignLyrics(lineIndex) {
    saving.value = true
    errorText.value = ''
    try {
      const positionMs = Number(playback.value.positionMs || 0)
      const doc = await AlignLyrics(Number(lineIndex), positionMs)
      if (state.value) {
        state.value.lyrics = doc
      }
      statusText.value = '歌词时间轴已校准'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function suggestLyricAlignment(lineIndex = activeLineIndex()) {
    saving.value = true
    errorText.value = ''
    try {
      const suggestion = await SuggestLyricAlignment(Number(lineIndex))
      calibrationSuggestion.value = suggestion
      statusText.value = `建议偏移 ${suggestion.offsetMs || 0} ms`
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function autoAlignLyrics(lineIndex = activeLineIndex()) {
    saving.value = true
    errorText.value = ''
    try {
      const result = await AutoAlignLyrics(Number(lineIndex))
      if (state.value) {
        state.value.lyrics = result.updatedLyrics
      }
      calibrationSuggestion.value = result.suggestion
      statusText.value = `已自动校准第 ${(result.suggestion?.lineIndex ?? 0) + 1} 行`
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function runAITask() {
    saving.value = true
    errorText.value = ''
    try {
      const result = await RunAITask({
        task: aiTask.value,
        targetLanguage: aiTargetLanguage.value,
        tone: aiTone.value
      })
      if (state.value) {
        state.value.lyrics = result.updatedLyrics
      }
      statusText.value = `AI 已处理 ${result.lineCount || 0} 行`
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function connectAMLL() {
    saving.value = true
    errorText.value = ''
    try {
      await ConnectAMLL(amllURL.value)
      await Promise.all([loadState(), loadConfig()])
      statusText.value = 'AMLL 连接中'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function disconnectAMLL() {
    saving.value = true
    errorText.value = ''
    try {
      await DisconnectAMLL()
      await Promise.all([loadState(), loadConfig()])
      statusText.value = 'AMLL 已断开'
    } catch (error) {
      errorText.value = String(error)
    } finally {
      saving.value = false
    }
  }

  async function sendLyricsToAMLL() {
    errorText.value = ''
    try {
      await SendLyricsToAMLL()
      statusText.value = '歌词已发送到 AMLL'
    } catch (error) {
      errorText.value = String(error)
    }
  }

  async function sendPlaybackCommand(command, positionMs = 0) {
    errorText.value = ''
    try {
      await ControlPlayback(command, positionMs)
      statusText.value = '控制命令已发送'
    } catch (error) {
      errorText.value = String(error)
    }
  }

  async function setVolume(level) {
    errorText.value = ''
    try {
      await SetVolume(Number(level || 0))
      if (state.value?.playback) {
        state.value.playback.volume = Number(level || 0)
      }
      statusText.value = '音量已调整'
    } catch (error) {
      errorText.value = String(error)
    }
  }

  async function hideWindow() {
    await runWindowAction(HideWindow, '窗口已隐藏')
  }

  async function minimizeWindow() {
    await runWindowAction(MinimizeWindow, '窗口已最小化')
  }

  async function quitApp() {
    await runWindowAction(Quit, '正在退出')
  }

  async function runWindowAction(action, successText) {
    errorText.value = ''
    try {
      await action()
      statusText.value = successText
    } catch (error) {
      errorText.value = String(error)
    }
  }

  function activeLineIndex() {
    const line = currentLine.value
    if (!line) return -1
    return (lyrics.value.lines || []).findIndex((item) => item.startMs === line.startMs && item.text === line.text)
  }

  return {
    activeTab,
    alignLyrics,
    amll,
    amllURL,
    amllWsURL,
    apiBase,
    apiRequests,
    applyLyricCandidate,
    audio,
    autoAlignLyrics,
    aiTargetLanguage,
    aiTask,
    aiTone,
    calibrationSuggestion,
    clients,
    config,
    configPath,
    connectAMLL,
    currentLine,
    currentTrack,
    disconnectAMLL,
    errorText,
    hideWindow,
    importLyricsFromPath,
    importPath,
    logs,
    lyricCandidates,
    lyricOffsetMs,
    lyrics,
    manualAlbum,
    manualArtist,
    manualTitle,
    minimizeWindow,
    offsetLyrics,
    performance,
    playback,
    progress,
    quitApp,
    restartServices,
    runAITask,
    saveConfig,
    saving,
    searchLyricsManual,
    selectSession,
    sendLyricsToAMLL,
    sendPlaybackCommand,
    services,
    setVolume,
    suggestLyricAlignment,
    snapshot,
    statusText,
    webSocketMessages,
    webSocketSeries,
    webSocketStats,
    wsURL
  }
}

function emptyState() {
  return {
    version: '0.1.0-dev',
    config: {
      server: { host: '127.0.0.1', port: 41917 },
      audio: {},
      amll: { url: 'ws://127.0.0.1:11444', sendAudio: true }
    },
    track: {},
    playback: {},
    lyrics: { lines: [] },
    lyricCandidates: [],
    amll: { enabled: false, url: 'ws://127.0.0.1:11444', status: 'disconnected' },
    sessions: [],
    clients: [],
    services: [],
    logs: [],
    audio: {},
    metrics: { webSockets: [], webSocketMessages: [], webSocketSeries: [], api: [], audioEnergy: [], performance: {} }
  }
}
