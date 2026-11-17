# Security

exBASIC Studio is a local desktop IDE. It has no account system, no
network service, no telemetry and no automatic updater. This document
describes what it protects against, how, and where the limits are.

To report a vulnerability, please use GitHub's private vulnerability
reporting for this repository rather than a public issue.

## Threat model

The main risk is **opening a folder you did not write**: a downloaded
project, an archive from a forum, a cloned repository. Such a folder can
contain a project configuration that names arbitrary programs as the
"compiler" or "emulator", Git hooks, and links that point outside itself.

Out of scope: an attacker who already controls your user account, and bugs
in the bundled compiler or emulator when processing a hostile *source
program* that you chose to trust and build.

## Workspace trust

A folder is either **trusted** or in **restricted mode**.

| Action | Restricted mode | Trusted |
|--------|-----------------|---------|
| Browse, open, edit, save, search | yes | yes |
| Git status, diff, stage, unstage | only after *Show Repository Status* (hooks disabled) | yes |
| Build, run, debug, live diagnostics | no | yes |
| Terminal | no | yes |
| Git commit, branch switch, merge continue (hooks may run) | no | yes |
| Git fetch, pull, push | no | yes |
| Project-configured compiler, emulator or ATR tool | never used | yes |

- Trust is an explicit decision in a dialog that shows the programs the
  folder is configured to run. Projects you create in Studio are trusted.
- Trust is bound to the configured tool paths. Changing the compiler,
  emulator or ATR tool, in Studio or by editing the file elsewhere, returns
  the folder to restricted mode.
- Opening a folder writes nothing into it. The `H` folder used by the
  emulator is created on the first trusted run.
- In restricted mode the emulator and ATR explorer use only the bundled
  tools and a private host folder.

## File access

- Every workspace path is relative, may not contain `..`, and is resolved
  component by component. Symbolic links and Windows reparse points
  (junctions, mount points) are rejected at each component, for reads,
  writes, renames, deletes and imports.
- Files are replaced atomically (temporary file, then rename).
- Only valid UTF-8 text without NUL bytes opens in the editor; the size
  limit is 2 MiB.
- Imports are capped (5,000 entries, 256 MiB) and never overwrite. Searches
  are capped by result count, file size and total bytes.
- Deleting, overwriting on extraction, discarding unsaved text and aborting
  a merge always ask first.

## Child processes

- Every external program is started with an argument vector. Nothing is
  passed through a shell, so file names cannot inject commands.
- Cancelling a build ends the whole process tree.
- Closing a terminal ends the shell and everything it started.
- Git runs with terminal prompts disabled. Studio never asks for or stores
  credentials; your Git credential helper and SSH agent are used as they
  are. Network operations run only when you start them.
- Read-only Git queries never run external diff or textconv programs.
  Remote URLs using the `ext::` and `fd::` transports are refused.

## Data kept on your computer

In the per-user configuration directory (`%AppData%\exbasic-studio`,
`~/Library/Application Support/exbasic-studio`, `~/.config/exbasic-studio`):

- `state.json`: recent projects, layout, preferences, per-workspace tabs,
  breakpoints and watches, and trust decisions.
- `recovery/`: copies of unsaved text, readable only by your user. A copy
  is deleted when the file is saved. *Settings → Delete Crash-Recovery Data*
  removes them all.
- `logs/`: up to five 1 MiB diagnostic logs.

Nothing is uploaded. *Help → Export Support Bundle* writes a ZIP that you
choose to share; it contains the logs with your home directory, user name
and workspace paths replaced by placeholders, and no project sources.

## The manual viewer

Manual chapters are bundled with the application and rendered through a
sanitizer that removes scripts, event handlers, raw HTML and images. Links
to web sites are shown as text and are not navigable from the application.

## Releases

Release artifacts are built by GitHub Actions from the pinned sources in
this repository. Windows executables are Authenticode-signed, the macOS
disk image is Developer ID-signed and notarized, and every release carries
SHA-256 checksums, an SPDX bill of materials and build provenance
attestations. Verify a download with:

```
sha256sum -c SHA256SUMS.txt
gh attestation verify <file> --repo <owner>/exbasic-studio
```

Contributor builds are unsigned.

## Known limits

- **Git filters.** A repository's own configuration can define clean and
  smudge filters that Git runs during `status`, `diff` and `add`. Studio
  disables hooks, external diff and textconv in restricted mode but cannot
  disable filters without breaking legitimate repositories. For that reason
  Studio does not run Git at all in a restricted folder until you choose
  *Show Repository Status* in the Source Control view.
- **Trusting is all-or-nothing** per folder.
- **The emulated machine** can read and write the project's `H` folder
  (and, with EOS, the generated host disk). Programs you run in the
  emulator are your own builds or media you chose.
- The debugger is read-only: it cannot change registers or memory.
- **Developer debugging port.** When the `EXBASIC_STUDIO_DEBUG_PORT`
  environment variable is set, the window accepts a debugger on that
  loopback port. It is off by default and meant for development and tests.
