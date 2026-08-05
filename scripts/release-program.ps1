#Requires -Version 5.1
param(
    [string]$Version,
    [string]$Arch,
    [string]$ProgramTargets = $env:PROGRAM_TARGETS,
    [string]$ProgramTargetMatrix = $env:PROGRAM_TARGET_MATRIX
)

$ErrorActionPreference = "Stop"
$AppName = "agent-container-hub"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$AssetsDir = Join-Path $ScriptDir "release-assets/program/windows"
$ProgramCommonTestPath = Join-Path $AssetsDir "program-common_test.ps1"
$ReleaseDir = Join-Path $RepoRoot "dist/release"
$TemplatePath = Join-Path $ScriptDir "release-assets/program/manifest.template.json"
$Utf8NoBom = New-Object Text.UTF8Encoding($false)

function Get-HostArch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { return "amd64" }
        "x86" { return "amd64" }
        default { throw "This release entry supports Windows AMD64 only" }
    }
}

function Get-Targets {
    $resolved = @()
    if ($ProgramTargetMatrix) {
        foreach ($entry in $ProgramTargetMatrix.Split(',')) {
            $parts = $entry.Trim().Split('/')
            if ($parts.Count -ne 2) { throw "PROGRAM_TARGET_MATRIX entries must be os/arch (got: $entry)" }
            $resolved += [PSCustomObject]@{ OS = $parts[0]; Arch = $parts[1] }
        }
    } elseif ($ProgramTargets) {
        foreach ($targetOS in $ProgramTargets.Split(',')) {
            $resolved += [PSCustomObject]@{ OS = $targetOS.Trim(); Arch = $Arch }
        }
    } else {
        $resolved += [PSCustomObject]@{ OS = "windows"; Arch = $Arch }
    }
    foreach ($pair in $resolved) {
        if ($pair.OS -ne "windows" -or $pair.Arch -ne "amd64") {
            throw "Native PowerShell Program Bundle supports windows/amd64 only (got: $($pair.OS)/$($pair.Arch))"
        }
    }
    return $resolved
}

function Write-Manifest {
    param([string]$Destination, [string]$BackendEntry, [string]$ArchiveName)
    Push-Location $RepoRoot
    try {
        & go run ./cmd/render-program-manifest --template $TemplatePath --output $Destination --version $Version --os windows --arch amd64 --backend $BackendEntry --asset $ArchiveName
        if ($LASTEXITCODE -ne 0) { throw "Manifest renderer failed" }
    } finally { Pop-Location }
}

function Test-BundleContract {
    param([string]$BundleRoot, [string]$Archive)
    $manifestPath = Join-Path $BundleRoot "manifest.json"
    $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($manifest.platform.os -ne "windows" -or $manifest.platform.arch -ne "amd64") {
        throw "Manifest platform does not match windows/amd64"
    }
    foreach ($relative in @($manifest.runtime.requiredPaths)) {
        $path = $BundleRoot
        foreach ($segment in ([string]$relative).Split('/')) { $path = Join-Path $path $segment }
        if (-not (Test-Path -LiteralPath $path)) { throw "Bundle required path is missing: $relative" }
    }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::OpenRead($Archive)
    try {
        foreach ($entry in $zip.Entries) {
            $entryPath = $entry.FullName.Replace("\", "/")
            if (-not $entryPath.StartsWith("$AppName/")) {
                throw "ZIP entry is outside the single $AppName top-level directory: $($entry.FullName)"
            }
        }
    } finally {
        $zip.Dispose()
    }
}

if (-not $Version) {
    $Version = if ($env:VERSION) { $env:VERSION } else { (Get-Content -LiteralPath (Join-Path $RepoRoot "VERSION") -Raw).Trim() }
}
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw "VERSION must match vX.Y.Z (got: $Version)" }
if (-not $Arch) { $Arch = if ($env:ARCH) { $env:ARCH } else { Get-HostArch } }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "go is required" }
foreach ($path in @(
    $TemplatePath,
    (Join-Path $RepoRoot ".env.example"),
    (Join-Path $RepoRoot "configs/hub.example.yml"),
    (Join-Path $RepoRoot "configs/environments"),
    (Join-Path $AssetsDir "deploy.ps1"),
    (Join-Path $AssetsDir "start.ps1"),
    (Join-Path $AssetsDir "stop.ps1"),
    (Join-Path $AssetsDir "program-common.ps1"),
    $ProgramCommonTestPath
)) {
    if (-not (Test-Path -LiteralPath $path)) { throw "Required release input is missing: $path" }
}

