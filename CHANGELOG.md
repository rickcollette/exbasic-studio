# Changelog

All changes to exBASIC Studio are here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/).

Fix-only releases retain their product version and increment `BUILD_NUMBER`.
Feature releases increment the minor version and reset `BUILD_NUMBER` to 1.

## [Unreleased]

## [1.0.0]

### Changed

- Replaced the bundled Atari800 7.2.1 emulator with the vendored ex800 1.0.0
  core and `ex800-studio` helper. Existing Atari800 project-setting names
  remain compatibility aliases. Atari 5200 profiles are no longer available,
  matching ex800's supported machines.
- The EOS cartridge is now assembled during the toolchain build from the
  vendored ExtendedOS source with vendored MADS 2.1.7, instead of copying a
  checked-in `EOS.car` image.

### Added

- **Claude Code integration.** Claude runs in an integrated terminal using the
  user's existing local Claude login; exBASIC Studio does not read or store
  credentials.
- **Sprite Studio.** A new activity-bar view and editor for player/missile
  sprites. Draw frames with pencil, eraser, fill, line and rectangle tools,
  with undo, onion skin, a per-plane view and an animation preview. Each
  palette colour lists the player planes it sets, so multi-player sprites
  (and their GTIA overlap colours) are edited as one image.
- **Sprite sheets** (`*.sprites.json`) hold the frames, palette, planes and
  output files, or point at a plain frames JSON so existing scripts that
  read it keep working. *Generate* writes the PMG frame bank, an index of
  frame and plane offsets (per-plane or simple layout), an exBASIC include
  with the bank as `DATA` statements, and a PNG atlas.
- **Sprite import and merge.** PMG banks open through the index beside them;
  PNGs are cut into frames and mapped to the palette; *Merge* combines
  sheets, frame files and banks into one sheet, unifying sizes, planes and
  palettes. Sprite files open in the Studio from the Explorer, and *Open
  Sheet as JSON Text* switches to the text editor.
- The backend gained `ReadBinaryFile` and `SaveBinaryFile` (base64, at most
  8 MiB, inside the workspace only) for these binary assets.

### Changed

- The project now uses the custom Flesh and Bone License instead of MIT. Read
  `LICENSE` before redistributing a modified version.

### Fixed

- The Keyboard panel now refreshes its held-key visualization reliably while
  emulator keys are pressed and released.
- Windows-origin source snapshots preserve the ex800 Autotools line endings
  and executable permissions required by Linux and macOS release builds.

## [1.0.0-rc.2]

### Added

- **Keyboard joystick.** The joystick button in the emulator toolbar (or
  *Atari800: Toggle Keyboard Joystick*) turns Q W E / A D / Z X C, the arrow
  keys (two together give a diagonal) and the numeric keypad into joystick 0,
  with Space, Numpad 0 and Ctrl as fire. Programs read the usual `STICK(0)`
  values (15 centred, 14 up, 13 down, 11 left, 7 right, 10/6/9/5 diagonals)
  and `STRIG(0)` = 0 while fire is held. The toolbar shows the current value.
- **Keyboard panel.** A new *Keyboard* tab beside Terminal shows a PC or Mac
  keyboard with the Atari key each key produces, highlights keys as they are
  pressed on the emulator screen, and shows the joystick keys while the
  keyboard joystick is on.
- **More Atari keys.** Home is CLEAR, End is HELP, and Pause or F8 is BREAK
  (F8 still pauses while debugging).

## [1.0.0-rc.1]

The first release candidate for 1.0. Version 1.0.0 is cut only after every
platform artifact passes the checks in `docs/RELEASE_CHECKLIST.md`.

### Added

- **Source-level debugger.** Breakpoints on exBASIC lines (moved to the
  nearest executable line), continue, pause, single 6502 instruction step,
  source-line step, step over `JSR`, step out, and run to line. The stopped
  line is highlighted and opened automatically. Read-only registers, return
  stack, disassembly, symbol browser, memory viewer and watches (symbols,
  decimal/hex addresses and `symbol ± constant`, decoded with the compiler's
  type information).
- **Real terminal sessions** on a pseudo-terminal (ConPTY on Windows, PTY on
  macOS and Linux) with streaming output, resize, Ctrl+C, a persistent
  working directory, exit status and explicit shutdown.
- **Workspace trust.** A folder you open is in restricted mode: you can
  browse and edit, but building, running, the terminal, Git hooks and Git
  network actions wait for explicit trust. Changing a compiler, emulator or
  ATR tool path revokes trust.
- **Crash recovery.** Unsaved text is kept in a private per-user store and
  offered back on the next start when it differs from the file on disk.
- **Recent projects, restored tabs and layout.** Up to 20 recent projects;
  open tabs, the active editor, panel layout, emulator profile, breakpoints
  and watches are remembered per workspace, outside the project folder.
- **File watching.** Clean files changed by other programs reload; files
  with unsaved edits show a prompt; the explorer updates incrementally.
- **Language tooling.** Grammar-based highlighting generated from the
  compiler's syntax files, context-aware completion, signature help, hover
  documentation linked to the manual, block matching and folding, document
  outline, workspace symbols, `INCLUDE` path completion and go to
  definition for functions, variables, arrays and included files.
