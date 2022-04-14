#Requires -Version 5.0
$ErrorActionPreference = 'Stop'

Import-Module -WarningAction Ignore -Name "$PSScriptRoot\utils.psm1"

$DIRTY = ""
if ("$(git status --porcelain --untracked-files=no)") {
    $DIRTY = "-dirty"
}

$COMMIT = $(git rev-parse --short HEAD)
$GIT_TAG = $env:GIT_TAG
if (-not $GIT_TAG) {
    $GIT_TAG = $(git tag -l --contains HEAD | Select-Object -First 1)
}
$env:COMMIT = $COMMIT

$VERSION = "${COMMIT}${DIRTY}"
if ((-not $DIRTY) -and ($GIT_TAG)) {
    $VERSION = "${GIT_TAG}"
}
$env:VERSION = $VERSION

$ARCH = $env:ARCH
if (-not $ARCH) {
    $ARCH = "amd64"
}
$env:ARCH = $ARCH

$buildTags = @{ "17763" = "1809"; "19042" = "20H2"; "20348" = "ltsc2022";}
$buildNumber = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\' -ErrorAction Ignore).CurrentBuildNumber
$SERVERCORE_VERSION = $buildTags[$buildNumber]
if (-not $SERVERCORE_VERSION) {
    $env:SERVERCORE_VERSION = "1809"
}

Write-LogInfo "ARCH: $ARCH"
Write-LogInfo "VERSION: $VERSION"
Write-LogInfo "GIT_SHA: $COMMIT"
Write-LogInfo "SERVERCORE_VERSION: $SERVERCORE_VERSION"
