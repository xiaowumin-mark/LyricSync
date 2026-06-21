<script setup>
import { computed } from 'vue'
import LyricCalibrationPanel from './LyricCalibrationPanel.vue'
import { formatMs } from '../utils/format'

const props = defineProps({
  lyrics: {
    type: Object,
    default: () => ({ lines: [] })
  },
  lyricCandidates: {
    type: Array,
    default: () => []
  },
  currentTrack: {
    type: Object,
    default: () => ({})
  },
  currentLine: {
    type: Object,
    default: null
  },
  currentPositionMs: {
    type: Number,
    default: 0
  },
  saving: {
    type: Boolean,
    default: false
  },
  manualTitle: {
    type: String,
    default: ''
  },
  manualArtist: {
    type: String,
    default: ''
  },
  manualAlbum: {
    type: String,
    default: ''
  },
  importPath: {
    type: String,
    default: ''
  },
  lyricOffsetMs: {
    type: Number,
    default: 0
  },
  calibrationSuggestion: {
    type: Object,
    default: null
  },
  aiTask: {
    type: String,
    default: 'translate'
  },
  aiTargetLanguage: {
    type: String,
    default: ''
  },
  aiTone: {
    type: String,
    default: ''
  }
})

const emit = defineEmits([
  'update:manualTitle',
  'update:manualArtist',
  'update:manualAlbum',
  'update:importPath',
  'update:lyricOffsetMs',
  'update:aiTask',
  'update:aiTargetLanguage',
  'update:aiTone',
  'search',
  'apply-candidate',
  'import-path',
  'offset',
  'align',
  'suggest-calibration',
  'auto-align',
  'ai'
])

const titleModel = computed({
  get: () => props.manualTitle,
  set: (value) => emit('update:manualTitle', value)
})
const artistModel = computed({
  get: () => props.manualArtist,
  set: (value) => emit('update:manualArtist', value)
})
const albumModel = computed({
  get: () => props.manualAlbum,
  set: (value) => emit('update:manualAlbum', value)
})
const importPathModel = computed({
  get: () => props.importPath,
  set: (value) => emit('update:importPath', value)
})
const offsetModel = computed({
  get: () => props.lyricOffsetMs,
  set: (value) => emit('update:lyricOffsetMs', Number(value || 0))
})
const aiTaskModel = computed({
  get: () => props.aiTask,
  set: (value) => emit('update:aiTask', value)
})
const aiTargetLanguageModel = computed({
  get: () => props.aiTargetLanguage,
  set: (value) => emit('update:aiTargetLanguage', value)
})
const aiToneModel = computed({
  get: () => props.aiTone,
  set: (value) => emit('update:aiTone', value)
})
const lines = computed(() => props.lyrics.lines || [])
const aiTasks = [
  { title: 'AI 翻译', value: 'translate' },
  { title: 'AI 润色', value: 'polish' },
  { title: 'AI 双语歌词', value: 'bilingual' },
  { title: 'AI 罗马音', value: 'romanize' }
]

function isActive(line) {
  if (!props.currentLine) return false
  return props.currentLine.startMs === line.startMs && props.currentLine.text === line.text
}
</script>

