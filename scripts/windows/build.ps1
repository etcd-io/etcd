#Requires -Version 5.0
$ErrorActionPreference = 'Stop'

Import-Module -WarningAction Ignore -Name "$PSScriptRoot\utils.psm1"


function Build {
    param (
        # [Parameter()]
        # [String]
        # $Version,
        [parameter()]
        [string]
        $BuildPath,
        [parameter()]
        [string]
        $Commit,
        [parameter()]
        [string]
        $Output        
    )
    $env:GO_LDFLAGS = '-s -w -gcflags=all=-dwarf=false -extldflags "-static"'

    if ($env:DEBUG) {
        $env:GO_LDFLAGS = '-v -gcflags=all=-N -l'
        Write-LogInfo ('Debug flag passed, changing ldflags to {0}' -f $env:GO_LDFLAGS )
        # go install github.com/go-delve/delve/cmd/dlv@latest
    }

    $GO_LDFLAGS = ("'{0} -X {1}/{2}/version.GitSHA={3}'" -f $env:GO_LDFLAGS, $env:ORG_PATH, $env:GIT_REPO, $Commit)
    if ($env:DEBUG){
        Write-LogInfo "[DEBUG] Running command: go build -o $Output -ldflags $GO_LDFLAGS"
    }

    Push-Location $BuildPath
    go build -o $Output -ldflags $GO_LDFLAGS .
    Pop-Location
    if (-Not $?) {
        Write-LogFatal "go build for $BuildPath failed!"
    }
}

trap {
    Write-Host -NoNewline -ForegroundColor Red "[ERROR]: "
    Write-Host -ForegroundColor Red "$_"

    Pop-Location
    exit 1
}

Invoke-Script -File "$PSScriptRoot\version.ps1"

$SRC_PATH = (Resolve-Path "$PSScriptRoot\..\..").Path
Push-Location $SRC_PATH
if ($env:DEBUG) {
    Write-LogInfo "[DEBUG] Build Path: $SRC_PATH"
}

Remove-Item -Path "$SRC_PATH/bin/*.exe" -Force -ErrorAction Ignore
$null = New-Item -Type Directory -Path bin -ErrorAction Ignore
$env:GOARCH = $env:ARCH
$env:GOOS = 'windows'
$env:CGO_ENABLED = 0

Build -BuildPath "$SRC_PATH/server" -Commit $env:COMMIT -Output "..\bin\etcd.exe" # -Version $env:VERSION
Build -BuildPath "$SRC_PATH/etcdctl" -Commit $env:COMMIT -Output "..\bin\etcdctl.exe" # -Version $env:VERSION
Write-LogInfo "Builds Complete"

Pop-Location
