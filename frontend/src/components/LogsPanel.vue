<script setup>
import { formatTime } from '../utils/format'

defineProps({
  logs: {
    type: Array,
    default: () => []
  }
})

function levelColor(level) {
  if (level === 'error') return 'error'
  if (level === 'warn') return 'warning'
  return 'info'
}
</script>

<template>
  <v-card class="panel-card full-panel" variant="flat">
    <div class="panel-heading">
      <span>日志</span>
      <strong>{{ logs.length }}</strong>
    </div>

    <div class="log-list">
      <div v-for="entry in logs" :key="`${entry.time}-${entry.message}`" class="log-row">
        <time>{{ formatTime(entry.time) }}</time>
        <v-chip :color="levelColor(entry.level)" size="x-small" variant="tonal">
          {{ entry.level }}
        </v-chip>
        <strong>{{ entry.source }}</strong>
        <p>{{ entry.message }}</p>
      </div>
      <div v-if="!logs.length" class="empty-cell">暂无日志</div>
    </div>
  </v-card>
</template>
