# Supported platforms

| Platform | Architectures | Artifacts | Minimum | Runtime dependencies |
|----------|---------------|-----------|---------|----------------------|
| Windows | x64 | signed NSIS installer (`-setup.exe`), portable ZIP | Windows 10 version 1809 | Microsoft Edge WebView2 Runtime (preinstalled on Windows 11; the installer adds it when missing) |
| macOS | universal (Intel and Apple silicon) | signed, notarized DMG | macOS 12 | none |
| Linux | x64, arm64 | AppImage, DEB, RPM | glibc-based distribution with GTK 4 and WebKitGTK 6.0 (for example Ubuntu 24.04, Debian 13, Fedora 40) | DEB: `libgtk-4-1`, `libwebkitgtk-6.0-4`; RPM: `gtk4`, `webkitgtk6.0` |

Notes:

- Windows 10 before version 1809 lacks the pseudo-console API used by the
  integrated terminal.
- The portable ZIP makes no registry changes. It needs the WebView2 Runtime
  to be present.
- The AppImage needs FUSE 2 (`libfuse2`) or can be run with
  `--appimage-extract-and-run`.
- Source control features use the `git` program found on `PATH`. Everything
  else works without Git.
- Not supported in 1.0: Windows on ARM, 32-bit systems, iOS, Android.

## What is bundled

Every artifact contains the exBASIC compiler distribution, the ex800
helper, mkatr-go and the EOS cartridge, built from the sources in this
repository for that platform, with their licenses in `tools/licenses/`.

| Platform | Tool location |
|----------|---------------|
| Windows | `tools\` beside `exbasic-studio.exe` |
| macOS | `exBASIC Studio.app/Contents/Resources/tools` |
| Linux packages | `/usr/lib/exbasic-studio/tools` |
| AppImage | `usr/lib/exbasic-studio/tools` inside the image |

## Verifying an installation

```
exbasic-studio --self-test
```

runs without opening a window. It locates the bundled tools, compiles and
boots a sample, stops on a source breakpoint, packages an ATR image and runs
a terminal command. It prints `PASS` and exits with 0 on success. On Windows
the application has no console; pass a file name to get the report:
`exbasic-studio.exe --self-test report.txt`.

## Where data is kept

| Platform | Directory |
|----------|-----------|
| Windows | `%AppData%\exbasic-studio` |
| macOS | `~/Library/Application Support/exbasic-studio` |
| Linux | `$XDG_CONFIG_HOME/exbasic-studio` (default `~/.config/exbasic-studio`) |

Uninstalling the application leaves this directory in place; delete it to
remove recent projects, layout, recovery copies and logs.
