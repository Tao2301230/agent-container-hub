$ErrorActionPreference = 'Stop'

$Script:ProgramCommonDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Script:BundleRoot = Split-Path -Parent $Script:ProgramCommonDir
$Script:AppName = 'agent-container-hub'
$Script:ManifestFile = Join-Path $Script:BundleRoot 'manifest.json'
$Script:EnvExampleFile = Join-Path $Script:BundleRoot '.env.example'
$Script:HubExampleFile = Join-Path (Join-Path $Script:BundleRoot 'configs') 'hub.example.yml'
$Script:BackendBin = Join-Path (Join-Path $Script:BundleRoot 'backend') 'agent-container-hub.exe'
$Script:ConfigDir = $Script:BundleRoot
$Script:DataDir = Join-Path $Script:BundleRoot 'data'
$Script:RunDir = Join-Path $Script:BundleRoot 'run'
$Script:LogDir = $Script:RunDir
$Script:ProgramBindAddr = ''
$Script:ProgramConfigDirExplicit = $false
$Script:ProgramDataDirExplicit = $false
$Script:ProgramStateDirExplicit = $false
$Script:ProgramLogDirExplicit = $false
$Script:EnvFile = ''
$Script:HubConfigFile = ''
$Script:ConfigEnvDir = ''
$Script:RootfsDir = ''
$Script:BuildDir = ''
$Script:PidFile = ''
$Script:LogFile = ''
$Script:ErrorLogFile = ''

function Update-ProgramLayoutPaths {
  $Script:EnvFile = Join-Path $Script:ConfigDir '.env'
  $Script:HubConfigFile = Join-Path (Join-Path $Script:ConfigDir 'configs') 'hub.yml'
  $Script:ConfigEnvDir = Join-Path (Join-Path $Script:ConfigDir 'configs') 'environments'
  $Script:RootfsDir = Join-Path $Script:DataDir 'rootfs'
  $Script:BuildDir = Join-Path $Script:DataDir 'builds'
  $Script:PidFile = Join-Path $Script:RunDir 'agent-container-hub.pid'
  $Script:LogFile = Join-Path $Script:LogDir 'agent-container-hub.log'
  $Script:ErrorLogFile = Join-Path $Script:LogDir 'agent-container-hub.stderr.log'
}

function Set-ProgramLayoutArgs {
  param([string[]]$Arguments)

  for ($i = 0; $i -lt $Arguments.Count; $i++) {
    $arg = $Arguments[$i]
    switch ($arg) {
      '--config-dir' {
        if ($i + 1 -ge $Arguments.Count) { Fail-Program 'missing value for --config-dir' }
        $i++
        $Script:ConfigDir = $Arguments[$i]
        $Script:ProgramConfigDirExplicit = $true
        continue
      }
      '--data-dir' {
        if ($i + 1 -ge $Arguments.Count) { Fail-Program 'missing value for --data-dir' }
        $i++
        $Script:DataDir = $Arguments[$i]
        $Script:ProgramDataDirExplicit = $true
        continue
      }
      '--state-dir' {
        if ($i + 1 -ge $Arguments.Count) { Fail-Program 'missing value for --state-dir' }
        $previousDefaultLogDir = Join-Path $Script:BundleRoot 'run'
        $i++
        $Script:RunDir = $Arguments[$i]
        $Script:ProgramStateDirExplicit = $true
        if ($Script:LogDir -eq $previousDefaultLogDir) {
          $Script:LogDir = $Script:RunDir
        }
        continue
      }
      '--log-dir' {
        if ($i + 1 -ge $Arguments.Count) { Fail-Program 'missing value for --log-dir' }
        $i++
        $Script:LogDir = $Arguments[$i]
        $Script:ProgramLogDirExplicit = $true
        continue
      }
      '--bind-addr' {
        if ($i + 1 -ge $Arguments.Count) { Fail-Program 'missing value for --bind-addr' }
        $i++
        $Script:ProgramBindAddr = $Arguments[$i]
        continue
      }
      default {
        Fail-Program "unsupported argument: $arg"
      }
    }
  }
  Update-ProgramLayoutPaths
}

Update-ProgramLayoutPaths

