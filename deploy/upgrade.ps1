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
$Repo = 'looplj/axonhub'
$Api = "https://api.github.com/repos/$Repo"

$IncludeBeta = $false
$IncludeRC   = $false
$VerboseFlag = $false
$Force       = $false
$Yes         = $false

function Show-Usage {
  Write-Host @'
AxonHub Upgrade Script (Windows)

Usage: upgrade.bat [options]

Options:
  -y, --yes        Skip confirmation prompt and upgrade automatically
  -f, --force      Force upgrade even if already on latest version
  -b, --beta       Consider beta pre-releases when checking for updates
  -r, --rc         Consider release-candidate pre-releases when checking for updates
  -v, --verbose    Print extra debug logs
  -h, --help       Show this help message

Examples:
  upgrade.bat              Check and prompt for upgrade
  upgrade.bat -y           Upgrade without confirmation
  upgrade.bat --beta       Check beta releases
'@
}

foreach($a in $ArgsFromCmd){
  switch -Regex ($a){
    '^(--yes|-y)$'    { $Yes = $true; continue }
    '^(--force|-f)$'  { $Force = $true; continue }
    '^(--beta|-b)$'   { $IncludeBeta = $true; continue }
    '^(--rc|-r)$'     { $IncludeRC = $true; continue }
    '^(--verbose|-v)$'{ $VerboseFlag = $true; continue }
    '^(--help|-h)$'   { Show-Usage; exit 0 }
    default           { Write-Warn "Unknown option: $a" }
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
    'User-Agent'='axonhub-upgrader'
  }
  if($env:GITHUB_TOKEN){ $headers['Authorization'] = "Bearer $($env:GITHUB_TOKEN)" }
  return Invoke-RestMethod -Method GET -Uri $url -Headers $headers -ErrorAction Stop
}

function Get-LatestReleaseTag {
  try {
    $json = Invoke-GHApi "$Api/releases/latest"
    return $json.tag_name
  } catch {
    Write-Warn "API failed or rate-limited, falling back to HTML redirect..."
    try {
      $resp = Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest" -Headers @{ 'User-Agent'='axonhub-upgrader' } -MaximumRedirection 0 -ErrorAction Stop
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
    return ($filtered | Select-Object -First 1).tag_name
  } catch {
    Write-Warn 'Failed to fetch releases; falling back to latest stable.'
    return Get-LatestReleaseTag
  }
}

function Get-AssetUrl([string]$version,[string]$platform){
  Write-Info "Resolving asset for $version ($platform) ..."
  try {
    $json = Invoke-GHApi "$Api/releases/tags/$version"
    $asset = $json.assets | Where-Object { $_.browser_download_url -match $platform -and $_.browser_download_url -like '*.zip' } | Select-Object -First 1
    if($asset){ return $asset.browser_download_url }
  } catch {}
  $clean = $version.TrimStart('v')
  $file = "axonhub_${clean}_${platform}.zip"
  $candidate = "https://github.com/$Repo/releases/download/$version/$file"
  try {
    $null = Invoke-WebRequest -Uri $candidate -Method Head -ErrorAction Stop
    return $candidate
  } catch {
    Write-Err "Could not find asset for platform $platform in release $version"
    exit 1
  }
}

function Get-CurrentVersion([string]$binary){
  if(Test-Path $binary){
    try {
      $output = & $binary version 2>$null
      if($output){
        return ($output -split "`n")[0].Trim()
      }
    } catch {}
  }
  return ''
}

function Compare-Version([string]$a,[string]$b){
  $aNorm = $a.TrimStart('v')
  $bNorm = $b.TrimStart('v')
  $aParts = $aNorm -split '[.\-]'
  $bParts = $bNorm -split '[.\-]'
  $maxLen = [Math]::Max($aParts.Length, $bParts.Length)
  for($i=0; $i -lt $maxLen; $i++){
    $aVal = if($i -lt $aParts.Length){ $aParts[$i] } else { '0' }
    $bVal = if($i -lt $bParts.Length){ $bParts[$i] } else { '0' }
    $aNum = 0; $bNum = 0
    [int]::TryParse($aVal, [ref]$aNum) | Out-Null
    [int]::TryParse($bVal, [ref]$bNum) | Out-Null
    if($aNum -lt $bNum){ return -1 }
    if($aNum -gt $bNum){ return 1 }
  }
  return 0
}

function Ensure-Dirs([string]$path){ if(-not (Test-Path $path)){ New-Item -ItemType Directory -Force -Path $path | Out-Null } }

Write-Info 'Checking for AxonHub updates...'

$BaseDir = Join-Path $env:LOCALAPPDATA 'AxonHub'
$BinaryPath = Join-Path $BaseDir 'axonhub.exe'

if(-not (Test-Path $BinaryPath)){
  Write-Err "AxonHub is not installed at $BinaryPath"
  Write-Info 'Please run install.bat first'
  exit 1
}

$CurrentVersion = Get-CurrentVersion $BinaryPath
if(-not $CurrentVersion){
  Write-Warn 'Could not determine current version'
  $CurrentVersion = 'unknown'
} else {
  Write-Info "Current version: $CurrentVersion"
}

$LatestVersion = Get-LatestVersion $IncludeBeta $IncludeRC
Write-Info "Latest version: $LatestVersion"

$NeedsUpgrade = $false
if($CurrentVersion -eq 'unknown'){
  $NeedsUpgrade = $true
} else {
  $cmp = Compare-Version $CurrentVersion $LatestVersion
  if($cmp -lt 0){ $NeedsUpgrade = $true }
}

if(-not $NeedsUpgrade -and -not $Force){
  Write-Success "AxonHub is already up to date ($CurrentVersion)"
  exit 0
}

if($Force -and $CurrentVersion -ne 'unknown'){
  Write-Warn "Force upgrade requested (current: $CurrentVersion, target: $LatestVersion)"
}

if(-not $Yes){
  $reply = Read-Host "Upgrade AxonHub from $CurrentVersion to $LatestVersion? [y/N]"
  if($reply -notmatch '^[Yy]$'){
    Write-Info 'Upgrade cancelled'
    exit 0
  }
}

$Platform = Get-Platform
Write-Info "Detected platform: $Platform"

$AssetUrl = Get-AssetUrl $LatestVersion $Platform
$TempDir = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())) -Force
$ZipPath = Join-Path $TempDir 'axonhub.zip'

Write-Info "Downloading: $AssetUrl"
Invoke-WebRequest -Uri $AssetUrl -OutFile $ZipPath -UseBasicParsing

Write-Info 'Extracting archive...'
Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

$NewBinary = Get-ChildItem -Path $TempDir -Recurse -Filter 'axonhub.exe' -File | Select-Object -First 1 | ForEach-Object { $_.FullName }
if(-not $NewBinary){ Write-Err 'axonhub.exe not found in archive'; exit 1 }

Write-Info 'Installing new binary...'
Copy-Item -Path $NewBinary -Destination $BinaryPath -Force

Write-Success "AxonHub upgraded to $LatestVersion"

Write-Info 'Restarting AxonHub...'
$restartScript = Join-Path $ScriptDir 'restart.ps1'
if(Test-Path $restartScript){
  & $restartScript
} else {
  Write-Warn 'restart.ps1 not found, please restart AxonHub manually'
}

Write-Success 'Upgrade completed!'
