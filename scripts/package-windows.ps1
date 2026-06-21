param(
  [string]$Version = "0.1.0-dev",
  [switch]$SkipBuild,
  [switch]$KeepStage,
  [string]$CertificateThumbprint = "",
  [string]$TimestampServer = "http://timestamp.digicert.com",
  [string]$SignToolPath = "signtool.exe"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$releaseDir = Join-Path $root "dist\release"
$exePath = Join-Path $root "build\bin\LyricSync.exe"
$packageName = "LyricSync-$Version-windows-amd64"
$stageDir = Join-Path $releaseDir $packageName
$zipPath = Join-Path $releaseDir "$packageName.zip"
$shaPath = "$zipPath.sha256"
$manifestPath = Join-Path $stageDir "release-manifest.json"
$manifestOutPath = "$zipPath.manifest.json"

function Assert-ChildPath {
  param(
    [Parameter(Mandatory = $true)][string]$Parent,
    [Parameter(Mandatory = $true)][string]$Child
  )
  $parentFull = [System.IO.Path]::GetFullPath($Parent)
  $childFull = [System.IO.Path]::GetFullPath($Child)
  if (-not $parentFull.EndsWith([System.IO.Path]::DirectorySeparatorChar)) {
    $parentFull = $parentFull + [System.IO.Path]::DirectorySeparatorChar
  }
  if (-not $childFull.StartsWith($parentFull, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to operate outside ${parentFull}: $childFull"
  }
}

Set-Location $root

if (-not $SkipBuild) {
  npm ci --prefix frontend
  wails build
}

if (!(Test-Path $exePath)) {
  throw "Expected executable not found: $exePath"
}

Assert-ChildPath -Parent $releaseDir -Child $stageDir
if (Test-Path $stageDir) {
  Remove-Item -LiteralPath $stageDir -Recurse -Force
}
New-Item -ItemType Directory -Path $stageDir | Out-Null

$stageExe = Join-Path $stageDir "LyricSync.exe"
Copy-Item -LiteralPath $exePath -Destination $stageExe
Copy-Item -LiteralPath (Join-Path $root "README.md") -Destination $stageDir
Copy-Item -LiteralPath (Join-Path $root "docs\requirements.md") -Destination $stageDir
Copy-Item -LiteralPath (Join-Path $root "docs\sdk.md") -Destination $stageDir
Copy-Item -LiteralPath (Join-Path $root "docs\release.md") -Destination $stageDir
Copy-Item -LiteralPath (Join-Path $root "sdk") -Destination (Join-Path $stageDir "sdk") -Recurse

Get-ChildItem -LiteralPath $stageDir -Directory -Filter "__pycache__" -Recurse | ForEach-Object {
  Assert-ChildPath -Parent $stageDir -Child $_.FullName
  Remove-Item -LiteralPath $_.FullName -Recurse -Force
}
Get-ChildItem -LiteralPath $stageDir -File -Filter "*.pyc" -Recurse | ForEach-Object {
  Assert-ChildPath -Parent $stageDir -Child $_.FullName
  Remove-Item -LiteralPath $_.FullName -Force
}

if (-not [string]::IsNullOrWhiteSpace($CertificateThumbprint)) {
  & $SignToolPath sign /sha1 $CertificateThumbprint /fd SHA256 /tr $TimestampServer /td SHA256 $stageExe
  if ($LASTEXITCODE -ne 0) {
    throw "Signing failed with exit code $LASTEXITCODE"
  }
}

$gitCommit = ""
try {
  $gitCommit = (git rev-parse HEAD 2>$null).Trim()
} catch {
  $gitCommit = ""
}

$stageFull = [System.IO.Path]::GetFullPath($stageDir)
if (-not $stageFull.EndsWith([System.IO.Path]::DirectorySeparatorChar)) {
  $stageFull = $stageFull + [System.IO.Path]::DirectorySeparatorChar
}

$fileEntries = Get-ChildItem -LiteralPath $stageDir -File -Recurse |
  Where-Object { $_.FullName -ne $manifestPath } |
  ForEach-Object {
    $fullName = [System.IO.Path]::GetFullPath($_.FullName)
    $relative = $fullName.Substring($stageFull.Length).Replace("\", "/")
    $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName
    [pscustomobject]@{
      path = $relative
      size = $_.Length
      sha256 = $hash.Hash
    }
  }

[pscustomobject]@{
  name = "LyricSync"
  version = $Version
  target = "windows-amd64"
  builtAt = (Get-Date).ToUniversalTime().ToString("o")
  gitCommit = $gitCommit
  signed = -not [string]::IsNullOrWhiteSpace($CertificateThumbprint)
  files = $fileEntries
} | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 -LiteralPath $manifestPath
Copy-Item -LiteralPath $manifestPath -Destination $manifestOutPath

if (Test-Path $zipPath) {
  Remove-Item -LiteralPath $zipPath -Force
}
Compress-Archive -Path (Join-Path $stageDir "*") -DestinationPath $zipPath

$hash = Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath
"$($hash.Hash)  $(Split-Path -Leaf $zipPath)" | Set-Content -Encoding ASCII -LiteralPath $shaPath

if (-not $KeepStage) {
  Assert-ChildPath -Parent $releaseDir -Child $stageDir
  Remove-Item -LiteralPath $stageDir -Recurse -Force
}

Write-Host "Package: $zipPath"
Write-Host "SHA256:  $shaPath"
Write-Host "Manifest: $manifestOutPath"