function Fail-Program([string]$Message) {
  throw "[program] $Message"
}

function Test-ProgramBundle {
  if (-not (Test-Path -LiteralPath $Script:ManifestFile -PathType Leaf)) {
    Fail-Program "required file not found: $Script:ManifestFile"
  }
  if (-not (Test-Path -LiteralPath $Script:EnvExampleFile -PathType Leaf)) {
    Fail-Program "required file not found: $Script:EnvExampleFile"
  }
  if (-not (Test-Path -LiteralPath $Script:HubExampleFile -PathType Leaf)) {
    Fail-Program "required file not found: $Script:HubExampleFile"
  }
  if (-not (Test-Path -LiteralPath $Script:BackendBin -PathType Leaf)) {
    Fail-Program "required file not found: $Script:BackendBin"
  }
}

function Initialize-ProgramConfig {
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Script:EnvFile), (Split-Path -Parent $Script:HubConfigFile), $Script:ConfigEnvDir | Out-Null
  if (-not (Test-Path -LiteralPath $Script:EnvFile -PathType Leaf)) {
    Copy-Item -LiteralPath $Script:EnvExampleFile -Destination $Script:EnvFile
  }
  if (-not (Test-Path -LiteralPath $Script:HubConfigFile -PathType Leaf)) {
    Copy-Item -LiteralPath $Script:HubExampleFile -Destination $Script:HubConfigFile
  }
  $sourceEnvDir = Join-Path (Join-Path $Script:BundleRoot 'configs') 'environments'
  if (Test-Path -LiteralPath $sourceEnvDir -PathType Container) {
    Get-ChildItem -LiteralPath $sourceEnvDir -Force | ForEach-Object {
      $target = Join-Path $Script:ConfigEnvDir $_.Name
      if (-not (Test-Path -LiteralPath $target)) {
        Copy-Item -LiteralPath $_.FullName -Destination $target -Recurse
      }
    }
  }
}

