@echo off
setlocal

REM AxonHub Windows start wrapper (.bat)

where powershell >NUL 2>NUL
if %ERRORLEVEL% NEQ 0 (
  echo [ERROR] PowerShell is required to run this script.
  exit /b 1
)

set "SCRIPT_DIR=%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%start.ps1" %*
exit /b %ERRORLEVEL%
