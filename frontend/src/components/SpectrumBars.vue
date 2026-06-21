<script setup>
import { computed } from 'vue'

const props = defineProps({
  bins: {
    type: Array,
    default: () => []
  }
})

const bars = computed(() => {
  const raw = props.bins.length ? props.bins.map((bin) => Math.max(0, Number(bin || 0))) : Array.from({ length: 32 }, () => 0)
  const max = Math.max(...raw, 0)
  if (max <= 0) return raw
  const reference = Math.max(max, 0.12)
  return raw.map((bin) => {
    if (bin < 0.002) return 0
    return Math.min(1, Math.pow(bin / reference, 0.38))
  })
})
</script>

<template>
  <div class="spectrum-bars" aria-label="音频频谱">
    <span
      v-for="(bin, index) in bars"
      :key="index"
      :style="{ height: `${Math.max(4, Number(bin || 0) * 112)}px`, opacity: 0.45 + Number(bin || 0) * 0.55 }"
    />
  </div>
</template>
