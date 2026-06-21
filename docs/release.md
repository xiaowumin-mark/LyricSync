# Release

Windows release packaging is handled by `scripts/package-windows.ps1`.

```powershell
.\scripts\package-windows.ps1 -Version 0.1.0-dev
```

The script:

1. Installs frontend dependencies with `npm ci --prefix frontend`.
2. Builds the Wails application with `wails build`.
3. Stages `LyricSync.exe`, README, requirements, SDK docs and the full `sdk/` directory.
4. Optionally signs the staged executable when `-CertificateThumbprint` is provided.
5. Writes `release-manifest.json` into the archive and next to the archive as `<zip>.manifest.json`.
6. Creates `dist/release/LyricSync-<version>-windows-amd64.zip`.
7. Writes a SHA256 checksum next to the archive.

Useful options:

```powershell
.\scripts\package-windows.ps1 -Version 0.1.0-dev -SkipBuild
.\scripts\package-windows.ps1 -Version 0.1.0-dev -SkipBuild -KeepStage
.\scripts\package-windows.ps1 -Version 0.1.0 -CertificateThumbprint "<cert-sha1>"
```

`-SkipBuild` reuses `build/bin/LyricSync.exe`. `-KeepStage` preserves the unzipped staging directory for inspection; by default it is removed after the ZIP and manifest are written so normal `go test ./...` runs do not scan staged SDK examples. Signing uses `signtool.exe` by default and timestamps with `http://timestamp.digicert.com`.

The same flow is available in GitHub Actions through `.github/workflows/windows-release.yml`.
