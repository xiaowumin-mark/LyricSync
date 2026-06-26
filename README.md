# LyricSync Basic

This branch keeps only the basic AMLL connector workflow.

## Scope

- Go desktop app using FluxUI React-style components.
- AMLL WebSocket client compatible with the Unilyric v2 connector shape.
- SMTC session monitoring and playback control through `smtc-suite-go`.
- WASAPI loopback audio capture with binary audio frames sent to the AMLL server.
- Local simulator fallback when native SMTC or audio capture is unavailable.

Removed from this branch:

- Wails/Vue frontend.
- Lyrics search and lyrics editing.
- HTTP API and SDK packages.
- AI features.
- Packaging scripts and release workflow.

## Run

```powershell
go test ./...
go run .
```

Default AMLL URL:

```text
ws://127.0.0.1:11444
```

Config is stored under the current user's config directory:

```text
LyricSyncBasic/config.json
```

## References

Local references are stored in `ref/` to reduce repeated GitHub requests. FluxUI is used through:

```go
replace github.com/xiaowumin-mark/FluxUI => ./ref/FluxUI
```
