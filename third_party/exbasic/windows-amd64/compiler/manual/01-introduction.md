# 1. Introduction

This chapter describes what exBASIC is, where it comes from, how it turns a
BASIC program into an Atari program, what you need to use it, and how the rest
of this manual is organized. It ends with the notation used in all the
reference entries.

## What exBASIC is

exBASIC is a compiled BASIC for the Atari 8-bit home computers: the Atari
400 and 800, the XL series and the XE series. You write programs on a modern
computer (Linux, Windows or macOS) with any text editor. The `exbasic`
compiler, which also runs on that computer, turns the program into an Atari
executable file (`.XEX`) or an 8 KB cartridge image (`.ROM`). The result runs
on a real Atari or in an emulator such as Altirra or Atari800.

There is no editor or interpreter on the Atari side. The Atari only runs the
finished program, which contains everything it needs, so you can give the
`.XEX` file to anyone.

A short example:

```basic
' Print a table of squares and cubes
FOR Number = 1 TO 5
  PRINT Number, Square(Number), Number * Square(Number)
NEXT Number

' Returns the square of a number
FUNC Square(Value)
  RETURN Value * Value
ENDFUNC
```

Output:

```
1       1         1
2       4         8
3       9         27
4       16        64
5       25        125
```

(The comma in `PRINT` moves to the next column that is a multiple of 10.
The screen has a left margin of two columns, so the first column is eight
characters wide. See [chapter 10](10-console-io.md).)

## Main features

- No line numbers. Programs use structured control flow: `IF` / `ELIF` /
  `ELSE` / `ENDIF`, `FOR` / `NEXT`, `WHILE` / `WEND`, `REPEAT` / `UNTIL`,
  `DO` / `LOOP` with `WHILE` or `UNTIL` conditions at either end, and `EXIT`.
- Functions with `FUNC` / `ENDFUNC`: typed parameters, local variables
  (`LOCAL`), one or several results, `RETURN`, recursion, and calls by name
  (`X = F(1)`, `A$ = G$()`, `Q, R = DivMod(7, 2)`, or `F(1)` as a statement).
  Functions can be called before they are defined.
- Integer arithmetic by default: variables without a suffix are 16-bit
  signed integers, which the 6502 handles quickly.
- Floating point variables (names ending in `%`) using the Atari 6-byte BCD
  format and the floating point routines of the Atari operating system.
- String variables (names ending in `$`) of up to 255 characters, string
  arrays, slicing with `[start, length]`, concatenation with `+`, and the
  string functions `LEFT$`, `RIGHT$`, `MID$`, `INSTR`, `UCASE$`, `LCASE$`,
  `LTRIM$` and `RTRIM$`.
- Arrays of words, bytes, floating point numbers and strings, and constant
  arrays defined with `DATA`, which can also be read from binary files at
  compile time or placed in ROM.
- All the graphics, sound and input statements of Atari BASIC, plus
  extensions taken from Turbo-BASIC XL (`FILLTO`, `PAUSE`, `MOVE`, `DPOKE`,
  `BGET` / `BPUT`, and others).
- Player/missile graphics, display list interrupts, direct SIO calls and
  FujiNet network statements.
- `INCLUDE "file"` to split a program into several source files, and
  `COMMON` with `RUN "D:FILE.XEX"` to chain programs that keep their
  variables.
- Machine code generation for the integer code that runs often, which makes
  it several times faster than interpreted code.
- Linking of your own 6502 assembly modules into the program.

## History

exBASIC is based on FastBasic, a fast BASIC for the Atari 8-bit computers
written by Daniel Serpell (dmsc) between 2017 and 2025. FastBasic introduced
the integer-first language without line numbers, the compact bytecode, the
small interpreter and the cross compiler on which exBASIC is built.

exBASIC 1.0.0 keeps the FastBasic compiler design and most of its language,
and changes the following:

- It is a PC compiler only. There is no editor or compiler running on the
  Atari, and no Atari 5200 target.
- Subroutines are replaced by functions (`FUNC` / `ENDFUNC`) with typed
  parameters, local variables, multiple results and recursion.
- New statements and features: `DO WHILE` / `LOOP UNTIL` forms, string
  concatenation with `+`, `INCLUDE`, `COMMON` and `RUN`, `STATUS`.