& $ProgramCommonTestPath

foreach ($pair in @(Get-Targets)) {
    $targetOS = $pair.OS
    $targetArch = $pair.Arch
    $archiveName = "$AppName-$Version-$targetOS-$targetArch.zip"
    $archive = Join-Path $ReleaseDir $archiveName
    $temporary = Join-Path ([IO.Path]::GetTempPath()) "$AppName-release.$([Guid]::NewGuid().ToString('N'))"
    $stageRoot = Join-Path $temporary "stage"
    $bundleRoot = Join-Path $stageRoot $AppName
    $backendDir = Join-Path $bundleRoot "backend"
    $scriptsDir = Join-Path $bundleRoot "scripts"
    $binary = Join-Path $backendDir "$AppName.exe"
    $oldCGO = $env:CGO_ENABLED
    $oldGOOS = $env:GOOS
    $oldGOARCH = $env:GOARCH
    try {
        New-Item -ItemType Directory -Path $backendDir -Force | Out-Null
        New-Item -ItemType Directory -Path $scriptsDir -Force | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $bundleRoot "configs") -Force | Out-Null
        New-Item -ItemType Directory -Path $ReleaseDir -Force | Out-Null
        $env:CGO_ENABLED = "0"
        $env:GOOS = $targetOS
        $env:GOARCH = $targetArch
        Push-Location $RepoRoot
        try {
            & go build -ldflags "-X main.buildVersion=$Version" -o $binary ./cmd/agent-container-hub
            if ($LASTEXITCODE -ne 0) { throw "go build failed for $targetOS/$targetArch" }
        } finally {
            Pop-Location
        }
        Copy-Item (Join-Path $RepoRoot ".env.example") (Join-Path $bundleRoot ".env.example")
        Copy-Item (Join-Path $RepoRoot "configs/hub.example.yml") (Join-Path $bundleRoot "configs/hub.example.yml")
        Copy-Item (Join-Path $RepoRoot "configs/environments") (Join-Path $bundleRoot "configs") -Recurse
        Get-ChildItem -LiteralPath (Join-Path $bundleRoot "configs/environments") -Filter ".DS_Store" -Recurse -Force -ErrorAction SilentlyContinue |
            Remove-Item -Force -ErrorAction SilentlyContinue
        Copy-Item (Join-Path $AssetsDir "deploy.ps1") $bundleRoot
        Copy-Item (Join-Path $AssetsDir "start.ps1") $bundleRoot
        Copy-Item (Join-Path $AssetsDir "stop.ps1") $bundleRoot
        Copy-Item (Join-Path $AssetsDir "program-common.ps1") $scriptsDir
        Write-Manifest -Destination (Join-Path $bundleRoot "manifest.json") -BackendEntry "backend/$AppName.exe" -ArchiveName $archiveName

        Add-Type -AssemblyName System.IO.Compression.FileSystem
        Remove-Item -LiteralPath $archive -Force -ErrorAction SilentlyContinue
        [IO.Compression.ZipFile]::CreateFromDirectory($stageRoot, $archive, [IO.Compression.CompressionLevel]::Optimal, $false)
        Test-BundleContract -BundleRoot $bundleRoot -Archive $archive
        $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
        [IO.File]::WriteAllText("$archive.sha256", "$hash  $archiveName`n", $Utf8NoBom)
        Write-Host "[release] done: $archive"
    } finally {
        if ($null -eq $oldCGO) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $oldCGO }
        if ($null -eq $oldGOOS) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $oldGOOS }
        if ($null -eq $oldGOARCH) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $oldGOARCH }
        Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue
    }
}
