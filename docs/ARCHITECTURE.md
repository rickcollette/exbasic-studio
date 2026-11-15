# Architecture and module boundaries

```
Svelte 5 + CodeMirror 6
    |
    |  Wails-generated typed bindings
    v
internal/app (Wails bootstrap)
    |
    v
StudioService (application facade)
    +-- project: workspace root, JSON config, source IO
    +-- compiler: exbasic -> XEX/ROM, Atari800 launch
    +-- git: explicit git CLI calls
    +-- manual: embedded local handbook + search
    +-- claudecode: local Claude Code process, shared login/settings
    +-- remote: system OpenSSH config resolution + persistent SFTP workspace
    +-- extensions: manifest discovery ONLY
    +-- workbench: project search + explicit integrated terminal commands
```

The root `main.go` owns the `frontend/dist` embed because Go embed patterns cannot traverse to a parent directory. All other Wails setup lives in `internal/app`; feature code remains in focused `internal/*` packages.

**Invariants**

- Only one active project in the starter. Files are always relative to it.
- Build entry is one `.bas` file, with source sharing via exBASIC `INCLUDE`.
- No shell interpolation for compiler, emulator, or Git processes.
- Claude Code runs in the open project directory. Plan mode is read-only; Edit and Auto are explicit user choices mapped to the local CLI's permission modes, with unanswered permission prompts denied.
- SSH workspaces delegate configuration and authentication to installed OpenSSH. The Go layer models the effective `ssh -G` output, uses SFTP for workspace I/O, and executes terminal/Git/build commands on the selected host.
- Credentials are not stored in the project JSON.
- Extensions do not run in the alpha.

**Future interfaces**

```go
type Compiler interface {
    Build(ctx context.Context, project Project) (BuildResult, error)
}
type Emulator interface {
    Launch(ctx context.Context, artifact string, target string) error
}
type PluginRuntime interface {
    Start(ctx context.Context, verifiedManifest Manifest, grants []Grant) error
}
```

These signatures are design proposals, not yet implemented runtime interfaces.
