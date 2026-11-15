# Appendix C. Differences from Other BASICs

This appendix is for programmers who know Atari BASIC, Turbo BASIC XL or
FastBasic. It lists what works differently in exBASIC and what is
missing, and gives tips to port programs. The examples written in other
BASICs are shown for comparison only and are marked as such; all exBASIC
examples compile and their output is real.

## Summary

| Feature | Atari BASIC | Turbo BASIC XL | FastBasic | exBASIC |
|---------|-------------|----------------|-----------|---------|
| Where programs are written | on the Atari | on the Atari | Atari IDE or PC | PC, any editor |
| How programs run | interpreted | interpreted | compiled to bytecode | compiled to bytecode and 6502 machine code |
| Line numbers | required | required | none | none |
| `GOTO`, `GOSUB` | yes | yes, also to `#labels` | no | no |
| Default number type | floating point | floating point | integer | integer |
| Floating point variables | all | all | names ending in `%` | names ending in `%` |
| Procedures | no | `PROC`/`EXEC` | `PROC`/`EXEC`/`@` | `FUNC` with typed parameters and results |
| Strings | `DIM` needed, up to 32767 chars | same | automatic, up to 255 | automatic, up to 255 |
| Joining strings | no | no | `=+` only | `+` and `=+` |
| Substrings | `A$(start, end)` | same | `A$[start, len]` | `A$[start, len]`, `LEFT$`, `RIGHT$`, `MID$` |
| `A`, `A$`, `A%` | different variables | different variables | one name, one type | different variables |
| `READ` / `DATA` | yes | yes | `DATA` arrays | `DATA` arrays |
| Error handling | `TRAP` | `TRAP`, `ERR`, `ERL` | `ERR()` | `ERR()` |
| One-line `IF ... THEN` | rest of the line | rest of the line | one statement | rest of the line, optional `ELSE` |
| `AND`, `OR` | both sides evaluated | both sides evaluated | both sides evaluated | right side only when needed |
| `XIO` order | `XIO cmd, #ch, ...` | same | `XIO #ch, cmd, ...` | `XIO cmd, #ch, ...` |
| Division by zero | error | error | no error | error, stops the program |
| Source files | one | one | one | several, with `INCLUDE` |
| Chaining programs | `RUN "D:FILE"` | same | no | `RUN "D:FILE.XEX"` with `COMMON` variables |

## Differences from Atari BASIC

### Writing and running programs

exBASIC has no editor or immediate mode on the Atari. You write the
program as a text file on a modern computer, compile it with `exbasic`
and run the resulting `.xex` file from DOS or in an emulator (see
[Getting Started](02-getting-started.md)). The commands of the Atari
BASIC editor, `LIST`, `ENTER`, `LOAD`, `SAVE`, `CLOAD`, `CSAVE`, `NEW`,
`CONT`, `BYE`, `DOS` and `RUN` without a file name, do not exist.

### No line numbers

Lines have no numbers, so there is no `GOTO`, `GOSUB`, `RETURN` to a
line, `ON ... GOTO`, `ON ... GOSUB`, `POP` or `RESTORE` to a line. Use
the structured statements instead:

| Atari BASIC | exBASIC |
|-------------|---------|
| `GOTO` back to repeat code | `DO`/`LOOP`, `REPEAT`/`UNTIL`, `WHILE`/`WEND` |
| `GOTO` forward to skip code | `IF`/`ELSE`/`ENDIF` |
| `GOTO` out of a loop | `EXIT` |
| `GOSUB` / `RETURN` | `FUNC` / `ENDFUNC`, called by name |
| `ON X GOTO 100, 200` | `IF X = 1` ... `ELIF X = 2` ... `ENDIF` |
| `IF X THEN 100` | `IF X` ... `ENDIF` |

A subroutine called with `GOSUB` becomes a `FUNC`. Atari BASIC (for
comparison only):

