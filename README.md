# exBASIC Studio

A lean, purpose-built desktop IDE scaffold for exBASIC on Atari 8-bit systems.

**Status:** alpha. Windows builds are verified locally; macOS and Linux builds still need verification on their target platforms.

## Technology

- Go 1.26+ and **Wails 3.0.0** (native WebView2/WebKitGTK/WebKit shell)
- Svelte 5 + TypeScript + Vite + CodeMirror 6
- Rebuilt exBASIC compiler distribution (including CA65/LD65) and Atari800 7.2.1 `libatari800` core, shipped as separate GPL executables
- Built-in ATR workflow backed by the separately licensed `mkatr-go` tool, with SDFS and EOSFS support
- Git executable (external, optional)
- OpenSSH client for remote workspaces (uses the operating system's SSH configuration)
- Local Claude Code integration using the same authentication and settings as the VS Code extension
- Local Markdown handbook search (all 24 supplied manual files are embedded)
- Versioned plugin manifest discovery (execution intentionally disabled)

## Quick start

Install Go 1.26+, Node.js 22+, Git, and the platform-specific Wails dependencies.

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
wails3 doctor
cd exbasic-studio
go mod download
npm --prefix frontend install
wails3 dev
```

The `wails3 dev` command uses `build/config.yml` and the platform Taskfiles to build the Go application, start Vite, and relaunch the native process when Go code changes. **Use Wails 3.0.0**, not Wails 2.

To build a production binary for the current platform:

```sh
wails3 build
```

For a bare executable without the platform metadata and packaging steps, the underlying commands are:

```sh
wails3 generate bindings -ts
npm --prefix frontend run build
go build -tags production -o bin/exbasic-studio .
wails3 task common:build:mkatr TARGET_OS=windows TARGET_ARCH=amd64 OUTPUT=bin/mkatr-go.exe
wails3 task common:stage:tools
```

The platform Taskfiles write `exbasic-studio` and `mkatr-go` into `bin/`, then stage the complete eXBASIC distribution, the headless Atari800 bridge, and `EOS.car` under `bin/tools/`. The default Windows NSIS installer installs the same layout. Tool paths in a project remain overridable.

For CI, run `go test ./...` and `npm --prefix frontend run check` before packaging. `wails3 task check` runs both locally.

### Platform requirements

| OS | Requirements |
| --- | --- |
| Windows 10/11 | WebView2 runtime, Go 1.26+, Node 22+, installed C toolchain where required |
| macOS | Xcode Command Line Tools, Go 1.26+, Node 22+ |
| Linux modern | gcc, pkg-config, GTK4, WebKitGTK 6.0 development packages; Go 1.26+, Node 22+ |
| Linux legacy | GTK3 + WebKit2GTK 4.1 with the Wails `gtk3` build tag (availability dependent on Wails version) |

See https://wails.io/quick-start/installation/ for current platform instructions.

## Your first Atari program

1. Start the IDE. Development builds launched from `bin/` correctly suggest the repository root rather than the binary directory.
2. Enter a project name such as `helloatari`. Click **Create in location above**.
3. The IDE creates `src/main.bas`, `lib/`, `.gitignore`, and `exbasic.project.json`.
4. The generated project uses the bundled `exbasic` compiler and `atari800-studio` bridge. Existing `atari800-headless` project settings are migrated automatically. Override either path on the **Setup** tab only when testing a different toolchain.
5. Save settings, edit `src/main.bas`, press **F6** to compile or **F5** to compile and run.
6. The output pane shows the exact compiler arguments and raw compiler output.

The Windows compiler in `third_party/exbasic/windows-amd64/compiler` was rebuilt from `E:/exBasic`, and that source snapshot is retained in `third_party/exbasic/source` for reproducible Windows, macOS, and Linux builds. Run `wails3 task toolchain:rebuild:exbasic EXBASIC_SOURCE=E:/exBasic` to rebuild and import the external working tree again; on another platform, point the same task at the bundled source snapshot.

### exBASIC compiler details

The compiler consumes **one BASIC source entry file**. Other BASIC sources should be included by the source code via `INCLUDE`; optional `.asm`, `.s` and `.o` modules are passed as extra inputs. The scaffold invokes the compiler via `os/exec`, with no shell:

```sh
exbasic -t:atari -g -keep -I:/absolute/path/src -I:/absolute/path/lib -o /absolute/path/build/demo.xex /absolute/path/src/main.bas
```

This respects the manual's `-o output input.bas` syntax. If configured, assembly inputs are appended. The default `native=auto` leaves compiler choice untouched; `hot`, `all`, and `none` select the corresponding flags. Studio also runs the real parser in `-c` mode after edits and maps `file:line:column` errors into CodeMirror lint markers and the Problems panel.

Run mode starts a separate, windowless bridge linked to the official Atari800 7.2.1 `libatari800` core. Its 336×240 framebuffer appears as an **Atari800 editor tab**, accepts keyboard input, and can be captured as PNG. XEX, ATR, cartridge, and ROM artifacts are supported without opening an SDL or console window.

The emulator workspace can also be opened without a project from **Run → Open Atari800** or the command palette. It has Start, Stop, and Restart controls and persistent named hardware profiles. Profiles configure the Atari 400/800, XL, XE, and 5200 machine families; NTSC/PAL video; 64K, 128K, 256K Rambo, 320K Rambo/Compy Shop, and 1088K memory; 1×, 1.5×, 2×, 3×, and 5× execution speeds; U1MB memory banking; Atari BASIC; fast SIO; EOS; display scale; and advanced Atari800 arguments.

Each profile also has four explicit, independently enabled D1:–D4: disk slots with read-only controls, plus the normal XEX/cartridge/ROM startup media. R: can use the Atari800 850-compatible network handler or an R-Verter-compatible TCP modem at 300, 1200, 2400, 4800, 9600, or 19.2K baud. The modem is bidirectional: Atari terminal software can dial out with `ATDI host port`, while BBS software can accept calls on the configured R1: port; R2:–R4: use the next three ports. Linux builds can connect the handler to a host serial device. IDE storage uses the core's 8/16-bit D500–D50F interface with raw disk and CompactFlash identify modes. Profiles are stored in the user's platform configuration directory, independently of project files. Closing Studio stops its headless emulator child process.

For every local project, Studio creates `<project>/H` as soon as the project is opened. Atari OS profiles map it read/write as `H1:`. EOS profiles reserve `D8:` as the `H` substitute and expose the same folder through a writable native EOSFS disk, leaving configurable `D1:`–`D4:` available for normal media. EOS changes are synchronized back into `H` when Atari800 stops or restarts. Standalone sessions use the platform configuration directory's `exbasic-studio/H` folder. The emulator toolbar shows the actual mapped device.

## ATR disk images

Enable **Create ATR disk image after build** in Project Settings to package the compiled XEX automatically. The output path, minimum or exact image sizing, bootloader page, and additional project files are configurable. A successful Run mounts the resulting ATR in Atari800 instead of passing the XEX directly.

The default **SDFS / SpartaDOS** mode creates a SpartaDOS/BW-DOS-compatible image and can mark the compiled application as the boot file. **EOSFS** creates the ExtendedOS native revision-1 format, including extent lists and per-directory name indexes. EOSFS images deliberately disable the SDFS boot-file flag because that bootloader does not understand EOS extents. Run automatically inserts `third_party/EOS.car` and mounts the EOSFS ATR as drive 1.

`mkatr-go` is a Go port of [dmsc/mkatr](https://github.com/dmsc/mkatr) and remains a separate GPL-2.0-only executable. For remote SSH builds, install or copy `mkatr-go` to the remote machine's `PATH`, or set the project ATR tool field to its remote path. Disk creation then occurs remotely next to the compiled artifact.

The **ATR Explorer** activity in the left navigation opens either filesystem as a collapsible tree. It supports native desktop file/folder drops, project-to-ATR and ATR-to-project dragging, internal moves, empty folders, extraction, rename, delete, and geometry-preserving rewrites. Open the configured project image directly or browse to any local `.atr` file. Native file drops are handled by Wails and passed to `mkatr-go` as absolute paths; image mutations are written atomically.

## Git

The Git panel supports structured status, repository initialization, per-file and stage-all/unstage-all actions, staged or unstaged diffs, and commits. Install Git separately; the IDE honors your existing Git configuration and credentials. No Git network automation runs silently. Push/pull, branch management, and remote setup remain explicit future additions.

## Claude Code

exBASIC Studio launches the local Claude Code executable instead of calling the Anthropic Messages API. It automatically discovers either:

- `claude` on `PATH`, or
- the native Claude Code binary bundled with the Anthropic VS Code extension.

Sign in through Claude Code or the VS Code extension as usual. The integration reuses that installation's authentication, selected model, settings, and project-level `CLAUDE.md`; exBASIC Studio does not request or store an API key. Set `EXBASIC_CLAUDE_PATH` only if the executable is installed in a nonstandard location.

Each opened project gets a resumable conversation for the lifetime of the app. The composer exposes three explicit modes: **Plan** is read-only, **Edit** accepts workspace edits while denying tools that would require an unanswered prompt, and **Auto** uses Claude Code's automatic permission mode. After an editing turn, clean editor tabs and source-control state refresh from disk; unsaved tabs are never overwritten. The **Start new conversation** button discards the current in-app session reference.

## Remote SSH workspaces

Click the remote indicator in the lower-left status bar or **Open Remote Project over SSH** on the welcome screen. Enter any hostname accepted by the installed OpenSSH client, including aliases from `~/.ssh/config`, and a remote directory containing `exbasic.project.json`.

The IDE asks `ssh -G` for the effective configuration and retains a typed view of connection, identity, host-key, keepalive, and proxy settings while preserving every resolved option in a raw map. The actual SFTP and command channels are launched through the system OpenSSH client, so `Host`, `Match`, `Include`, `ProxyJump`, ssh-agent, platform key providers, `known_hosts`, and system/user defaults behave exactly as they do at the command line. Interactive password entry is intentionally not captured by the IDE; configure a key or agent for seamless connections.

Explorer reads and writes use a persistent SFTP session. Search, file operations, the integrated terminal, Git, settings, and compiler builds execute against the remote workspace. Remote **Run** currently completes the build and reports the artifact; launch GUI emulators from the remote terminal or a remote desktop session.

## Extensions

The backend discovers manifests in:

- Windows: `%AppData%/exbasic-studio/extensions/<extension-id>/extension.json` (actual base from Go `os.UserConfigDir`)
- macOS: `$HOME/Library/Application Support/exbasic-studio/extensions/<extension-id>/extension.json`
- Linux: `$XDG_CONFIG_HOME/exbasic-studio/extensions/<extension-id>/extension.json`, or `$HOME/.config/...`

Example: `extensions/example/extension.json`.

**No untrusted extension code executes yet.** The `sdk/extension-api/README.md` defines the planned capability-based API surface. Before enabling execution, build a sandboxed WASI host and enforce permission checks at every native API boundary. Do not execute arbitrary local JavaScript or binaries based on manifest content.

## Source tree

```
main.go                        Minimal Wails entry point and frontend embed
internal/app/                  Wails application bootstrap and window wiring
internal/project/             JSON project config, safe workspace file operations
internal/compiler/            exBASIC compile/run adapter
internal/emulator/            Headless Atari800 process/session and PNG frame adapter
internal/diskimage/           ATR tool discovery and local/remote argument adapter
internal/git/                 Git CLI integration
internal/claudecode/          Local Claude Code CLI adapter and session integration
internal/remote/              OpenSSH resolution, SFTP workspace, remote commands
internal/manual/              Embedded 24-file Markdown handbook and search
internal/extensions/          Extension manifest discovery
internal/studio/              Wails service facade / native API
cmd/mkatr-go/                 Separate ATR command-line executable
third_party/mkatrgo/          GPL Go port of mkatr plus EOSFS implementation
third_party/exbasic/          Rebuilt, complete platform compiler distributions
third_party/atari800/         Official 7.2.1 source plus GPL headless bridge
frontend/src/App.svelte       IDE shell, editor, panels
frontend/src/lib/exbasic.ts   exBASIC syntax highlighting starter
frontend/bindings/            Wails-generated TypeScript service bindings
sdk/extension-api/            Future extension API notes
build/config.yml              Product metadata and live-reload pipeline
build/{windows,darwin,linux}/ Platform build and packaging assets
build/{ios,android}/          Wails mobile build assets (experimental upstream)
build/Taskfile.yml            Shared Wails build tasks
Taskfile.yml                  Project-level build, run, test, and check tasks
```

The workbench follows the VS Code interaction model: native folder picking, workspace file/folder creation, rename and delete, multi-file tabs, editor navigation history and split preview, breadcrumbs, command palette (`Ctrl/Cmd+Shift+P`), quick open (`Ctrl/Cmd+P`), workspace search, structured source control, run/build views, a stateful integrated terminal, bottom problems/output panels, and a Claude Code secondary sidebar.

## Development next steps

1. Replace keyword-stream highlighting with a parser derived from exBASIC syntax definitions (`.syn` files) and add completion; compiler-backed diagnostics are already active.
2. Add recent projects and crash-recovery backups.
3. Add machine-readable diagnostic and `.lbl`/`.lst` inspection; integrate debugger APIs after testing emulator capabilities.
4. Ship runtime-sandboxed extensions with explicit user grants and version compatibility.
5. Improve Git branching, conflict presentation, and remote authentication.
6. Add AI streaming, selectable context scope, and reviewable patches; do not auto-apply edits.
7. Add packaging/signing pipelines on each target OS and test Wails 3.0.0 native build.

## Security notes

- All workspace file operations reject relative path traversal and symlink paths. The first-pass path guards do not eliminate filesystem TOCTOU races; harden these guards before opening untrusted workspaces.
- Compiler/emulator processes are explicitly launched when the user clicks Build or Run; they execute local user-supplied tools and should only be used with trusted projects/toolchains.
- The integrated terminal executes exactly the shell command entered by the user and preserves working-directory changes between commands. It is an explicit interactive action and should only be used in trusted workspaces.
- Claude authentication remains owned by the local Claude Code installation; exBASIC Studio does not collect or persist credentials.
- The plugin manifest schema is **not** permission enforcement by itself; execution remains disabled.

## Licensing and provenance

Studio code is provided under the MIT license (see `LICENSE`). `mkatr-go` and `third_party/mkatrgo` are GPL-2.0-only. The rebuilt eXBASIC distribution retains its upstream GPL/runtime licensing in its bundled `LICENSE`. Atari800 7.2.1 and `studio-helper` are GPL-2.0-or-later; see `third_party/atari800/NOTICE.md` and the upstream `COPYING`. These components remain separate processes and are not linked into the MIT application. The user-supplied EOSFS version 1 specification is CC0. The supplied exBASIC handbook remains subject to its authors' original rights.

## Verification status

- `go test ./...`: passing with Go 1.26.
- `npm --prefix frontend run check`: passing with zero diagnostics.
- `npm --prefix frontend run build`: passing with the Wails runtime/Vite plugin enabled.
- Native eXBASIC rebuild: passing on Windows from `E:/exBasic`; the imported compiler successfully compiles the upstream `sieve.bas` sample.
- Headless Atari800: Windows bridge built from official 7.2.1 source and verified to boot that compiled XEX and return a 336×240 RGB frame without opening a window.
- `mkatr-go`: verified against upstream `lsatr` and file extraction; EOSFS output passes the ExtendedOS filesystem checker with zero errors and warnings.
- macOS, Linux, iOS, and Android builds still require verification on their target toolchains before release.
