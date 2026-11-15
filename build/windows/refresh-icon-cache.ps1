param(
    [Parameter(Mandatory = $true)]
    [string]$Path
)

$ErrorActionPreference = 'Stop'
$resolved = (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path

Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;

public static class ExBasicShellIconRefresh
{
    [DllImport("shell32.dll", CharSet = CharSet.Unicode, EntryPoint = "SHChangeNotify")]
    public static extern void UpdateItem(uint eventId, uint flags, string item1, IntPtr item2);

    [DllImport("shell32.dll", EntryPoint = "SHChangeNotify")]
    public static extern void RefreshAssociations(uint eventId, uint flags, IntPtr item1, IntPtr item2);
}
'@

# SHCNE_UPDATEITEM with SHCNF_PATHW refreshes this executable. The association
# notification makes already-open Explorer windows discard stale icon entries.
[ExBasicShellIconRefresh]::UpdateItem(0x00002000, 0x0005, $resolved, [IntPtr]::Zero)
[ExBasicShellIconRefresh]::RefreshAssociations(0x08000000, 0, [IntPtr]::Zero, [IntPtr]::Zero)

$iconInitialiser = Join-Path $env:SystemRoot 'System32\ie4uinit.exe'
if (Test-Path -LiteralPath $iconInitialiser -PathType Leaf) {
    Start-Process -FilePath $iconInitialiser -ArgumentList '-show' -WindowStyle Hidden -Wait
}
