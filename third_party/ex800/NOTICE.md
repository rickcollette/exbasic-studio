# ex800 integration notice

This directory contains ex800 1.0.0 and the small `studio-helper` program that
exposes emulator frames to exBASIC Studio over a local standard-I/O protocol.
ex800 is based on the official Atari800 7.2.1 source release.

- Project: ex800
- Version: 1.0.0
- Upstream: Atari800 7.2.1 (https://github.com/atari800/atari800)
- License: GNU General Public License, version 2 or later

The complete license is in `ex800-1.0.0/COPYING`. The helper links to the
retained `libatari800` compatibility API and is distributed under the same
license. It remains a separate executable; the MIT-licensed Studio application
does not link to it.

The Studio fork adds a 256K Rambo configuration. Its bank-selection behavior
was implemented with Altirra 4.40's GPLv2 source and hardware documentation as
the compatibility reference:

- Project: Altirra, by Avery Lee
- Source: https://www.virtualdub.org/altirra.html
- Version consulted: 4.40
- License: GNU General Public License, version 2

Changes made for ex800 1.0.0 and exBASIC Studio:

- Atari 5200 emulation, BIOS selection, controller input, cartridge handlers
  and bundled replacement BIOS assets were removed.
- The user-facing executable, package, manual and configuration names are
  `ex800`. Internal `Atari800_*` symbols and the `libatari800` API remain for
  source and Studio-helper compatibility.

- `configure.ac` and `configure` allow `--enable-monitorbreak` for the
  `libatari800` target, so the CPU core's code-breakpoint support is
  available to the helper's debugger. Studio builds the core with it on every
  platform.
- `studio-helper/main.c` speaks protocol version 2: a capability handshake,
  bounded newline-delimited JSON requests, and debugger operations (pause,
  continue, instruction and line stepping, run-until, breakpoint tables, CPU
  state and bounded memory reads). It provides `PLATFORM_Exit`, the hook that
  `libatari800` leaves to the embedding program.

Builds remain out of tree. `build/tools/build-release.sh` creates native Linux
x86_64, Windows x86_64, macOS arm64 and macOS x86_64 release archives; the
GitHub Actions workflow builds the same four-target matrix.
