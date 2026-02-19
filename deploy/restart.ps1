param(
    [Parameter(ValueFromRemainingArguments=$true)]
    [string[]]$ArgsFromCmd
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Write-Info([string]$m){ Write-Host "[INFO] $m" -ForegroundColor Cyan }
function Write-Success([string]$m){ Write-Host "[SUCCESS] $m" -ForegroundColor Green }
function Write-Warn([string]$m){ Write-Host "[WARNING] $m" -ForegroundColor Yellow }
function Write-Err([string]$m){ Write-Host "[ERROR] $m" -ForegroundColor Red }

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

function Show-Usage {
  Write-Host @" 
Usage: restart.bat [--force]

This script restarts AxonHub by stopping and starting it.
Options:
  --force     Force kill before restart
  --help, -h  Show this help message
"@
}

$Force = $false
foreach($a in $ArgsFromCmd){
  switch -Regex ($a){
    '^(--force)$' { $Force = $true; continue }
    '^(--help|-h)$' { Show-Usage; exit 0 }
    default { Write-Warn "Unknown option: $a" }
  }
}

Write-Info 'Restarting AxonHub...'

# Stop AxonHub
Write-Info 'Stopping AxonHub...'
$stopScript = Join-Path $ScriptDir 'stop.ps1'
if($Force){
  & $stopScript --force
} else {
  & $stopScript
}

# Brief pause to ensure clean shutdown
Start-Sleep -Seconds 5

# Start AxonHub
Write-Info 'Starting AxonHub...'
$startScript = Join-Path $ScriptDir 'start.ps1'
& $startScript

Write-Success 'AxonHub has been restarted'
