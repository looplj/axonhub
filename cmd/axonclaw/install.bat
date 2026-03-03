@echo off
setlocal EnableDelayedExpansion

set "REPO=looplj/axonhub"
set "BINARY_NAME=axonclaw"
set "GITHUB_API=https://api.github.com/repos/%REPO%"
if not defined INSTALL_DIR set "INSTALL_DIR=."

echo [INFO] AxonClaw Installer
echo [INFO] ==================
echo [INFO] Note: This installer targets the 'axonclaw' component from the AxonHub project.

:: Detect platform
echo [INFO] Detected platform: windows/amd64

:: Build common request headers (including optional GITHUB_TOKEN)
set "GH_HEADERS=-Headers @{'Accept'='application/vnd.github+json';'X-GitHub-Api-Version'='2022-11-28';'User-Agent'='axonclaw-installer'"
if defined GITHUB_TOKEN (
    set "GH_HEADERS=%GH_HEADERS%;'Authorization'='Bearer %GITHUB_TOKEN%'"
)
set "GH_HEADERS=%GH_HEADERS%}"

:: Get latest axonclaw version - list recent releases and find first axonclaw-v* tag
echo [INFO] Fetching latest axonclaw release...
set "VERSION="
for /f "tokens=*" %%a in ('powershell -Command "try { $r = Invoke-RestMethod -Uri '%GITHUB_API%/releases?per_page=30' %GH_HEADERS% -UseBasicParsing; $tag = ($r | Where-Object { $_.tag_name -like 'axonclaw-v*' -and -not $_.draft } | Select-Object -First 1).tag_name; if ($tag) { Write-Output $tag } else { exit 1 } } catch { exit 1 }"') do set "VERSION=%%a"

:: Fallback: list tags via API
if "%VERSION%"=="" (
    echo [WARNING] Release list failed, falling back to tags API...
    for /f "tokens=*" %%a in ('powershell -Command "try { $r = Invoke-RestMethod -Uri '%GITHUB_API%/tags?per_page=30' %GH_HEADERS% -UseBasicParsing; $tag = ($r | Where-Object { $_.name -like 'axonclaw-v*' } | Select-Object -First 1).name; if ($tag) { Write-Output $tag } else { exit 1 } } catch { exit 1 }"') do set "VERSION=%%a"
)

if "%VERSION%"=="" (
    echo [ERROR] Could not determine latest axonclaw release version
    exit /b 1
)

echo [INFO] Latest version: %VERSION%

:: Resolve asset download URL
set "ASSET_NAME=axonclaw_windows_amd64.zip"
echo [INFO] Resolving download URL for %VERSION% (windows/amd64^)...
set "DOWNLOAD_URL="
for /f "tokens=*" %%a in ('powershell -Command "try { $r = Invoke-RestMethod -Uri '%GITHUB_API%/releases/tags/%VERSION%' %GH_HEADERS% -UseBasicParsing; $url = ($r.assets | Where-Object { $_.name -eq '%ASSET_NAME%' } | Select-Object -First 1).browser_download_url; if ($url) { Write-Output $url } else { exit 1 } } catch { exit 1 }"') do set "DOWNLOAD_URL=%%a"

:: Fallback: construct the URL directly
if "%DOWNLOAD_URL%"=="" (
    echo [WARNING] API lookup failed, using direct download URL...
    set "DOWNLOAD_URL=https://github.com/%REPO%/releases/download/%VERSION%/%ASSET_NAME%"
)

echo [INFO] Download URL: %DOWNLOAD_URL%

:: Create temp directory
set "TEMP_DIR=%TEMP%\axonclaw-install-%RANDOM%"
mkdir "%TEMP_DIR%" 2>nul

:: Download with retries
echo [INFO] Downloading %ASSET_NAME%...
set "MAX_RETRIES=3"
set "ATTEMPT=1"

:download_retry
powershell -Command "try { Invoke-WebRequest -Uri '%DOWNLOAD_URL%' -OutFile '%TEMP_DIR%\axonclaw.zip' -UseBasicParsing } catch { exit 1 }"

if errorlevel 1 (
    if !ATTEMPT! lss %MAX_RETRIES% (
        echo [WARNING] Download attempt !ATTEMPT!/%MAX_RETRIES% failed, retrying...
        set /a ATTEMPT+=1
        timeout /t 2 /nobreak >nul
        goto :download_retry
    )
    echo [ERROR] Failed to download after %MAX_RETRIES% attempts
    rmdir /s /q "%TEMP_DIR%" 2>nul
    exit /b 1
)

echo [SUCCESS] Download completed

:: Extract
echo [INFO] Extracting to %INSTALL_DIR%...
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"

powershell -Command "try { Expand-Archive -Path '%TEMP_DIR%\axonclaw.zip' -DestinationPath '%INSTALL_DIR%' -Force } catch { exit 1 }"

if errorlevel 1 (
    echo [ERROR] Failed to extract archive
    rmdir /s /q "%TEMP_DIR%" 2>nul
    exit /b 1
)

:: Cleanup temp
rmdir /s /q "%TEMP_DIR%" 2>nul

:: Rename platform-specific binary to generic name
if exist "%INSTALL_DIR%\axonclaw_windows_amd64.exe" (
    move /y "%INSTALL_DIR%\axonclaw_windows_amd64.exe" "%INSTALL_DIR%\%BINARY_NAME%.exe" >nul
    echo [INFO] Renamed binary to %BINARY_NAME%.exe
)

echo [SUCCESS] Extraction completed

echo [SUCCESS] AxonClaw %VERSION% installed successfully to %INSTALL_DIR%

:: Check environment variables
if defined AXONCLAW_NAME goto :start
if defined AXONCLAW_BASE_URL goto :start
if defined AXONCLAW_API_KEY goto :start
goto :no_start

:start
echo [INFO] Starting axonclaw...
cd /d "%INSTALL_DIR%"
if exist "start.bat" (
    call start.bat
) else (
    echo [WARNING] start.bat not found, axonclaw not started automatically
    echo [INFO] To start manually, run: cd %INSTALL_DIR% ^&^& start.bat
)
goto :end

:no_start
echo [INFO] Environment variables not set, skipping auto-start
echo [INFO] To start axonclaw manually:
echo [INFO]   cd %INSTALL_DIR% ^&^& start.bat
echo [INFO]
echo [INFO] Or with environment variables:
echo [INFO]   set AXONCLAW_NAME=my-agent
echo [INFO]   set AXONCLAW_BASE_URL=http://localhost:8090
echo [INFO]   set AXONCLAW_API_KEY=your-key
echo [INFO]   start.bat

:end
endlocal
