<script setup>
import AMLLConnectorPanel from './AMLLConnectorPanel.vue'
import StatusDot from './StatusDot.vue'
import { formatTime } from '../utils/format'

defineProps({
  clients: {
    type: Array,
    default: () => []
  },
  webSocketStats: {
    type: Array,
    default: () => []
  },
  amll: {
    type: Object,
    default: () => ({})
  },
  amllUrl: {
    type: String,
    default: ''
  },
  saving: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits([
  'update:amllUrl',
  'connect-amll',
  'disconnect-amll',
  'send-lyrics'
])
</script>

<template>
  <div class="clients-view">
    <v-card class="panel-card" variant="flat">
      <AMLLConnectorPanel
        :amll="amll"
        :amll-url="amllUrl"
        :saving="saving"
        @connect="emit('connect-amll')"
        @disconnect="emit('disconnect-amll')"
        @send-lyrics="emit('send-lyrics')"
        @update:amll-url="emit('update:amllUrl', $event)"
      />
    </v-card>

    <v-card class="panel-card" variant="flat">
      <div class="panel-heading">
        <span>本机 WebSocket 服务</span>
        <strong>{{ clients.length }} 个连接</strong>
      </div>

      <v-table density="compact" class="data-table monitor-table">
        <thead>
          <tr>
            <th>端点</th>
            <th>连接</th>
            <th>消息</th>
            <th>二进制</th>
            <th>流量</th>
            <th>频率</th>
            <th>最近消息</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="stats in webSocketStats" :key="stats.endpoint">
            <td><strong>{{ stats.endpoint }}</strong></td>
            <td>{{ stats.activeClients }}</td>
            <td>{{ stats.messagesSent }}</td>
            <td>{{ stats.binaryMessagesSent }}</td>
            <td>{{ Math.round((stats.bytesSent || 0) / 1024) }} KB</td>
            <td>{{ Math.round(stats.messagesPerMinute || 0) }}/min</td>
            <td>{{ stats.lastMessageType || '-' }}</td>
          </tr>
          <tr v-if="!webSocketStats.length">
            <td colspan="7" class="empty-cell">暂无 WebSocket 统计</td>
          </tr>
        </tbody>
      </v-table>

      <v-table density="comfortable" class="data-table">
        <thead>
          <tr>
            <th>状态</th>
            <th>名称</th>
            <th>地址</th>
            <th>延迟</th>
            <th>最后活跃</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="client in clients" :key="client.id">
            <td><StatusDot color="success" /></td>
            <td><strong>{{ client.name || '-' }}</strong></td>
            <td>{{ client.remoteAddr || '-' }}</td>
            <td>{{ client.latencyMs || 0 }} ms</td>
            <td>{{ formatTime(client.lastSeenAt) }}</td>
          </tr>
          <tr v-if="!clients.length">
            <td colspan="5" class="empty-cell">暂无客户端连接</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
  </div>
</template>
