# LyricSync SDK

LyricSync exposes a local HTTP API and two WebSocket endpoints. The `sdk/` directory contains small clients for automation and integration work:

- `sdk/go/lyricsync`: Go client using the shared `pkg/model` types.
- `sdk/js`: JavaScript client for browser, Wails, Node 22+ or any runtime with `fetch`.
- `sdk/python`: Python 3.10+ HTTP client using the standard library.

Default endpoint:

```text
http://127.0.0.1:41917
```

Core methods:

```text
health, state, song, lyric, sessions, clients, logs, config, metrics
```

`metrics.audioEnergy` contains the recent RMS/peak history used by the GUI's lyric calibration tools and by external diagnostics.

WebSocket helpers:

```text
/ws       LyricSync event stream
/amll/ws  AMLL v2-style stream with optional binary audio frames
```

Examples:

```text
sdk/go/examples/health
sdk/js/examples/health.mjs
sdk/python/examples/health.py
```

Packaging metadata:

- JavaScript: `sdk/js/package.json`
- Python: `sdk/python/pyproject.toml`
- Go: use the repository module path and import `lyricsync/sdk/go/lyricsync`
