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
$DefaultPort = 8090

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

function Get-ConfiguredPort {
  $port = $DefaultPort
  if(Test-Path $BinaryPath){
    try {
      $configPort = $null
      $configPort = & $BinaryPath config get server.port 2>$null
      if($configPort -match '^[0-9]+$'){
        $port = [int]$configPort
      }
    } catch {}
  }
  return $port
}

function Check-Port([int]$port){
  try {
    $lines = netstat -ano | Select-String -Pattern "^\s*TCP\s+[^:]+:$port\s+" -ErrorAction SilentlyContinue
    if($lines){
      Write-Warn "Port $port is already in use"
      Write-Info "Processes using port ${port}:"
      $lines | ForEach-Object { $_.Line } | Write-Host
      return $false
    }
  } catch {}
  return $true
}

$stdoutTempFile = Join-Path $env:TEMP ("axonhub-" + [guid]::NewGuid().ToString() + '-stdout.log')
$stderrTempFile = Join-Path $env:TEMP ("axonhub-" + [guid]::NewGuid().ToString() + '-stderr.log')

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

# Check configured port
$port = Get-ConfiguredPort
if(-not (Check-Port $port)){
  Write-Err "Cannot start AxonHub: port $port is already in use"
  exit 1
}

$ConfigArgs = @()
if(Test-Path $ConfigFile){ 
  # Config exists, binary will auto-detect it from $HOME/.config/axonhub/
  Write-Info "Configuration found at $ConfigFile, binary will auto-detect it" 
} else { 
  Write-Warn "Configuration not found at $ConfigFile, starting with defaults" 
}

Write-Info 'Starting AxonHub process...'
try {
  if($ConfigArgs.Count -gt 0){
    $p = Start-Process -FilePath $BinaryPath -ArgumentList $ConfigArgs -RedirectStandardOutput $stdoutTempFile -RedirectStandardError $stderrTempFile -PassThru -WindowStyle Hidden
  } else {
    $p = Start-Process -FilePath $BinaryPath -RedirectStandardOutput $stdoutTempFile -RedirectStandardError $stderrTempFile -PassThru -WindowStyle Hidden
  }
  Start-Sleep -Seconds 2
  if($p -and (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)){
    $p.Id | Out-File -FilePath $PidFile -Encoding ascii -Force
    Remove-Item -Force $stdoutTempFile,$stderrTempFile -ErrorAction SilentlyContinue
    Write-Success "AxonHub started successfully (PID: $($p.Id))"
    Write-Info 'Process information:'
    Write-Host "  • PID: $($p.Id)"
    Write-Host "  • Log file: $LogFile"
    Write-Host "  • Config: " -NoNewline; if(Test-Path $ConfigFile){ Write-Host $ConfigFile } else { Write-Host 'default' }
    Write-Host "  • Web interface: http://localhost:$port"
    Write-Host ''
    Write-Info 'To stop AxonHub: stop.bat'
    Write-Info "To view logs: Get-Content -Path '$LogFile' -Tail 100 -Wait"
  } else {
    Write-Err 'AxonHub failed to start'
    if(Test-Path $LogFile){
      Write-Info 'Last few log lines:'
      Get-Content -Path $LogFile -Tail 20
    }
    if(Test-Path $stdoutTempFile){
      Write-Info 'Captured stdout:'
      Get-Content -Path $stdoutTempFile -Tail 20
    }
    if(Test-Path $stderrTempFile){
      Write-Info 'Captured stderr:'
      Get-Content -Path $stderrTempFile -Tail 20
    }
    if(Test-Path $PidFile){ Remove-Item -Force $PidFile -ErrorAction SilentlyContinue }
    Remove-Item -Force $stdoutTempFile,$stderrTempFile -ErrorAction SilentlyContinue
    exit 1
  }
} catch {
  Write-Err $_.Exception.Message
  if(Test-Path $PidFile){ Remove-Item -Force $PidFile -ErrorAction SilentlyContinue }
  Remove-Item -Force $stdoutTempFile,$stderrTempFile -ErrorAction SilentlyContinue
  exit 1
}
