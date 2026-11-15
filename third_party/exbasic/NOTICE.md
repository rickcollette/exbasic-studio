# eXBASIC toolchain notice

`source/` is the eXBASIC language/compiler tree imported from `E:/exBasic` on
2026-11-07. The Windows amd64 distribution in `windows-amd64/compiler/` was
rebuilt from that exact tree; it includes eXBASIC, CA65, LD65, AR65, target
definitions, syntax definitions, runtime libraries, and documentation.

eXBASIC is based on FastBasic and is licensed under the GNU GPL version 2 or
later. Its runtime has the exception documented by the upstream project. The
complete compiler-distribution license is copied into each `compiler/`
directory, and the source license is in `source/LICENSE`.

These programs remain separate command-line executables. The MIT-licensed
Studio application invokes them as child processes and does not link to them.
