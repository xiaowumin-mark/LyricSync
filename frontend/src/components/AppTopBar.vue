<script setup>
import { playbackColor } from '../utils/format'

defineProps({
  sourceApp: {
    type: String,
    default: ''
  },
  title: {
    type: String,
    default: ''
  },
  artist: {
    type: String,
    default: ''
  },
  album: {
    type: String,
    default: ''
  },
  statusText: {
    type: String,
    default: ''
  },
  playbackState: {
    type: String,
    default: ''
  }
})
</script>

<template>
  <header class="topbar">
    <div class="topbar-title">
      <p class="eyebrow">{{ sourceApp || 'No Session' }}</p>
      <h2>{{ title || '等待播放' }}</h2>
      <p class="subtle">{{ artist || '-' }} · {{ album || '-' }}</p>
    </div>

    <div class="topbar-status">
      <span>{{ statusText }}</span>
      <v-chip :color="playbackColor(playbackState)" size="small" variant="tonal">
        {{ playbackState || 'stopped' }}
      </v-chip>
    </div>
  </header>
</template>