- **Structured builds.** Builds stream their output, can be cancelled, and
  return structured diagnostics shown in the editor and the Problems panel.
- **Git.** Branch create/switch/delete, remote add/edit/remove, explicit
  fetch/pull/push with progress and cancellation, and merge conflict
  resolution with a base/current/incoming view.
- **Manual.** Rendered as sanitized Markdown with anchors, tables, cross
  links, search highlighting and back/forward navigation.
- **Accessibility.** Full keyboard operation of menus, dialogs, trees, tabs,
  the debugger and the terminal; a high-contrast theme; visible focus.
- Diagnostic logs kept locally with rotation, and an exportable support
  bundle with file paths redacted.
- `exbasic-studio --self-test`, which verifies an installed copy without
  opening a window.
- Packages: Windows installer and portable ZIP, macOS universal DMG, Linux
  AppImage, DEB and RPM (x64 and arm64), with checksums, SBOM and build
  provenance.

### Added (since the first RC build)

- **Cartridge slots.** Each emulator profile has a primary and a secondary
  cartridge slot: empty, Atari BASIC, the included EOS cartridge, a
  SpartaDOS X image or any other cartridge image, with a type choice for raw
  images. Profiles that used the old EOS and BASIC switches are converted.
- **H: folder choice.** Each emulator profile can name the folder the Atari
  sees as H: (D8: under EOS). Left empty, it is the H folder of the project
  as before; a path without a drive is taken from the project folder.
- **Application settings.** Theme (dark, light, high contrast or follow the
  system), four syntax colour schemes, editor font size, automatic saving
  (after a pause or when focus is lost) and startup behaviour (reopen the
  last folder or show the welcome page).

### Fixed

- Opening a very large folder (a home directory, a drive) no longer scans it
  for minutes without feedback and then redirects saves, trust and runs to
  that folder while the window still shows the previous one. The open now
  fails at once with an explanation and the previous folder stays open.
- Saving retries briefly when another program holds the file for a moment.
- Rename and Delete are on the selected Explorer row.
- The side-bar Open Folder button is gone; the folder icon opens the system
  folder picker and opens the folder you choose.
- Include files (.inc) get highlighting, completion and outline. A problem
  inside an included file is shown in that file, and the INCLUDE line that
  pulls it in names the file and line.
- Tab accepts the highlighted suggestion. Block templates (FUNC, IF, FOR,
  WHILE, REPEAT, DO) are offered at the start of a statement, and a FUNC
  without parentheses has an Add () fix.
- Brighter, higher-contrast syntax colours.
- Live checking uses the unsaved text of other open files, so fixing an
  include clears its error before you save, and editing an include re-checks
  the open programs. Checking no longer writes temporary files into the
  project.
- One-click fixes for mechanical compiler errors (a missing ENDFUNC, NEXT,
  WEND, LOOP or ENDIF, a stray closing statement, an unclosed parenthesis or
  string, an undefined FUNC), also on Ctrl+. for the cursor's line.
- The ATR views use a folder icon with the Fuji mark.
- Run and Debug with an EOS profile: the program is put on an EOS disk in
  D1: and started at the EOS prompt. Before, EOS was handed the bare
  executable and reported "148 Unknown filesystem".
- Build output shows each compiler progress line once instead of once per
  optimisation attempt.
- Selected text is visible on the current line; other occurrences of the
  selection are outlined instead of filled.

### Changed

- `exbasic.project.json` now carries `schemaVersion: 1`. Existing projects
  open unchanged and are rewritten only when you save settings; see
  `docs/PROJECT-CONFIG.md`.
- The bundled exBASIC compiler gained `--capabilities=json` and
  `--metadata-json <file>`. Other compilers still build; completion metadata
  and source debugging are then unavailable, with an explanation.
- The Atari800 helper speaks protocol version 2 (handshake, debugger
  commands, bounded requests) and is built with the emulator's
  code-breakpoint support.
- Git status uses the porcelain v2 format, so renames and unusual file
  names are handled correctly. Diffs never run external diff or textconv
  programs.
- The project `H` folder is created on the first trusted run, not when a
  folder is opened.
- Bundled tools are located in the application's resources. The
  `EXBASIC_STUDIO_TOOLS` variable remains as a development and test override.
- All tools are built from the pinned sources in `third_party/` by
  `build/tools/build-toolchain.sh`.
- Continuous integration moved to GitHub Actions.

### Removed

- SSH remote workspaces.
- The extensions view and manifest discovery. Extensions are planned after
  1.0; see `docs/ROADMAP-EXTENSIONS.md`.
- iOS and Android build targets.
- The command-at-a-time terminal.

### Security

- File operations reject symbolic links and Windows reparse points at every
  path component, replace files atomically, refuse non-text content in the
  editor and cap imports and searches.
- External programs are always started with an argument vector, never
  through a shell.
- See `docs/SECURITY.md` for the threat model and known limits.

## [0.1.0]

Alpha snapshot: single-window editor, synchronous build and run, embedded
Atari800 screen, ATR explorer and basic Git status.
