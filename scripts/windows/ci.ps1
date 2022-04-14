#Requires -Version 5.0

$ErrorActionPreference = 'Stop'

Import-Module -WarningAction Ignore -Name "$PSScriptRoot\utils.psm1"

# TODO: Invoke-Script -File "$PSScriptRoot\test.ps1"
Invoke-Script -File "$PSScriptRoot\build.ps1"
# TODO: Invoke-Script -File "$PSScriptRoot\integration.ps1"
