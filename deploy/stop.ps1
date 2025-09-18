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
$PidFile = Join-Path $BaseDir 'axonhub.pid'
$ProcessName = 'axonhub'

function Show-Usage {
  Write-Host @" 
Usage: stop.bat [--force]

This script stops AxonHub directly (no service manager).
Options:
  --force     Force kill all AxonHub processes
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

function Stop-ByPid(){
  Write-Info 'Stopping AxonHub using PID file...'
  if(-not (Test-Path $PidFile)){
    Write-Warn "PID file not found at $PidFile"
    return $false
  }
  try {
    $pid = Get-Content -Path $PidFile -ErrorAction Stop
  } catch {
    Write-Warn 'Unable to read PID file'
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    return $false
  }
  if(-not ($pid -match '^[0-9]+$')){
    Write-Err "Invalid PID in file: $pid"
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    return $false
  }
  $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
  if(-not $proc){
    Write-Warn "Process with PID $pid is not running"
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    return $false
  }
  Write-Info "Stopping process $pid ..."
  try { Stop-Process -Id $pid -ErrorAction SilentlyContinue } catch {}
  # Wait up to 10 seconds
  $timeout = 10
  for($i=0; $i -lt $timeout; $i++){
    Start-Sleep -Seconds 1
    if(-not (Get-Process -Id $pid -ErrorAction SilentlyContinue)){ break }
  }
  if(Get-Process -Id $pid -ErrorAction SilentlyContinue){
    Write-Warn 'Process did not stop gracefully, forcing termination...'
    try { Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue } catch {}
    Start-Sleep -Seconds 2
  }
  if(-not (Get-Process -Id $pid -ErrorAction SilentlyContinue)){
    Write-Success "AxonHub stopped successfully (PID: $pid)"
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    return $true
  } else {
    Write-Err 'Failed to stop AxonHub process'
    return $false
  }
}

function Stop-ByProcessName(){
  Write-Info 'Stopping AxonHub by process name...'
  $procs = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match '^axonhub' }
  if(-not $procs){
    Write-Warn 'No AxonHub processes found'
    return $false
  }
  Write-Info ("Found AxonHub processes: " + ($procs.Id -join ' '))
  foreach($p in $procs){
    Write-Info ("Stopping process " + $p.Id + ' ...')
    try { Stop-Process -Id $p.Id -ErrorAction SilentlyContinue } catch {}
    # Wait up to 10 seconds each
    $timeout = 10
    for($i=0; $i -lt $timeout; $i++){
      Start-Sleep -Seconds 1
      if(-not (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)){ break }
    }
    if(Get-Process -Id $p.Id -ErrorAction SilentlyContinue){
      Write-Warn ("Process " + $p.Id + ' did not stop gracefully, forcing...')
      try { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue } catch {}
    }
  }
  Start-Sleep -Seconds 2
  $remaining = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match '^axonhub' }
  if(-not $remaining){
    Write-Success 'All AxonHub processes stopped successfully'
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    return $true
  } else {
    Write-Err ("Some AxonHub processes are still running: " + ($remaining.Id -join ' '))
    return $false
  }
}

function Check-Running(){
  $procs = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match '^axonhub' }
  if($procs){
    Write-Info 'Running AxonHub processes:'
    $procs | Select-Object Id,ProcessName,Path | Format-Table -AutoSize | Out-String | Write-Host
    return $true
  }
  return $false
}

if($Force){
  Write-Info 'Force stopping all AxonHub processes...'
  $procs = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match '^axonhub' }
  if($procs){
    foreach($p in $procs){ try { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue } catch {} }
    Start-Sleep -Seconds 2
    $still = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match '^axonhub' }
    if(-not $still){
      Write-Success 'All AxonHub processes force-stopped'
      Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
    } else {
      Write-Err 'Failed to force-stop some processes'
      exit 1
    }
  } else {
    Write-Info 'No AxonHub processes found'
  }
  exit 0
}

Write-Info 'Stopping AxonHub...'
$stopped = $false
if(Stop-ByPid){ $stopped = $true }
if(-not $stopped){ if(Stop-ByProcessName){ $stopped = $true } }
if(-not $stopped){
  if(Check-Running){
    Write-Err 'Failed to stop all AxonHub processes'
    exit 1
  } else {
    Write-Info 'No AxonHub processes were running'
  }
}
Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
Write-Success 'AxonHub has been stopped'
