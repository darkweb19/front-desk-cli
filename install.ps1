[CmdletBinding()]
param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"
$repository = "darkweb19/front-desk-cli"
$asset = "tm_windows_amd64.exe"

$architecture = if ($env:PROCESSOR_ARCHITEW6432) {
    $env:PROCESSOR_ARCHITEW6432
} else {
    $env:PROCESSOR_ARCHITECTURE
}

if ($architecture -ne "AMD64") {
    throw "Unsupported Windows architecture: $architecture. Releases currently support Windows amd64 only."
}

if ($Version -eq "latest") {
    $releaseUrl = "https://github.com/$repository/releases/latest/download"
} elseif ($Version -like "v*") {
    $releaseUrl = "https://github.com/$repository/releases/download/$Version"
} else {
    throw "Version must be a tag beginning with v, for example v1.2.0."
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("tm-install-" + [guid]::NewGuid().ToString("N"))
$downloadedBinary = Join-Path $tempDir $asset
$checksumFile = Join-Path $tempDir "SHA256SUMS"
$stagedBinary = $null
$backupBinary = $null
$replacementCompleted = $false

try {
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseUrl/$asset" -OutFile $downloadedBinary
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseUrl/SHA256SUMS" -OutFile $checksumFile

    $escapedAsset = [regex]::Escape($asset)
    $checksumLine = @(Get-Content -LiteralPath $checksumFile | Where-Object {
        $_ -match "^[0-9a-fA-F]{64}\s+\*?$escapedAsset$"
    })

    if ($checksumLine.Count -ne 1) {
        throw "The release checksum file does not contain exactly one entry for $asset."
    }

    $expectedHash = ($checksumLine[0] -split "\s+")[0]
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $downloadedBinary).Hash
    if ($actualHash -ne $expectedHash) {
        throw "Checksum verification failed for $asset."
    }

    $installDir = Join-Path $env:LOCALAPPDATA "Programs\tm"
    $installedBinary = Join-Path $installDir "tm.exe"
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $stagedBinary = Join-Path $installDir (".tm.install." + [guid]::NewGuid().ToString("N") + ".exe")
    $backupBinary = Join-Path $installDir (".tm.backup." + [guid]::NewGuid().ToString("N") + ".exe")
    Copy-Item -LiteralPath $downloadedBinary -Destination $stagedBinary

    if (Test-Path -LiteralPath $installedBinary) {
        try {
            [System.IO.File]::Replace($stagedBinary, $installedBinary, $backupBinary, $true)
        } catch {
            if (Test-Path -LiteralPath $backupBinary) {
                Write-Warning "Update failed. The previous binary is preserved at $backupBinary"
            }
            throw
        }
    } else {
        [System.IO.File]::Move($stagedBinary, $installedBinary)
    }
    $replacementCompleted = $true
    $stagedBinary = $null

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = @($userPath -split ";" | Where-Object { $_ })
    $pathAlreadyConfigured = $pathEntries | Where-Object {
        $_.TrimEnd("\") -ieq $installDir.TrimEnd("\")
    }

    if (-not $pathAlreadyConfigured) {
        $updatedPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $installDir
        } else {
            $userPath.TrimEnd(";") + ";" + $installDir
        }
        [Environment]::SetEnvironmentVariable("Path", $updatedPath, "User")
        Write-Host "Installed tm to $installedBinary and added it to your user PATH."
        Write-Host "Open a new terminal before running tm."
    } else {
        Write-Host "Installed tm to $installedBinary"
    }
} finally {
    if ($stagedBinary -and (Test-Path -LiteralPath $stagedBinary)) {
        Remove-Item -Force -LiteralPath $stagedBinary
    }
    if ($replacementCompleted -and $backupBinary -and (Test-Path -LiteralPath $backupBinary)) {
        Remove-Item -Force -LiteralPath $backupBinary
    }
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -Recurse -Force -LiteralPath $tempDir
    }
}
