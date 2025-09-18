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

$ServiceName = 'axonhub'
$BaseDir = Join-Path $env:LOCALAPPDATA 'AxonHub'
$ConfigFile = Join-Path $BaseDir 'config.yml'
$BinaryPath = Join-Path $BaseDir 'axonhub.exe'
$PidFile = Join-Path $BaseDir 'axonhub.pid'
$LogFile = Join-Path $BaseDir 'axonhub.log'

function Show-Usage {
  Write-Host @" 
Usage: start.bat

This script starts AxonHub directly (no service manager).
Logs: $LogFile
PID file: $PidFile
"@
}

foreach($a in $ArgsFromCmd){
  switch -Regex ($a){
    '^(--help|-h)$' { Show-Usage; exit 0 }
    default { Write-Warn "Unknown option: $a" }
  }
}

function Ensure-Dirs([string]$path){ if(-not (Test-Path $path)){ New-Item -ItemType Directory -Force -Path $path | Out-Null } }

function Check-Port([int]$port){
  try {
    $lines = netstat -ano | Select-String -Pattern ":$port\s" -ErrorAction SilentlyContinue
    if($lines){
      Write-Warn "Port $port is already in use"
      Write-Info "Processes using port $port:"
      $lines | ForEach-Object { $_.Line } | Write-Host
      return $false
    }
  } catch {}
  return $true
}

Write-Info 'Starting AxonHub...'

if(-not (Test-Path $BinaryPath)){
  Write-Err "AxonHub binary not found at $BinaryPath"
  Write-Info 'Please run the installer first: install.bat'
  exit 1
}

Ensure-Dirs $BaseDir

# Already running?
if(Test-Path $PidFile){
  try {
    $pid = Get-Content -Path $PidFile -ErrorAction Stop
    if($pid -and (Get-Process -Id $pid -ErrorAction SilentlyContinue)){
      Write-Warn "AxonHub is already running (PID: $pid)"
      exit 0
    } else {
      Write-Info 'Removing stale PID file'
      Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    }
  } catch {
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
  }
}

# Check default port 8090
if(-not (Check-Port 8090)){
  Write-Err 'Cannot start AxonHub: port 8090 is already in use'
  exit 1
}

$ConfigArgs = @()
if(Test-Path $ConfigFile){ $ConfigArgs += @('--config', $ConfigFile) } else { Write-Warn "Configuration not found at $ConfigFile, starting with defaults" }

Write-Info 'Starting AxonHub process...'
try {
  $p = Start-Process -FilePath $BinaryPath -ArgumentList $ConfigArgs -RedirectStandardOutput $LogFile -RedirectStandardError $LogFile -PassThru -WindowStyle Hidden
  Start-Sleep -Seconds 2
  if($p -and (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)){
    $p.Id | Out-File -FilePath $PidFile -Encoding ascii -Force
    Write-Success "AxonHub started successfully (PID: $($p.Id))"
    Write-Info 'Process information:'
    Write-Host "  • PID: $($p.Id)"
    Write-Host "  • Log file: $LogFile"
    Write-Host "  • Config: " -NoNewline; if(Test-Path $ConfigFile){ Write-Host $ConfigFile } else { Write-Host 'default' }
    Write-Host '  • Web interface: http://localhost:8090'
    Write-Host ''
    Write-Info 'To stop AxonHub: stop.bat'
    Write-Info "To view logs: Get-Content -Path '$LogFile' -Tail 100 -Wait"
  } else {
    Write-Err 'AxonHub failed to start'
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
