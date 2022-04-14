#Requires -Version 5.0
<#
.SYNOPSIS 
    Native etcd binary builds for Windows
.DESCRIPTION 
    Run the script to build etcd binaries on a Windows machine
.NOTES
    Environment Variables
    - DEBUG (Sets specific Go ldflags for debugging purposes | Switch parameter)
    - GIT_VERSION (Local version of Git to install | default: 2.35.2)
    - GO_VERSION (Local version of Go to install | default: 1.17.8)
    - GIT_ORG (default: etcd-io}
    - GIT_REPO (default: etcd}

    
.EXAMPLE
    make.ps1 -GoDebug
    make.ps1 -Script ci
    $env:GIT_ORG="YOUR-GITHUB-ORG"; $env:GIT_REPO="YOUR-GITHUB-REPO"; make.ps1 -Script build
#>

# Make sure these params matches the CmdletBinding below
param (
    # [Parameter(Mandatory = $true)]
    # [ValidateNotNullOrEmpty()]
    # [String]
    # $Version,
    [Switch]
    $GoDebug,
    [AllowEmptyString()]
    [String]
    $Script = "build"  # Default invocation is full CI    
)

function Invoke-EtcdBuild() {
    # [CmdletBinding()]
    # param (
    #     [Parameter(Mandatory = $true)]
    #     [ValidateNotNullOrEmpty()]
    #     [String]
    #     $Version
    # )

    # TODO: Integration Tests
    if ($env:SCRIPT_PATH.ToLower().Contains("integration")) {
        Invoke-EtcdIntegrationTests
    }

    # TODO: Additional Tests
    if ($env:SCRIPT_PATH.ToLower().Contains("all")) {
        Invoke-AllEtcd
    }

    if ($env:SCRIPT_PATH.ToLower().Contains("build")) {
        Write-LogInfo "Starting Builds for etcd and etcdctl"
        Invoke-Script -File scripts\windows\build.ps1
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
        exit 0
    }

    if (Test-Path $env:SCRIPT_PATH) {
        Write-LogInfo ("Running {0}.ps1" -f $env:SCRIPT_PATH)
        Invoke-Script -File $env:SCRIPT_PATH
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
        exit 0
    }
}

function Get-Args() {
    # if ($Version) {
    #     $env:VERSION = $Version
    #     $env:GIT_TAG = $env:VERSION
    # }

    if ($GoDebug.IsPresent) {
        $env:DEBUG = "true"
    }

    if ($Script) {
        $env:SCRIPT_PATH = ("{0}\scripts\windows\{1}.ps1" -f $PSScriptRoot, $Script)
    }
}

function Set-Environment() {
    $GIT_VERSION = $env:GIT_VERSION
    if (-Not $GIT_VERSION) {        
        $env:GIT_VERSION = "2.35.2"
    }

    $GOLANG_VERSION = $env:GOLANG_VERSION
    if (-Not $GOLANG_VERSION) {        
        $env:GOLANG_VERSION = "1.17.8"
    }

    $ORG_PATH = $env:ORG_PATH
    if (-Not $ORG_PATH) {
        $env:ORG_PATH = "go.etcd.io"
    }

    $GIT_ORG = $env:GIT_ORG
    if (-Not $GIT_ORG) {
        $env:GIT_ORG = "etcd-io"
    }

    $GIT_REPO = $env:GIT_REPO
    if (-Not $GIT_REPO) {
        $env:GIT_REPO = "etcd"
    }

    # $VERSION = $env:VERSION
    # if (-Not $VERSION) {
    #     $env:VERSION = "$(git rev-parse --short HEAD)"
    # }
}

function Set-Path() {
    # ideally, gopath would be C:\go to match Linux a bit closer
    # but C:\go is the recommended install path for Go itself on Windows, so we use C:\gopath
    $env:PATH += ";C:\git\cmd;C:\git\mingw64\bin;C:\git\usr\bin;C:\gopath\bin;C:\go\bin"
    $environment = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    $environment = $environment.Insert($environment.Length, ";C:\git\cmd;C:\git\mingw64\bin;C:\git\usr\bin;C:\gopath\bin;C:\go\bin")
    [System.Environment]::SetEnvironmentVariable("Path", $environment, "Machine")
}
    
