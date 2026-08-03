$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
. (Join-Path $ScriptDir 'scripts/program-common.ps1')

$DesktopConfigReset = $false
$DesktopConfigBackupDir = ''
$DesktopVersionFrom = ''
$DesktopVersionTo = ''
for ($i = 0; $i -lt $args.Count; $i++) {
  $arg = $args[$i]
  switch ($arg) {
    '--output-dir' {
      if ($i + 1 -ge $args.Count) { Fail-Program 'missing value for --output-dir' }
      $i++
      $Script:ConfigDir = $args[$i]
      Update-ProgramLayoutPaths
      continue
    }
    '--desktop-config-reset' {
      $DesktopConfigReset = $true
      continue
    }
    '--desktop-config-backup-dir' {
      if ($i + 1 -ge $args.Count) { Fail-Program 'missing value for --desktop-config-backup-dir' }
      $i++
      $DesktopConfigBackupDir = $args[$i]
      continue
    }
    '--desktop-version-from' {
      if ($i + 1 -ge $args.Count) { Fail-Program 'missing value for --desktop-version-from' }
      $i++
      $DesktopVersionFrom = $args[$i]
      continue
    }
    '--desktop-version-to' {
      if ($i + 1 -ge $args.Count) { Fail-Program 'missing value for --desktop-version-to' }
      $i++
      $DesktopVersionTo = $args[$i]
      continue
    }
    { $_ -in @('--config-dir', '--data-dir', '--state-dir', '--log-dir', '--bind-addr', '--daemon') } {
      Fail-Program "$arg is not a deploy argument"
    }
    default {
      Fail-Program "unsupported deploy argument: $arg"
    }
  }
}

Set-Location $ScriptDir
Test-ProgramBundle
$AuthToken = $null
if ($DesktopConfigReset) {
  Assert-DesktopConfigResetArgs $DesktopConfigBackupDir $DesktopVersionFrom $DesktopVersionTo
  Reset-DesktopProgramConfig $DesktopConfigBackupDir
  $AuthToken = Get-ProgramEnvLiteralValue (Join-Path $DesktopConfigBackupDir '.env') 'AUTH_TOKEN'
}
Initialize-ProgramConfig
if ($DesktopConfigReset -and -not [string]::IsNullOrWhiteSpace($AuthToken)) {
  Set-ProgramEnvValue $Script:EnvFile 'AUTH_TOKEN' $AuthToken
}
if ($DesktopConfigReset) {
  Protect-ProgramConfigTree $Script:ConfigDir
}

Write-Host "[program-deploy] bundle validated"
Write-Host "[program-deploy] backend binary: $Script:BackendBin"
Write-Host "[program-deploy] config initialized under $Script:ConfigDir"
if ($DesktopConfigReset) {
  Write-Host "[program-deploy] Desktop config rebuilt: $DesktopVersionFrom -> $DesktopVersionTo"
}
