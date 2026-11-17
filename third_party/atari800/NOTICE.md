# Atari800 integration notice

This directory contains the official Atari800 7.2.1 source release and the
small `studio-helper` program that exposes `libatari800` frames to exBASIC
Studio over a local standard-I/O protocol.

- Project: Atari800
- Version: 7.2.1 (`ATARI800_7_2_1`)
- Source: https://github.com/atari800/atari800
- License: GNU General Public License, version 2 or later

The complete upstream license is in `atari800-7.2.1/COPYING`. The helper links
to `libatari800` and is distributed under the same license. It remains a
separate executable; the MIT-licensed Studio application does not link to it.

The Studio fork adds a 256K Rambo configuration. Its bank-selection behavior
was implemented with Altirra 4.40's GPLv2 source and hardware documentation as
the compatibility reference:

- Project: Altirra, by Avery Lee
- Source: https://www.virtualdub.org/altirra.html
- Version consulted: 4.40
- License: GNU General Public License, version 2

Changes made for exBASIC Studio 1.0:

- `configure.ac` and `configure` allow `--enable-monitorbreak` for the
  `libatari800` target, so the CPU core's code-breakpoint support is
  available to the helper's debugger. Studio builds the core with it on every
  platform.
- `studio-helper/main.c` speaks protocol version 2: a capability handshake,
  bounded newline-delimited JSON requests, and debugger operations (pause,
  continue, instruction and line stepping, run-until, breakpoint tables, CPU
  state and bounded memory reads). It provides `PLATFORM_Exit`, the hook that
  `libatari800` leaves to the embedding program.

The source tree contains no build output: Studio builds the core out of tree
(`build/tools/build-toolchain.sh`).