- The compiler generates 6502 machine code for the integer code that runs
  often (see [How exBASIC works](#how-exbasic-works)).
- Floating point support is added to a program automatically, only when the
  program uses it.

If you know FastBasic, Atari BASIC or Turbo-BASIC XL, see
[Appendix C](C-differences.md) for the complete list of differences and tips
for porting programs.

## How exBASIC works

Compiling a program has three steps. The `exbasic` command runs all of them
for you:

1. The compiler reads the BASIC source and translates it to an assembly
   language file. The language itself is described by grammar files (the
   `.syn` files in the `syntax` folder of the compiler), which the compiler
   reads each time it runs.
2. The CA65 assembler (from the CC65 tools, included with exBASIC) assembles
   that file to an object file.
3. The LD65 linker joins the object file with the exBASIC runtime library
   and writes the `.XEX` or `.ROM` file. Only the parts of the runtime that
   the program uses are included, so a small program gives a small file.

The compiled program has three parts:

- **Bytecode.** Most statements are translated to a compact bytecode: a
  sequence of one-byte instructions ("tokens") with their arguments. A small
  and fast interpreter, part of the runtime and partly located in zero page,
  executes the bytecode. Bytecode is about as compact as tokenized BASIC.
- **Native code.** The integer parts of the program (variables, constants,
  arithmetic, comparisons, `PEEK` / `POKE`, arrays, `INC` / `DEC`, loop
  control and `FUNC` calls) can also be compiled to 6502 machine code, which
  runs 2 to 6 times faster than bytecode but takes more memory. Statements
  without a machine code version, such as `PRINT`, string operations and
  floating point, stay as bytecode; the machine code calls the interpreter
  for them and continues afterwards.
- **Runtime.** The interpreter, the routines for each bytecode instruction
  that the program uses, and the support code for input/output, graphics,
  sound and floating point.

By default the compiler builds the program six times: with machine code
for everything possible (`-native:all`), only for the statements that run
often (`-native:hot`), and with that selection less the loops where machine
code saves little time; each one with and without the extended
instructions. It keeps the smallest.
The options `-native:hot`, `-native:all` and `-no-native` select one mode
directly. [Chapter 19](19-compiler-reference.md) describes the options and
[chapter 20](20-performance-and-memory.md) explains how to get the best speed
and size.

Floating point support (the `%` variables and functions such as `SIN`) is
linked in only when the program uses it, so integer programs stay small.

## Requirements

To build and use the compiler you need:

- A computer running Linux, Windows or macOS.
- To build exBASIC from source: a C and a C++ compiler (for example GCC or
  Clang) and GNU make. See [chapter 2](02-getting-started.md).
- A text editor. Any editor that saves plain text works; the compiler
  accepts Unix (LF) and Windows (CR LF) line endings, and Atari (ATASCII
  EOL, byte 155) line endings.
- An emulator, such as Altirra or Atari800, to test programs quickly.

To run compiled programs you need:

- An Atari 400, 800, XL or XE computer, or an emulator. Executables load at
  address `$2000` by default and can use memory up to `$BC1F`, so 48 KB of
  RAM or more is recommended. On XL and XE computers, turn off the built-in
  BASIC (hold OPTION while the computer starts) so the program can use the
  memory from `$A000` to `$BFFF`.
- A way to load a `.XEX` file: a DOS disk, a bootable disk image made with
  the included `mkatr` tool, or a loader in your SIO device or emulator.
  Cartridge images need a cartridge emulator, a flash cartridge or an
  EPROM.

## How this manual is organized

| Chapter | Contents |
| --- | --- |
| [1. Introduction](01-introduction.md) | This chapter. |
| [2. Getting started](02-getting-started.md) | Building the compiler, the first program, running it in an emulator and on real hardware. |
| [3. Language basics](03-language-basics.md) | Lines, statements, comments, names, keywords and abbreviations, numbers, errors. |
| [4. Variables and data](04-variables-and-data.md) | Integer, floating point and string variables, arrays, `DIM`, `DATA`, `CLR`, `INC`, `DEC`. |
| [5. Expressions](05-expressions.md) | Operators, precedence, integer arithmetic rules, conversions, standard functions, the list of all functions. |
| [6. Control flow](06-control-flow.md) | `IF`, `FOR`, `WHILE`, `REPEAT`, `DO` / `LOOP`, `EXIT`, `END`. |
| [7. Functions](07-functions.md) | `FUNC`, parameters, results, `RETURN`, `LOCAL`, recursion. |
| [8. Strings](08-strings.md) | String constants, operators, slicing and string functions. |
| [9. Floating point](09-floating-point.md) | Floating point numbers, operators and functions. |
| [10. Console input and output](10-console-io.md) | `PRINT`, `INPUT`, `GET`, `PUT`, `POSITION`, `CLS`, the keyboard. |
| [11. Files and devices](11-files-and-devices.md) | The Atari CIO system, `OPEN`, `CLOSE`, `XIO`, `NOTE`, `POINT`, `BGET`, `BPUT`, `STATUS`, error codes. |
| [12. Graphics](12-graphics.md) | Graphics modes, `PLOT`, `DRAWTO`, `FILLTO`, `SETCOLOR`, and others. |
| [13. Player/missile graphics](13-player-missile-graphics.md) | `PMGRAPHICS`, `PMADR`, `PMHPOS`. |
| [14. Display list interrupts](14-display-list-interrupts.md) | `DLI SET`, `DLI`. |
| [15. Sound and controllers](15-sound-and-controllers.md) | `SOUND`, joysticks, paddles, `TIME`, `PAUSE`, `RAND`. |
| [16. Memory and machine code](16-memory-and-machine-code.md) | `PEEK`, `POKE`, `MOVE`, `USR`, assembly modules, the memory map. |
| [17. Program organization](17-program-organization.md) | `INCLUDE`, `COMMON`, `RUN`, structuring large programs. |
| [18. SIO and FujiNet](18-sio-and-fujinet.md) | `SIO`, `SERR()`, the FujiNet `N:` statements. |
| [19. Compiler reference](19-compiler-reference.md) | All command line options, targets and output files. |
| [20. Performance and memory](20-performance-and-memory.md) | What is fast, native code, memory budget, tips. |
| [Appendix A](A-keyword-reference.md) | Alphabetical list of all keywords. |
| [Appendix B](B-error-messages.md) | Compiler and runtime error messages. |
| [Appendix C](C-differences.md) | Differences from Atari BASIC, Turbo-BASIC XL and FastBasic. |

If you are new to exBASIC, read chapters 2 to 7 in order. The later chapters
are a reference for each area of the language and can be read in any order.

## Conventions used in this manual

### Statement and function entries

Each statement and function is described in an entry like this one:

> ### PLOT
> Plots a single point.
>
> ```
> PLOT x, y
> ```
> Abbreviation: `PL.`

The first line says what the statement does. The box shows its syntax. The
abbreviation is the shortest form you can type (see
[Keywords and abbreviations](03-language-basics.md#keywords-and-abbreviations)).
Then follow the description of each parameter, the behavior and the limits,
and an example.

### Syntax notation

In the syntax boxes:

| Notation | Meaning |
| --- | --- |
| `PLOT` | Keywords are written in uppercase. Type them as shown, in uppercase or lowercase, or abbreviated. |
| `x`, `addr`, `text$` | Lowercase words are placeholders for values you supply. |
| `[ ... ]` | The part between brackets is optional. In string slicing, `[` and `]` are also part of the language; the text says so when that is the case. |
| `a \| b` | Write one of the alternatives. |
| `...` | The previous item can be repeated. |
| `#chan` | An I/O channel number, written with `#` (see [chapter 11](11-files-and-devices.md)). |

The names of placeholders show what kind of value is expected:

| Placeholder | Expected value |
| --- | --- |
| `n`, `x`, `y`, `value`, `addr`, ... | An integer expression (-32768 to 32767, or 0 to 65535 for addresses). |
| `cond` | A condition: a comparison or logical expression, or an integer (0 is false, any other value is true). |
| `fp` or a name ending in `%` | A floating point expression. |
| `text$` or a name ending in `$` | A string expression. |
| `var` | A variable name or an array element. |

### Examples and output

Examples are complete programs that you can compile as they are. Keywords
are written in uppercase. Variable names are sometimes written in mixed case
(`Count`, `NumIter`) for readability; exBASIC ignores case, so `COUNT`,
`Count` and `count` are the same variable.

When an example prints text, the output is shown after it under the word
"Output:". The output shown was produced by compiling the example and running
it. Commands typed in a terminal on your computer are shown like this:

```sh
exbasic hello.bas
```

### Numbers

Hexadecimal numbers are written with a `$` prefix, as in exBASIC programs:
`$2000` is 8192. Addresses of Atari hardware registers are given in both
forms where useful, for example `$D01F` (53279).