function Assert-DesktopConfigResetArgs([string]$BackupDir, [string]$VersionFrom, [string]$VersionTo) {
  if (-not [System.IO.Path]::IsPathRooted($BackupDir)) {
    Fail-Program '--desktop-config-backup-dir must be absolute'
  }
  if ([string]::IsNullOrWhiteSpace($VersionFrom)) { Fail-Program 'missing value for --desktop-version-from' }
  if ([string]::IsNullOrWhiteSpace($VersionTo)) { Fail-Program 'missing value for --desktop-version-to' }
  $configPath = [System.IO.Path]::GetFullPath($Script:ConfigDir).TrimEnd('\', '/')
  $backupPath = [System.IO.Path]::GetFullPath($BackupDir).TrimEnd('\', '/')
  if ($backupPath -eq $configPath -or $backupPath.StartsWith($configPath + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    Fail-Program 'Desktop config backup directory must be outside the service config directory'
  }
}

function Protect-ProgramConfigTree([string]$Target) {
  if (-not (Test-Path -LiteralPath $Target)) { return }
  $identity = '*' + [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
  $items = @((Get-Item -LiteralPath $Target -Force)) + @(Get-ChildItem -LiteralPath $Target -Recurse -Force)
  foreach ($item in $items) {
    $permissions = if ($item.PSIsContainer) { '(OI)(CI)F' } else { 'F' }
    & icacls.exe $item.FullName '/inheritance:r' '/grant:r' ("{0}:{1}" -f $identity, $permissions) ("*S-1-5-18:{0}" -f $permissions) | Out-Null
    if ($LASTEXITCODE -ne 0) { Fail-Program "failed to restrict permissions for $($item.FullName)" }
  }
}

function Reset-DesktopProgramConfig([string]$BackupDir) {
  $backupParent = Split-Path -Parent $BackupDir
  $failedDir = $BackupDir + '.failed'
  New-Item -ItemType Directory -Force -Path $backupParent | Out-Null
  if (Test-Path -LiteralPath $BackupDir) {
    Remove-Item -LiteralPath $failedDir -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $Script:ConfigDir) {
      Move-Item -LiteralPath $Script:ConfigDir -Destination $failedDir
      Protect-ProgramConfigTree $failedDir
    }
  } elseif (Test-Path -LiteralPath $Script:ConfigDir) {
    Move-Item -LiteralPath $Script:ConfigDir -Destination $BackupDir
    Protect-ProgramConfigTree $BackupDir
  }
  New-Item -ItemType Directory -Force -Path $Script:ConfigDir | Out-Null
}

function Get-ProgramEnvLiteralValue([string]$Path, [string]$Name) {
  if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
  foreach ($line in [System.IO.File]::ReadAllLines($Path)) {
    $match = [regex]::Match($line, ("^\s*(?:export\s+)?{0}\s*=(.*)$" -f [regex]::Escape($Name)))
    if ($match.Success) { return $match.Groups[1].Value }
  }
  return $null
}

function Set-ProgramEnvValue([string]$Path, [string]$Name, [string]$Value) {
  $lines = [System.Collections.Generic.List[string]]::new()
  $found = $false
  foreach ($line in [System.IO.File]::ReadAllLines($Path)) {
    if ($line -match ("^\s*#?\s*{0}\s*=" -f [regex]::Escape($Name))) {
      $lines.Add(("{0}={1}" -f $Name, $Value))
      $found = $true
    } else {
      $lines.Add($line)
    }
  }
  if (-not $found) { $lines.Add(("{0}={1}" -f $Name, $Value)) }
  [System.IO.File]::WriteAllLines($Path, $lines.ToArray(), [System.Text.UTF8Encoding]::new($false))
}

function Import-ProgramEnv {
  if (-not (Test-Path -LiteralPath $Script:EnvFile -PathType Leaf)) {
    Fail-Program 'missing .env (copy from .env.example first)'
  }
  foreach ($rawLine in Get-Content -LiteralPath $Script:EnvFile) {
    $line = $rawLine.Trim()
    if ([string]::IsNullOrWhiteSpace($line) -or $line.StartsWith('#')) {
      continue
    }
    $index = $line.IndexOf('=')
    if ($index -lt 1) {
      continue
    }
    $name = $line.Substring(0, $index).Trim()
    $value = $line.Substring($index + 1)
    [Environment]::SetEnvironmentVariable($name, $value, 'Process')
  }
}

function Test-ProgramEngine {
  $engine = [Environment]::GetEnvironmentVariable('ENGINE', 'Process')
  if (-not [string]::IsNullOrWhiteSpace($engine)) {
    if ($engine -eq 'local') {
      Fail-Program ('ENGINE=' + 'local has been removed; use auto, docker, or podman')
    }
    if ($engine -ne 'auto') {
      if (-not (Get-Command $engine -ErrorAction SilentlyContinue)) {
        Fail-Program "ENGINE=$engine is not available in PATH"
      }
      $reachable = $false
      $savedEAP = $ErrorActionPreference
      try {
        $ErrorActionPreference = 'Continue'
        & $engine info *> $null
        $reachable = ($LASTEXITCODE -eq 0)
      } catch {
        $reachable = $false
      } finally {
        $ErrorActionPreference = $savedEAP
      }
      if (-not $reachable) {
        Fail-Program "ENGINE=$engine daemon is not reachable"
      }
      return
    }
  }
  foreach ($candidate in @('docker', 'podman')) {
    if (-not (Get-Command $candidate -ErrorAction SilentlyContinue)) {
      continue
    }
    $reachable = $false
    $savedEAP = $ErrorActionPreference
    try {
      $ErrorActionPreference = 'Continue'
      & $candidate info *> $null
      $reachable = ($LASTEXITCODE -eq 0)
    } catch {
      $reachable = $false
    } finally {
      $ErrorActionPreference = $savedEAP
    }
    if ($reachable) {
      return
    }
  }
  Fail-Program 'docker or podman is required in PATH and its daemon must be reachable'
}

function Initialize-ProgramRuntime {
  New-Item -ItemType Directory -Force -Path $Script:DataDir, $Script:RootfsDir, $Script:BuildDir, $Script:RunDir, $Script:LogDir | Out-Null
}

function Clear-StaleProgramPid {
  if (-not (Test-Path -LiteralPath $Script:PidFile -PathType Leaf)) {
    return
  }
  $pidValue = (Get-Content -LiteralPath $Script:PidFile -Raw).Trim()
  if (-not [string]::IsNullOrWhiteSpace($pidValue)) {
    try {
      $null = Get-Process -Id ([int]$pidValue) -ErrorAction Stop
      Fail-Program "$Script:AppName is already running with pid $pidValue"
    } catch [Microsoft.PowerShell.Commands.ProcessCommandException] {
      Remove-Item -LiteralPath $Script:PidFile -Force -ErrorAction SilentlyContinue
      return
    }
  }
  Remove-Item -LiteralPath $Script:PidFile -Force -ErrorAction SilentlyContinue
}

function Start-ProgramBackend {
  param(
    [switch]$Daemon
  )

  $backendArgs = @()
  if ($Script:ProgramConfigDirExplicit) {
    $backendArgs += @('--config-dir', $Script:ConfigDir)
  }
  if ($Script:ProgramDataDirExplicit) {
    $backendArgs += @('--data-dir', $Script:DataDir)
  }
  if ($Script:ProgramStateDirExplicit) {
    $backendArgs += @('--state-dir', $Script:RunDir)
  }
  if ($Script:ProgramLogDirExplicit) {
    $backendArgs += @('--log-dir', $Script:LogDir)
  }
  if ($Script:ProgramBindAddr) {
    $backendArgs += @('--bind-addr', $Script:ProgramBindAddr)
  } elseif ($env:BIND_ADDR) {
    $backendArgs += @('--bind-addr', $env:BIND_ADDR)
  }

  if ($Daemon) {
    Clear-StaleProgramPid
    if (Test-Path -LiteralPath $Script:LogFile) {
      Clear-Content -LiteralPath $Script:LogFile
    } else {
      New-Item -ItemType File -Path $Script:LogFile -Force | Out-Null
    }
    if (Test-Path -LiteralPath $Script:ErrorLogFile) {
      Clear-Content -LiteralPath $Script:ErrorLogFile
    } else {
      New-Item -ItemType File -Path $Script:ErrorLogFile -Force | Out-Null
    }
    $proc = Start-Process -FilePath $Script:BackendBin -ArgumentList $backendArgs -WorkingDirectory $Script:BundleRoot -WindowStyle Hidden -RedirectStandardOutput $Script:LogFile -RedirectStandardError $Script:ErrorLogFile -PassThru
    $proc.Id | Set-Content -LiteralPath $Script:PidFile
    Start-Sleep -Seconds 1
    if ($proc.HasExited) {
      Remove-Item -LiteralPath $Script:PidFile -Force -ErrorAction SilentlyContinue
      Fail-Program "backend failed to start; see $Script:LogFile and $Script:ErrorLogFile"
    }
    Write-Host "[program-start] started $Script:AppName in daemon mode (pid=$($proc.Id))"
    Write-Host "[program-start] log file: $Script:LogFile"
    Write-Host "[program-start] stderr file: $Script:ErrorLogFile"
    return
  }

  & $Script:BackendBin @backendArgs
}

function Stop-ProgramBackend {
  if (-not (Test-Path -LiteralPath $Script:PidFile -PathType Leaf)) {
    Write-Host "[program-stop] pid file not found: $Script:PidFile"
    return
  }

  $pidValue = (Get-Content -LiteralPath $Script:PidFile -Raw).Trim()
  if ([string]::IsNullOrWhiteSpace($pidValue)) {
    Fail-Program "pid file is empty: $Script:PidFile"
  }

  try {
    $proc = Get-Process -Id ([int]$pidValue) -ErrorAction Stop
  } catch [Microsoft.PowerShell.Commands.ProcessCommandException] {
    Remove-Item -LiteralPath $Script:PidFile -Force -ErrorAction SilentlyContinue
    Write-Host "[program-stop] process $pidValue is not running; removed stale pid file"
    return
  }

  Stop-Process -Id $proc.Id -ErrorAction Stop
  for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    if ($proc.HasExited) {
      Remove-Item -LiteralPath $Script:PidFile -Force -ErrorAction SilentlyContinue
      Write-Host "[program-stop] stopped $Script:AppName (pid=$($proc.Id))"
      return
    }
    $proc.Refresh()
  }

  Fail-Program "process $($proc.Id) did not stop within 30s"
}
