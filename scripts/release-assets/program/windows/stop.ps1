[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
. (Join-Path $ScriptDir 'scripts/program-common.ps1')

$layoutArgs = @()
for ($i = 0; $i -lt $args.Count; $i++) {
  $arg = $args[$i]
  switch ($arg) {
    '--state-dir' {
      if ($i + 1 -ge $args.Count) { Fail-Program 'missing value for --state-dir' }
      $i++
      $layoutArgs += @('--state-dir', $args[$i])
      continue
    }
    { $_ -in @('--config-dir', '--data-dir', '--log-dir', '--bind-addr', '--daemon') } {
      Fail-Program "$arg is not a stop argument"
    }
    default {
      Fail-Program "unsupported stop argument: $arg"
    }
  }
}

Set-ProgramLayoutArgs $layoutArgs

Set-Location $ScriptDir
Test-ProgramBundle
Stop-ProgramBackend
