<script setup>
import StatusDot from './StatusDot.vue'
import { formatTime } from '../utils/format'

defineProps({
  sessions: {
    type: Array,
    default: () => []
  },
  saving: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['select'])
</script>

<template>
  <v-card class="panel-card full-panel" variant="flat">
    <div class="panel-heading">
      <span>SMTC 会话</span>
      <strong>{{ sessions.length }}</strong>
    </div>

    <v-table density="comfortable" class="data-table">
      <thead>
        <tr>
          <th>状态</th>
          <th>名称</th>
          <th>App ID</th>
          <th>更新时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="session in sessions" :key="session.id">
          <td><StatusDot :color="session.active ? 'success' : 'grey'" /></td>
          <td><strong>{{ session.name || '-' }}</strong></td>
          <td>{{ session.appId || '-' }}</td>
          <td>{{ formatTime(session.updatedAt) }}</td>
          <td>
            <v-btn
              :disabled="saving || session.active"
              density="comfortable"
              prepend-icon="mdi-target"
              size="small"
              variant="tonal"
              @click="emit('select', session.id)"
            >
              {{ session.active ? '当前' : '选择' }}
            </v-btn>
          </td>
        </tr>
        <tr v-if="!sessions.length">
          <td colspan="5" class="empty-cell">暂无可用会话</td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
