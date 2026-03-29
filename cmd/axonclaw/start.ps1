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

$ServiceName = 'axonclaw'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$BaseDir = Join-Path $ScriptDir '.axonclaw'
$BinaryPath = Join-Path $ScriptDir 'axonclaw.exe'
$PidFile = Join-Path $BaseDir 'axonclaw.pid'
$LogFile = Join-Path $BaseDir 'logs\axonclaw.log'

function Show-Usage {
  Write-Host @" 
Usage: start.bat

This script starts AxonClaw directly (no service manager).
Logs: $LogFile
PID file: $PidFile

Environment variables (all optional if config exists):
  AXONCLAW_BASE_URL      Optional. AxonHub server URL
  AXONCLAW_API_KEY       Optional. Agent API key for authentication
  AXONCLAW_AUTO_SYNC_CONFIG Optional. Set to 'true' to enable --auto-sync-config
  DEBUG_MODE             Optional. Set to 'true' to enable debug logging

Example (first start):
  set AXONCLAW_API_KEY=your-key && start.bat

Example (subsequent starts with saved config):
  start.bat
"@
}

foreach($a in $ArgsFromCmd){
  switch -Regex ($a){
    '^(--help|-h)$' { Show-Usage; exit 0 }
    default { Write-Warn "Unknown option: $a" }
  }
}

function Ensure-Dirs([string]$path){ if(-not (Test-Path $path)){ New-Item -ItemType Directory -Force -Path $path | Out-Null } }

function Find-Binary {
  if(Test-Path $BinaryPath){
    return $BinaryPath
  }
  $cmd = Get-Command 'axonclaw' -ErrorAction SilentlyContinue
  if($cmd){
    return $cmd.Source
  }
  return $null
}

$stdoutTempFile = Join-Path $env:TEMP ("axonclaw-" + [guid]::NewGuid().ToString() + '-stdout.log')
$stderrTempFile = Join-Path $env:TEMP ("axonclaw-" + [guid]::NewGuid().ToString() + '-stderr.log')

Write-Info 'Starting AxonClaw...'

$binaryPath = Find-Binary
if(-not $binaryPath){
  Write-Err "AxonClaw binary not found"
  Write-Info 'Please build the binary first: go build -o axonclaw.exe ./cmd/axonclaw'
  exit 1
}

Write-Info "Using binary: $binaryPath"

Ensure-Dirs $BaseDir
Ensure-Dirs (Join-Path $BaseDir 'logs')

if(Test-Path $PidFile){
  try {
    $pid = Get-Content -Path $PidFile -ErrorAction Stop
    if($pid -and (Get-Process -Id $pid -ErrorAction SilentlyContinue)){
      Write-Warn "AxonClaw is already running (PID: $pid)"
      exit 0
    } else {
      Write-Info 'Removing stale PID file'
      Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    }
  } catch {
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
  }
}

$args = @()
if($env:AXONCLAW_AUTO_SYNC_CONFIG){
  $args += "--auto-sync-config"
}

if($env:DEBUG_MODE){
  $args += "--debug"
}

Write-Info 'Starting AxonClaw process...'
if($env:AXONCLAW_BASE_URL){
  Write-Info "  Base URL: $env:AXONCLAW_BASE_URL"
}
if($env:AXONCLAW_AUTO_SYNC_CONFIG){
  Write-Info "  Auto Sync Config: enabled"
}

try {
  $p = Start-Process -FilePath $binaryPath -ArgumentList $args -RedirectStandardOutput $LogFile -RedirectStandardError $LogFile -PassThru -WindowStyle Hidden -NoNewWindow
  Start-Sleep -Seconds 2
  if($p -and (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)){
    $p.Id | Out-File -FilePath $PidFile -Encoding ascii -Force
    Write-Success "AxonClaw started successfully (PID: $($p.Id))"
    Write-Info 'Process information:'
    Write-Host "  • PID: $($p.Id)"
    Write-Host "  • Log file: $LogFile"
    Write-Host ''
    Write-Info 'To stop AxonClaw: stop.bat'
    Write-Info "To view logs: Get-Content -Path '$LogFile' -Tail 100 -Wait"
  } else {
    Write-Err 'AxonClaw failed to start'
    if(Test-Path $LogFile){
      Write-Info 'Last few log lines:'
      Get-Content -Path $LogFile -Tail 20
    }
    if(Test-Path $PidFile){ Remove-Item -Force $PidFile -ErrorAction SilentlyContinue }
    exit 1
  }
} catch {
  Write-Err $_.Exception.Message
  if(Test-Path $PidFile){ Remove-Item -Force $PidFile -ErrorAction SilentlyContinue }
  exit 1
}
