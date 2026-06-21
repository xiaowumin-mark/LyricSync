<script setup>
import { computed } from 'vue'
import { formatMs } from '../utils/format'

const props = defineProps({
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
  suggestion: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['suggest', 'auto-align'])

const confidencePercent = computed(() => Math.round(Number(props.suggestion?.confidence || 0) * 100))
const offsetText = computed(() => {
  const offset = Number(props.suggestion?.offsetMs || 0)
  if (!offset) return '0 ms'
  return `${offset > 0 ? '+' : ''}${offset} ms`
})
</script>

<template>
  <section class="tool-panel calibration-panel">
    <div class="tool-heading">
      <span>音频辅助校准</span>
      <strong>当前 {{ formatMs(currentPositionMs) }}</strong>
    </div>

    <div class="calibration-summary">
      <div>
        <span>当前行</span>
        <strong>{{ currentLine?.text || '未命中播放行' }}</strong>
      </div>
      <div>
        <span>建议位置</span>
        <strong>{{ suggestion ? formatMs(suggestion.suggestedPositionMs) : '-' }}</strong>
      </div>
      <div>
        <span>偏移</span>
        <strong>{{ suggestion ? offsetText : '-' }}</strong>
      </div>
      <div>
        <span>置信度</span>
        <strong>{{ suggestion ? `${confidencePercent}%` : '-' }}</strong>
      </div>
    </div>

    <v-progress-linear
      :model-value="confidencePercent"
      color="accent"
      height="8"
      rounded
    />

    <div class="calibration-actions">
      <v-btn
        :disabled="saving"
        prepend-icon="mdi-waveform"
        variant="tonal"
        @click="emit('suggest')"
      >
        分析建议
      </v-btn>
      <v-btn
        color="primary"
        :disabled="saving"
        prepend-icon="mdi-timeline-check-outline"
        @click="emit('auto-align')"
      >
        自动校准
      </v-btn>
    </div>
  </section>
</template>
