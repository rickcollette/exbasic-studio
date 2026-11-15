exBASIC
=======

exBASIC is a fast, compiled BASIC for the Atari 8-bit home computers.  You
write programs on a modern computer with any text editor, and the `exbasic`
compiler turns them into Atari executables (`.XEX`) or cartridge images that
run on the real machine or in an emulator.

exBASIC is based on [FastBasic](https://github.com/dmsc/fastbasic) by Daniel
Serpell (dmsc).

```basic
Q, R = DivMod(17, 5)
? Q; " remainder "; R
? Fib(20)

FUNC DivMod(A, B) Q, R
  Q = A / B
  R = A MOD B
ENDFUNC

FUNC Fib(N)
  IF N < 2 THEN RETURN N
  RETURN Fib(N - 1) + Fib(N - 2)
ENDFUNC
```

The language has no line numbers, uses integer arithmetic by default for
speed, and adds to the Atari BASIC commands:

- functions with typed parameters, local variables, multiple results and
  recursion: `FUNC` / `ENDFUNC`, `LOCAL`, `RETURN a, b`, `A, B = F(X)`,
- structured loops, including `DO WHILE` / `LOOP UNTIL`,
- string concatenation with `+`, as in `? "N=" + STR$(N)`,
- 6502 machine code for the integer code that runs often: 2 to 6 times
  faster than bytecode, selected to keep programs small (see the `-native`
  options),
- `INCLUDE "file"` to split programs in several files,
- `COMMON` and `RUN "D:NEXT.XEX"` to pass variables to the next program,
- `STATUS`, floating point, P/M graphics, display list interrupts, SIO and
  FujiNet support.

See the [programmer's manual](doc/manual/README.md) for the language and
[compiler/USAGE.md](compiler/USAGE.md) for the compiler options.


Building
--------

You need a C and a C++ compiler and GNU make.  Type:

    make

This builds, in `build/compiler`, the `exbasic` compiler with the `ca65` and
`ld65` tools, the runtime libraries and the documentation, ready to be
copied to any folder.  It also builds `build/exbasic.atr`, a disk image with
the compiled sample programs.

Other targets:

- `make test` runs the test suite.
- `make profile` profiles the sample programs, see the
  [test suite documentation](testsuite/README.md).
- `make dist` builds the `build/exbasic.zip` archive of the compiler.


Usage
-----

    build/compiler/exbasic myprog.bas

produces `myprog.xex`, or a cartridge image with `-t:atari-cart`.
Floating point support is included automatically, only when the program
uses it.


Source layout
-------------

- `src/compiler/` - the compiler (C++), `native.cc` generates the machine
  code of loops.
- `src/syntax/` - the language syntax, read by the compiler.
- `src/interp/`, `src/*.asm` - the 6502 runtime (bytecode interpreter).
- `compiler/` - target definitions, linker configurations and documentation.
- `doc/manual/` - the programmer's manual, one Markdown file per chapter.
- `samples/` - sample programs.
- `testsuite/` - test suite, simulator and profiler.
- `cc65/` - the CA65 assembler and LD65 linker from the CC65 project.
- `mkatr/` - tools to build ATR disk images.
- `disk/` - DOS files for the disk image.


License
-------

exBASIC is free software, under the GNU General Public License version 2 or
later, see [LICENSE](LICENSE).  The runtime library included in compiled
programs has an exception that allows distributing your programs without
restrictions, see [compiler/LICENSE](compiler/LICENSE).
