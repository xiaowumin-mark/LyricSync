<script setup>
import { computed } from 'vue'

const props = defineProps({
  config: {
    type: Object,
    required: true
  },
  apiBase: {
    type: String,
    default: ''
  },
  configPath: {
    type: String,
    default: ''
  },
  saving: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['save', 'restart'])

const lyricSourcesText = computed({
  get: () => arrayToText(props.config.lyrics.sources),
  set: (value) => {
    props.config.lyrics.sources = textToArray(value)
  }
})
const localScanPathsText = computed({
  get: () => arrayToText(props.config.lyrics.localScanPaths),
  set: (value) => {
    props.config.lyrics.localScanPaths = textToArray(value)
  }
})
const preferredText = computed({
  get: () => arrayToText(props.config.session.preferred),
  set: (value) => {
    props.config.session.preferred = textToArray(value)
  }
})
const blacklistText = computed({
  get: () => arrayToText(props.config.session.blacklist),
  set: (value) => {
    props.config.session.blacklist = textToArray(value)
  }
})
const whitelistText = computed({
  get: () => arrayToText(props.config.session.whitelist),
  set: (value) => {
    props.config.session.whitelist = textToArray(value)
  }
})

function arrayToText(values) {
  return (values || []).join('\n')
}

function textToArray(value) {
  return String(value || '')
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function updateNumber(path, value) {
  const next = Number(value || 0)
  const [group, key] = path.split('.')
  props.config[group][key] = next
}
</script>

<template>
  <div class="settings-view">
    <v-row dense>
      <v-col cols="12" lg="6">
        <v-card class="panel-card" variant="flat">
          <div class="panel-heading">
            <span>服务</span>
            <strong>{{ apiBase }}</strong>
          </div>
          <v-text-field v-model="config.server.host" density="compact" hide-details label="Host" variant="outlined" />
          <v-text-field
            :model-value="config.server.port"
            density="compact"
            hide-details
            label="Port"
            max="65535"
            min="1"
            type="number"
            variant="outlined"
            @update:model-value="updateNumber('server.port', $event)"
          />
          <v-switch
            v-model="config.server.enabled"
            color="primary"
            density="compact"
            hide-details
            label="启用 HTTP / WebSocket"
          />
        </v-card>
      </v-col>

      <v-col cols="12" lg="6">
        <v-card class="panel-card" variant="flat">
          <div class="panel-heading">
            <span>音频</span>
            <strong>{{ config.audio.mode || '-' }}</strong>
          </div>
          <v-text-field v-model="config.audio.mode" density="compact" hide-details label="Mode" variant="outlined" />
          <v-text-field
            :model-value="config.audio.sampleRate"
            density="compact"
            hide-details
            label="Sample Rate"
            min="8000"
            type="number"
            variant="outlined"
            @update:model-value="updateNumber('audio.sampleRate', $event)"
          />
          <v-text-field
            :model-value="config.audio.frameDurationMs"
            density="compact"
            hide-details
            label="Frame ms"
            min="10"
            type="number"
            variant="outlined"
            @update:model-value="updateNumber('audio.frameDurationMs', $event)"
          />
          <div class="switch-grid">
            <v-switch v-model="config.audio.enabled" color="primary" hide-details label="启用音频同步" />
            <v-switch v-model="config.audio.includeFeatures" color="primary" hide-details label="同步频谱特征" />
            <v-switch v-model="config.audio.includePcm" color="primary" hide-details label="JSON 暴露 PCM" />
          </div>
        </v-card>
      </v-col>

      <v-col cols="12" lg="6">
        <v-card class="panel-card" variant="flat">
          <div class="panel-heading">
            <span>窗口</span>
            <strong>{{ config.ui.hideOnClose ? '后台' : '退出' }}</strong>
          </div>
          <div class="switch-grid">
            <v-switch v-model="config.ui.hideOnClose" color="primary" hide-details label="关闭窗口时隐藏到后台" />
            <v-switch v-model="config.ui.minimizeToTray" color="primary" hide-details label="最小化时隐藏到后台" />
            <v-switch v-model="config.ui.startHidden" color="primary" hide-details label="启动后隐藏窗口" />
          </div>
          <code class="config-path">{{ configPath }}</code>
        </v-card>
      </v-col>

      <v-col cols="12" lg="6">
        <v-card class="panel-card" variant="flat">
          <div class="panel-heading">
            <span>AI</span>
            <strong>{{ config.ai.enabled ? '启用' : '关闭' }}</strong>
          </div>
          <v-text-field v-model="config.ai.baseUrl" density="compact" hide-details label="Base URL" variant="outlined" />
          <v-text-field v-model="config.ai.model" density="compact" hide-details label="Model" variant="outlined" />
          <v-text-field
            v-model="config.ai.apiKey"
            density="compact"
            hide-details
            label="API Key"
            type="password"
            variant="outlined"
          />
          <v-text-field
            :model-value="config.ai.timeoutSeconds"
            density="compact"
            hide-details
            label="Timeout seconds"
            min="1"
            type="number"
            variant="outlined"
            @update:model-value="updateNumber('ai.timeoutSeconds', $event)"
          />
          <v-switch v-model="config.ai.enabled" color="primary" hide-details label="启用 AI 功能" />
        </v-card>
      </v-col>

      <v-col cols="12" lg="6">
        <v-card class="panel-card" variant="flat">
          <div class="panel-heading">
            <span>歌词</span>
            <strong>{{ config.lyrics.autoSearch ? '自动' : '手动' }}</strong>
          </div>
          <div class="switch-grid">
            <v-switch v-model="config.lyrics.autoSearch" color="primary" hide-details label="歌曲变化时自动搜索" />
            <v-switch v-model="config.lyrics.cacheEnabled" color="primary" hide-details label="启用歌词缓存" />
          </div>
          <v-text-field
            v-model="config.lyrics.cacheDirectory"
            density="compact"
            hide-details
            label="缓存目录"
            variant="outlined"
          />
          <v-textarea v-model="lyricSourcesText" auto-grow hide-details label="来源顺序" rows="4" variant="outlined" />
          <v-textarea v-model="localScanPathsText" auto-grow hide-details label="本地扫描目录" rows="4" variant="outlined" />
        </v-card>
      </v-col>

      <v-col cols="12" lg="6">
        <v-card class="panel-card" variant="flat">
          <div class="panel-heading">
            <span>会话偏好</span>
            <strong>{{ config.session.autoSelect ? '自动选择' : '手动选择' }}</strong>
          </div>
          <v-switch v-model="config.session.autoSelect" color="primary" hide-details label="自动选择活动会话" />
          <v-textarea v-model="preferredText" auto-grow hide-details label="优先级" rows="3" variant="outlined" />
          <v-textarea v-model="whitelistText" auto-grow hide-details label="白名单" rows="3" variant="outlined" />
          <v-textarea v-model="blacklistText" auto-grow hide-details label="黑名单" rows="3" variant="outlined" />
        </v-card>
      </v-col>
    </v-row>

    <div class="settings-actions">
      <v-btn color="primary" :loading="saving" prepend-icon="mdi-content-save" @click="emit('save')">
        保存配置
      </v-btn>
      <v-btn :disabled="saving" prepend-icon="mdi-restart" variant="tonal" @click="emit('restart')">
        重启服务
      </v-btn>
    </div>
  </div>
</template>