function Test-Architecture() {
    if ($env:PROCESSOR_ARCHITECTURE -ne "AMD64" -and $env:PROCESSOR_ARCHITECTURE -ne "ARM64") {
        Write-LogFatal "Unsupported architecture $( $env:PROCESSOR_ARCHITECTURE )"
    }
}

function Install-Git() {
    # install git
    if ($null -eq (Get-Command "git" -ErrorAction SilentlyContinue)) {
        Push-Location C:\
        Write-LogInfo ('not found in PATH, installing git v{0} ...' -f $env:GIT_VERSION)
        Invoke-WebRequest -Uri "https://github.com/git-for-windows/git/releases/download/v$env:GIT_VERSION.windows.1/MinGit-$env:GIT_VERSION-64-bit.zip" -OutFile 'go.zip'
        Expand-Archive -Force -Path c:\git.zip -DestinationPath c:\git\.
        Remove-Item -Force -Recurse -Path c:\git.zip
        Pop-Location
    } else {
        Write-LogInfo ('{0} found in PATH, skipping install ...' -f $(git version))
    }
}
    
function Install-Go() {
    # install go
    if ($null -eq (Get-Command "go" -ErrorAction SilentlyContinue)) {
        Write-LogInfo ("go not found in PATH, installing go{0}" -f $env:GOLANG_VERSION)
        Push-Location C:\
        Invoke-WebRequest -Uri ('https://go.dev/dl/go{0}.windows-amd64.zip' -f $env:GOLANG_VERSION) -OutFile 'go.zip'
        Expand-Archive go.zip -DestinationPath C:\
        Remove-Item go.zip -Force
        Pop-Location
        Write-LogInfo ('Installed go{0}' -f $env:GOLANG_VERSION)
    } else {
        Write-LogInfo ('{0} found in PATH, skipping go install ...' -f $(go version))
    }
}

function Install-Ginkgo() {
    if ($null -eq (Get-Command "ginkgo" -ErrorAction SilentlyContinue)) {
        Write-LogInfo "ginkgo not found in PATH, installing"
        Push-Location c:\
        go install github.com/onsi/ginkgo/ginkgo@latest
        go get github.com/onsi/gomega/...
        Pop-Location
    } else {
        Write-LogInfo ('{0} found in PATH, skipping install ...' -f $(ginkgo version))
    }
}

function Initialize-Environment() {
    Write-LogInfo 'Preparing local etcd build environment'
    Install-Git
    Install-Go
    # Install-Ginkgo
}

function Invoke-EtcdIntegrationTests() {
    Write-LogInfo "Running Integration Tests"
    Invoke-Script -File scripts\windows\build.ps1
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    Invoke-Script -File scripts\windows\integration.ps1
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    exit 0
}

function Invoke-AllEtcd() {
    Write-LogInfo "Running CI and Integration Tests"
    Invoke-Script -File scripts\windows\build.ps1
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    Invoke-Script -File scripts\windows\integration.ps1
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    exit 0
}

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
Set-StrictMode -Version Latest

if (-Not (Test-Path "$PSScriptRoot\scripts\windows\$Script.ps1")) {
    throw "$Script is not a valid script name in $(Write-Output $PSScriptRoot\scripts\windows)"
}
$env:SCRIPT_PATH = $Script

Import-Module -WarningAction Ignore -Name "$PSScriptRoot\scripts\windows\utils.psm1"

if ($GoDebug.IsPresent) {
    Write-LogInfo "Debug mode is enabled"
    $env:DEBUG = "true"
}

Get-Args
Test-Architecture
Initialize-Environment
Set-Environment
Set-Path
# This is required as long as the symlinks for client/v3/*_test.go -> tests/integration/clientv3/examples/*_test.go exist
git config --global core.symlinks true

Invoke-EtcdBuild # -Version $env:VERSION
