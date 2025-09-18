@echo off
setlocal

REM AxonHub Windows installer wrapper (.bat)

where powershell >NUL 2>NUL
if %ERRORLEVEL% NEQ 0 (
  echo [ERROR] PowerShell is required to run this installer.
  echo Please run on Windows 7+ with PowerShell available in PATH.
  exit /b 1
)

set "SCRIPT_DIR=%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%install.ps1" %*
exit /b %ERRORLEVEL%
