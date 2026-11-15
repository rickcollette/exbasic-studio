# 19. Compiler Reference

This chapter is the complete reference of the `exbasic` command-line
compiler: how to call it, every option, the targets and their definition
files, the build steps and the tools they use, the linker configurations,
the files it reads and writes, the environment variables and the exit
codes. The last section shows how to call the compiler from `make`.

## Synopsis

```sh
exbasic [options] <input.bas> [<file.asm>...] [<file.o>...]
```

The compiler reads one exBASIC source file and produces an Atari
executable (`.xex`) or cartridge image (`.rom`). It does this in three
steps, compiling the BASIC source to 6502 assembly, assembling it with
`ca65` and linking it with `ld65` and the exBASIC runtime library. The
`ca65` assembler and `ld65` linker come from the CC65 project and are
included with the compiler.

The arguments that are not options are input files. The compiler decides
what to do with each one from its extension, without regard to case:

| Extension         | Treated as      | What the compiler does                         |
|-------------------|-----------------|------------------------------------------------|
| `.o`, `.obj`      | object file     | passes it to the linker                        |
| `.s`, `.asm`      | assembly source | assembles it with `ca65`, then links it        |
| anything else     | exBASIC source  | compiles it, assembles it, then links it       |

Only one exBASIC source can be linked into a program, because the compiler
checks the whole program at once (FUNC calls, parameter types and
variables). Passing two BASIC files stops with the error `only one BASIC
file can be linked into a program`. To build a program from several BASIC
files, use `INCLUDE` (see
[INCLUDE](17-program-organization.md#include)). Assembly and object files
are for machine code modules called with `USR` (see
[Memory and Machine Code](16-memory-and-machine-code.md)).

Options and files can be given in any order, with one exception: `-o`
applies to the input file that follows it (see the `-o` entry below).

The compiler writes its progress messages and errors to the standard error
output. A normal compilation prints one line per step:

```sh
exbasic hello.bas
```

Output:

```
BAS compile 'hello.bas' to 'hello.asm'
ASM assemble 'hello.asm' to 'hello.o'
LINK hello.xex
BAS compile 'hello.bas' to 'hello.asm'
ASM assemble 'hello.asm' to 'hello.o'
LINK hello.xex
```

The steps appear several times because, by default, the compiler builds
the program with different amounts of machine code and keeps the
smallest result. See [Native code modes](#native-code-modes).

## Options

Options that take an argument accept either `:` or `=` between the option
and the argument, so `-t:atari-cart` and `-t=atari-cart` are the same. The
only exception is `-o`, described below. An unknown option stops the
compiler with `invalid option '-x', try -h for help`.

### Summary

| Option                 | Meaning                                                       |
|------------------------|---------------------------------------------------------------|
| `-h`                   | show the help and exit                                        |
| `-v`                   | show the version and exit                                     |
| `-t:`*target*          | select the target (`atari` or `atari-cart`)                   |
| `-o` *name*            | name of the output file                                       |
| `-native:hot`          | machine code only for the code that runs often                |
| `-native:all`          | machine code for all the code possible                        |
| `-no-native`           | no machine code, bytecode only                                |
| `-no-ext`              | don't use the extended (combined) bytecode instructions       |
| `-n`                   | don't run the optimizer                                       |
| `-I:`*path*            | add a folder to search for `INCLUDE` files                    |
| `-g`                   | write the label file and the assembly listing                 |
| `-keep`                | keep the intermediate files (implies `-g`)                    |
| `-c`                   | only compile to assembly, don't assemble or link              |
| `-l`                   | write an expanded listing of the source                       |
| `-l:`*ext*             | same, with the given file extension                           |
| `-ls`, `-ls:`*num*     | write an abbreviated listing with lines of *num* characters   |
| `-prof`                | show bytecode usage statistics                                |
| `-d`                   | show the parser debug trace                                   |
| `-s:`*segment*         | put the compiled code in the given segment                    |
| `-C:`*file*            | use the given linker configuration file                       |
| `-S:`*address*         | set the start address of the program                          |
| `-X:`*option*          | pass an option to the assembler                               |
| `-DL:`*symbol*`=`*val* | define a linker symbol                                        |
| `-target-path:`*list*  | folders where target files are searched                       |
| `-syntax-path:`*list*  | folders where syntax files are searched                       |

The last two options are not listed by `-h`.

### -h
Shows the version, the usage line and the list of options, then exits
with code 0 without compiling anything.

```sh
exbasic -h
```

### -v
Shows the compiler version and exits with code 0.

```sh
exbasic -v
```

Output:

```
exBASIC 1.0.0 - based on FastBasic by Daniel Serpell (dmsc)
```

### -t
Selects the compilation target.

```
-t:target
```

A target is a definition file, *target*`.tgt`, that tells the compiler
which syntax files, runtime library, linker configuration and output
extension to use. Two targets are included:

| Target       | Output             | Description                                   |
|--------------|--------------------|-----------------------------------------------|
| `atari`      | `.xex` executable  | Atari 8-bit executable, loaded from DOS       |
| `atari-cart` | `.rom` image       | 8 KB cartridge image for `$A000`-`$BFFF`      |

Without `-t`, the compiler loads `default.tgt`, which includes
`atari.tgt`. If the name contains no `.`, the extension `.tgt` is added.
A name with a path that starts with `/` (or a drive letter on Windows) is
used as given; other names are searched in the target path (see
[Search paths](#search-paths)). See [Targets and target
files](#targets-and-target-files) for the file format.

```sh
exbasic -t:atari-cart game.bas
```

This writes `game.rom`.

An empty name, or a name containing a `"`, gives `invalid compiler target
name`. A target file that does not exist gives `Can't open target
definition file 'name.tgt'`.

### -o
Sets the name of the output file.

```
-o name
-oname
```

The name can be the next argument or follow `-o` directly. Do not use `:`
or `=` with this option: `-o:x.xex` names the file `:x.xex`.

The name applies to the next input file on the command line:

- Before an exBASIC source, it names the executable, and the assembly and
  object files take the same name with the extensions `.asm` and `.o`.
  `exbasic -o out.xex hello.bas` writes `out.asm`, `out.o` and `out.xex`.
- Before an assembly source, it names the object file of that source.
- After the last input file, it names only the executable:
  `exbasic hello.bas -o game.xex` compiles to `hello.asm` and `hello.o`
  and links `game.xex`.
- With `-c`, it names the assembly file: `exbasic -c -o x.asm hello.bas`.

The executable keeps exactly the name given, so `-o GAME.XEX` writes
`GAME.XEX` even when the target extension is `.xex`. Without `-o`, the
executable is named after the first input file with the target extension
(`.xex` or `.rom`) in place of its own.

Errors: `multiple '-o' option for the same file` if two `-o` options apply
to the same file, and `option '-o' must supply a file name` if `-o` is the
last argument.

### -native:hot, -native:all, -no-native
Select how much of the program is compiled to 6502 machine code.

| Option        | Code compiled to machine code                              |
|---------------|------------------------------------------------------------|
| (none)        | builds with `-native:all`, `-native:hot` and a lean selection, with and without the extended instructions, and keeps the smallest |
| `-native:hot` | only the code that runs often                              |
| `-native:all` | all the code that has a machine code version               |
| `-no-native`  | nothing: the whole program is bytecode                     |

Machine code covers the integer code: variables, constants, arithmetic,
comparisons, `PEEK`/`POKE`, arrays, `INC`/`DEC`, multiplication and
division, loop control and `FUNC` calls. Statements without a machine code
version, like `PRINT`, string operations and floating point, always stay
as bytecode.

`-no-native` takes precedence over the other two. The optimizer must be on
for machine code, so `-n` also turns it off. See [Native code
modes](#native-code-modes) for the details, and
[Performance and Memory](20-performance-and-memory.md) for how to choose.

### -no-ext
Doesn't use the extended bytecode instructions.

By default the optimizer replaces common sequences of bytecode
instructions with combined instructions that run faster: a single
instruction for `NEXT`, for the address of an array element (`A(I)`), for
storing a constant in an array element (`A(I) = 1`) and for a division by
a constant power of two (`X / 4`). Inside `FOR` loops that do not use
`PAUSE`, a division and a modulus with the same operands (`X / D` and
`X MOD D`) divide only once.

Each extended instruction a program uses adds its code, from 8 to about
120 bytes, to the runtime included in the program. Use `-no-ext` together
with `-no-native` to get the smallest program. Without `-no-ext`, the
default mode also tries the builds without the extended instructions and
keeps them when they are smaller (see [Native code
modes](#native-code-modes)). The bytecode interpreter
supports at most 127 different instructions in one program; the extended
instructions count towards that limit, and the compiler retries without
them when the link fails (see [Automatic retries](#automatic-retries)).

### -n
Doesn't run the optimizer.

The bytecode is written as the parser produces it, without the peephole
optimizer, without extended instructions and without machine code. The
result is bigger and much slower. This option is only useful to find
problems in the optimizer.

### -I
Adds a folder to the list of folders searched for `INCLUDE` files.

```
-I:path
```

The option can be repeated. A relative `INCLUDE` file name is searched in
this order:

1. the folder of the file that contains the `INCLUDE` line,
2. the folders given with `-I`, in the order given,
3. the `include` folder of the compiler installation (see
   [Search paths](#search-paths)).

An absolute file name is only tried as given.

```sh
exbasic -I:../lib -I:../common game.bas
```

### -g
Writes the label file and the assembly listings.

With `-g`, the linker writes a label file with the same name as the
executable and the extension `.lbl`, and the assembler writes a listing
for each assembled file, named like its object file with the extension
`.lst` (for `game.bas`, `game.lst`). These files are kept after the
compilation.

The label file lists every symbol of the program with its address, one
per line, in the format of the VICE emulator (`al 002507 .EXB_FREE`). It
includes a label for each line of the source (`@exBASIC_LINE_n`), the
start and size of each memory segment and the symbols of the runtime. It
is used by debuggers such as Altirra to show symbols, by the `exbprof`
profiler of the test suite and to measure the memory used by a program
(see [Performance and Memory](20-performance-and-memory.md#measuring-memory)).

### -keep
Keeps the intermediate files.

Normally the `.asm` and `.o` files are deleted after a successful
compilation. With `-keep` they stay. `-keep` also turns on `-g`, so the
`.lbl` and `.lst` files are written too. The intermediate files are also
kept when the compilation fails.

### -c
Only compiles the exBASIC source to an assembly file.

The compiler writes `prog.asm` and stops; it does not assemble or link,
and it does not delete the `.asm` file. Assembly files given on the same
command line are assembled to object files, but nothing is linked. Use
this option to look at the generated code or to build the program with
your own commands.

Because nothing is linked, the compiler cannot compare program sizes, and
the default native mode compiles as `-native:hot`. Give `-native:all` or
`-no-native` to get the other forms.

```sh
exbasic -c hello.bas
```

This writes `hello.asm`.

### -l
Writes a listing of the source with all abbreviations expanded.

```
-l
-l:extension
```

The listing has one statement per line, keywords and variable names in
uppercase, keywords in full, blocks indented by two spaces, and the
comments kept. It is
written next to the source, with the same name and the extension `.list`,
or the extension given after `-l:`. The program is compiled as usual.

The extension can't be empty or `bas` (which would overwrite the source):
`invalid BASIC listing extension`.

For this source, `prog.bas`, written with abbreviations:

```basic
' Add the numbers from 1 to 3
F.I=1 TO 3:Total=Total+I:N.I
I.Total>5 T. PRI. "BIG"
PRI. "TOTAL=";Total
```

`exbasic -l prog.bas` writes `prog.list`:

```basic
' Add the numbers from 1 to 3
FOR I = 1 TO 3
  TOTAL = TOTAL + I
NEXT I
IF TOTAL > 5 THEN PRINT "BIG"
PRINT "TOTAL=" ; TOTAL
```

The expanded listing is useful to read abbreviated programs and to find
the cause of an error in a line with many statements.

### -ls
Writes a listing in the shortest form instead: abbreviated keywords,
statements joined with `:`, no comments and no unneeded spaces.

```
-ls
-ls:columns
```

Each line of the listing has at most *columns* characters, from 1 to 256;
`-ls` alone uses 120. Lines end with the ATASCII end of line character
(`$9B`), so the file can be loaded on the Atari. The file name follows the
`-l` rules (`.list` by default, or the extension of a `-l:` option).

For the same `prog.bas`, `exbasic -l:txt -ls:40 prog.bas` writes
`prog.txt` with these two lines:

```
F.I=1T.3:TOTAL=TOTAL+I:N.I
I.TOTAL>5T.?"BIG":?"TOTAL=";TOTAL
```

The minimizer does not check the whole program: always compile the
minimized listing and check that it gives the same program. It also does
not remove extra parentheses or join `DATA` statements. A bad number gives
`'-ls' option needs line length from 1 to 256`.

### -prof
Shows statistics of the bytecode instructions used by the program: how
many times each instruction appears, each pair of consecutive
instructions, and the constants loaded most often. This helps to find
new combined instructions for the optimizer. It counts instructions in the
program, not executions; to see where a program spends its time, use the
`exbprof` profiler of the test suite (see
[Performance and Memory](20-performance-and-memory.md#profiling-with-exbprof)).

```sh
exbasic -prof hello.bas
```

Output (part):

```
BAS compile 'hello.bas' to 'hello.asm'
	1	TOK_CSTRING
	1	TOK_END
	1	TOK_PRINT_STR
	1	TOK_PRINT_STR	TOK_END
```

### -d
Prints a trace of the parser, with every syntax rule tried on every line,
to the standard output. It is very long and only useful to debug the
syntax files.

### -s
Places the compiled bytecode in the given linker segment.

```
-s:segment
```

The default segment is `BYTECODE`. The segment must exist in the linker
configuration, or the link fails with `Missing memory area assignment for
segment`. An empty name, or one with a `"`, gives `invalid segment name`.
This option is for custom linker configurations.

### -C
Uses a different linker configuration file.

```
-C:file.cfg
```

The default is the file named by the target: `exbasic.cfg` for `atari`
and `exbasic-cart.cfg` for `atari-cart`, found in the compiler folder. The
file given with `-C` is not searched: a relative name is relative to the
current folder. See [Linker configurations](#linker-configurations).

### -S
Sets the start address of the program in memory.

```
-S:address
```

The address can be decimal (`12288`), hexadecimal with `0x` (`0x3000`) or
hexadecimal with `$` (`$3000`; quote it in the shell). The default,
`$2000`, is set in the linker configuration. It leaves room for DOS 2.5
and similar DOSes. Use a higher address for a DOS that uses more memory,
or a lower one when the program runs without DOS, to get more free
memory. A program whose start is below the DOS's MEMLO is not loaded: it
prints `ERROR- 19` and returns to DOS (see [Appendix
B](B-error-messages.md#error-6-not-loaded)).

For cartridges, the code is always in the ROM at `$A000`; there `-S` sets
the start of the RAM used for variables, arrays and strings, which is also
`$2000` by default.

```sh
exbasic -S:0x2800 game.bas
```

### -X
Passes an option to the `ca65` assembler.

```
-X:option
```

Each `-X` passes one argument, and the argument cannot contain spaces. To
pass an option and its value, use either one `-X` with both joined, or two
`-X` options: `-X:-Ipath` and `-X:-I -X:path` are the same. Useful
options for your own assembly modules:

| Option           | Meaning                                      |
|------------------|----------------------------------------------|
| `-X:-I`*path*    | add a folder to search for `.include` files  |
| `-X:-D`*symbol*  | define an assembly symbol                    |
| `-X:-D`*sym*`=`*val* | define an assembly symbol with a value   |

### -DL
Defines a linker symbol.

```
-DL:symbol=value
```

The definition is passed to `ld65` with `--define`. It is used to change
values that the linker configuration declares as weak symbols. For
example, the cartridge configuration declares `__CARTFLAGS__`, the
cartridge option byte, with the default value `$05` (boot the disk, then
start the cartridge). `$04` starts the cartridge without booting the
disk:

```sh
exbasic -t:atari-cart -DL:__CARTFLAGS__=4 game.bas
```

### -target-path
Sets the list of folders where target files are searched.

```
-target-path:folder1:folder2
```

The folders are separated by `:` (also on Windows). The list replaces the
default search path (see [Search paths](#search-paths)). The syntax files,
runtime libraries and linker configurations are still searched in the
default path.

### -syntax-path
Sets the list of folders where the syntax files (`.syn`) named in the
target files are searched.

```
-syntax-path:folder1:folder2
```

The folders are separated by `:`. The default is the `syntax` folder of
each folder of the default search path. A syntax file that is not found
gives `can't open syntax file: 'name.syn'`.

## Native code modes

The compiler can compile the integer parts of a program to 6502 machine
code ("native code") instead of bytecode. Machine code runs several times
faster, but it is bigger than bytecode. On the other hand, bytecode needs
the interpreter routine of every instruction it uses, so a program that is
all machine code can be smaller than the same program as bytecode.

With `-native:hot`, the compiler estimates how many times each statement
runs, from the loops around it and the calls of the `FUNC` it is in, and
compiles only the statements that run at least 64 times and at least
1/64 as often as the most frequent ones. With `-native:all` it compiles
everything that has a machine code version.

The lean selection is the `-native:hot` selection without the loops where
the machine code would save less than 10% of the time, because most of
their time goes to floating point, strings or input and output, which
stay bytecode (see [Loops that gain
little](20-performance-and-memory.md#loops-that-gain-little)). It has no
option of its own.

Without a native option, the compiler builds the program six times, in
this order: with `-native:all`, with `-native:hot` and with the lean
selection, first with the extended instructions and then without them
(only without them if `-no-ext` is given). It notes the size of each
output file and keeps the smallest. On a tie it keeps the first one, the
faster. When the kept build is not the first one, it prints a message
and builds the program again with that selection (unless it was the last
build). The message gives the size kept and the size of the first build,
for example:

```
Using machine code only for the code that runs often, without extended instructions (3904 bytes instead of 5467).
Using machine code only where it saves time, without extended instructions (2995 bytes instead of 5071).
```

The comparison uses the size of the `.xex` or `.rom` file. The repeated
builds happen only when the optimizer is on, the program has an exBASIC
source and the compiler links (not with `-c`). With `-native:hot`,
`-native:all` or `-no-native` the program is built once.

When the program has a compile error, the compiler stops after the first
build and reports the error once.

The following table shows the effect of the modes on two sample programs
(`samples/sieve.bas` and `samples/pi.bas`). The size is the `.xex` file in
bytes; the time is the number of 6502 cycles measured with `exbprof`.

| Program   | Mode          | Size | Cycles      |
|-----------|---------------|-----:|------------:|
| sieve.bas | default (all) | 1434 |  27,139,888 |
| sieve.bas | `-native:hot` | 1570 |  27,433,413 |
| sieve.bas | `-no-native`  | 1443 | 100,521,491 |
| pi.bas    | default (hot, no extended instructions) | 3904 |  47,034,906 |
| pi.bas    | `-native:hot` | 4156 |  38,828,093 |
| pi.bas    | `-native:all` | 5467 |  37,187,653 |
| pi.bas    | `-no-native`  | 2761 | 100,703,282 |

For the sieve, all machine code is the smallest build and also the
fastest. For the pi program, the default keeps the hot code without the
extended instructions: it is 252 bytes smaller and 21% slower than
`-native:hot`, and 1563 bytes smaller and 26% slower than `-native:all`.

## Automatic retries

If the link fails, the compiler tries again with smaller code before
reporting the error:

1. If the extended instructions were on, it prints `LINK failed, retrying
   without extended instructions.` and builds again with `-no-ext`.
2. If that also fails and machine code was on, it prints `LINK failed,
   retrying without machine code.` and builds again with `-no-native`
   (and still without extended instructions).
3. If the last build also fails, it stops with `exbasic: can't link file`.

The retries handle two problems: a very big program that would use more
than the 127 different bytecode instructions the interpreter supports, and
a program that is a little too big for the memory available. The retries
are only done when the optimizer is on. When a retry succeeds, the program
is valid but slower than usual: compile with `-g` and check the messages if
speed matters.

## The build steps

For `exbasic -g prog.bas`, the compiler runs these steps.

**1. Compile.** The compiler reads `prog.bas` and the files it includes,
checks the syntax, optimizes the bytecode, compiles the selected code to
machine code and writes `prog.asm`. The assembly file contains the list
of runtime routines the program needs, the variables, the bytecode and the
machine code, with a label for each source line.

**2. Assemble.** It runs the `ca65` assembler:

```sh
ca65 -I <compiler>/asminc -o prog.o -l prog.lst -g -tatari prog.asm
```

`-l prog.lst` is only added with `-g` or `-keep`. Options given with `-X`
go after `-g`, followed by the options of the target (`-tatari`). The
`asminc` folder holds the include files with the Atari hardware and OS
definitions (`atari.inc` and others) and `target.inc`, included by the
generated code. Assembly files from the command line are assembled in the
same way.

**3. Link.** It runs the `ld65` linker:

```sh
ld65 -C <compiler>/exbasic.cfg -o prog.xex -Ln prog.lbl prog.o <compiler>/exbasic.lib
```

`-Ln prog.lbl` is only added with `-g` or `-keep`. Options from `-S`
(`--start-addr`) and `-DL` (`--define`) go before the object files. The
linker takes from the runtime library `exbasic.lib` only the routines the
program uses, so a program that never uses floating point, graphics or
files does not include their code.

**4. Clean up.** On success the compiler deletes `prog.asm` and `prog.o`,
unless `-keep` or `-c` was given.

Floating point support is included automatically, only when the program
uses floating point numbers: the compiler then adds the floating point
initialization to the program, and the linker adds the floating point
routines.

## Output files

| File        | Written                       | Contents                                     |
|-------------|-------------------------------|----------------------------------------------|
| `prog.xex`  | always (`atari` target)       | Atari DOS executable                         |
| `prog.rom`  | always (`atari-cart` target)  | 8 KB cartridge image                         |
| `prog.lbl`  | with `-g` or `-keep`          | labels, in VICE format                       |
| `prog.lst`  | with `-g` or `-keep`          | assembler listing of `prog.asm`              |
| `prog.list` | with `-l` or `-ls`            | expanded or minimized BASIC listing          |
| `prog.asm`  | deleted, kept with `-keep`, `-c` or on errors | generated assembly source    |
| `prog.o`    | deleted, kept with `-keep` or on errors | assembled object file              |

All files except the `.list` listing are written next to the name of the
executable (or next to the source, without `-o`). The listing is always
written next to the source. The `.rom` image is a raw 8192-byte file, the
contents of the cartridge from `$A000` to `$BFFF`; emulators load it as a
"standard 8 KB" cartridge (a `.car` file needs a header, which you can add
with a cartridge tool of your emulator).

## Targets and target files

A target file is a text file with one command per line. Blank lines and
lines starting with `#` are ignored. The commands are:

| Command     | Arguments            | Meaning                                            |
|-------------|----------------------|----------------------------------------------------|
| `include`   | target name          | reads another target file (`.tgt` added if no extension) |
| `syntax`    | list of `.syn` files | adds syntax files, read in order and merged        |
| `library`   | file name            | runtime library to link; the last one read is used |
| `config`    | file name            | linker configuration; the last one read is used    |
| `extension` | for example `.xex`   | extension of the output file                       |
| `ca65`      | list of options      | options added to the assembler command             |

Any other command stops the compiler with `Bad key 'name' in target file
'file'`. These are the included target files:

```
# Default target: Atari 8-bit executable
include atari
```

```
# Atari 8-bit computers, executable (XEX) file
syntax a800.syn basic.syn dli.syn fileio.syn pm.syn graphics.syn sound.syn extended.syn sio.syn exbasic.syn exbasic-a800.syn float.syn fujinet.syn exbasic-fp.syn
config exbasic.cfg
ca65 -tatari
library exbasic.lib
extension .xex
```

```
# Atari 8-bit computers, cartridge (ROM) image
include atari
config exbasic-cart.cfg
library exbasic-cart.lib
extension .rom
```

The language itself is defined by the syntax files, so a new target can
add statements and functions. For example, a target file
`atari-extra.tgt` in the compiler folder with these lines gives the
`atari` language plus the definitions in `console.syn`:

```
include atari
syntax console.syn
```

The file `compiler/USAGE.md` of the distribution explains the syntax file
format and has complete examples of new functions and statements.

## Linker configurations

The linker configuration file tells `ld65` where each part of the
program goes in memory. The compiler includes two.

### exbasic.cfg (executables)

| Memory area | Address          | Contents                                       |
|-------------|------------------|------------------------------------------------|
| `ZP`        | `$9C`-`$D3`      | zero page variables of the runtime             |
| `INTERP`    | `$80`-`$9B`      | the interpreter main loop, loaded in zero page |
| `MAIN`      | start to `$BC1F` | the program                                    |

The `MAIN` area starts at the start address (`$2000`, or the `-S` value)
and can reach `$BC20`. A program that does not fit gives a link error
such as `Segment 'RUNTIME' overflows memory area 'MAIN' by 704 bytes`.

The segments of `MAIN`, in memory order:

| Segment     | Contents                                                          |
|-------------|-------------------------------------------------------------------|
| `JUMPTAB`   | the interpreter jump table, page aligned                          |
| `HEAP`      | the program variables, in the page of the jump table if they fit  |
| `RUNTIME`   | the interpreter routines                                          |
| `DATA`      | the interpreter data                                              |
| `BYTECODE`  | the program bytecode and machine code (see `-s`)                  |
| `CODE`      | code of assembly modules linked with the program, `DATA ... ROM` arrays |
| `ALIGNDATA` | page aligned data (optional)                                      |
| `BSS`       | uninitialized runtime data, like the `FUNC` frame stack and P/M state |
| `HEAPA`     | the variables, when they don't fit in the jump table page         |
| `ENDMARK`   | the label `EXB_FREE`: start of the free memory                    |

Arrays and strings are allocated at run time from `EXB_FREE` up to
`MEMTOP`, the top of free memory set by the OS. The executable file loads
everything from the start address to the end of the program in one block,
plus the zero page interpreter.

### exbasic-cart.cfg (cartridges)

| Memory area | Address          | Contents                                          |
|-------------|------------------|---------------------------------------------------|
| `ROM`       | `$A000`-`$BFF9`  | jump table, runtime, bytecode, data to copy to RAM |
| `CARTID`    | `$BFFA`-`$BFFF`  | cartridge header: start address and option byte   |
| `MAIN`      | start, `$8000` bytes | RAM: variables, runtime data, arrays and strings |
| `INTERP`    | `$80`-`$9B`      | interpreter loop, copied from the ROM at start    |
| `ZP`        | `$9C`-`$D3`      | zero page variables                               |

The whole program, with the runtime it uses, must fit in the 8 KB
cartridge. Unused ROM bytes are filled with `$FF`. The option byte at
`$BFFD` comes from the linker symbol `__CARTFLAGS__` (default `$05`, see
[-DL](#-dl)).

## Search paths

The compiler looks for its own files (target files, syntax files, the
runtime libraries, the default linker configurations, `ca65`, `ld65`, the
`asminc` folder and the `include` folder for `INCLUDE`) in these folders,
in order:

1. the folder in the `EXBASIC_HOME` environment variable, if it is set,
2. the folder of the `exbasic` program itself,
3. on Linux and other Unix systems, `/usr/local/share/exbasic`,
4. then `/usr/share/exbasic`.

The first folder that has the file is used. Syntax files are searched in
the `syntax` subfolder of each of these folders. So the simplest
installation is to keep the whole compiler folder together (as `make`
builds it in `build/compiler`) and call `exbasic` from there or through
a link in your `PATH`.

The compiler folder contains:

| File                                | Purpose                                 |
|-------------------------------------|-----------------------------------------|
| `exbasic`                           | the compiler                            |
| `ca65`, `ld65`, `ar65`              | assembler, linker and librarian (CC65)  |
| `default.tgt`, `atari.tgt`, `atari-cart.tgt` | target files                   |
| `exbasic.cfg`, `exbasic-cart.cfg`   | linker configurations                   |
| `exbasic.lib`, `exbasic-cart.lib`   | runtime libraries                       |
| `syntax/`                           | syntax files (the language definition)  |
| `asminc/`                           | include files for the assembler         |
| `mkatr`, `lsatr`                    | tools to make and list ATR disk images  |
| `USAGE.md`, `manual/`, `LICENSE`    | documentation and license               |

## Environment variables

| Variable                  | Effect                                                         |
|---------------------------|----------------------------------------------------------------|
| `EXBASIC_HOME`            | first folder searched for the compiler files                   |
| `EXBASIC_HOT_DEBUG`       | if set, prints the estimated iterations of each loop and calls of each `FUNC`, and the selection limit, for `-native:hot`; and for the lean selection, the cycles each loop saves and the choice |
| `EXBASIC_HOT_RATIO`       | changes the 1/64 ratio of `-native:hot` (default `64`)         |
| `EXBASIC_HOT_MIN`         | changes the minimum of 64 estimated executions (default `64`)  |
| `EXBASIC_NATIVE_STATS`    | if set, prints the bytes of machine code generated for each instruction |
| `EXBASIC_PEEPHOLE_DEBUG`  | if set, prints the rules applied by the peephole optimizer     |

Only `EXBASIC_HOME` is meant for normal use. The others are tools for
experiments and for the development of the compiler; their output goes to
the standard error and they work in every mode (with the default mode
their output appears once for each build). For example:

```sh
EXBASIC_HOT_DEBUG=1 exbasic -native:hot sieve.bas
```

Output:

```
func  called 1 times
loop at line 12: 16 trips, weight 2.0969e+06
loop at line 9: 8191 trips, weight 131056
loop at line 6: 16 trips, weight 16
max weight 2.0969e+06, compiling from 32764
BAS compile 'sieve.bas' to 'sieve.asm'
ASM assemble 'sieve.asm' to 'sieve.o'
LINK sieve.xex
```

[Performance and Memory](20-performance-and-memory.md#how-the-compiler-selects-the-hot-code)
explains these numbers.

## Messages and exit codes

The compiler exits with code 0 when the program was built (and after
`-h` or `-v`), and with code 1 on any error: a bad option, a syntax error,
a missing file, an assembler error or a link error. Errors in the
program are shown with the file, line and column; see
[Error Messages](B-error-messages.md) for the full list.

Errors of the command line and of the files start with `exbasic:`:

| Message                                              | Cause                                    |
|------------------------------------------------------|------------------------------------------|
| `missing input file name`                            | no input file given                      |
| `invalid option '-x', try -h for help`               | unknown option                           |
| `invalid argument, try -h for help`                  | an empty argument (`""`)                 |
| `multiple '-o' option for the same file`             | two `-o` before the same file            |
| `option '-o' must supply a file name`                | `-o` at the end of the command line      |
| `invalid BASIC listing extension`                    | `-l:` with an empty extension or `bas`   |
| `'-ls' option needs line length from 1 to 256`       | bad `-ls` number                         |
| `invalid segment name`                               | `-s:` empty or with `"`                  |
| `invalid compiler target name`                       | `-t:` empty or with `"`                  |
| `can't open input file 'name'`                       | the source file does not exist           |
| `can't open output file 'name'`                      | the `.asm` file can't be written         |
| `can't open listing file 'name'`                     | the listing file can't be written        |
| `only one BASIC file can be linked into a program, 'x.bas' is the second one; add it with INCLUDE "x.bas" instead` | two BASIC files on the command line |
| `prog.bas: too many variables, they use N bytes and the limit is 512 ...` | the variables need more than 512 bytes; use arrays |
| `can't assemble file`                                | `ca65` reported an error                 |
| `can't link file`                                    | `ld65` reported an error, after the retries |

Errors of the assembler and the linker are printed by `ca65` and `ld65`
just before the compiler message.

## Using the compiler from make

The compiler works well from a `Makefile`. List the included files as
dependencies so the program is rebuilt when any of them changes:

```make
EXBASIC = /path/to/exbasic
EXBFLAGS =

all: game.xex

game.xex: game.bas util.bas
	$(EXBASIC) $(EXBFLAGS) $< -o $@

clean:
	rm -f game.xex game.lbl game.lst
```

Here `game.bas` includes `util.bas`. `make` builds `game.xex`, and
`make EXBFLAGS=-no-native` builds it without machine code. Since the
compiler returns 1 on errors, `make` stops at the first program that
does not compile. With `-g` in `EXBFLAGS` you also get the label file for
the emulator's debugger.
