# Project configuration and migration

A project is a folder. `exbasic.project.json` in its root is optional: a
folder without it opens with defaults, and the file is created the first
time you save settings.

## Schema version

```json
{
  "schemaVersion": 1,
  "name": "game",
  "entry": "src/main.bas",
  "target": "atari",
  "output": "build/game.xex",
  "compiler": "exbasic",
  "emulator": "atari800-studio",
  "native": "auto",
  "debug": true,
  "includePaths": ["src", "lib"],
  "assembly": [],
  "atr": {
    "enabled": false,
    "tool": "mkatr-go",
    "filesystem": "sparta",
    "output": "build/game.atr",
    "boot": true,
    "exactSize": false,
    "minimumSize": 0,
    "bootPage": 7,
    "files": []
  }
}
```

| `schemaVersion` | Meaning |
|-----------------|---------|
| missing | a pre-1.0 (alpha) project |
| `1` | exBASIC Studio 1.0 |
| higher | written by a newer release; this release refuses to open it rather than risk losing settings |

## Migrating alpha projects

Nothing to do. An alpha project:

- opens as it is; missing fields receive their defaults in memory
  (`atr.tool = "mkatr-go"`, `atr.filesystem = "sparta"`,
  `atr.output = "build/<name>.atr"`, `atr.bootPage = 7`, and `atr.boot = true`
  when the project had no `atr` section at all);
- is **not rewritten** when opened, built or run;
- gains `"schemaVersion": 1` and the defaulted fields the first time you
  choose *Save Settings*, or when you rename the entry file from the
  explorer (which has to update `entry`).

All 1.0 fields existed in the alpha format, so a 1.0 project file still
opens in the alpha; it ignores `schemaVersion`.

Three behaviours changed for existing projects:

- A folder you open is in **restricted mode** until you trust it. Projects
  created by the alpha are treated like any other existing folder.
- The `H` folder is no longer created when a folder is opened; it appears
  on the first trusted run.
- `"emulator": "atari800"` and `"atari800-headless"` from early alphas are
  treated as the bundled helper.

## Fields

| Field | Values | Notes |
|-------|--------|-------|
| `entry` | path to a `.bas` file | other sources are added with `INCLUDE` |
| `target` | `atari`, `atari-cart` | executable or 8 KB cartridge |
| `native` | `auto`, `hot`, `all`, `none` | how much is compiled to 6502 code |
| `debug` | boolean | keeps listings and symbols; required for source-level debugging |
| `compiler`, `emulator`, `atr.tool` | a name or a path | a bare name uses the bundled tool, then `PATH`. Changing any of these revokes folder trust. |
| `atr.filesystem` | `sparta`, `eos` | EOSFS images cannot use `boot` |

All paths are relative to the project folder and may not leave it.

## Application state is separate

Recent projects, open tabs, layout, breakpoints, watches and trust are kept
in the per-user configuration directory, not in the project. They have
their own `schemaVersion`; a state file from a newer release is set aside
and defaults are used.