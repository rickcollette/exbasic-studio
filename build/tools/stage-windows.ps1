param(
    [Parameter(Mandatory = $true)][string]$Root
)

$tools = Join-Path $Root 'bin\tools'
$compiler = Join-Path $Root 'third_party\exbasic\windows-amd64\compiler'
New-Item -ItemType Directory -Force -Path (Join-Path $tools 'exbasic') | Out-Null
Copy-Item -LiteralPath (Join-Path $compiler 'asminc') -Destination (Join-Path $tools 'exbasic') -Recurse -Force
Copy-Item -LiteralPath (Join-Path $compiler 'manual') -Destination (Join-Path $tools 'exbasic') -Recurse -Force
Copy-Item -LiteralPath (Join-Path $compiler 'syntax') -Destination (Join-Path $tools 'exbasic') -Recurse -Force
Get-ChildItem -LiteralPath $compiler -File | Copy-Item -Destination (Join-Path $tools 'exbasic') -Force
Copy-Item -LiteralPath (Join-Path $Root 'bin\atari800-studio.exe') -Destination (Join-Path $tools 'atari800-studio.exe') -Force
Copy-Item -LiteralPath (Join-Path $Root 'third_party\EOS.car') -Destination (Join-Path $tools 'EOS.car') -Force
