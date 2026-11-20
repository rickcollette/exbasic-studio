# Third-party notices

Original exBASIC Studio material uses the custom Flesh and Bone License (see
`LICENSE`).

It ships with, and runs as **separate processes**, the programs below. They
are not linked into the Studio executable. Each is distributed under its own
license, and the complete license text is installed beside the tools in
`tools/licenses/`.

| Component | Version | License | Source in this repository | Shipped as |
|-----------|---------|---------|---------------------------|------------|
| exBASIC compiler, with the CC65 assembler/linker/archiver (`ca65`, `ld65`, `ar65`) and `mkatr`/`lsatr` | 1.0.0 + Studio patch | GPL-2.0-or-later, with the FastBasic runtime exception | `third_party/exbasic/source` | `tools/exbasic/` |
| ex800 emulator core (`libatari800`) and the `ex800-studio` helper that links it | 1.0.0 | GPL-2.0-or-later | `third_party/ex800/ex800-1.0.0`, `third_party/ex800/studio-helper` | `tools/ex800-studio` |
| mkatr-go (Go port of dmsc/mkatr, with EOSFS support) | `2d1f341c` | GPL-2.0-only | `third_party/mkatrgo`, `cmd/mkatr-go` | `tools/mkatr-go` |
| ExtendedOS source and source-built EOS cartridge image | `983a8b0e` | MIT; see the vendored license | `third_party/exoperatingsystem` | `tools/EOS.car` |

ex800 is based on Atari800 7.2.1, removes Atari 5200 support, adds a 256K
Rambo memory configuration (implemented with Altirra 4.40, GPL-2.0, as the
compatibility reference), and enables the core's code-breakpoint support for
the library target; see `third_party/ex800/NOTICE.md`. The exBASIC compiler is based on FastBasic
by Daniel Serpell; see `third_party/exbasic/NOTICE.md`.

The corresponding source for every GPL component is the pinned source tree
in this repository at the tag of the release you are using; the release
build uses exactly those trees (`build/tools/build-toolchain.sh`).
The exact upstream commits and local portability adjustments are recorded in
`third_party/VENDORED.md`.

The release build uses the vendored MADS 2.1.7 assembler in
`third_party/mads` to produce `EOS.car` from the ExtendedOS source. MADS is an
MIT-licensed build-time tool and is not shipped in the application packages.

## Libraries compiled into exBASIC Studio

The Studio executable and its user interface include open-source libraries
under permissive licenses (MIT, BSD, Apache-2.0, ISC and similar), among them
Wails, CodeMirror, Lezer, xterm.js, Svelte, marked, DOMPurify, the VS Code
codicons font (CC-BY-4.0), fsnotify and creack/pty.

The exact list with versions and license identifiers for a given release is
in the software bill of materials published with it (`*.spdx.json`), which is
generated from `go.mod` and `frontend/package-lock.json`.