```
10 FOR N=1 TO 3
20 GOSUB 100
30 NEXT N
40 END
100 PRINT "SQUARE OF ";N;" IS ";N*N
110 RETURN
```

exBASIC:

```basic
' Print a table of squares with a FUNC instead of GOSUB
FOR Number = 1 TO 3
  ShowSquare(Number)
NEXT Number

' Print one line of the table
FUNC ShowSquare(Value)
  PRINT "SQUARE OF "; Value; " IS "; Value * Value
ENDFUNC
```

Output:

```
SQUARE OF 1 IS 1
SQUARE OF 2 IS 4
SQUARE OF 3 IS 9
```

See [Control Flow](06-control-flow.md) and [Functions](07-functions.md).

### Integer arithmetic by default

In Atari BASIC every number is floating point. In exBASIC, variables
without a suffix are 16-bit integers from -32768 to 32767, and
arithmetic on them is integer arithmetic: division drops the fraction,
and results that don't fit wrap around. Variables whose names end in `%`
are floating point:

```basic
' Integer arithmetic is the default
PRINT 7 / 2; " "; -7 / 2; " "; 7 MOD 3
Ratio% = 7 / 2
PRINT Ratio%
```

Output:

```
3 -3 1
3.5
```

When you port a program, decide for each variable whether it needs
fractions or big values. Counters, coordinates, screen addresses and
`PEEK`/`POKE` values are integers; money with cents, angles and physics
need `%`. Integer code is much faster (see [Performance and
Memory](20-performance-and-memory.md)). Addresses above 32767, like
`53279`, are fine as integer constants: they are stored as negative
numbers but work with `PEEK`, `POKE` and the other memory statements.

Exponentiation `^` exists only for floating point values. There is no
`CLOG`; use `LOG10`. `RND(0)` is `RND()` in exBASIC, and `RAND(N)` gives
a random integer from 0 to N - 1, which is what most programs compute
with `INT(RND(0)*N)`.

### Names and types

As in Atari BASIC, `A`, `A$` and `A%` are different variables: the suffix
is part of the name. In exBASIC `A` is an integer and `A%` is the
floating point variable. The arrays `A()`, `A$()` and `A%()` are also
different. A variable and an array with the same name and suffix, like
`A` and `A()`, can't both exist: rename one of them when you port a
program.

Keywords are recognized only as whole words, so names like `COLORS`,
`PRINTER` or `TOTAL` are ordinary variable names. Write a space between
a keyword and a name that follows it: `FOR I = 1 TO N`, not `FORI=1TON`.

### Strings

Strings don't need `DIM`. Every string variable holds up to 255
characters and takes 256 bytes of memory once assigned. `DIM A$(10)` does
not set the size of a string: it creates an array of 11 strings, `A$(0)`
to `A$(10)`. Remove the `DIM` of simple strings when porting.

Substrings use square brackets, with a start position and a length
instead of an end position:

| Atari BASIC | exBASIC |
|-------------|---------|
| `A$(3,5)` | `A$[3, 3]` |
| `A$(4)` | `A$[4]` |
| `A$(I,I)` | `A$[I, 1]` |

```basic
' Atari BASIC A$(3,5) is A$[3, 3] in exBASIC
Word$ = "COMPUTER"
PRINT Word$[3, 3]
PRINT Word$[4]
PRINT Word$[1, 1] + Word$[8]
```

Output:

```
MPU
PUTER
CR
```

The functions `LEFT$`, `RIGHT$`, `MID$`, `INSTR`, `UCASE$`, `LCASE$`,
`LTRIM$` and `RTRIM$` work as in other BASICs, with positions starting at
1. You can't assign to a substring with brackets; use the statement
`MID$(A$, Start, Count) = Text$` to replace characters, `+` to join the
parts or `=+` to append. `ADR(A$)` returns the address of the length
byte, one byte before the first character. See [Strings](08-strings.md).

### DATA

There is no `READ` or `RESTORE`. A `DATA` statement defines a named
array of constants that you index like any array:

