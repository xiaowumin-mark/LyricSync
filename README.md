# LyricSync

LyricSync is a Windows desktop lyrics synchronization and distribution tool built with Go, Wails2, Vue, JavaScript and Vuetify.

Current build status: usable development shell with GUI, local API, WebSocket broadcast, AMLL v2-style WebSocket output, native Windows SMTC monitoring, native WASAPI loopback audio capture with simulator fallback, online/local lyrics, audio-energy-assisted lyric calibration, and dynamic-background-ready audio sync.

## Requirements

- Go 1.25.1+
- Node.js 22+
- npm
- Wails CLI v2
- Windows WebView2 runtime

## Development

```powershell
npm install --prefix frontend
wails dev
```

## Build

```powershell
npm run build --prefix frontend
go test ./...
wails build
```

## Package

```powershell
.\scripts\package-windows.ps1 -Version 0.1.0-dev
```

Release archives and SHA256 files are written to `dist/release/`.
The release archive includes `LyricSync.exe`, documentation, SDK sources/examples, and `release-manifest.json`.

The Windows executable is written to:

```text
build/bin/LyricSync.exe
```

## Runtime Endpoints

Default host and port:

```text
127.0.0.1:41917
```

HTTP API:

```text
GET /api/health
GET /api/state
GET /api/song
GET /api/lyric
GET /api/session
GET /api/clients
GET /api/logs
GET /api/config
GET /api/metrics
GET/POST /api/window/show
GET/POST /api/window/hide
GET/POST /api/window/minimize
```

`/api/health` returns `ok`, `status`, `version`, `time` and current service summaries. `status` is `ok`, `degraded` or `error`.

WebSocket endpoints:

```text
/ws        LyricSync JSON event stream
/amll/ws   AMLL v2-style state stream
```

The `/ws` stream emits rich internal events such as `song_changed`, `playback_changed`, `lyric_changed`, `client_changed`, `log_added` and `audio_frame`.

The `/amll/ws` stream emits AMLL v2-style messages such as `initialize`, `state:setMusic`, `state:setLyric`, `state:progress`, `state:volume`, `state:resumed` and `state:audioData`. When native loopback PCM is available, it also emits AMLL v2 binary audio frames with magic `0`, little-endian payload size, and raw PCM bytes.

## Local Reference Repositories

Reference repositories are cached under `ref/` and ignored by git:

```text
ref/smtc-suite-go
ref/AMLX-MUSIC-API
ref/amll-ttml
ref/VoxBackend
ref/Unilyric
ref/amll-player
```

They are for implementation reference and license/interface review. Do not copy code from them into this project without checking license and boundaries.

## Current Implementation Notes

- The desktop shell uses Wails2, Vue + JavaScript, and Vuetify. The frontend is split into focused view components under `frontend/src/components`, shared navigation constants under `frontend/src/constants`, and Wails state/actions under `frontend/src/composables`.
- Go, JavaScript and Python SDK skeletons live under `sdk/`; see `docs/sdk.md`.
- Windows ZIP release packaging is available through `scripts/package-windows.ps1`; see `docs/release.md`.
- Closing the window hides it by default; explicit Quit exits the app. The Minimize action defaults to the Windows taskbar and can be configured to hide the window to the background. A hidden window can be restored through `GET /api/window/show`.
- HTTP and WebSocket services start automatically with the app.
- SMTC uses `smtc-suite-go/pkg/smtc/monitor` on Windows+cgo and falls back to a simulator when native initialization fails.
- SMTC session selection supports manual selection from the Sessions tab plus preferred, whitelist and blacklist configuration. Playback commands target the selected/current SMTC session.
- Audio capture uses `smtc-suite-go/pkg/audio/loopback` on Windows+cgo and falls back to a simulator when native initialization fails.
- Audio frames provide dynamic-background-ready features: RMS, peak and spectrum bins. Raw PCM is retained for AMLL binary frames; API JSON exposure can be enabled through configuration with `includePcm`.
- Recent RMS/peak energy history is retained in memory and exposed through `/api/metrics` as `audioEnergy` for diagnostics and lyric calibration.
- Lyric search supports local `.ttml`/`.lrc` files, cache lookup, and `AMLX-MUSIC-API` online providers.
- Manual lyric search, local `.ttml`/`.lrc` path import, drag/drop lyric import, millisecond timeline offset, playback-position anchor alignment, and audio-energy-assisted auto calibration are available from the Lyrics tab.
- AI lyric processing uses an OpenAI-compatible `/chat/completions` endpoint and supports translation, polishing, bilingual lyric generation, and romanization. AI tasks are serialized, transient provider failures are retried, and repeated identical tasks are served from an in-memory cache.
- The Clients tab includes a WebSocket monitor for active clients, sent messages, binary audio frames, byte counts and message rate.
- Playback controls support play, pause, previous, next and seek through `smtc-suite-go/pkg/smtc/control`.
- Windows system volume control uses the default render endpoint through `IAudioEndpointVolume`; non-Windows builds report that volume control is unsupported.
- The Monitor tab shows runtime counters and recent API request traces, including method, path, status, latency and response bytes.
- AMLL protocol output includes JSON v2-style state updates and binary PCM audio payloads for clients that need raw audio frames.

## Next Implementation Targets

1. Add richer installer metadata and signing options on top of the current ZIP release package.
2. Expand SDKs with packaged distribution metadata and examples.
3. Persist AI result cache across restarts if that becomes useful.
4. Tune audio calibration heuristics against real playback captures from the target dynamic-background workflow.
