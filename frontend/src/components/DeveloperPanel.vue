<script setup>
import { computed } from 'vue'
import { formatTime } from '../utils/format'

const props = defineProps({
  apiRequests: {
    type: Array,
    default: () => []
  },
  webSocketMessages: {
    type: Array,
    default: () => []
  },
  webSocketSeries: {
    type: Array,
    default: () => []
  },
  performance: {
    type: Object,
    default: () => ({})
  }
})

const recentAPI = computed(() => [...props.apiRequests].reverse().slice(0, 80))
const recentWS = computed(() => [...props.webSocketMessages].reverse().slice(0, 100))
const chartPoints = computed(() => props.webSocketSeries.slice(-80))
const maxChartMessages = computed(() => Math.max(1, ...chartPoints.value.map((point) => Number(point.messages || 0))))

function formatBytes(value) {
  const bytes = Number(value || 0)
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`
  return `${bytes} B`
}

function formatUptime(seconds) {
  const value = Number(seconds || 0)
  const hours = Math.floor(value / 3600)
  const minutes = Math.floor((value % 3600) / 60)
  const secs = Math.floor(value % 60)
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${secs}s`
  return `${secs}s`
}
</script>

<template>
  <div class="developer-view">
    <v-row dense>
      <v-col cols="12" md="3">
        <v-card class="metric-card" variant="flat">
          <span>运行时间</span>
          <strong>{{ formatUptime(performance.uptimeSeconds) }}</strong>
        </v-card>
      </v-col>
      <v-col cols="12" md="3">
        <v-card class="metric-card" variant="flat">
          <span>内存</span>
          <strong>{{ formatBytes(performance.memoryAllocBytes) }}</strong>
        </v-card>
      </v-col>
      <v-col cols="12" md="3">
        <v-card class="metric-card" variant="flat">
          <span>Goroutine</span>
          <strong>{{ performance.goroutines || 0 }}</strong>
        </v-card>
      </v-col>
      <v-col cols="12" md="3">
        <v-card class="metric-card" variant="flat">
          <span>网络发送</span>
          <strong>{{ formatBytes(performance.networkBytesSent) }}</strong>
        </v-card>
      </v-col>
    </v-row>

    <v-card class="panel-card" variant="flat">
      <div class="panel-heading">
        <span>性能计数</span>
        <strong>{{ formatTime(performance.updatedAt) }}</strong>
      </div>
      <v-table density="compact" class="data-table monitor-table">
        <tbody>
          <tr>
            <th>HTTP 请求</th>
            <td>{{ performance.httpRequests || 0 }}</td>
            <th>HTTP 错误</th>
            <td>{{ performance.httpErrors || 0 }}</td>
          </tr>
          <tr>
            <th>HTTP 响应</th>
            <td>{{ formatBytes(performance.httpBytesSent) }}</td>
            <th>WebSocket 响应</th>
            <td>{{ formatBytes(performance.webSocketBytesSent) }}</td>
          </tr>
          <tr>
            <th>WS 消息</th>
            <td>{{ performance.webSocketMessages || 0 }}</td>
            <th>WS 二进制</th>
            <td>{{ performance.webSocketBinaryMessages || 0 }}</td>
          </tr>
          <tr>
            <th>内存 Sys</th>
            <td>{{ formatBytes(performance.memorySysBytes) }}</td>
            <th>CPU</th>
            <td>{{ Number(performance.cpuPercent || 0).toFixed(1) }}%</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>

    <v-card class="panel-card" variant="flat">
      <div class="panel-heading">
        <span>WebSocket 消息图表</span>
        <strong>{{ chartPoints.length }} 点</strong>
      </div>
      <div class="ws-chart">
        <div
          v-for="(point, index) in chartPoints"
          :key="`${point.endpoint}-${point.time}-${index}`"
          class="ws-chart-bar"
          :title="`${point.endpoint} ${point.messages || 0} msg ${formatBytes(point.bytesSent)}`"
        >
          <span :style="{ height: `${Math.max(4, (Number(point.messages || 0) / maxChartMessages) * 100)}%` }" />
        </div>
      </div>
    </v-card>

    <v-card class="panel-card" variant="flat">
      <div class="panel-heading">
        <span>WebSocket 消息历史</span>
        <strong>{{ recentWS.length }}</strong>
      </div>
      <v-table density="compact" class="data-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>端点</th>
            <th>类型</th>
            <th>格式</th>
            <th>大小</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="message in recentWS" :key="message.id">
            <td>{{ formatTime(message.time) }}</td>
            <td>{{ message.endpoint }}</td>
            <td>{{ message.messageType }}</td>
            <td>
              <v-chip :color="message.binary ? 'accent' : 'info'" size="x-small" variant="tonal">
                {{ message.binary ? 'binary' : 'json' }}
              </v-chip>
            </td>
            <td>{{ formatBytes(message.bytesSent) }}</td>
          </tr>
          <tr v-if="!recentWS.length">
            <td colspan="5" class="empty-cell">暂无 WebSocket 消息历史</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>

    <v-card class="panel-card full-panel" variant="flat">
      <div class="panel-heading">
        <span>API 请求追踪</span>
        <strong>{{ recentAPI.length }}</strong>
      </div>
      <v-table density="compact" class="data-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>方法</th>
            <th>路径</th>
            <th>状态</th>
            <th>耗时</th>
            <th>响应</th>
            <th>来源</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="request in recentAPI" :key="request.id">
            <td>{{ formatTime(request.time) }}</td>
            <td>{{ request.method }}</td>
            <td>{{ request.path }}</td>
            <td>
              <v-chip
                :color="request.status >= 500 ? 'error' : request.status >= 400 ? 'warning' : 'success'"
                size="x-small"
                variant="tonal"
              >
                {{ request.status }}
              </v-chip>
            </td>
            <td>{{ request.durationMs }} ms</td>
            <td>{{ formatBytes(request.bytesSent) }}</td>
            <td>{{ request.remoteAddr || '-' }}</td>
          </tr>
          <tr v-if="!recentAPI.length">
            <td colspan="7" class="empty-cell">暂无 API 请求记录</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
  </div>
</template>
