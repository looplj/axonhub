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
$BinaryPath = Join-Path $BaseDir 'axonhub.exe'
$StartupFolder = [Environment]::GetFolderPath('Startup')
$ShortcutPath = Join-Path $StartupFolder 'AxonHub.lnk'
$TaskName = 'AxonHubAutoStart'

function Show-Usage {
  Write-Host @'
AxonHub Setup (Windows)

Usage:
  setup.bat [command] [options]

Commands:
  install-autostart    Install AxonHub to start automatically on boot
  uninstall-autostart  Remove AxonHub from automatic startup
  status               Check autostart status

Options:
  -h, --help           Show this help and exit

Examples:
  setup.bat install-autostart      # Enable auto-start on boot
  setup.bat uninstall-autostart    # Disable auto-start
  setup.bat status                 # Check current status
'@
}

function Test-Admin {
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-AutostartStatus {
    $status = @{
        StartupFolder = $false
        TaskScheduler = $false
        TaskEnabled = $false
    }

    # Check startup folder
    if (Test-Path $ShortcutPath) {
        $status.StartupFolder = $true
    }

    # Check task scheduler
    try {
        $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        if ($task) {
            $status.TaskScheduler = $true
            $status.TaskEnabled = ($task.State -ne 'Disabled')
        }
    } catch {}

    return $status
}

function Install-StartupFolder {
    Write-Info "Installing startup shortcut..."

    if (-not (Test-Path $BinaryPath)) {
        Write-Err "AxonHub binary not found at $BinaryPath"
        Write-Info "Please run install.bat first to install AxonHub"
        return $false
    }

    $WshShell = New-Object -ComObject WScript.Shell
    $Shortcut = $WshShell.CreateShortcut($ShortcutPath)
    $Shortcut.TargetPath = $BinaryPath
    $Shortcut.WorkingDirectory = $BaseDir
    $Shortcut.IconLocation = $BinaryPath
    $Shortcut.Save()

    Write-Success "Startup shortcut created at: $ShortcutPath"
    return $true
}

function Uninstall-StartupFolder {
    Write-Info "Removing startup shortcut..."

    if (Test-Path $ShortcutPath) {
        Remove-Item -Path $ShortcutPath -Force
        Write-Success "Startup shortcut removed"
        return $true
    } else {
        Write-Warn "No startup shortcut found"
        return $false
    }
}

function Install-TaskScheduler {
    Write-Info "Installing scheduled task for auto-start..."

    if (-not (Test-Path $BinaryPath)) {
        Write-Err "AxonHub binary not found at $BinaryPath"
        Write-Info "Please run install.bat first to install AxonHub"
        return $false
    }

    if (-not (Test-Admin)) {
        Write-Warn "Administrator privileges required for Task Scheduler method"
        Write-Info "Attempting to elevate privileges..."

        $scriptPath = $MyInvocation.MyCommand.Path
        $arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" install-autostart"
        Start-Process PowerShell -Verb RunAs -ArgumentList $arguments
        return $true
    }

    # Remove existing task if any
    try {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    } catch {}

    $Action = New-ScheduledTaskAction -Execute $BinaryPath -WorkingDirectory $BaseDir
    $Trigger = New-ScheduledTaskTrigger -AtLogOn
    $Principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -RunLevel Highest
    $Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable

    try {
        Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -Force | Out-Null
        Write-Success "Scheduled task '$TaskName' created successfully"
        Write-Info "AxonHub will start automatically when you log in"
        return $true
    } catch {
        Write-Err "Failed to create scheduled task: $_"
        return $false
    }
}

function Uninstall-TaskScheduler {
    Write-Info "Removing scheduled task..."

    if (-not (Test-Admin)) {
        Write-Warn "Administrator privileges required"
        Write-Info "Attempting to elevate privileges..."

        $scriptPath = $MyInvocation.MyCommand.Path
        $arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" uninstall-autostart"
        Start-Process PowerShell -Verb RunAs -ArgumentList $arguments
        return $true
    }

    try {
        $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        if ($task) {
            Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
            Write-Success "Scheduled task '$TaskName' removed"
            return $true
        } else {
            Write-Warn "No scheduled task found with name '$TaskName'"
            return $false
        }
    } catch {
        Write-Warn "Could not remove scheduled task: $_"
        return $false
    }
}

function Show-Status {
    Write-Info "Checking AxonHub autostart status..."

    $status = Get-AutostartStatus

    Write-Host ""
    Write-Host "Autostart Status:"
    Write-Host "  Startup Folder:  $(if ($status.StartupFolder) { 'ENABLED' } else { 'DISABLED' })"
    Write-Host "  Task Scheduler:  $(if ($status.TaskScheduler) { if ($status.TaskEnabled) { 'ENABLED' } else { 'DISABLED (task exists but is disabled)' } } else { 'DISABLED' })"
    Write-Host ""

    # Check if AxonHub is currently running
    $procs = Get-Process -Name 'axonhub' -ErrorAction SilentlyContinue
    if ($procs) {
        Write-Host "Process Status:"
        foreach ($proc in $procs) {
            Write-Host "  PID $($proc.Id): Running"
        }
    } else {
        Write-Host "Process Status: Not running"
    }

    Write-Host ""
    Write-Host "Installation Path: $BaseDir"
    Write-Host "Binary Path:       $BinaryPath"
}

function Install-Autostart {
    Write-Info "Installing AxonHub auto-start..."

    # Try Task Scheduler first (more reliable), fallback to Startup Folder
    $result = Install-TaskScheduler

    if (-not $result) {
        Write-Info "Falling back to Startup Folder method..."
        $result = Install-StartupFolder
    }

    if ($result) {
        Write-Host ""
        Write-Success "Auto-start installation completed!"
        Write-Info "AxonHub will start automatically on next boot/login"
    }

    return $result
}

function Uninstall-Autostart {
    Write-Info "Uninstalling AxonHub auto-start..."

    $result1 = Uninstall-TaskScheduler
    $result2 = Uninstall-StartupFolder

    if ($result1 -or $result2) {
        Write-Host ""
        Write-Success "Auto-start removal completed!"
        Write-Info "AxonHub will no longer start automatically on boot"
    }
}

# Parse arguments
$Command = $null

foreach ($a in $ArgsFromCmd) {
    switch -Regex ($a) {
        '^(--help|-h)$' { Show-Usage; exit 0 }
        '^(install-autostart|enable-autostart)$' { $Command = 'install-autostart'; continue }
        '^(uninstall-autostart|disable-autostart)$' { $Command = 'uninstall-autostart'; continue }
        '^(status|check)$' { $Command = 'status'; continue }
        default { Write-Warn "Unknown argument: $a" }
    }
}

# Main execution
switch ($Command) {
    'install-autostart' {
        Install-Autostart
    }
    'uninstall-autostart' {
        Uninstall-Autostart
    }
    'status' {
        Show-Status
    }
    default {
        Show-Usage
    }
}
