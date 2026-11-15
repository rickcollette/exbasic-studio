# Appendix A. Keyword Reference

This appendix lists every keyword of exBASIC: statements, functions,
operators, the words used inside statements, and the special symbols.
For each one it gives the shortest abbreviation, the kind, a one-line
summary and a link to the chapter that documents it. The list is taken
from the syntax files of the compiler (`src/syntax/*.syn`), and every
abbreviation was checked by compiling it.

## Abbreviations

Most keywords can be abbreviated: type the first letters followed by a
dot. The table gives the shortest abbreviation; any longer one also works,
so `PRI.`, `PRIN.` and `PRINT` are all `PRINT`. A keyword without an
abbreviation (shown as "-") must be written in full.

The same abbreviation can mean different keywords in different places.
`E.` is `ENDIF` at the start of a statement, `ERR()` where a value is
expected, and `EXOR` between two values. `P.` is `POKE` as a statement and
`PEEK` in an expression. The compiler picks the keyword that fits the
place. For functions written with `()`, the abbreviation replaces the
parentheses too: `K.` is `KEY()` and `E.` is `ERR()`. `F.` is `FRE`.

A keyword is recognized only when the word ends there, so names that
start with a keyword, like `COLORS`, `PRINTER` or `NEXT_TIME`, are
ordinary names (see [Reserved words](03-language-basics.md#reserved-words)).
The period of an abbreviation ends the keyword.

Use the `-l` compiler option to get a listing of a program with all
abbreviations expanded (see [-l](19-compiler-reference.md#-l)).

## Keywords

Kinds: *statement* starts a statement; *function* returns a value;
*operator* joins values; *word* is part of a statement; *directive* is
handled before compilation.

| Keyword | Abbr. | Kind | Summary | Chapter |
|---------|-------|------|---------|---------|
| `ABS(n)` | `A.` | function | absolute value of an integer or floating point number | [Expressions](05-expressions.md#abs) |
| `ADR(x)` | - | function | address of a string, array or variable; also written `&x` | [Memory](16-memory-and-machine-code.md#adr) |
| `AND` | `A.` | operator | logical AND, result 0 or 1; the right side is evaluated only when needed | [Expressions](05-expressions.md#logical-operators) |
| `ASC(s)` | `AS.` | function | ATASCII code of the first character of a string | [Strings](08-strings.md#asc) |
| `ATN(x)` | `AT.` | function | arc tangent (floating point) | [Floating Point](09-floating-point.md#atn) |
| `BGET` | `BG.` | statement | read a block of bytes from a channel to memory | [Files](11-files-and-devices.md#bget) |
| `BPUT` | `BP.` | statement | write a block of bytes from memory to a channel | [Files](11-files-and-devices.md#bput) |
| `BYTE` | `B.` | word | byte array type in `DIM` and `DATA` | [Variables](04-variables-and-data.md#dim) |
| `CHR$(n)` | - | function | one-character string with the given ATASCII code | [Strings](08-strings.md#chr) |
| `CLOSE` | `CL.` | statement | close an I/O channel | [Files](11-files-and-devices.md#close) |
| `CLR` | - | statement | free all arrays and strings, set variables to 0 | [Variables](04-variables-and-data.md#clr) |
| `CLS` | - | statement | clear the screen, or send a clear code to a channel | [Console I/O](10-console-io.md#cls) |
| `COLOR` | `C.` | statement | set the color used by `PLOT`, `DRAWTO` and `FILLTO` | [Graphics](12-graphics.md#color) |
| `COLOR` (in `PRINT`) | `C.` | word | print the next item with its characters changed (inverse video) | [Console I/O](10-console-io.md#print) |
| `COMMON` | - | statement | declare the variables kept across `RUN` | [Program Organization](17-program-organization.md#common) |
| `COS(x)` | `CO.` | function | cosine (floating point) | [Floating Point](09-floating-point.md#cos) |
| `DATA` | `DA.` | statement | define a constant array in the program | [Variables](04-variables-and-data.md#data) |
| `DEC` | `DE.` | statement | subtract 1 from an integer variable or array element | [Variables](04-variables-and-data.md#dec) |
| `DEG` | - | statement | trigonometric functions use degrees | [Floating Point](09-floating-point.md#deg) |
| `DIM` | `DI.` | statement | create arrays (and variables) | [Variables](04-variables-and-data.md#dim) |
| `DLI` | `DL.` | statement | define, enable or disable a display list interrupt | [DLIs](14-display-list-interrupts.md#dli) |
| `DO` | - | statement | start a `DO`/`LOOP` loop, optionally with `WHILE` or `UNTIL` | [Control Flow](06-control-flow.md#do) |
| `DPEEK(a)` | `D.` | function | read a 16-bit value from memory | [Memory](16-memory-and-machine-code.md#dpeek) |
| `DPOKE` | `D.` | statement | write a 16-bit value to memory | [Memory](16-memory-and-machine-code.md#dpoke) |
| `DRAWTO` | `DR.` | statement | draw a line to a point | [Graphics](12-graphics.md#drawto) |
| `ELIF` | `ELI.` | statement | "else if" branch of a multi-line `IF` | [Control Flow](06-control-flow.md#elif) |
| `ELSE` | `EL.` | statement | "else" branch of a multi-line `IF`, or of a one-line `IF ... THEN` | [Control Flow](06-control-flow.md#else) |
| `END` | - | statement | end the program | [Control Flow](06-control-flow.md#end) |
| `ENDFUNC` | - | statement | end a `FUNC` definition | [Functions](07-functions.md#endfunc) |
| `ENDIF` | `E.` | statement | end a multi-line `IF` | [Control Flow](06-control-flow.md#endif) |
| `ERR()` | `E.` | function | result code of the last I/O operation | [Files](11-files-and-devices.md#err) |
| `EXIT` | `EX.` | statement | leave the innermost loop | [Control Flow](06-control-flow.md#exit) |
| `EXOR` | `E.` | operator | bitwise exclusive OR | [Expressions](05-expressions.md) |
| `EXP(x)` | - | function | e raised to x (floating point) | [Floating Point](09-floating-point.md#exp) |
| `EXP10(x)` | `EX.` | function | 10 raised to x (floating point) | [Floating Point](09-floating-point.md#exp10) |
| `FCOLOR` | `FC.` | statement | set the fill color of `FILLTO` | [Graphics](12-graphics.md#fcolor) |
| `FILE` | `F.` | word | `DATA` array loaded from a file at compile time | [Variables](04-variables-and-data.md#data) |
| `FILLTO` | `FI.` | statement | draw a line and fill the area to its right | [Graphics](12-graphics.md#fillto) |
| `FOR` | `F.` | statement | start a counting loop | [Control Flow](06-control-flow.md#for) |
| `FRE`, `FRE()`, `FRE(x)` | `F.` | function | free memory in bytes, never shown negative | [Memory](16-memory-and-machine-code.md#fre) |
| `FUNC` | - | statement | start a function definition | [Functions](07-functions.md#func) |
| `GET` | `GE.` | statement | read a key, or a byte from a channel with `#` | [Console I/O](10-console-io.md#get) |
| `GRAPHICS` | `G.` | statement | set a graphics mode | [Graphics](12-graphics.md#graphics) |
| `IF` | `I.` | statement | conditional execution, one line or multi-line | [Control Flow](06-control-flow.md#if) |
| `INC` | - | statement | add 1 to an integer variable or array element | [Variables](04-variables-and-data.md#inc) |
| `INCLUDE` | - | directive | insert another source file | [Program Organization](17-program-organization.md#include) |
| `INPUT` | `IN.` | statement | read values from the keyboard or a channel | [Console I/O](10-console-io.md#input) |
| `INSTR(s, f [, n])` | - | function | position of the string f in s, from position n, or 0 | [Strings](08-strings.md#instr) |
| `INT(x)` | `I.` | function | a floating point number rounded to the nearest integer | [Floating Point](09-floating-point.md#int) |
| `INTO` | `I.` | word | destination register in `DLI SET` | [DLIs](14-display-list-interrupts.md#dli) |
| `KEY()` | `K.` | function | nonzero if a key was pressed | [Sound and Controllers](15-sound-and-controllers.md#key) |
| `LCASE$(s)` | - | function | a string with its letters in lowercase | [Strings](08-strings.md#ucase-and-lcase) |
| `LEFT$(s, n)` | - | function | the first n characters of a string | [Strings](08-strings.md#left) |
| `LEN(s)` | `L.` | function | length of a string | [Strings](08-strings.md#len) |
| `LOCAL` | - | statement | declare local variables of a `FUNC` | [Functions](07-functions.md#local) |
| `LOCATE` | `LOC.` | statement | read the color of a point of the screen | [Graphics](12-graphics.md#locate) |
| `LOG(x)` | - | function | natural logarithm (floating point) | [Floating Point](09-floating-point.md#log) |
| `LOG10(x)` | `LO.` | function | base 10 logarithm (floating point) | [Floating Point](09-floating-point.md#log10) |
| `LOOP` | `L.` | statement | end a `DO` loop, optionally with `WHILE` or `UNTIL` | [Control Flow](06-control-flow.md#loop) |
| `LTRIM$(s)` | - | function | a string without the spaces at its start | [Strings](08-strings.md#ltrim-and-rtrim) |
| `MID$(s, p [, n])` | - | function | n characters of a string from position p | [Strings](08-strings.md#mid) |
| `MID$(v$, p [, n]) =` | - | statement | replace characters of a string variable, keeping its length | [Strings](08-strings.md#mid-statement) |
| `MOD` | `M.` | operator | remainder of an integer division | [Expressions](05-expressions.md) |
| `MOVE` | `M.` | statement | copy a memory block, from the first byte up | [Memory](16-memory-and-machine-code.md#move) |
| `-MOVE` | `-.` | statement | copy a memory block, from the last byte down | [Memory](16-memory-and-machine-code.md#-move) |
| `MSET` | `MS.` | statement | fill a memory block with a byte | [Memory](16-memory-and-machine-code.md#mset) |
| `NCLOSE` | `NC.` | statement | close a FujiNet network channel | [SIO and FujiNet](18-sio-and-fujinet.md#nclose) |
| `NEXT` | `N.` | statement | end a `FOR` loop | [Control Flow](06-control-flow.md#next) |
| `NGET` | `NG.` | statement | read bytes from a FujiNet network channel | [SIO and FujiNet](18-sio-and-fujinet.md#nget) |
| `NOPEN` | `NO.` | statement | open a FujiNet network channel | [SIO and FujiNet](18-sio-and-fujinet.md#nopen) |
| `NOT` | `N.` | operator | logical NOT, result 0 or 1 | [Expressions](05-expressions.md) |
| `NOTE` | - | statement | get the position in a disk file | [Files](11-files-and-devices.md#note) |
| `NPUT` | `NP.` | statement | write bytes to a FujiNet network channel | [SIO and FujiNet](18-sio-and-fujinet.md#nput) |
| `NSTATUS` | `NS.` | statement | read the status of a FujiNet network channel | [SIO and FujiNet](18-sio-and-fujinet.md#nstatus) |
| `OPEN` | `O.` | statement | open an I/O channel to a device or file | [Files](11-files-and-devices.md#open) |
| `OR` | `O.` | operator | logical OR, result 0 or 1; the right side is evaluated only when needed | [Expressions](05-expressions.md#logical-operators) |
| `PADDLE(n)` | `PA.` | function | position of a paddle controller | [Sound and Controllers](15-sound-and-controllers.md#paddle) |
| `PAUSE` | `PA.` | statement | wait a number of screen frames | [Sound and Controllers](15-sound-and-controllers.md#pause) |
| `PEEK(a)` | `P.` | function | read a byte from memory | [Memory](16-memory-and-machine-code.md#peek) |
| `PLOT` | `PL.` | statement | draw a point | [Graphics](12-graphics.md#plot) |
| `PMADR(n)` | `PM.` | function | address of the memory of a player or the missiles | [P/M Graphics](13-player-missile-graphics.md#pmadr) |
| `PMGRAPHICS` | `PMG.` | statement | enable or disable player/missile graphics | [P/M Graphics](13-player-missile-graphics.md#pmgraphics) |
| `PMHPOS` | `PM.` | statement | set the horizontal position of a player or missile | [P/M Graphics](13-player-missile-graphics.md#pmhpos) |
| `POINT` | `POI.` | statement | move to a position in a disk file, given by `NOTE` | [Files](11-files-and-devices.md#point) |
| `POKE` | `P.` | statement | write a byte to memory | [Memory](16-memory-and-machine-code.md#poke) |
| `POSITION` | `POS.` | statement | move the cursor or graphics position | [Console I/O](10-console-io.md#position) |
| `PRINT` | `PRI.` or `?` | statement | print values to the screen or a channel | [Console I/O](10-console-io.md#print) |
| `PTRIG(n)` | `PT.` | function | state of a paddle trigger | [Sound and Controllers](15-sound-and-controllers.md#ptrig) |
| `PUT` | `PU.` | statement | write one character to the screen or a channel | [Console I/O](10-console-io.md#put) |
| `RAD` | `RA.` | statement | trigonometric functions use radians | [Floating Point](09-floating-point.md#rad) |
| `RAND(n)` | `R.` | function | random integer from 0 to n - 1 | [Sound and Controllers](15-sound-and-controllers.md#rand) |
| `REPEAT` | `R.` | statement | start a `REPEAT`/`UNTIL` loop | [Control Flow](06-control-flow.md#repeat) |
| `RETURN` | `RET.` | statement | return from a `FUNC`, with its results | [Functions](07-functions.md#return) |
| `RIGHT$(s, n)` | - | function | the last n characters of a string | [Strings](08-strings.md#right) |
| `RND()` | `RN.` | function | random floating point number from 0 to 1 | [Floating Point](09-floating-point.md#rnd) |
| `ROM` | - | word | `DATA` array placed with the program code | [Variables](04-variables-and-data.md#data) |
| `RTAB` | `RT.` | word | in `PRINT`, right-align the next item | [Console I/O](10-console-io.md#print) |
| `RTRIM$(s)` | - | function | a string without the spaces at its end | [Strings](08-strings.md#ltrim-and-rtrim) |
| `RUN` | - | statement | load and run another program, keeping `COMMON` variables | [Program Organization](17-program-organization.md#run) |
| `SERR()` | `SE.` | function | status of the last `SIO` operation | [SIO and FujiNet](18-sio-and-fujinet.md#serr) |
| `SET` | `S.` | word | define a display list interrupt in `DLI SET` | [DLIs](14-display-list-interrupts.md#dli) |
| `SETCOLOR` | `SE.` | statement | set a color register | [Graphics](12-graphics.md#setcolor) |
| `SGN(n)` | `SG.` | function | sign of a number: -1, 0 or 1 | [Expressions](05-expressions.md#sgn) |
| `SIN(x)` | `SI.` | function | sine (floating point) | [Floating Point](09-floating-point.md#sin) |
| `SIO` | `SI.` | statement | call the serial I/O routine with a device control block | [SIO and FujiNet](18-sio-and-fujinet.md#sio) |
| `SOUND` | `S.` | statement | play a tone, or silence all voices | [Sound and Controllers](15-sound-and-controllers.md#sound) |
| `SQR(x)` | `SQ.` | function | square root (floating point) | [Floating Point](09-floating-point.md#sqr) |
| `STATUS` | `STAT.` | statement | get the status of an I/O channel | [Files](11-files-and-devices.md#status) |
| `STEP` | `S.` | word | increment of a `FOR` loop | [Control Flow](06-control-flow.md#for) |
| `STICK(n)` | `S.` | function | direction of a joystick | [Sound and Controllers](15-sound-and-controllers.md#stick) |
| `STR$(n)` | - | function | a number converted to a string | [Strings](08-strings.md#str) |
| `STRIG(n)` | `STR.` | function | state of a joystick button | [Sound and Controllers](15-sound-and-controllers.md#strig) |
| `TAB` | `T.` | word | in `PRINT`, move to the next multiple of a column count | [Console I/O](10-console-io.md#print) |
| `THEN` | `T.` | word | one-line `IF`: the rest of the line is conditional, with an optional `ELSE` | [Control Flow](06-control-flow.md#if) |
| `TIME` | `T.` | function | frames counted since the computer started or `TIMER` | [Sound and Controllers](15-sound-and-controllers.md#time) |
| `%TIME` | `%T.` | function | `TIME` as a floating point number, without the 16-bit limit | [Sound and Controllers](15-sound-and-controllers.md#time) |
| `TIMER` | `T.` | statement | reset `TIME` to 0 | [Sound and Controllers](15-sound-and-controllers.md#timer) |
| `TO` | `T.` | word | limit of a `FOR` loop | [Control Flow](06-control-flow.md#for) |
| `UCASE$(s)` | - | function | a string with its letters in uppercase | [Strings](08-strings.md#ucase-and-lcase) |
| `UNTIL` | `U.` | statement | end a `REPEAT` loop; also a condition of `DO` and `LOOP` | [Control Flow](06-control-flow.md#until) |
| `USR(a, ...)` | - | function | call a machine code routine | [Memory](16-memory-and-machine-code.md#usr) |
| `VAL(s)` | `V.` | function | number from a string (integer or floating point) | [Strings](08-strings.md#val) |
| `WEND` | `WE.` | statement | end a `WHILE` loop | [Control Flow](06-control-flow.md#wend) |
| `WHILE` | `W.` | statement | start a `WHILE` loop; also a condition of `DO` and `LOOP` | [Control Flow](06-control-flow.md#while) |
| `WORD` | `W.` | word | word (16-bit) array type in `DIM` and `DATA` | [Variables](04-variables-and-data.md#dim) |
| `WSYNC` | `W.` | word | in `DLI SET`, wait for the next scan line | [DLIs](14-display-list-interrupts.md#dli) |
| `XIO` | `X.` | statement | call a CIO command on a channel: `XIO cmd, #ch, aux1, aux2, dev$` | [Files](11-files-and-devices.md#xio) |

Notes:

- `ABS`, `SGN` and `VAL` work with integers and with floating point: the
  type of the argument or of the expression decides.
- `GET`, `INPUT`, `PRINT` and `PUT` also work on I/O channels with `#`;
  see [Files and Devices](11-files-and-devices.md).
- Functions with one argument can be written without parentheses when the
  argument is a single value: `PEEK 1536` is `PEEK(1536)`. Parentheses
  are needed for an expression: `PEEK(1536 + I)`.

## Operators and symbols

| Symbol | Kind | Meaning | Chapter |
|--------|------|---------|---------|
| `'` | comment | the rest of the line is a comment | [Language Basics](03-language-basics.md) |
| `.` | statement | a comment line, like `REM` in other BASICs | [Language Basics](03-language-basics.md) |
| `:` | separator | separates statements on a line | [Language Basics](03-language-basics.md) |
| `?` | statement | short form of `PRINT` | [Console I/O](10-console-io.md#print) |
| `"..."` | constant | string constant; `""` inside is a quote, `"..."$9B` adds a character by its code | [Strings](08-strings.md) |
| `$` before digits | constant | hexadecimal number, as in `$D01A` | [Language Basics](03-language-basics.md) |
| `$` after a name | type | string variable, array or `FUNC` | [Variables](04-variables-and-data.md) |
| `%` after a name | type | floating point variable, array or `FUNC` | [Variables](04-variables-and-data.md) |
| `$(a)` | function | the string stored at address a | [Strings](08-strings.md) |
| `%(a)` | function | the floating point number stored at address a | [Floating Point](09-floating-point.md) |
| `&x` | function | address of a variable, array or string, same as `ADR(x)` | [Memory](16-memory-and-machine-code.md#adr) |
| `@name` | constant | address of an assembler symbol (16 bits) | [Memory](16-memory-and-machine-code.md) |
| `@@name` | constant | address of an assembler zero page symbol | [Memory](16-memory-and-machine-code.md) |
| `#` | word | I/O channel number, as in `PRINT #1, X` (or `PRINT #1; X`) | [Files](11-files-and-devices.md) |
| `+` | operator | addition; joins strings | [Expressions](05-expressions.md), [Strings](08-strings.md) |
| `-` | operator | subtraction, negation | [Expressions](05-expressions.md) |
| `*` | operator | multiplication | [Expressions](05-expressions.md) |
| `/` | operator | division (integer or floating point) | [Expressions](05-expressions.md) |
| `^` | operator | power (floating point only) | [Floating Point](09-floating-point.md) |
| `&` | operator | bitwise AND (between two values) | [Expressions](05-expressions.md) |
| `!` | operator | bitwise OR | [Expressions](05-expressions.md) |
| `=` | operator | assignment; comparison "equal" | [Expressions](05-expressions.md) |
| `<>` | operator | comparison "not equal" | [Expressions](05-expressions.md) |
| `<`, `>`, `<=`, `>=` | operator | comparisons | [Expressions](05-expressions.md) |
| `=+` | statement | append a string to a string variable | [Strings](08-strings.md) |
| `[s, n]`, `[s]` | operator | part of a string: n characters from position s, or from s to the end | [Strings](08-strings.md) |
| `,` in `PRINT` | word | move to the next column multiple of 10 | [Console I/O](10-console-io.md#print) |
| `;` in `PRINT` | word | no space between items; at the end, no new line | [Console I/O](10-console-io.md#print) |
| `A, B = F(...)` | statement | assign the results of a `FUNC` with several results | [Functions](07-functions.md) |

## Words that are not keywords

These words of other BASICs are not part of exBASIC. Used as statements,
they give a syntax error, and some can be used as variable names: `REM`
(use `'` or `.`), `LET`, `GOTO`, `GOSUB`, `ON`, `TRAP`, `READ`,
`RESTORE`, `POP`, `STOP`, `CONT`, `LIST`, `RUN` with a line number, `LOAD`,
`SAVE`, `PROC`, `EXEC`, `ENDPROC`, `DIV`, `UINSTR`, `HEX$`. See
[Differences from Other BASICs](C-differences.md).
