# 3. Language Basics

This chapter describes the form of an exBASIC program: how lines and
statements are written, comments, upper and lower case, names, keywords and
their abbreviations, how numbers are written, and how the compiler reports
errors.

## Program structure

An exBASIC program is a plain text file, usually with the extension `.bas`.
There are no line numbers. The program runs from the first line to the last,
except where control statements (`IF`, `FOR`, `WHILE`, `DO`, `REPEAT`,
`EXIT`, function calls) change the order. The program ends when it reaches
the end of the file or executes `END`.

Some parts of the program are not executed when the program reaches them:

- Function definitions (`FUNC` ... `ENDFUNC`) run only when the function is
  called. You can put them anywhere in the file, before or after the code
  that calls them; most programs put them at the end.
- `DATA` statements define constant arrays at compile time and are skipped
  when execution reaches them.

```basic
' Shows the order of execution
PRINT "START"

' Returns twice its argument
FUNC Twice(Value)
  RETURN Value * 2
ENDFUNC

PRINT "TWICE 21 IS "; Twice(21)
IF Twice(1) = 2 THEN END
PRINT "NOT PRINTED"
```

Output:

```
START
TWICE 21 IS 42
```

A program can be split into several files with `INCLUDE "file"`, which
inserts another source file at that place (see
[chapter 17](17-program-organization.md)).

## Lines and statements

Each line of the program can be:

