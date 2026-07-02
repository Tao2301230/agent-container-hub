$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
. (Join-Path $ScriptDir 'scripts/program-common.ps1')

$layoutArgs = @()
for ($i = 0; $i -lt $args.Count; $i++) {
  $arg = $args[$i]
  switch ($arg) {
    '--output-dir' {
      if ($i + 1 -ge $args.Count) { Fail-Program 'missing value for --output-dir' }
      $i++
      $layoutArgs += @('--config-dir', $args[$i])
      continue
    }
    { $_ -in @('--data-dir', '--state-dir', '--log-dir', '--bind-addr') } {
      if ($i + 1 -ge $args.Count) { Fail-Program "missing value for $arg" }
      $i++
      $layoutArgs += @($arg, $args[$i])
      continue
    }
    { $_ -in @('--config-dir', '--daemon') } {
      Fail-Program "$arg is not a deploy argument"
    }
    default {
      Fail-Program "unsupported deploy argument: $arg"
    }
  }
}

Set-ProgramLayoutArgs $layoutArgs

Set-Location $ScriptDir
Test-ProgramBundle
Initialize-ProgramConfig
Initialize-ProgramRuntime

Write-Host "[program-deploy] bundle validated"
Write-Host "[program-deploy] backend binary: $Script:BackendBin"
Write-Host "[program-deploy] runtime directories prepared under $Script:DataDir and $Script:RunDir"