```basic
' Add up a table of prices kept in a DATA array
DATA Prices() WORD = 120, 250, 75
Total = 0
FOR I = 0 TO 2
  Total = Total + Prices(I)
NEXT I
PRINT "TOTAL: "; Total
```

Output:

```
TOTAL: 445
```

Replace each group of `DATA` lines read in a loop with a `DATA` array,
and the `READ` with an index that you increment. See
[DATA](04-variables-and-data.md#data).

### IF ... THEN and ELSE

As in Atari BASIC, everything after `THEN` up to the end of the line runs
only when the condition is true. exBASIC also accepts an `ELSE` on the
same line:

```basic
' THEN applies to the rest of the line
Lives = 0
IF Lives = 0 THEN PRINT "GAME OVER" : PRINT "THE END"
IF Lives > 0 THEN PRINT "PLAYING" ELSE PRINT "NO LIVES LEFT"
```

Output:

```
GAME OVER
THE END
NO LIVES LEFT
```

`IF X THEN 100` (a jump to a line) does not exist: write an `IF` ...
`ENDIF` block instead. A loop opened after `THEN` must end on the same
line.

### AND and OR

In Atari BASIC both sides of `AND` and `OR` are always evaluated. In
exBASIC the right side is evaluated only when it is needed, so
`IF I <= 10 AND Table(I) = 0` never reads `Table(11)`, and a `FUNC` on
the right side is not called when the left side already decides the
result. `AND`, `OR` and `NOT` give 1 or 0; the bitwise operators are `&`,
`!` and `EXOR`, and `MOD` works on integers.

### Errors

There is no `TRAP`, and I/O errors don't stop the program. Instead, check
`ERR()` after each operation that can fail. A few errors, like a
division by zero or running out of memory, stop the program with a
message (see [Runtime errors](B-error-messages.md#runtime-errors)):

```basic
' Open a file and check for errors, instead of TRAP
OPEN #1, 4, 0, "D:SCORES.DAT"
Result = ERR()
IF Result <> 1
  PRINT "CAN'T OPEN SCORES.DAT, ERROR "; Result
ENDIF
CLOSE #1
```

Output (when the file does not exist):

```
CAN'T OPEN SCORES.DAT, ERROR 170
```

See [Files and Devices](11-files-and-devices.md) and [Error
Messages](B-error-messages.md#runtime-errors).

### Statements that changed

| Atari BASIC | exBASIC |
|-------------|---------|
| `REM text` | `' text` or `. text` |
| `LET X = 1` | `X = 1` |
| `PRINT #1;X` | the same, or `PRINT #1, X` |
| `INPUT #1;A$` | `INPUT #1, A$` (comma required) |
| `XIO 18,#6,0,0,"S:"` | the same, or `FILLTO` |
| `POSITION`, `PLOT`, `DRAWTO`, `LOCATE`, `GRAPHICS`, `COLOR`, `SETCOLOR`, `SOUND` | the same |
| `STOP`, `END` | `END` |
| `FRE(0)` | the same, or `FRE()`; the value is never shown negative |
| `PEEK(195)` after `TRAP` | `ERR()` |
| `NOTE #1,S,B`, `POINT #1,S,B` | the same, see [Files and Devices](11-files-and-devices.md#note) |
| `LPRINT` | `OPEN #1, 8, 0, "P:"` and `PRINT #1, ...` |

`PRINT` with `,` moves to the next column multiple of 10, as in Atari
BASIC. New statements such as `PAUSE`, `CLS`, `FILLTO`, `MOVE`, `DPOKE`
and `INC` replace common `POKE` and `FOR` loop idioms.

## Differences from Turbo BASIC XL

Turbo BASIC XL programs are closer to exBASIC: they already use
`REPEAT`/`UNTIL`, `WHILE`/`WEND`, `DO`/`LOOP`, `IF`/`ELSE`/`ENDIF`,
`EXIT`, `DPEEK`, `DPOKE`, `MOVE`, `-MOVE`, `BGET`, `BPUT`, `PAUSE`,
`CLS`, `FILLTO`, `TIME`, `RAND`, hexadecimal constants with `$` and the
operators `MOD`, `EXOR`, `&` and `!`. Everything in the Atari BASIC
section above also applies (line numbers, integers, strings, `DATA`,
`AND` and `OR`).

| Turbo BASIC XL | exBASIC |
|----------------|---------|
| `PROC Name` ... `ENDPROC`, `EXEC Name` | `FUNC Name()` ... `ENDFUNC`, `Name()` |
| `#Label`, `GO# Label` | structured statements |
| `X DIV Y` | `X / Y` (integer division) |
| `%0`, `%1`, `%2`, `%3` | `0`, `1`, `2`, `3` |
| `TRUNC(X)` | `INT(X)` for positive values; `INT` rounds down, so `INT(-2.5)` is `-3` |
| `ERR`, `ERL` | `ERR()` (no line numbers) |
| `INKEY$` | `KEY()` and `GET` |
| `DSOUND` | not available; `SOUND` only |
| `TIME$` | not available; use `TIME` |
| `INSTR` | `INSTR(A$, B$ [, Start])`, the same |
| `UINSTR(A$, B$)` | `INSTR(UCASE$(A$), UCASE$(B$))` |
| `HEX$`, `DEC` | not available; write a `FUNC` |
| `CIRCLE`, `PAINT`, `TEXT` | not available |
| `DIR`, `RENAME`, `DELETE`, `LOCK`, `UNLOCK`, `BLOAD`, `BRUN` | `XIO` with the DOS commands, or `RUN` for executables |
| `FRAC(X)`, `RND(0)` | `X% - INT(X%)` (for positive values), `RND()` |
| `TRACE`, `DUMP`, `RENUM`, `DEL` | not needed: the source is on the PC |

Note that `DEC` is a statement in exBASIC (subtract 1), not the Turbo
BASIC function that converts a hexadecimal string.

## Differences from FastBasic

exBASIC is based on FastBasic 4.x and keeps most of its language, so
FastBasic programs need few changes.

### PROC and EXEC are replaced by FUNC

FastBasic procedures are defined with `PROC Name` ... `ENDPROC` and
called with `EXEC Name` or `@Name`. exBASIC replaces them with `FUNC`,
which has typed parameters, local variables, several results and
recursion, and is called by name. FastBasic (for comparison only):

```
EXEC ShowArea 3, 4

PROC ShowArea W H
  ? "AREA: "; W * H
ENDPROC
```

exBASIC:

```basic
' A FastBasic PROC with a result, written as a FUNC
Area = RectArea(3, 4)
PRINT "AREA: "; Area

' Area of a rectangle
FUNC RectArea(Width, Height)
  RETURN Width * Height
ENDFUNC
```

Output:

```
AREA: 12
```

To port: change `PROC Name A B` to `FUNC Name(A, B)`, `ENDPROC` to
`ENDFUNC`, and `EXEC Name X, Y` or `@Name X, Y` to `Name(X, Y)`. FastBasic
procedure parameters are global variables; `FUNC` parameters are
private to the function, so a procedure that changed a global through its
parameter must now return the value. See [Functions](07-functions.md).

The `@name` syntax remains for the address of an assembler symbol, as in
`USR(@MyRoutine, 1)` (see [Memory and Machine
Code](16-memory-and-machine-code.md)).

### New in exBASIC

- `FUNC`, `ENDFUNC`, `LOCAL` and `RETURN` with values.
- String concatenation with `+` in any string expression, and the string
  functions `LEFT$`, `RIGHT$`, `MID$` (also as a statement), `INSTR`,
  `UCASE$`, `LCASE$`, `LTRIM$` and `RTRIM$`.
- `NOTE` and `POINT` for positions in disk files.
- An `ELSE` on the line of a one-line `IF ... THEN`.
- `DO WHILE`, `DO UNTIL`, `LOOP WHILE` and `LOOP UNTIL`.
- `INCLUDE "file"` to split a program in several files.
- `COMMON` and `RUN "D:FILE.XEX"` to pass variables to the next program,
  and `STATUS`.
- Compilation of the integer code to 6502 machine code, with the options
  `-native:hot`, `-native:all` and `-no-native`, and more combined
  bytecode instructions. Integer programs run 2.6 to 6 times faster than
  with FastBasic (see [Benchmarks](20-performance-and-memory.md#benchmarks)).

### Removed

- The integrated editor and compiler on the Atari (the FastBasic IDE).
  Programs are written and compiled on a PC only.
- The Atari 5200 target.
- The separate integer and floating point targets (`atari-int`,
  `atari-fp`, `atari-cart-int`, `atari-cart-fp`). There are two targets,
  `atari` and `atari-cart`, and floating point is included automatically
  when the program uses it.
- `PROC`, `ENDPROC`, `EXEC` and the `@Name` call shorthand.

### Renamed

| FastBasic | exBASIC |
|-----------|---------|
| `fastbasic` compiler | `exbasic` |
| `FASTBASIC_HOME` environment variable | `EXBASIC_HOME` |
| `fastbasic.cfg`, `fastbasic-*.lib` | `exbasic.cfg`, `exbasic.lib`, `exbasic-cart.cfg`, `exbasic-cart.lib` |
| `fbprof` profiler | `exbprof` |

Other changes:

- A `FUNC` name can't be used as a variable or `DATA` array name.
- The abbreviation of `RETURN` is `RET.`.
- `IF cond THEN` applies to the rest of the line, as in Atari BASIC. In
  FastBasic, `THEN` applies to one statement only, so a FastBasic line
  like `IF A THEN B = 1 : C = 2` now makes `C = 2` conditional too.
- `AND` and `OR` evaluate the right side only when it is needed.
- `XIO` takes its parameters in the Atari BASIC order:
  `XIO cmd, #channel, aux1, aux2, dev$` instead of FastBasic's
  `XIO #channel, cmd, aux1, aux2, dev$`.
- `A`, `A$` and `A%` are different variables. A keyword is recognized
  only as a whole word: `COLORS` is a name, not `COLOR S`.
- Integer constants larger than 65535 are an error instead of being read
  as floating point; mixed integer and floating point expressions in
  `PRINT` and conditions are calculated in floating point.
- `FRE()` is never shown negative.
- A division by zero stops the program with error 11 (`ERROR- 11`). Running
  out of memory, too deep recursion, a bad `DIM` size and the BREAK key
  also stop the program with an error number (see
  [Runtime errors](B-error-messages.md#runtime-errors)).
- `PRINT #1; X` is accepted, as in Atari BASIC.

## Porting checklist

1. Remove the line numbers. Replace `GOTO` and `GOSUB` with loops, `IF`
   blocks and `FUNC`s.
2. Convert every `IF ... THEN` with a line number after `THEN` to an
   `IF` ... `ENDIF` block. One-line `IF`s with several statements after
   `THEN` work as in Atari BASIC (but not as in FastBasic).
3. Decide which variables need floating point and add `%` to their names;
   leave the rest as integers.
4. Rename variables that share a name with an array of the same type.
5. Remove the `DIM` of simple strings; convert substrings to
   `[start, length]`.
6. Convert `READ`/`DATA` to `DATA` arrays.
7. Replace `TRAP` with `ERR()` checks, saved in a variable right after the
   operation.
8. Put a comma after `#channel` in `INPUT`, `GET` and the other I/O
   statements (`PRINT` and `PUT` also accept `;`). Programs from
   FastBasic must reorder the `XIO` arguments.
9. Compile, and read the error messages: [Error
   Messages](B-error-messages.md) explains each of them.
