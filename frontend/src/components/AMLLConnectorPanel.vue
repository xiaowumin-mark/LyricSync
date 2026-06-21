<script setup>
import { computed } from 'vue'

const props = defineProps({
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

const emit = defineEmits(['update:amllUrl', 'connect', 'disconnect', 'send-lyrics'])

const urlModel = computed({
  get: () => props.amllUrl,
  set: (value) => emit('update:amllUrl', value)
})

const statusColor = computed(() => {
  if (props.amll.status === 'connected') return 'success'
  if (props.amll.status === 'connecting') return 'warning'
  if (props.amll.status === 'error') return 'error'
  return 'grey'
})
</script>

<template>
  <section class="tool-panel amll-connector">
    <div class="tool-heading">
      <span>AMLL Player 连接</span>
      <strong>{{ amll.status || 'disconnected' }}</strong>
    </div>

    <div class="amll-connector-row">
      <v-text-field
        v-model="urlModel"
        density="compact"
        hide-details
        label="WebSocket URL"
        placeholder="ws://127.0.0.1:11444"
        prepend-inner-icon="mdi-connection"
        variant="outlined"
      />
      <v-chip :color="statusColor" label variant="tonal">
        {{ amll.message || amll.status || '未连接' }}
      </v-chip>
    </div>

    <div class="amll-metrics">
      <div>
        <span>发送消息</span>
        <strong>{{ amll.messagesSent || 0 }}</strong>
      </div>
      <div>
        <span>音频帧</span>
        <strong>{{ amll.binaryMessagesSent || 0 }}</strong>
      </div>
      <div>
        <span>流量</span>
        <strong>{{ Math.round((amll.bytesSent || 0) / 1024) }} KB</strong>
      </div>
    </div>

    <div class="amll-actions">
      <v-btn
        color="primary"
        :disabled="saving || amll.status === 'connected' || !urlModel"
        prepend-icon="mdi-lan-connect"
        @click="emit('connect')"
      >
        连接
      </v-btn>
      <v-btn
        :disabled="saving || amll.status !== 'connected'"
        prepend-icon="mdi-send"
        variant="tonal"
        @click="emit('send-lyrics')"
      >
        发送当前歌词
      </v-btn>
      <v-btn
        :disabled="saving || amll.status === 'disconnected'"
        prepend-icon="mdi-lan-disconnect"
        variant="tonal"
        @click="emit('disconnect')"
      >
        断开
      </v-btn>
    </div>
  </section>
</template>
