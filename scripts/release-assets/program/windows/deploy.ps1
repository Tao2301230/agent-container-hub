$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
. (Join-Path $ScriptDir 'scripts/program-common.ps1')

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
Initialize-ProgramConfig

Write-Host "[program-deploy] bundle validated"
Write-Host "[program-deploy] backend binary: $Script:BackendBin"
Write-Host "[program-deploy] config initialized under $Script:ConfigDir"
