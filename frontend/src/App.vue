<script setup>
import AppSidebar from './components/AppSidebar.vue'
import AppTopBar from './components/AppTopBar.vue'
import ClientsPanel from './components/ClientsPanel.vue'
import DeveloperPanel from './components/DeveloperPanel.vue'
import LogsPanel from './components/LogsPanel.vue'
import LyricsPanel from './components/LyricsPanel.vue'
import OverviewPanel from './components/OverviewPanel.vue'
import SessionsPanel from './components/SessionsPanel.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import { navigationTabs } from './constants/navigation'
import { useLyricSyncApp } from './composables/useLyricSyncApp'

const tabs = navigationTabs
const {
  activeTab,
  alignLyrics,
  autoAlignLyrics,
  aiTargetLanguage,
  aiTask,
  aiTone,
  amll,
  amllURL,
  amllWsURL,
  apiBase,
  apiRequests,
  applyLyricCandidate,
  audio,
  calibrationSuggestion,
  clients,
  config,
  connectAMLL,
  configPath,
  currentLine,
  currentTrack,
  disconnectAMLL,
  errorText,
  hideWindow,
  importLyricsFromPath,
  importPath,
  logs,
  lyricCandidates,
  lyricOffsetMs,
  lyrics,
  manualAlbum,
  manualArtist,
  manualTitle,
  minimizeWindow,
  offsetLyrics,
  performance,
  playback,
  progress,
  quitApp,
  restartServices,
  runAITask,
  saveConfig,
  saving,
  searchLyricsManual,
  selectSession,
  sendLyricsToAMLL,
  sendPlaybackCommand,
  services,
  setVolume,
  suggestLyricAlignment,
  snapshot,
  statusText,
  webSocketMessages,
  webSocketSeries,
  webSocketStats,
  wsURL
} = useLyricSyncApp()
</script>

<template>
  <v-app>
    <v-layout class="app-layout">
      <v-navigation-drawer class="app-drawer" permanent width="268">
        <AppSidebar
          v-model:active-tab="activeTab"
          :services="services"
          :tabs="tabs"
          :ui-config="snapshot.config?.ui"
          :version="snapshot.version"
          @hide="hideWindow"
          @minimize="minimizeWindow"
          @quit="quitApp"
        />
      </v-navigation-drawer>

      <v-main class="app-main">
        <div class="workspace">
          <AppTopBar
            :album="currentTrack.album"
            :artist="currentTrack.artist"
            :playback-state="playback.state"
            :source-app="currentTrack.sourceApp"
            :status-text="statusText"
            :title="currentTrack.title"
          />

          <v-alert
            v-if="errorText"
            class="mb-4"
            density="compact"
            icon="mdi-alert-circle-outline"
            type="error"
            variant="tonal"
          >
            {{ errorText }}
          </v-alert>

          <OverviewPanel
            v-if="activeTab === 'overview'"
            :amll-ws-url="amllWsURL"
            :api-base="apiBase"
            :audio="audio"
            :clients-count="clients.length"
            :current-line="currentLine"
            :current-track="currentTrack"
            :playback="playback"
            :progress="progress"
            :ws-url="wsURL"
            @command="sendPlaybackCommand"
            @volume="setVolume"
          />

          <SessionsPanel
            v-else-if="activeTab === 'sessions'"
            :saving="saving"
            :sessions="snapshot.sessions"
            @select="selectSession"
          />

          <LyricsPanel
            v-else-if="activeTab === 'lyrics'"
            v-model:ai-target-language="aiTargetLanguage"
            v-model:ai-task="aiTask"
            v-model:ai-tone="aiTone"
            v-model:import-path="importPath"
            v-model:lyric-offset-ms="lyricOffsetMs"
            v-model:manual-album="manualAlbum"
            v-model:manual-artist="manualArtist"
            v-model:manual-title="manualTitle"
            :current-line="currentLine"
            :current-position-ms="playback.positionMs || 0"
            :current-track="currentTrack"
            :calibration-suggestion="calibrationSuggestion"
            :lyric-candidates="lyricCandidates"
            :lyrics="lyrics"
            :saving="saving"
            @ai="runAITask"
            @align="alignLyrics"
            @apply-candidate="applyLyricCandidate"
            @auto-align="autoAlignLyrics"
            @import-path="importLyricsFromPath"
            @offset="offsetLyrics"
            @search="searchLyricsManual"
            @suggest-calibration="suggestLyricAlignment"
          />

          <ClientsPanel
            v-else-if="activeTab === 'clients'"
            v-model:amll-url="amllURL"
            :amll="amll"
            :clients="clients"
            :saving="saving"
            :web-socket-stats="webSocketStats"
            @connect-amll="connectAMLL"
            @disconnect-amll="disconnectAMLL"
            @send-lyrics="sendLyricsToAMLL"
          />

          <DeveloperPanel
            v-else-if="activeTab === 'developer'"
            :api-requests="apiRequests"
            :performance="performance"
            :web-socket-messages="webSocketMessages"
            :web-socket-series="webSocketSeries"
          />

          <SettingsPanel
            v-else-if="activeTab === 'settings' && config"
            :api-base="apiBase"
            :config="config"
            :config-path="configPath"
            :saving="saving"
            @restart="restartServices"
            @save="saveConfig"
          />

          <LogsPanel v-else-if="activeTab === 'logs'" :logs="logs" />
        </div>
      </v-main>
    </v-layout>
  </v-app>
</template>
