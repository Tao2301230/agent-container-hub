$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$Version = if ($env:VERSION) { $env:VERSION } else { (Get-Content (Join-Path $RepoRoot 'VERSION') -Raw).Trim() }
$env:VERSION = $Version
if (-not $env:ARCH) {
  $env:ARCH = 'amd64'
}
$env:PROGRAM_TARGETS = 'windows'
bash (Join-Path $RepoRoot 'scripts/release-program.sh')
