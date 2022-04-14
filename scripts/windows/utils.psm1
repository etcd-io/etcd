#Requires -Version 5.0

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

function Write-LogInfo {
    Write-Host -NoNewline -ForegroundColor Blue "INFO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($Args -join " "))
}

function Write-LogWarn {
    Write-Host -NoNewline -ForegroundColor DarkYellow "WARN: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Write-LogError {
    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}


function Write-LogFatal {
    Write-Host -NoNewline -ForegroundColor DarkRed "FATA: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))

    exit 255
}

function Invoke-Script {
    param (
        [parameter(Mandatory = $true)] [string]$File
    )

    try {
        Invoke-Expression -Command $File
        if (-not $?) {
            Write-LogFatal "Failed to invoke $File"
        }
    }
    catch {
        Write-LogFatal "Could not invoke $File, $($_.Exception.Message)"
    }
}

Export-ModuleMember -Function Write-LogInfo
Export-ModuleMember -Function Write-LogWarn
Export-ModuleMember -Function Write-LogError
Export-ModuleMember -Function Write-LogFatal
Export-ModuleMember -Function Invoke-Script
