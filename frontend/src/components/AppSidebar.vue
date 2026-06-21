<script setup>
import StatusDot from './StatusDot.vue'
import { serviceColor } from '../utils/format'

defineProps({
  version: {
    type: String,
    default: '0.1.0-dev'
  },
  tabs: {
    type: Array,
    default: () => []
  },
  activeTab: {
    type: String,
    default: 'overview'
  },
  services: {
    type: Array,
    default: () => []
  },
  uiConfig: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['update:activeTab', 'minimize', 'hide', 'quit'])
</script>

<template>
  <div class="sidebar-content">
    <div class="brand">
      <div class="brand-mark">LS</div>
      <div>
        <h1>LyricSync</h1>
        <p>{{ version }}</p>
      </div>
    </div>

    <v-list class="nav-list" density="comfortable" nav>
      <v-list-item
        v-for="tab in tabs"
        :key="tab.id"
        :active="activeTab === tab.id"
        :prepend-icon="tab.icon"
        rounded="lg"
        @click="emit('update:activeTab', tab.id)"
      >
        <v-list-item-title>{{ tab.label }}</v-list-item-title>
      </v-list-item>
    </v-list>

    <div class="service-strip">
      <v-chip
        v-for="service in services"
        :key="service.name"
        class="service-chip"
        :color="serviceColor(service.status)"
        size="small"
        variant="tonal"
      >
        <template #prepend>
          <StatusDot :color="serviceColor(service.status)" />
        </template>
        {{ service.name }}
      </v-chip>
    </div>

    <div class="sidebar-spacer" />

    <div class="window-actions">
      <v-btn block prepend-icon="mdi-window-minimize" variant="tonal" @click="emit('minimize')">
        {{ uiConfig.minimizeToTray ? '隐藏到后台' : '最小化到任务栏' }}
      </v-btn>
      <v-btn block prepend-icon="mdi-eye-off-outline" variant="tonal" @click="emit('hide')">
        隐藏
      </v-btn>
      <v-btn block color="error" prepend-icon="mdi-power" variant="text" @click="emit('quit')">
        退出
      </v-btn>
    </div>
  </div>
</template>
