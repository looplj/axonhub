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

This script restarts AxonClaw by stopping and starting it.
Options:
  --force     Force kill before restart
  --help, -h  Show this help message

Environment variables (passed to start.ps1):
  AXONCLAW_BASE_URL      Optional. AxonHub server URL
  AXONCLAW_API_KEY       Optional. Agent API key for authentication
  AXONCLAW_AUTO_SYNC_CONFIG Optional. Set to 'true' to enable --auto-sync-config
  DEBUG_MODE             Optional. Set to 'true' to enable debug logging

Example:
  restart.bat
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

Write-Info 'Restarting AxonClaw...'

Write-Info 'Stopping AxonClaw...'
$stopScript = Join-Path $ScriptDir 'stop.ps1'
if($Force){
  & $stopScript --force
} else {
  & $stopScript
}

Start-Sleep -Seconds 1

Write-Info 'Starting AxonClaw...'
$startScript = Join-Path $ScriptDir 'start.ps1'
& $startScript

Write-Success 'AxonClaw has been restarted'
