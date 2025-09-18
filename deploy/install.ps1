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

$Repo = 'looplj/axonhub'
$Api = "https://api.github.com/repos/$Repo"

$IncludeBeta = $false
$IncludeRC   = $false
$VerboseFlag = $false
$Version     = $env:AXONHUB_VERSION

function Show-Usage {
  Write-Host @'
AxonHub Installer (Windows)

Usage:
  install.bat [options] [version]

Options:
  -b, --beta       Consider beta pre-releases
  -r, --rc         Consider release-candidate pre-releases
  -v, --verbose    Print extra debug logs
  -h, --help       Show this help and exit
'@
}

# Parse args
foreach($a in $ArgsFromCmd){
  switch -Regex ($a){
    '^(--beta|-b)$'    { $IncludeBeta = $true; continue }
    '^(--rc|-r)$'      { $IncludeRC   = $true; continue }
    '^(--verbose|-v)$' { $VerboseFlag = $true; continue }
    '^(--help|-h)$'    { Show-Usage; exit 0 }
    default            { if(-not $Version){ $Version = $a } else { Write-Warn "Ignoring extra argument: $a" } }
  }
}

function Get-Platform {
  $archEnv = $env:PROCESSOR_ARCHITECTURE
  switch ($archEnv.ToLower()){
    'amd64' { return 'windows_amd64' }
    'arm64' { return 'windows_arm64' }
    default { Write-Err "Unsupported architecture: $archEnv"; exit 1 }
  }
}

function Invoke-GHApi([string]$url){
  $headers = @{
    'Accept'='application/vnd.github+json'
    'X-GitHub-Api-Version'='2022-11-28'
    'User-Agent'='axonhub-installer'
  }
  if($env:GITHUB_TOKEN){ $headers['Authorization'] = "Bearer $($env:GITHUB_TOKEN)" }
  return Invoke-RestMethod -Method GET -Uri $url -Headers $headers -ErrorAction Stop
}

function Get-LatestReleaseTag {
  try {
    $json = Invoke-GHApi "$Api/releases/latest"
    return $json.tag_name
  } catch {
    # Fallback to HTML redirect (best-effort)
    Write-Warn "API failed or rate-limited, falling back to HTML redirect..."
    try {
      $resp = Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest" -Headers @{ 'User-Agent'='axonhub-installer' } -MaximumRedirection 0 -ErrorAction Stop
    } catch { $resp = $_.Exception.Response }
    if($resp -and $resp.Headers['Location']){
      $loc = $resp.Headers['Location']
      if($loc -match '/tag/([^/]+)$'){ return $matches[1] }
    }
    Write-Err "Could not determine latest release version"
    exit 1
  }
}

function Get-LatestVersion([bool]$includeBeta,[bool]$includeRC){
  if(-not $includeBeta -and -not $includeRC){ return Get-LatestReleaseTag }
  Write-Info "Fetching releases (beta=$includeBeta, rc=$includeRC) ..."
  try {
    $rels = Invoke-GHApi "$Api/releases?per_page=100"
    $filtered = $rels | Where-Object { -not $_.draft }
    if($includeBeta -and $includeRC){
      $filtered = $filtered | Where-Object { $_.tag_name -match '-beta' -or $_.tag_name -match '-rc' }
    } elseif($includeBeta){
      $filtered = $filtered | Where-Object { $_.tag_name -match '-beta' }
    } else {
      $filtered = $filtered | Where-Object { $_.tag_name -match '-rc' }
    }
    if(-not $filtered){
      Write-Warn 'No matching pre-release found; falling back to latest stable.'
      return Get-LatestReleaseTag
    }
    # GitHub API returns releases in reverse chronological order; take the first
    return ($filtered | Select-Object -First 1).tag_name
  } catch {
    Write-Warn 'Failed to fetch releases; falling back to latest stable.'
    return Get-LatestReleaseTag
  }
}

function Get-AssetUrl([string]$version,[string]$platform){
  Write-Info "Resolving asset for $version ($platform) ..."
  try {
    $tag = $version
    $json = Invoke-GHApi "$Api/releases/tags/$tag"
    $asset = $json.assets | Where-Object { $_.browser_download_url -match $platform -and $_.browser_download_url -like '*.zip' } | Select-Object -First 1
    if($asset){ return $asset.browser_download_url }
  } catch {}
  # Fallback by pattern
  $clean = $version.TrimStart('v')
  $file = "axonhub_${clean}_${platform}.zip"
  $candidate = "https://github.com/$Repo/releases/download/$version/$file"
  try {
    $head = Invoke-WebRequest -Uri $candidate -Method Head -ErrorAction Stop
    return $candidate
  } catch {
    Write-Err "Could not find asset for platform $platform in release $version"
    exit 1
  }
}

function Ensure-Dirs([string]$path){ if(-not (Test-Path $path)){ New-Item -ItemType Directory -Force -Path $path | Out-Null } }

# Main
Write-Info 'Starting AxonHub installation...'

$Platform = Get-Platform
Write-Info "Detected platform: $Platform"

if(-not $Version){ $Version = Get-LatestVersion $IncludeBeta $IncludeRC }
Write-Info "Using version: $Version"

$BaseDir   = Join-Path $env:LOCALAPPDATA 'AxonHub'
$ConfigDir = $BaseDir
$DataDir   = $BaseDir
$LogDir    = $BaseDir
Ensure-Dirs $BaseDir
Ensure-Dirs (Join-Path $BaseDir 'logs')

$AssetUrl = Get-AssetUrl $Version $Platform
$TempDir = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())) -Force
$ZipPath = Join-Path $TempDir 'axonhub.zip'
Write-Info "Downloading: $AssetUrl"
Invoke-WebRequest -Uri $AssetUrl -OutFile $ZipPath -UseBasicParsing

Write-Info 'Extracting archive...'
Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

$BinaryPath = Get-ChildItem -Path $TempDir -Recurse -Filter 'axonhub.exe' -File | Select-Object -First 1 | ForEach-Object { $_.FullName }
if(-not $BinaryPath){ Write-Err 'axonhub.exe not found in archive'; exit 1 }

$TargetBinary = Join-Path $BaseDir 'axonhub.exe'
Copy-Item -Path $BinaryPath -Destination $TargetBinary -Force

# Create default config if missing
$ConfigFile = Join-Path $BaseDir 'config.yml'
if(-not (Test-Path $ConfigFile)){
  Write-Info 'Creating default configuration...'
  $baseForDSN = ($BaseDir -replace '\\','/')
  @"
server:
  port: 8090
  name: "AxonHub"
  debug: false

db:
  dialect: "sqlite3"
  dsn: "$baseForDSN/axonhub.db?cache=shared&_fk=1&journal_mode=WAL"

log:
  level: "info"
  encoding: "json"
  output: "file"
  file:
    path: "$baseForDSN/logs/axonhub.log"
    max_size: 100
    max_age: 30
    max_backups: 10
    local_time: true
"@ | Set-Content -Path $ConfigFile -Encoding UTF8
}

Write-Success 'AxonHub installation completed!'
Write-Info "Next steps:"
Write-Host "  1. Edit configuration: $ConfigFile"
Write-Host "  2. Start AxonHub: start.bat"
Write-Host "  3. Stop AxonHub: stop.bat"
Write-Host "  4. View logs: $BaseDir\\axonhub.log (or logs\\axonhub.log in config)"
Write-Host "  5. Access web interface: http://localhost:8090"
