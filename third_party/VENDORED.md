# Vendored source revisions

The source trees below are checked into this repository so release builds do
not depend on GitHub. Update them deliberately and keep the pinned commit and
local-adjustment notes current.

| Directory | Upstream | Commit |
|-----------|----------|--------|
| `exbasic/source` | <https://github.com/rickcollette/exbasic> | `6b3d87a84e8235c05dc1ad7124ac97961c59951b` |
| `mkatrgo` | <https://github.com/rickcollette/mkatrgo> | `2d1f341cb461e683233cbaf348a1fe1409c544fb` |
| `exoperatingsystem` | <https://github.com/rickcollette/exoperatingsystem> | `983a8b0e91eb6c8575f47c066bfe8719b344f4d2` |
| `ex800` | <https://github.com/rickcollette/ex800> | `5d821b57a80467a720ed8eede5867e8f18a63ddd` |

The exBASIC commit is the upstream reference revision. `exbasic/source`
carries the Studio compiler metadata patch described in `exbasic/NOTICE.md`
and retains distribution assets needed by Studio, so it is not byte-for-byte
identical to that revision. `mkatrgo` is an unmodified snapshot. The ex800
snapshot adds narrow exceptions to its `.gitignore` so all files tracked
upstream remain visible when the snapshot is added to this parent repository.
Its `.gitattributes` also pins the Autotools helpers and `config.h.in` to LF;
Windows-origin source snapshots otherwise cannot configure correctly on Linux
or macOS. No ex800 source logic was changed by this normalization.

The ExtendedOS snapshot renames three Windows-reserved source filenames:
`CON.asm`, `CON.MAN`, and `man/CON.txt` have a trailing underscore before the
extension. Its Makefile and documentation tools map those names back to `CON`
in Atari build artifacts. No source content was changed by the rename.
