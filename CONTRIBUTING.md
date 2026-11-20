# Contributing to exBASIC Studio

## What you need

| Tool | Version | Used for |
|------|---------|----------|
| Go | the version in `go.mod` | backend, tools |
| Node.js | 22 | frontend |
| C and C++ compiler, `make`, `bash` | GCC or Clang | exBASIC and ex800 |
| Wails CLI | `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0` | dev server, bindings, packaging |
| Task | `go install github.com/go-task/task/v3/cmd/task@latest` | build tasks |
| Git | any recent | source control features and tests |

Platform notes:

- **Windows:** MinGW-w64 (`gcc`, `g++`, `make`) and Git for Windows. The
  toolchain script runs in Git Bash. NSIS is needed only for the installer.
- **macOS:** Xcode command line tools.
- **Linux:** `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`.

## Build and run

```
task toolchain        # exBASIC, ex800, ex800-studio, mkatr-go -> build/stage/<os>-<arch>/tools
cd frontend && npm ci && cd ..
task dev              # live-reload development build
task build            # production build in bin/
task package          # installer / archive for this platform
```

`task toolchain` builds everything from the pinned sources in
`third_party/`. Intermediate files go to `build/toolchain/`; the vendored
source trees stay untouched (exBASIC uses its own ignored `build/` folder).

A development build finds the staged tools through `EXBASIC_STUDIO_TOOLS`,
which `task dev`, `task run` and `task test` set for you. A packaged build
finds them in its own resources; there is no source-tree lookup.

## Tests

```
task test             # Go tests against the real staged toolchain
task test:frontend    # type check, generated-file check, unit tests, build
task test:e2e         # browser tests with accessibility checks
task check            # everything CI runs
```

- Go tests that need the real compiler, emulator or ATR tool skip when
  `EXBASIC_STUDIO_TOOLS` is not set. With `EXBASIC_STUDIO_REQUIRE_TOOLS=1`
  (set by `task test` and CI) a missing tool is a failure instead.
- Frontend tests run against `frontend/src/lib/mockNative.ts`, a
  deterministic in-memory backend. Add behaviour there when a test needs it.
- `VITE_MOCK_NATIVE=1 npm run dev` in `frontend/` runs the UI in a normal
  browser without the Go backend.

## Generated files

These are checked in, and CI fails if they are stale:

| Files | Regenerate with |
|-------|-----------------|
| `frontend/bindings/` | `task bindings` after changing `StudioService` or its types |
| `frontend/src/lib/language/*.generated.*` | `npm run gen:language` in `frontend/` after changing the `.syn` files, the grammar or Appendix A |
| version fields in `build/` and `frontend/package.json` | `task version:sync` after editing `VERSION` |

## Conventions

- Documentation follows [`WRITING_STYLE.md`](WRITING_STYLE.md). Keep it direct,
  useful and technically accurate; do not turn ordinary features into a sales
  pitch.
- Backend: every method of `StudioService` is callable from the UI, so it
  validates its arguments. External programs are started with an argument
  vector, never a shell string. Anything that runs workspace-controlled
  programs calls `requireTrust` first.
- Frontend: components call the backend only through `native()`; they do
  not import `@wailsio/runtime` or the bindings. Every interactive element
  works from the keyboard and has an accessible name; decorative icons are
  `aria-hidden`.
- User-visible changes get a line in `CHANGELOG.md`.
- The compiler, the ex800 helper and mkatr-go are GPL programs that run
  as separate processes. Do not link their code into the Studio executable.

## Changing the bundled tools

- **exBASIC:** edit `third_party/exbasic/source`. Changes to the metadata
  document need a matching change in `internal/compiler/metadata.go` and a
  schema version bump if they are not backwards compatible.
- **ex800 helper:** `third_party/ex800/studio-helper/main.c`. The
  protocol version is checked in a handshake; bump it on both sides
  (`PROTOCOL_VERSION` and `emulator.ProtocolVersion`) for breaking changes.
- **ex800 core:** the tree is the pinned ex800 1.0.0 snapshot, derived from
  Atari800 7.2.1; see `third_party/ex800/NOTICE.md`.

See `docs/ARCHITECTURE.md` for the structure and `docs/SECURITY.md` for the
security model.
## Testing the real application

The Playwright suite in `frontend/e2e` runs the interface against a mock
backend. `npm run test:real` (in `frontend/`, Windows, after `task build`)
starts the built executable with a private configuration directory and
drives the real window: open a folder, create and save a file, trust, build
and run. Run it before a release; the mock cannot see faults in the native
layer.

It attaches through the WebView2 debugging port, which the application opens
on the loopback interface only when `EXBASIC_STUDIO_DEBUG_PORT` is set. The
variable is for development and tests; do not set it for normal use.
