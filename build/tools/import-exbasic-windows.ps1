param(
    [Parameter(Mandatory = $true)][string]$Root,
    [Parameter(Mandatory = $true)][string]$Source
)

$built = Join-Path $Source 'build\compiler'
if (-not (Test-Path -LiteralPath (Join-Path $built 'exbasic.exe'))) {
    throw "The rebuilt compiler was not found at $built"
}
$target = Join-Path $Root 'third_party\exbasic\windows-amd64\compiler'
New-Item -ItemType Directory -Force -Path $target | Out-Null
Copy-Item -Path (Join-Path $built '*') -Destination $target -Recurse -Force
