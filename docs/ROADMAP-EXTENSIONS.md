# Roadmap: extensions (after 1.0)

**Status: not part of 1.0.** exBASIC Studio 1.0 has no extension loading,
no extension installation UI and no extension API. Nothing in the
application discovers or executes third-party code. This document records
the intended direction so the design is not lost; none of it is implemented.

The alpha contained a manifest *discovery* stub (it listed manifests and ran
nothing). It was removed for 1.0 rather than shipped as an unfinished
control.

The Sprite Studio ([SPRITES.md](SPRITES.md)) is a built-in, first-party
feature, not an extension: it runs no third-party code and needs none of the
machinery below. It is a reference for what a sprite or asset extension
should be able to do.

## Goals

- Add language-adjacent tools without forking the application: formatters,
  asset converters (fonts, sprites, display lists), project templates,
  extra emulator profiles.
- Keep the security model intact: an extension must not be able to do more
  than the user explicitly granted.

## Intended design

- **Declarative first.** Templates, snippets, emulator profiles and keyword
  documentation need no code. A manifest contributes them as data.
- **Sandboxed code second.** Executable extensions would run as WebAssembly
  (WASI) modules in a separate process, with no ambient access.
- **Explicit capabilities.** A manifest lists what it needs, and the user
  approves each: read the workspace, write under a named subfolder, run as
  a build step, contribute a panel. No network and no process spawning in
  the first version.
- **Signed manifests** with a publisher identity, and a per-workspace
  enable switch that is off in restricted mode.
- **A versioned API** with the same compatibility promise as the project
  file: additive within a major version.

Sketch of a manifest (subject to change):

```json
{
  "apiVersion": "1",
  "id": "publisher.sprite-tools",
  "name": "Sprite Tools",
  "version": "0.1.0",
  "contributes": { "templates": ["templates/pmg.bas"] },
  "entrypoint": "sprite-tools.wasm",
  "permissions": ["workspace.read", "workspace.write:assets"]
}
```

## Before any of this ships

- A threat model review of the capability set.
- An answer for updates and revocation that does not require an in-app
  updater (1.0 has none by design).
- At least two real extensions built against the API.

## Other post-1.0 items

- Mobile targets (iOS, Android). The desktop toolchain does not run there,
  and 1.0 makes no claim that it does.
- Debugger: conditional breakpoints, data watchpoints, writable registers
  and memory, remote debugging.
- Git: rebase, cherry-pick, bisect, submodules and cloning from the UI (use
  the integrated terminal in 1.0).
- Remote (SSH) workspaces are not scheduled.
