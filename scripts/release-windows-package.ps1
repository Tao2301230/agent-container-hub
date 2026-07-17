#Requires -Version 5.1
param([string]$Version, [string]$Arch = "amd64")
$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$arguments = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $ScriptDir "release-program.ps1"))
if ($Version) { $arguments += @("-Version", $Version) }
$arguments += @("-Arch", $Arch)
& powershell @arguments
if ($LASTEXITCODE -ne 0) { throw "Windows Program Bundle release failed" }