- an empty line,
- a comment (see [Comments](#comments)),
- one statement, or
- several statements separated by colons (`:`).

A statement is a keyword followed by its parameters (`PRINT "HI"`,
`POKE 712, 0`), an assignment (`X = 10`, `A$ = "TEXT"`), or a call to a
function defined with `FUNC` (`DrawBox(1, 2)`).

```basic
X = 1 : Y = 2 : PRINT X + Y
```

Output:

```
3
```

A line ends at a line feed (Unix text files), at a carriage return followed
by a line feed (Windows text files) or at the ATASCII end of line character
(byte 155, Atari text files). The compiler accepts the three forms, also
mixed in the same file.

There is no limit on the length of a line, and no way to continue a
statement on the next line: each statement must be complete on its line.
To keep lines short, split long calculations using variables, and long
`DATA` lists using continuation lines (see
[DATA](04-variables-and-data.md#data)).

### Colons and IF ... THEN

A colon separates two statements. The one exception is the one-line form
`IF cond THEN statements`: everything after `THEN` up to the end of the
line depends on the condition, also the statements after a colon, as in
Atari BASIC. An `ELSE` on the same line gives the statements that run
when the condition is false:

```basic
Count = 0
IF Count = 1 THEN PRINT "ONE" : PRINT "ALSO ONE"
IF Count = 1 THEN PRINT "ONE" ELSE PRINT "NOT ONE"
PRINT "ALWAYS PRINTED"
```

Output:

```
NOT ONE
ALWAYS PRINTED
```

For more than a few statements, use the block form of `IF` (see
[chapter 6](06-control-flow.md)):

```basic
Count = 1
IF Count = 1
  PRINT "ONE"
  PRINT "ALSO ONE"
ENDIF
```

### Blocks and indentation

Block statements such as `IF` / `ENDIF`, `FOR` / `NEXT`, `WHILE` / `WEND`
and `FUNC` / `ENDFUNC` can span many lines and be nested. The compiler
ignores spaces and tabs at the start of a line, so indentation is only for
the reader. The examples in this manual indent each block by two spaces.

## Comments

### ' (apostrophe)
Starts a comment that extends to the end of the line.

```
' text
statement ' text
```

An apostrophe at the start of a line makes the whole line a comment. After a
statement, an apostrophe starts a comment that ends the line; no colon is
needed before it. An apostrophe inside a string constant is part of the
string and does not start a comment.

### . (period)
Starts a comment line.

```
. text
```

A period written where a statement is expected (at the start of a line, or
after a colon) starts a comment that extends to the end of the line. It is
the exBASIC equivalent of `REM` in Atari BASIC. There is no `REM` keyword.

```basic
' This program shows the comment forms
. This is also a comment
Total = 1 ' a comment after a statement
Total = Total + 1 : . a comment after a colon
PRINT "IT'S "; Total ' the apostrophe in the string is not a comment
```

Output:

```
IT'S 2
```

## Spaces and case

exBASIC is not case sensitive. Keywords and names can be written in
uppercase, lowercase or mixed case: `PRINT`, `print` and `Print` are the
same keyword, and `Count`, `COUNT` and `count` are the same variable. The
text inside string constants keeps its case.

Spaces and tabs around operators, parentheses, commas, colons and quotes
are optional and ignored. A keyword must end where a name could not go
on: put a space between a keyword and a name or a number that follows it
(see [Reserved words](#reserved-words)). The two following lines are the
same loop, written on one line with colons (the second line also uses
`?`, the short form of `PRINT`):

```basic
FOR I = 1 TO 3 : PRINT I; : NEXT I
FOR I=1 TO 3:?I;:NEXT I
```

Output:

```
123123
```

`FORI=1TO3` does not work: `FORI` and `TO3` are read as names. Spaces
cannot be put inside a keyword, a name or a number. Leaving out spaces
makes programs hard to read, so use them.

## Names

Variables, arrays, functions and `DATA` arrays have names. A name:

- starts with a letter (`A` to `Z`) or an underscore (`_`),
- continues with letters, digits (`0` to `9`) and underscores,
- can have any length; all characters are significant,
- is not case sensitive.

The last character of a variable or function name can be a type suffix:

| Suffix | Type | Examples |
| --- | --- | --- |
| none | Integer, 16-bit signed | `X`, `Count`, `num_1`, `_tmp` |
| `%` | Floating point | `Speed%`, `X1%` |
| `$` | String | `Name$`, `LINE1$` |

The suffix is part of the name: `A`, `A$` and `A%` are three different
variables (an integer, a string and a floating point number), as in Atari
BASIC. The same holds for arrays: `A()`, `A$()` and `A%()` are three
different arrays. A variable and an array with the same name and suffix
can't both exist: if the program has an integer variable `A`, it can't
also have a word array `A()`. The compiler reports an error at the second
use (see [chapter 4](04-variables-and-data.md)).

```basic
Player_Score = 1500
Name$ = "ANA"
Speed% = 2.5
PRINT Name$; " "; Player_Score; " "; Speed%
' Three different variables
Total = 3
Total$ = "THREE"
Total% = 3.5
PRINT Total; " "; Total$; " "; Total%
```

Output:

```
ANA 1500 2.5
3 THREE 3.5
```

Using one name for several types is allowed, but descriptive and
different names make a program easier to read.

`DATA` arrays are named like variables, followed by `()` in the `DATA`
statement. Function names follow the same rules; a function returning a
string ends in `$` and one returning a floating point number ends in `%`.

## Keywords and abbreviations

Keywords are the words of the language: statement names (`PRINT`, `FOR`),
function names (`ABS`, `STR$`) and other words (`TO`, `STEP`, `THEN`,
`AND`). Most keywords can be abbreviated: type the beginning of the keyword
followed by a period. Each keyword has a minimum abbreviation, listed in the
tables below. Any longer beginning also works: `PRINT` can be written
`PRI.`, `PRIN.` or `PRINT`. Abbreviations can be written in lowercase too
(`pri.`).

Some keywords have no abbreviation and must be written in full, for example
`CLR`, `CLS`, `DEG`, `DO`, `END`, `EXP`, `FUNC`, `INC`, `LOCAL` and `LOG`.

The same abbreviation can mean different keywords in different places. For
example, `A.` is `ABS` where a value is expected and `AND` between two
values; `F.` is `FOR` at the start of a statement and `FRE()` in an
expression; `T.` is `TIMER` as a statement, `TIME` in an expression and
`TO` in a `FOR` statement.

```basic
F. I = 1 T. 3 : ? I; " "; : N. I
? : ? A.(-5); " "; 17 M. 5; " "; 1 A. 0
```

Output:

```
1 2 3 
5 2 0
```

The compiler option `-l` writes `name.list` with the program as the compiler
read it, with all abbreviations expanded and blocks indented. Use it to read
heavily abbreviated code or to check how a line was understood.

### Statement keywords

| Keyword | Abbreviation | Chapter |
| --- | --- | --- |
| `?` (`PRINT`) | `?` | [10](10-console-io.md) |
| `BGET` | `BG.` | [11](11-files-and-devices.md) |
| `BPUT` | `BP.` | [11](11-files-and-devices.md) |
| `CLOSE` | `CL.` | [11](11-files-and-devices.md) |
| `CLR` | none | [4](04-variables-and-data.md#clr) |
| `CLS` | none | [10](10-console-io.md) |
| `COLOR` | `C.` | [12](12-graphics.md) |
| `COMMON` | none | [17](17-program-organization.md) |
| `DATA` | `DA.` | [4](04-variables-and-data.md#data) |
| `DEC` | `DE.` | [4](04-variables-and-data.md#dec) |
| `DEG` | none | [9](09-floating-point.md) |
| `DIM` | `DI.` | [4](04-variables-and-data.md#dim) |
| `DLI` | `DL.` | [14](14-display-list-interrupts.md) |
| `DO` | none | [6](06-control-flow.md) |
| `DPOKE` | `D.` | [16](16-memory-and-machine-code.md) |
| `DRAWTO` | `DR.` | [12](12-graphics.md) |
| `ELIF` | `ELI.` | [6](06-control-flow.md) |
| `ELSE` | `EL.` | [6](06-control-flow.md) |
| `END` | none | [6](06-control-flow.md) |
| `ENDFUNC` | none | [7](07-functions.md) |
| `ENDIF` | `E.` | [6](06-control-flow.md) |
| `EXIT` | `EX.` | [6](06-control-flow.md) |
| `FCOLOR` | `FC.` | [12](12-graphics.md) |
| `FILLTO` | `FI.` | [12](12-graphics.md) |
| `FOR` | `F.` | [6](06-control-flow.md) |
| `FUNC` | none | [7](07-functions.md) |
| `GET` | `GE.` | [10](10-console-io.md) |
| `GRAPHICS` | `G.` | [12](12-graphics.md) |
| `IF` | `I.` | [6](06-control-flow.md) |
| `INC` | none | [4](04-variables-and-data.md#inc) |
| `INCLUDE` | none | [17](17-program-organization.md) |
| `INPUT` | `IN.` | [10](10-console-io.md) |
| `LOCAL` | none | [7](07-functions.md) |
| `LOCATE` | `LOC.` | [12](12-graphics.md) |
| `LOOP` | `L.` | [6](06-control-flow.md) |
| `MID$(` | none | [8](08-strings.md#mid-statement) |
| `MOVE` | `M.` | [16](16-memory-and-machine-code.md) |
| `-MOVE` | `-.` | [16](16-memory-and-machine-code.md) |
| `MSET` | `MS.` | [16](16-memory-and-machine-code.md) |
| `NCLOSE` | `NC.` | [18](18-sio-and-fujinet.md) |
| `NEXT` | `N.` | [6](06-control-flow.md) |
| `NGET` | `NG.` | [18](18-sio-and-fujinet.md) |
| `NOPEN` | `NO.` | [18](18-sio-and-fujinet.md) |
| `NOTE` | none | [11](11-files-and-devices.md#note) |
| `NPUT` | `NP.` | [18](18-sio-and-fujinet.md) |
| `NSTATUS` | `NS.` | [18](18-sio-and-fujinet.md) |
| `OPEN` | `O.` | [11](11-files-and-devices.md) |
| `PAUSE` | `PA.` | [15](15-sound-and-controllers.md) |
| `PLOT` | `PL.` | [12](12-graphics.md) |
| `PMGRAPHICS` | `PMG.` | [13](13-player-missile-graphics.md) |
| `PMHPOS` | `PM.` | [13](13-player-missile-graphics.md) |
| `POINT` | `POI.` | [11](11-files-and-devices.md#point) |
| `POKE` | `P.` | [16](16-memory-and-machine-code.md) |
| `POSITION` | `POS.` | [10](10-console-io.md) |
| `PRINT` | `PRI.` or `?` | [10](10-console-io.md) |
| `PUT` | `PU.` | [10](10-console-io.md) |
| `RAD` | `RA.` | [9](09-floating-point.md) |
| `REPEAT` | `R.` | [6](06-control-flow.md) |
| `RETURN` | `RET.` | [7](07-functions.md) |
| `RUN` | none | [17](17-program-organization.md) |
| `SETCOLOR` | `SE.` | [12](12-graphics.md) |
| `SIO` | `SI.` | [18](18-sio-and-fujinet.md) |
| `SOUND` | `S.` | [15](15-sound-and-controllers.md) |
| `STATUS` | `STAT.` | [11](11-files-and-devices.md) |
| `TIMER` | `T.` | [15](15-sound-and-controllers.md) |
| `UNTIL` | `U.` | [6](06-control-flow.md) |
| `WEND` | `WE.` | [6](06-control-flow.md) |
| `WHILE` | `W.` | [6](06-control-flow.md) |
| `XIO` | `X.` | [11](11-files-and-devices.md) |

### Function keywords

| Keyword | Abbreviation | Chapter |
| --- | --- | --- |
| `ABS` | `A.` | [5](05-expressions.md#abs) |
| `ADR(` | none (also `&`) | [16](16-memory-and-machine-code.md) |
| `ASC` | `AS.` | [8](08-strings.md) |
| `ATN` | `AT.` | [9](09-floating-point.md) |
| `CHR$` | none | [8](08-strings.md) |
| `COS` | `CO.` | [9](09-floating-point.md) |
| `DPEEK` | `D.` | [16](16-memory-and-machine-code.md) |
| `ERR()` | `E.` | [11](11-files-and-devices.md) |
| `EXP` | none | [9](09-floating-point.md) |
| `EXP10` | `EX.` | [9](09-floating-point.md) |
| `FRE` | `F.` | [5](05-expressions.md#fre) |
| `INSTR(` | none | [8](08-strings.md#instr) |
| `INT` | `I.` | [9](09-floating-point.md) |
| `KEY()` | `K.` | [10](10-console-io.md) |
| `LCASE$(` | none | [8](08-strings.md#ucase-and-lcase) |
| `LEFT$(` | none | [8](08-strings.md#left) |
| `LEN` | `L.` | [8](08-strings.md) |
| `LOG` | none | [9](09-floating-point.md) |
| `LOG10` | `LO.` | [9](09-floating-point.md) |
| `LTRIM$(` | none | [8](08-strings.md#ltrim-and-rtrim) |
| `MID$(` | none | [8](08-strings.md#mid) |
| `PADDLE` | `PA.` | [15](15-sound-and-controllers.md) |
| `PEEK` | `P.` | [16](16-memory-and-machine-code.md) |
| `PMADR` | `PM.` | [13](13-player-missile-graphics.md) |
| `PTRIG` | `PT.` | [15](15-sound-and-controllers.md) |
| `RAND` | `R.` | [5](05-expressions.md#rand) |
| `RIGHT$(` | none | [8](08-strings.md#right) |
| `RND()` | `RN.` | [9](09-floating-point.md) |
| `RTRIM$(` | none | [8](08-strings.md#ltrim-and-rtrim) |
| `SERR()` | `SE.` | [18](18-sio-and-fujinet.md) |
| `SGN` | `SG.` | [5](05-expressions.md#sgn) |
| `SIN` | `SI.` | [9](09-floating-point.md) |
| `SQR` | `SQ.` | [9](09-floating-point.md) |
| `STICK` | `S.` | [15](15-sound-and-controllers.md) |
| `STR$` | none | [8](08-strings.md) |
| `STRIG` | `STR.` | [15](15-sound-and-controllers.md) |
| `TIME` | `T.` | [15](15-sound-and-controllers.md) |
| `%TIME` | `%T.` | [15](15-sound-and-controllers.md) |
| `UCASE$(` | none | [8](08-strings.md#ucase-and-lcase) |
| `USR(` | none | [16](16-memory-and-machine-code.md) |
| `VAL` | `V.` | [8](08-strings.md) |

Functions written with `()` in the table (`KEY()`, `ERR()`, `RND()`,
`SERR()`) take no argument; their abbreviation replaces the whole word
including the parentheses: `E.` is `ERR()`. `FRE` can be written `FRE`,
`FRE()` or `FRE(0)` (any argument is ignored), and `F.` is `FRE`.
Functions written with `(` in the table must have the parenthesis right
after the name.

### Other keywords

| Keyword | Abbreviation | Used in |
| --- | --- | --- |
| `AND` | `A.` | Logical operator, [chapter 5](05-expressions.md#logical-operators) |
| `BYTE` | `B.` | `DIM`, `DATA` |
| `COLOR` | `C.` | `PRINT` separator |
| `EXOR` | `E.` | Bitwise operator, [chapter 5](05-expressions.md#bitwise-operators) |
| `FILE` | `F.` | `DATA` (`BYTEFILE`, `WORDFILE`) |
| `INTO` | `I.` | `DLI SET` |
| `MOD` | `M.` | Integer operator, [chapter 5](05-expressions.md#integer-arithmetic) |
| `NOT` | `N.` | Logical operator |
| `OR` | `O.` | Logical operator |
| `ROM` | none | `DATA` |
| `RTAB` | `RT.` | `PRINT` separator |
| `SET` | `S.` | `DLI SET` |
| `STEP` | `S.` | `FOR` |
| `TAB` | `T.` | `PRINT` separator |
| `THEN` | `T.` | `IF` |
| `TO` | `T.` | `FOR` |
| `WORD` | `W.` | `DIM`, `DATA` |
| `WSYNC` | `W.` | `DLI SET` |

## Reserved words

exBASIC has no list of reserved words: the compiler recognizes keywords by
position. A statement keyword is recognized at the start of a statement, a
function name where a value is expected, and an operator between two values.

A keyword is recognized only when the word ends there: the next character
must not be a letter, a digit or an underscore. So a name that starts with
a keyword is an ordinary name. `TOTAL`, `FORM`, `ENDX`, `COLORS`,
`PRINTER`, `NOTES`, `TIMEOUT`, `COST%` and `NEXT_TIME` are all valid
variable names:

```basic
' Names that start with keywords are ordinary names
COLORS = 3
PRINTER = 4
TIMEOUT = COLORS + PRINTER
PRINT TIMEOUT
```

Output:

```
7
```

Abbreviations with a period (`F.`, `PRI.`, `X.`) and the `?` for `PRINT`
work as before, because the period or the `?` ends the keyword.

The same rule means that a space is needed between a keyword and a name
or number that follows it: `FOR I = 1 TO 3` works, `FOR I = 1 TO3` does
not. Avoid names that are exactly a keyword (`Color`, `Step`, `Val`):
some are accepted where the keyword does not fit, but they make the
program hard to read. Use the `-l` option to check how the compiler read a
line when a result looks wrong.

## Numbers

### Integer constants

Integer constants are written in decimal or in hexadecimal:

- Decimal: digits `0` to `9`, for example `0`, `123`, `32767`.
- Hexadecimal: a `$` followed by the digits `0` to `9` and `A` to `F` (or
  `a` to `f`), for example `$FF`, `$D01F`, `$2000`.

A minus sign before a constant makes it negative: `-1`, `-32768`, `-$10`.

Integer values are 16-bit signed numbers from -32768 to 32767. Constants
from 32768 to 65535 (`$8000` to `$FFFF`) are accepted too, and stand for the
16-bit value with the same bits, which is negative when read as a signed
number. This is convenient for addresses: `PEEK($D01F)` and `PEEK(53279)`
read the same location.

```basic
PRINT 32767; " "; 32768; " "; 65535; " "; $FFFF; " "; $8000; " "; -$10
```

Output:

```
32767 -32768 -1 -1 -32768 -16
```

A decimal constant without a decimal point and without an exponent must
be from 0 to 65535. A larger one is an error:

```
prog.bas:1:8: integer constant out of range (max 65535), write 70000.0 for a floating point number
```

for the line `Total = 70000`. Write `70000.0` (or `7.0E4`) and use a
floating point variable such as `Total%`. A hexadecimal constant larger
than `$FFFF` gives the error `hex constant out of range (max $FFFF)`.

Hexadecimal constants can also be used in floating point expressions:
`X% = $10` stores 16.

### Floating point constants

A floating point constant has a decimal point, an exponent, or both:
`3.14159`, `-0.5`, `.25`, `1E3`, `1.0E10`, `2.5E-3`. The exponent is the
letter `E` followed by an optional sign and one or two digits. Floating
point numbers range from about 1E-98 to 1E+98, with 9 or 10 significant
digits (see [chapter 9](09-floating-point.md)).

A number with an exponent is always floating point, also without a
decimal point: `1E3` is 1000.0 and `2.5E-1` is 0.25.

```basic
PRINT 1.5E-3; " "; 2.0E2; " "; .25; " "; 1.0E10; " "; 1E3
```

Output:

```
1.5E-03 200 0.25 1E+10 1000
```

### Character constants

exBASIC has no special syntax for the code of a character. Use the `ASC`
function with a one-character string, or write the ATASCII code as a number:

```basic
LetterA = ASC("A")
PRINT LetterA; " "; ASC("a"); " "; ASC(" ")
```

Output:

```
65 97 32
```

### String constants

String constants are written between double quotes: `"HELLO"`. Two double
quotes inside the string stand for one double quote. A `$` and a
hexadecimal number written right after the closing quote add the character
with that ATASCII code, which is how you include characters that cannot be
typed in a text file, such as the end of line (`$9B`):

```basic
PRINT "SAY ""HI"""
PRINT "ONE"$9B"TWO"
```

Output:

```
SAY "HI"
ONE
TWO
```

Do not split a string constant across two lines of the source file. The
compiler accepts it, but the line break becomes part of the string as the
byte 10, which the Atari prints as a graphics character, not as a new line.

A string constant can have at most 255 characters. The compiler reports
these errors in string constants:

| Error | Cause |
| --- | --- |
| 2 | Out of memory: not enough free memory for an array or a string. |
| 9 | Bad DIM size: `DIM` with a computed size out of range, such as a negative size. |
| 10 | Stack overflow: too many nested function calls, or an expression or loops nested too deeply. |
| 11 | Division by zero: a division or `MOD` by zero, integer or floating point. |
| 128 | The user pressed the BREAK key. |

The numbers are those of Atari BASIC for the same errors.

You can stop a running program with the BREAK key. See
[Appendix B](B-error-messages.md#runtime-errors) for details.
