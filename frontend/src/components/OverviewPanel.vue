<script setup>
import { computed } from 'vue'
import SpectrumBars from './SpectrumBars.vue'
import { audioPercent, formatMs, percent } from '../utils/format'

const props = defineProps({
  currentTrack: {
    type: Object,
    default: () => ({})
  },
  playback: {
    type: Object,
    default: () => ({})
  },
  currentLine: {
    type: Object,
    default: null
  },
  audio: {
    type: Object,
    default: () => ({})
  },
  progress: {
    type: Number,
    default: 0
  },
  clientsCount: {
    type: Number,
    default: 0
  },
  apiBase: {
    type: String,
    default: ''
  },
  wsUrl: {
    type: String,
    default: ''
  },
  amllWsUrl: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['command', 'volume'])

const rmsPercent = computed(() => audioPercent(props.audio.rms))
const peakPercent = computed(() => audioPercent(props.audio.peak))
const playbackCommand = computed(() => (props.playback.state === 'playing' ? 'pause' : 'play'))
const playbackIcon = computed(() => (props.playback.state === 'playing' ? 'mdi-pause' : 'mdi-play'))
const volumePercent = computed({
  get: () => percent(props.playback.volume ?? 1),
  set: (value) => emit('volume', Number(value || 0) / 100)
})
</script>

<template>
  <v-row dense>
    <v-col cols="12" md="7">
      <v-card class="panel-card now-card" variant="flat">
        <div class="panel-heading">
          <span>当前播放</span>
          <strong>{{ formatMs(playback.positionMs) }} / {{ formatMs(currentTrack.durationMs) }}</strong>
        </div>

        <v-progress-linear :model-value="progress" color="primary" height="10" rounded />

        <div class="transport-controls">
          <v-btn prepend-icon="mdi-skip-previous" variant="tonal" @click="emit('command', 'previous')">
            上一曲
          </v-btn>
          <v-btn :prepend-icon="playbackIcon" color="primary" variant="flat" @click="emit('command', playbackCommand)">
            {{ playback.state === 'playing' ? '暂停' : '播放' }}
          </v-btn>
          <v-btn prepend-icon="mdi-skip-next" variant="tonal" @click="emit('command', 'next')">
            下一曲
          </v-btn>
          <v-btn
            prepend-icon="mdi-fast-forward-10"
            variant="tonal"
            @click="emit('command', 'seek', (playback.positionMs || 0) + 10000)"
          >
            +10s
          </v-btn>
        </div>

        <div class="volume-control">
          <v-icon icon="mdi-volume-high" size="18" />
          <v-slider
            v-model="volumePercent"
            color="primary"
            density="compact"
            hide-details
            max="100"
            min="0"
            step="1"
            thumb-label
          />
          <strong>{{ volumePercent }}%</strong>
        </div>

        <div class="lyric-focus">
          <p>{{ currentLine?.text || '等待歌词同步' }}</p>
          <span>{{ currentLine?.translation || 'Waiting for lyric sync' }}</span>
        </div>
      </v-card>
    </v-col>

    <v-col cols="12" md="5">
      <v-card class="panel-card audio-card" variant="flat">
        <div class="panel-heading">
          <span>音频同步</span>
          <strong>#{{ audio.sequence || 0 }}</strong>
        </div>

        <div class="meter-row">
          <label>RMS</label>
          <v-progress-linear :model-value="rmsPercent" color="accent" height="8" rounded />
          <strong>{{ rmsPercent }}%</strong>
        </div>
        <div class="meter-row">
          <label>Peak</label>
          <v-progress-linear :model-value="peakPercent" color="warning" height="8" rounded />
          <strong>{{ peakPercent }}%</strong>
        </div>

        <SpectrumBars :bins="audio.spectrum || []" />

        <div class="audio-meta">
          <span>{{ audio.sampleRate || 0 }} Hz</span>
          <span>{{ audio.channels || 0 }} ch</span>
          <span>{{ audio.format || '-' }}</span>
          <span>{{ audio.providerMode || '-' }}</span>
        </div>
      </v-card>
    </v-col>

    <v-col cols="12">
      <v-card class="panel-card endpoint-card" variant="flat">
        <div class="panel-heading">
          <span>开放接口</span>
          <strong>{{ clientsCount }} 连接</strong>
        </div>

        <v-table density="compact" class="endpoint-table">
          <tbody>
            <tr>
              <th>HTTP</th>
              <td>{{ apiBase }}</td>
            </tr>
            <tr>
              <th>WebSocket</th>
              <td>{{ wsUrl }}</td>
            </tr>
            <tr>
              <th>AMLL v2</th>
              <td>{{ amllWsUrl }}</td>
            </tr>
          </tbody>
        </v-table>
      </v-card>
    </v-col>
  </v-row>
</template>