<template>
  <v-card class="panel-card full-panel lyrics-panel" variant="flat">
    <div class="panel-heading">
      <span>{{ lyrics.source || '歌词' }} · {{ lyrics.format || 'ttml' }}</span>
      <strong>{{ lines.length }} 行 · {{ lyrics.ttml ? 'TTML' : 'Structured' }}</strong>
    </div>

    <div class="lyrics-controls">
      <v-row dense class="manual-search">
        <v-col cols="12" md="3">
          <v-text-field
            v-model="titleModel"
            density="compact"
            hide-details
            label="歌曲名"
            :placeholder="currentTrack.title || '歌曲名'"
            variant="outlined"
          />
        </v-col>
        <v-col cols="12" md="3">
          <v-text-field
            v-model="artistModel"
            density="compact"
            hide-details
            label="歌手"
            :placeholder="currentTrack.artist || '歌手'"
            variant="outlined"
          />
        </v-col>
        <v-col cols="12" md="3">
          <v-text-field
            v-model="albumModel"
            density="compact"
            hide-details
            label="专辑"
            :placeholder="currentTrack.album || '专辑'"
            variant="outlined"
          />
        </v-col>
        <v-col cols="12" md="3">
          <v-btn block color="primary" :disabled="saving" prepend-icon="mdi-magnify" @click="emit('search')">
            搜索候选
          </v-btn>
        </v-col>
      </v-row>

      <section v-if="lyricCandidates.length" class="tool-panel candidate-panel">
        <div class="tool-heading">
          <span>歌词候选</span>
          <strong>手动选择平台与结果</strong>
        </div>
        <v-table density="compact" class="data-table candidate-table">
          <thead>
            <tr>
              <th>来源</th>
              <th>歌曲</th>
              <th>歌手</th>
              <th>时长</th>
              <th>匹配</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="candidate in lyricCandidates" :key="candidate.id">
              <td><v-chip size="small" label variant="tonal">{{ candidate.source }}</v-chip></td>
              <td>{{ candidate.title || '-' }}</td>
              <td>{{ candidate.artist || '-' }}</td>
              <td>{{ candidate.durationMs ? formatMs(candidate.durationMs) : '-' }}</td>
              <td>{{ candidate.score }}</td>
              <td class="table-action-cell">
                <v-btn
                  :disabled="saving"
                  density="compact"
                  prepend-icon="mdi-check"
                  size="small"
                  variant="tonal"
                  @click="emit('apply-candidate', candidate)"
                >
                  使用
                </v-btn>
              </td>
            </tr>
          </tbody>
        </v-table>
      </section>

      <v-row dense class="lyric-tools">
        <v-col cols="12" lg="6">
          <section class="tool-panel">
            <div class="tool-heading">
              <span>本地导入</span>
              <strong>支持 .ttml / .lrc 拖拽</strong>
            </div>
            <v-text-field
              v-model="importPathModel"
              density="compact"
              hide-details
              label="歌词文件路径"
              placeholder="拖拽文件到窗口，或输入完整路径"
              prepend-inner-icon="mdi-file-music-outline"
              variant="outlined"
            />
            <v-btn
              block
              :disabled="saving || !importPathModel"
              prepend-icon="mdi-import"
              variant="tonal"
              @click="emit('import-path')"
            >
              导入歌词文件
            </v-btn>
          </section>
        </v-col>

        <v-col cols="12" lg="6">
          <section class="tool-panel">
            <div class="tool-heading">
              <span>时间轴</span>
              <strong>当前播放 {{ formatMs(currentPositionMs) }}</strong>
            </div>
            <v-text-field
              v-model="offsetModel"
              density="compact"
              hide-details
              label="偏移毫秒"
              prepend-inner-icon="mdi-timeline-clock-outline"
              type="number"
              variant="outlined"
            />
            <div class="offset-actions">
              <v-btn :disabled="saving" variant="tonal" @click="offsetModel = -500">-500ms</v-btn>
              <v-btn :disabled="saving" variant="tonal" @click="offsetModel = 500">+500ms</v-btn>
              <v-btn color="primary" :disabled="saving || !offsetModel" prepend-icon="mdi-sync" @click="emit('offset')">
                应用偏移
              </v-btn>
            </div>
          </section>
        </v-col>

        <v-col cols="12">
          <LyricCalibrationPanel
            :current-line="currentLine"
            :current-position-ms="currentPositionMs"
            :saving="saving"
            :suggestion="calibrationSuggestion"
            @auto-align="emit('auto-align')"
            @suggest="emit('suggest-calibration')"
          />
        </v-col>

        <v-col cols="12">
          <section class="tool-panel">
            <div class="tool-heading">
              <span>AI 歌词处理</span>
              <strong>OpenAI 兼容接口</strong>
            </div>
            <v-row dense>
              <v-col cols="12" md="3">
                <v-select
                  v-model="aiTaskModel"
                  density="compact"
                  hide-details
                  :items="aiTasks"
                  label="任务"
                  variant="outlined"
                />
              </v-col>
              <v-col cols="12" md="3">
                <v-text-field
                  v-model="aiTargetLanguageModel"
                  density="compact"
                  hide-details
                  label="目标语言"
                  placeholder="中文 / English / 日本語 / 한국어"
                  variant="outlined"
                />
              </v-col>
              <v-col cols="12" md="4">
                <v-text-field
                  v-model="aiToneModel"
                  density="compact"
                  hide-details
                  label="润色语气"
                  placeholder="自然，适合演唱"
                  variant="outlined"
                />
              </v-col>
              <v-col cols="12" md="2">
                <v-btn block color="primary" :disabled="saving || !lines.length" prepend-icon="mdi-creation" @click="emit('ai')">
                  执行 AI
                </v-btn>
              </v-col>
            </v-row>
          </section>
        </v-col>
      </v-row>

      <v-table density="compact" class="lyric-meta">
        <tbody>
          <tr>
            <th>Track</th>
            <td>{{ lyrics.trackId || '-' }}</td>
          </tr>
          <tr>
            <th>Provider ID</th>
            <td>{{ lyrics.providerTrackId || '-' }}</td>
          </tr>
          <tr>
            <th>AMLL Binary</th>
            <td>{{ lyrics.amlxBase64 ? `${Math.round(lyrics.amlxBase64.length / 1024)} KB` : '-' }}</td>
          </tr>
        </tbody>
      </v-table>
    </div>

    <div class="lyrics-list">
      <div
        v-for="(line, index) in lines"
        :key="`${line.startMs}-${line.text}`"
        :class="['lyric-row', { active: isActive(line) }]"
      >
        <time>{{ formatMs(line.startMs) }}</time>
        <div>
          <p>{{ line.text }}</p>
          <span>{{ line.translation }}</span>
          <em v-if="line.romanization">{{ line.romanization }}</em>
        </div>
        <v-btn
          class="lyric-align-btn"
          :disabled="saving"
          density="compact"
          prepend-icon="mdi-crosshairs-gps"
          size="small"
          variant="tonal"
          @click="emit('align', index)"
        >
          对齐
        </v-btn>
      </div>
      <div v-if="!lines.length" class="empty-cell">暂无歌词</div>
    </div>
  </v-card>
</template>
