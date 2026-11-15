exBASIC test suite
==================

The test suite compiles small programs with the `exbasic` compiler and runs
them in the mini65 6502 simulator, comparing the output with the expected one.
Type `make test` from the parent directory to run all the tests.  To run a
single test:

    build/bin/exbtest testsuite/tests/<name>.chk

`make eostest` runs the programs in `eos/` on a real DOS instead: EOS, the
SpartaDOS X compatible DOS of the ExtendedOS project, from its cartridge
image `EOS.car` (by default in the folder above the exBASIC tree, or set
`EOS_CAR`), in the Altirra emulator under wine and Xvfb.  They test file
I/O, `NOTE`/`POINT`, `XIO`, directories, `RUN`/`COMMON`, the load-time
memory check and graphics; the expected output (in `eos/run.py`) is the
output of the same programs on SpartaDOS X 4.50.  `testsuite/eos/run.py
<test>` runs some of them.


File format
-----------

Each test is a pair of files: `tests/<name>.chk` (the test definition) and
`tests/<name>.bas` (the source program).  A test may also have a companion
`tests/<name>.asm`; `exbtest` appends it to the generated assembly before
calling `ca65`.  This lets a test define a `USR` routine or page-aligned
bytecode using the runtime `TOK_*` symbols.  See `interp-page-cross` for an
example.  Files included with `INCLUDE` go in `tests/include/`, and programs
loaded with `RUN` go in `tests/run/`.

The `.chk` file is a sequence of `Key: value` lines, followed by multi-line
sections terminated by a line containing only `.`.  Lines starting with `#` in
the header are comments.  Recognised keys:

| Key             | Value                                                          |
|:----------------|:---------------------------------------------------------------|
| `Name:`         | Human-readable test name (shown in pass/fail output).          |
| `Test:`         | comma/space/hyphen separated list of test modifiers (below).   |
| `Max-cycles:`   | Cycle limit for the simulator (default 20 000 000).            |
| `Error:`        | Expected compiler error message (for `error` tests), a substring of the compiler output. |
| `Error-pos:`    | Expected error location as `line:column`.                      |
| `Input:`        | Keystrokes fed to the running program (section, terminated by `.`). |
| `Output:`       | Expected captured output stream (rest of file, exact match).   |
| `Exists:`       | Filename that must exist after the run.                        |
| `Check <file>:` | Section with expected content of a file written during the run.|
| `Options:`      | Extra compiler options, for example `-native:all`.             |
| `Program:`      | Extra BASIC source (relative to `tests/`) compiled to the `D:` disk before the run, as `<NAME>.XEX`, so the test can load it with `RUN`. May be repeated. |

### Test modifiers (`Test:`)

| Modifier  | Meaning                                                       |
|:----------|:--------------------------------------------------------------|
| `run`     | Compile and run under mini65.                                 |
| `compile` | Compile only, do not run.                                     |
| `error`   | Expect a compile error; `Error:` gives the message.           |

Each test is compiled and run three times: as an executable (XEX), as a
cartridge image (ROM), and as an executable with all the code possible
compiled to machine code (`-native:all`), unless the test gives its own
`Options:`.


Input notation
--------------

The `Input:` section supports readable notation for control and special
characters.  All other bytes pass through literally (newlines in the file are
converted to ATASCII EOL `$9B`).

| Notation     | Maps to                                               |
|:-------------|:------------------------------------------------------|
| `^A` .. `^Z` | Ctrl-A (`$01`) .. Ctrl-Z (`$1A`). Case-insensitive.   |
| `<ESC>`      | Escape (`$1B`).                                       |
| `<UP>`       | Cursor up (`$1C`).                                    |
| `<DOWN>`     | Cursor down (`$1D`).                                  |
| `<LEFT>`     | Cursor left (`$1E`).                                  |
| `<RIGHT>`    | Cursor right (`$1F`).                                 |
| `<RETURN>`   | Return / ATASCII EOL (`$9B`).                         |
| `<CLS>`      | Clear screen (`$7D`).                                 |
| `<BACKSP>`   | Backspace (`$7E`).                                    |
| `<TAB>`      | Tab (`$7F`).                                          |
| `<DELLN>`    | Delete line (`$9C`).                                  |
| `<INSLN>`    | Insert line (`$9D`).                                  |
| `<DELTAB>`   | Clear tab stop (`$9E`).                               |
| `<SETTAB>`   | Set tab stop (`$9F`).                                 |
| `<BELL>`     | Bell (`$FD`).                                         |
| `<DEL>`      | Delete character at cursor (`$FE`).                   |
| `<INS>`      | Insert character at cursor (`$FF`).                   |
| `\xHH`       | Explicit hex byte (e.g. `\x9B` for ATASCII EOL).      |
| UTF-8 chars  | Multi-byte UTF-8 sequences from the ATASCII character set (e.g. `⇱` for CLS, `▄` for block graphic, `𝗔` for inverse A). |


Debugging
---------

Set `EXBTEST_DEBUG=1` before running `make test` or `build/bin/exbtest` to log
the CIO and device activity of the simulated programs.


Peephole optimizer coverage
---------------------------

The compiler can report every peephole rule and every successful match when
`EXBASIC_PEEPHOLE_DEBUG` is set.  The coverage check compiles all test sources
for both targets, then fails if any rule was not reached:

    make peephole-coverage

The report lists matched and rewritten rule counts.  It treats a rule as
covered when its pattern plus guard matched, not only when it produced a
rewrite, because some rules deliberately only update optimizer state.  The
full compiler output is kept in `build/peephole-coverage/debug.log`.

When adding or changing an optimizer rule, add a test that exercises it.

The machine code generated for loops can be inspected with
`EXBASIC_NATIVE_STATS=1`: the compiler then lists, for each instruction (or
combination of instructions), how many times it was compiled to machine code
and the bytes it used.  The `exb-native` test checks that the machine code
gives the same results as the bytecode.  `EXBASIC_HOT_DEBUG=1` shows the estimated
iterations of each loop and calls of each `FUNC`, used to select the code;
`EXBASIC_HOT_RATIO` and `EXBASIC_HOT_MIN` change the selection limits (64
and 64) for experiments.


Profiling
---------

`build/bin/exbprof` runs a compiled program in the same simulator and shows
where the CPU time goes: the cycles spent in each runtime routine, and the
most executed bytecode instructions and instruction pairs.  The pairs are
the best candidates for new combined instructions in the optimizer.

    make profile

builds the profiler and profiles the samples listed in `PROFILE_SAMPLES`
(`sieve`, `pi` and `ahlbench` by default).  To profile any program, compile
it with `-g` to get the label file, then run the profiler:

    build/compiler/exbasic -g prog.bas
    build/bin/exbprof [-q] [-c max_cycles] [-d path] prog.xex prog.lbl

`-q` hides the program output, `-a` uses the original Atari math package
(slower than the default replacement, as on a real Atari), `-d` sets the
folder used as the `D:` disk
(for programs that load files or `RUN` other programs), and `-c` stops
programs that never end (like
games) after the given number of cycles.  Programs that wait for input never
end, as the profiler does not provide any input.


Architecture
------------

- `testsuite/src/exbtest.c` — the test harness (compilation, execution,
  output comparison).
- `testsuite/src/exbprof.c` — the profiler.
- `testsuite/mini65/` — the mini65 6502 simulator with Atari BIOS
  emulation.
- `testsuite/tests/*.chk` — test definitions.
- `testsuite/tests/*.bas` — source programs.
- `testsuite/tests/include/` — files used by the `INCLUDE` tests.
