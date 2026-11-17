# Creates the portable ZIP for Windows: the application, the bundled tools
# (in the "tools" folder the application looks for beside its executable)
# and the license notices. No installer and no registry changes.
param(
    [Parameter(Mandatory = $true)][string]$Root,
    [Parameter(Mandatory = $true)][string]$Version,
    [string]$Arch = 'amd64'
)
$ErrorActionPreference = 'Stop'

$exe = Join-Path $Root 'bin\exbasic-studio.exe'
$tools = Join-Path $Root "build\stage\windows-$Arch\tools"
foreach ($required in @($exe, $tools, (Join-Path $tools 'exbasic\exbasic.exe'), (Join-Path $tools 'atari800-studio.exe'), (Join-Path $tools 'mkatr-go.exe'))) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Missing $required. Run 'task build' first." }
}

$name = "exbasic-studio-$Version-windows-$Arch-portable"
$work = Join-Path $Root "bin\$name"
if (Test-Path -LiteralPath $work) { Remove-Item -LiteralPath $work -Recurse -Force }
New-Item -ItemType Directory -Path $work | Out-Null
Copy-Item -LiteralPath $exe -Destination $work
Copy-Item -LiteralPath $tools -Destination (Join-Path $work 'tools') -Recurse
Copy-Item -LiteralPath (Join-Path $Root 'LICENSE') -Destination $work
Copy-Item -LiteralPath (Join-Path $Root 'THIRD_PARTY_NOTICES.md') -Destination $work

$zip = Join-Path $Root "bin\$name.zip"
if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }
Compress-Archive -Path (Join-Path $work '*') -DestinationPath $zip -CompressionLevel Optimal
Remove-Item -LiteralPath $work -Recurse -Force
Write-Output "Portable archive: $zip"