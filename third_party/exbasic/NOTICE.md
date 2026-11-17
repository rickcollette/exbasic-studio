# eXBASIC toolchain notice

`source/` is the eXBASIC language/compiler tree imported from the upstream
project on 2026-11-07, with the changes listed below. exBASIC Studio builds
the complete compiler distribution (eXBASIC, CA65, LD65, AR65, target
definitions, syntax definitions, runtime libraries and documentation) from
this tree for each platform with `build/tools/build-toolchain.sh`; no
prebuilt binaries are kept in the repository.

Changes made for exBASIC Studio 1.0 (`source/src/compiler`):

- `--capabilities=json` prints the supported tool features.
- `--metadata-json <file>` writes a versioned JSON document, on success and
  on failure, with structured diagnostics, source hashes, executable
  line-to-address ranges and typed symbols (`metadata.cc`, `metadata.h`).
- With that option, per-line labels are emitted as ordinary assembly labels
  so the linker reports their addresses.

The human-readable compiler output and all existing options are unchanged.

eXBASIC is based on FastBasic and is licensed under the GNU GPL version 2 or
later. Its runtime has the exception documented by the upstream project. The
source license is in `source/LICENSE` and is shipped with the compiler.

These programs remain separate command-line executables. The MIT-licensed
Studio application invokes them as child processes and does not link to them.