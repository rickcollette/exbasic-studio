# Appendix B. Error Messages

This appendix lists the error messages of the compiler and of compiled
programs. For each message it explains the cause and the fix, with a
short program that produces it and the exact text the compiler prints.
The first sections explain how the compiler reports errors; then come
the messages grouped by subject; the last section covers the errors a
program can meet while it runs.

## How errors are reported

The compiler stops at the first error. It prints the file name, the line
number and the column, the message, and the line with a marker:

```
b03.bas:1:11: parse error, expected: variable name but got 'SCORE'
  PRINT Score 
  -----------^
```

- The line number counts from 1. The column counts from 0: it is the
  number of characters before the error position.
- The second line shows the source line split at the error position, with
  one extra space inserted there. The `^` marks the position.
- Lines longer than 76 characters are shown in part.
- Names of variables and functions are shown in uppercase, whatever case
  you used.
- The exit code of the compiler is 1 (see [Messages and exit
  codes](19-compiler-reference.md#messages-and-exit-codes)).

The compiler builds a program twice by default, to choose how much
machine code to use (see [Native code
modes](19-compiler-reference.md#native-code-modes)), but it stops after the
first build when the program has an error, so each error is reported once.

## Reading "parse error, expected:" messages

Most syntax errors have the form `parse error, expected:` followed by a
list. The parser tries every form of statement that could start the line,
and when none fits it lists what it was expecting at the position where it
got furthest. The list mixes three kinds of items:

| Item                       | Meaning                                               |
|----------------------------|-------------------------------------------------------|
| a word in quotes, `'THEN'` | that keyword or symbol would be valid here            |
| a description, `integer expression` | an item of that kind would be valid here     |
| a sentence, `missing IF`   | a specific error found by the compiler (see below)    |

The descriptions come from the grammar:

| Description                                | Expected                                          |
|--------------------------------------------|---------------------------------------------------|
| `statement`                                | a statement or an assignment                      |
| `integer expression`                       | an integer value or expression                    |
| `floating point expression`                | a floating point value or expression              |
| `string expression`                        | a string constant, variable or function           |
| `integer constant, variable or function`   | an integer operand                                |
| `floating point constant, variable or function` | a floating point operand                     |
| `left parenthesis`, `right parenthesis`    | `(` or `)`                                        |
| `right bracket`                            | `]` closing a string slice                        |
| `comma`, `comma followed by an expression` | `,` and the next parameter                        |
| `integer comparison operator`, `comparison operator`, `floating-point comparison operator` | `=`, `<>`, `<`, `>`, `<=` or `>=` |
| `variable name`                            | the name of a variable                            |
| `new variable name`                        | a name for `DIM`                                  |
| `I/O channel number`                       | `#` and a channel number                          |
| `string or variable`                       | the argument of `ADR` or `&`                      |
| `function argument`                        | an argument of a `FUNC` call                      |
| `return value`                             | a value after `RETURN`                            |
| `data type`, `extended data type`          | `BYTE`, `WORD`, `ROM`, `FILE` or `[segment]` in `DATA` |
| `new data array name`, `data byte`, `data word`, `data number` | the parts of a `DATA` statement |
| `segment name`                             | a name between `[` and `]` in `DATA`              |
| `DLI source data`, `DLI destination`       | the parts of a `DLI SET` statement                |
| `'SET', label`                             | what follows `DLI`                                |
| `'WHILE' or 'UNTIL'`                       | the condition of `DO` or `LOOP`                   |
| `NOPEN URL argument`                       | the URL of `NOPEN`                                |

Two items need explaining because they appear in many messages that have
nothing to do with arrays:

- `variable name but got 'X'` means the name `X` was used as a value but
  no variable of that name exists yet. A variable is created by its first
  assignment, `DIM`, `FOR`, `INPUT`, `GET` or `LOCATE`, so this message
  usually means a misspelled name or a value read before it was ever set.
- `byte array and got 'X'`, `float and got 'X'`, `string and got 'X'` and
  `word and got 'X'` mean that a variable `X` exists, but with another
  type than the one that would fit here.

When the first word of a line is not a statement, the parser takes it as
the name of a variable in an assignment and expects `=`. That is why an
unknown statement gives a message that mentions `'='`.

## Syntax errors

### Unknown statement

```basic
GOTO 100
```

```
b02.bas:1:5: parse error, expected: '=', byte array and got 'GOTO', float and got 'GOTO', string and got 'GOTO'
  GOTO  100
  -----^
```

`GOTO` is not an exBASIC statement, so the parser read it as a variable
name and expected `=` after it. The same message appears for any
misspelled statement and for the statements of other BASICs that exBASIC
does not have (`GOTO`, `GOSUB`, `REM`, `LET`, `ON`, `TRAP`, `READ`,
`RESTORE`, `POP`). Check the spelling, or see [Differences from Other
BASICs](C-differences.md) for the exBASIC replacement.

### Incomplete expression

```basic
Total = 
```

```
b01.bas:1:8: parse error, expected: floating point expression, integer expression, string expression
  Total =  
  --------^
```

A value is missing. The same happens after an operator, here expecting an
integer:

```basic
Total = 1 +
```

```
b19.bas:1:11: parse error, expected: '(', '+', '-', floating point constant, variable or function, integer expression
  Total = 1 + 
  -----------^
```

### Variable used before it has a value

```basic
PRINT Score
```

```
b03.bas:1:11: parse error, expected: variable name but got 'SCORE'
  PRINT Score 
  -----------^
```

`Score` was never assigned, so it does not exist. Assign it first
(`Score = 0`), or correct the spelling. The same applies to conditions:
`UNTIL Done` fails if `Done` was never assigned.

### Wrong type of value

```basic
Name$ = 5
```

```
b04.bas:1:8: parse error, expected: string expression
  Name$ =  5
  --------^
```

A string variable needs a string: use `STR$(5)` to convert a number. The
opposite case gives a less direct message, because the parser reads
`"TEN"` as the start of a string comparison, which would give an integer:

```basic
Count = "TEN"
```

```
b05.bas:1:13: parse error, expected: '<', '<=', '<>', '=', '>', '>=', comparison operator
  Count = "TEN" 
  -------------^
```

Use `VAL()` to convert a string to a number.

### A variable and an array with the same name

```basic
Lives = 3
DIM Lives(5)
```

```
b06.bas:2:9: parse error, expected: ',', comma
  DIM Lives (5)
  ---------^
```

The suffix is part of the name, so `Lives`, `Lives$` and `Lives%` are
three different variables, and `Lives()`, `Lives$()` and `Lives%()` three
different arrays. But a variable and an array with the same name and
suffix can't exist together: here `Lives` is already an integer variable,
so `DIM` can't create the array `Lives()`. Use different names. A similar
message appears when you assign to an array name without an index:

```basic
DIM Scores(10)
Scores = 1
```

```
b21.bas:2:7: parse error, expected: byte array and got 'SCORES', float and got 'SCORES', string and got 'SCORES', word and got 'SCORES'
  Scores  = 1
  -------^
```

### Missing parenthesis

```basic
Total = (1 + 2
```

```
b07.bas:1:8: missing ')' for this '('
  Total =  (1 + 2
  --------^
```

The marker points at the `(` that has no matching `)`. A `)` without a
`(` before it gives `')' without '('`, with the marker at the `)`.

### Missing comma after a channel

```basic
OPEN #1, 4, 0, "D:SCORES.DAT"
INPUT #1; Score
```

```
b08.bas:2:8: parse error, expected: ','
  INPUT #1 ; Score
  --------^
```

After `#channel`, `INPUT`, `GET` and the other I/O statements need a
comma, as in `INPUT #1, Score`. Only `PRINT` and `PUT` also accept a
semicolon (`PRINT #1; "HELLO"`), as in Atari BASIC.

### String constant errors

| Message | Cause and fix |
|---------|---------------|
| `unterminated string, missing '"'` | the closing quote of a string constant is missing |
| `string too long (max 255 characters)` | a string constant has more than 255 characters; split it and join the parts with `+` or `=+` |
| `bad character code in string, use $00 to $FF` | a `$` after the closing quote is not followed by a hexadecimal code, as in `"A"$GG` |

```basic
PRINT "HELLO
```

```
b09.bas:1:6: unterminated string, missing '"'
  PRINT  "HELLO
  ------^
```

### Number out of range

```basic
Total = 70000
```

```
e1.bas:1:8: integer constant out of range (max 65535), write 70000.0 for a floating point number
  Total =  70000
  --------^
```

Integer constants go from 0 to 65535 (with a `-` before them, down to
-32768). A number with a decimal point or an exponent, like `70000.0` or
`7E4`, is a floating point constant; store it in a floating point
variable (`Total% = 70000.0`). A hexadecimal constant larger than `$FFFF`
gives `hex constant out of range (max $FFFF)`.

### Floating point value in an integer variable

```basic
Count = 3
Total = Count * 1.5
```

```
b22.bas:2:19: parse error, expected: '<', '<=', '<>', '=', '>', '>=', floating-point comparison operator
  Total = Count * 1.5
  -------------------^
```

An integer variable can only receive an integer value. Use a floating
point variable (`Total% = Count * 1.5`) or convert with `INT`
(`Total = INT(Count * 1.5)`). In `PRINT` and in conditions, mixed
expressions are calculated in floating point, so `PRINT Count * 1.5`
works. The integer operators `MOD`, `&`, `!` and `EXOR` need parentheses
in a floating point expression: `PRINT Count MOD 2 + 1.5` gives
`expected: integer expression`; write `PRINT (Count MOD 2) + 1.5`.

### THEN without a statement

```basic
Count = 1
IF Count = 1 THEN
```

```
b23.bas:2:17: expected a statement after THEN
  IF Count = 1 THEN
  -----------------^
```

With `THEN`, the statements must follow on the same line. For a block
that goes on the next lines, leave out `THEN` and close it with `ENDIF`.
A loop started after `THEN` that does not end on the same line gives
`blocks started after THEN must end in the same line`.

### Array size out of range

```basic
DIM Table(20000)
```

```
b24.bas:1:16: array size out of range (0 to 16383)
  DIM Table(20000)
  ----------------^
```

The largest size is 32767 for a `BYTE` array, 16383 for a word or string
array and 5375 for a floating point array. A size computed while the
program runs is checked then (see [Error 9: bad DIM size](#error-9-bad-dim-size)).

### Too many variables

```
exbasic: prog.bas: too many variables, they use 600 bytes and the limit is 512 (integer and string variables use 2 bytes, floating point 6); use arrays for big tables
```

All the variables, `FUNC` parameters, locals and hidden strings of `+`
share an area of 512 bytes. Use arrays for big tables of values, and
`LOCAL` variables only where they are needed.

### Expression too complex

```
d11.bas:1:57: expression too complex for the compiler
```

The expression nests too many parentheses or operators for the parser
(about 50 levels). Split it with intermediate variables.

## Block structure errors

These errors come from statements that open and close blocks: loops,
`IF`, `FUNC` and `DATA`. A closing statement must match the last block
still open. When another block is still open inside, the message also
names it, as in `NEXT without FOR loop (an IF is still open)`.

| Message | Cause |
|---------|-------|
| `NEXT without FOR loop`, `NEXT J without FOR` | `NEXT` without an open `FOR`, or another block (like `WHILE`) is still open inside the `FOR` |
| `NEXT J does not match FOR I` | the variable after `NEXT` is not the counter of the innermost `FOR` |
| `WEND without WHILE loop` | `WEND` without an open `WHILE` |
| `UNTIL without REPEAT loop` | `UNTIL` without an open `REPEAT` |
| `ELIF without IF`, `ELSE without IF`, `ENDIF without IF` | no open multi-line `IF`, or a second `ELSE` in the same `IF` |
| `LOOP without DO loop` | `LOOP` without an open `DO` |
| `missing FOR loop`, `missing IF`, ... | the same errors, when the closing statement is abbreviated (`N.`, `E.`) |
| `blocks started after THEN must end in the same line` | a loop or block started in a one-line `IF ... THEN` is not closed on that line |
| `EXIT without loop` | `EXIT` outside any loop or `FUNC` |
| `invalid EXIT` | `EXIT` between a `DATA` line that ends with `,` and its continuation |
| `unclosed FOR loop` | the program ends with a block still open; also `unclosed WHILE loop`, `unclosed REPEAT loop`, `unclosed DO loop`, `unclosed IF`, `unclosed FUNC` |

```basic
FOR I = 1 TO 3
  PRINT I
NEXT J
```

```
b26.bas:3:6: NEXT J does not match FOR I
  NEXT J
  ------^
```

```basic
FOR I = 1 TO 10
  PRINT I
```

```
b16.bas:1: unclosed FOR loop
```

The "unclosed" message has no column. Its line number is the line where
the open block starts, in the file that contains it (also in a file read
with `INCLUDE`).

Indent the body of each block by two spaces (see [Control
Flow](06-control-flow.md)); a missing `ENDIF` or `NEXT` is then easy to
see.

## FUNC errors

### Errors in the definition line

The compiler reads all `FUNC` definition lines before compiling, so these
errors show only the line number, without a column.

| Message | Cause and fix |
|---------|---------------|
| `expected '(' after FUNC name` | write `()` even without parameters: `FUNC Beep()` |
| `expected ')' after parameters` | close the parameter list |
| `expected parameter name` | parameters must be variable names, not values |
| `expected result name` | named results must be variable names |
| `expected end of FUNC definition` | results must be separated by commas: `FUNC DivMod(A, B) Quot, Rest` |
| `FUNC 'NAME' already defined at line N` | two `FUNC`s with the same name; rename one |
| `repeated parameter name 'X'` | two parameters with the same name |
| `repeated result name 'X'` | a result with the name of another result or of a parameter |
| `FUNC name 'F$' must have the type of its first result 'X'` | with named results, the `FUNC` name and the first result need the same suffix: `FUNC Greeting$() Text$` |
| `FUNC 'F$' must return a value` | a string or floating point `FUNC` needs a `RETURN` with a value or named results |

```basic
FUNC Square
  RETURN 1
ENDFUNC
```

```
c02.bas:1: expected '(' after FUNC name
```

```basic
FUNC Beep()
ENDFUNC

FUNC Beep()
ENDFUNC
```

```
c07.bas:4: FUNC 'BEEP' already defined at line 1
```

### Errors in the body

| Message | Cause and fix |
|---------|---------------|
| `FUNC can't be inside another FUNC` | a `FUNC` line before the `ENDFUNC` of the previous one; close the first `FUNC` |
| `FUNC definition must be in its own line` | `FUNC` after `:` on a line; put it on its own line |
| `'NAME' is a FUNC` | a `DATA` array or `COMMON` variable with the name of a `FUNC` |
| `LOCAL can only be used inside a FUNC` | `LOCAL` in the main program |
| `'X' is already a local variable` | the same name twice in `LOCAL`, or a `LOCAL` with the name of a parameter |
| `ENDFUNC without FUNC` | an extra `ENDFUNC` |
| `RETURN outside FUNC` | `RETURN` in the main program; use `END` to stop the program |
| `RETURN value N must be string` (or `integer`, `floating point`) | the value has the wrong type for the `FUNC` |
| `FUNC 'NAME' returns N value(s)` | `RETURN` with more values than the `FUNC` has results |
| `RETURN needs a value in FUNC 'NAME'` | a `RETURN` without value in a `FUNC` that returns one |
| `too many local variables in FUNC 'NAME'` | a recursive `FUNC` with more than 255 parameters and locals |

```basic
FUNC Square(Value)
  IF Value = 0
    RETURN
  ENDIF
  RETURN Value * Value
ENDFUNC
```

```
c27.bas:3:10: RETURN needs a value in FUNC 'SQUARE'
      RETURN 
  ----------^
```

A `FUNC` that returns a value must return one on every `RETURN`: here,
`RETURN 0`.

### Errors in calls

| Message | Cause and fix |
|---------|---------------|
| `FUNC 'NAME' is not defined` | a call of a `FUNC` that does not exist; check the name. Also shown for calls of functions of other BASICs, like `RND(0)` |
| `FUNC 'NAME' needs N argument(s)` | too many arguments |
| `FUNC 'NAME' needs N argument(s), got M` | too few arguments |
| `argument N of 'NAME' must be integer` (or `string`) | an argument of the wrong type |
| `FUNC 'NAME' does not return a value` | a `FUNC` without results used in an expression |
| `FUNC 'NAME' returns 2 values, use 'A, B = NAME(...)'` | a `FUNC` with several results used in an expression |
| `FUNC 'NAME' returns N value(s), but there are M variables` | `A, B, C = F()` with a different number of results |
| `result N of 'NAME' is integer, but 'X$' is string` | a variable of the wrong type receives a result |
| `variable 'X' is not integer` | the variable receiving a result already exists with another type |
| `'NAME' is a FUNC, it can't be used as a variable` | an assignment to the name of a `FUNC` |

```basic
Result = Square(2, 3)

FUNC Square(Value)
  RETURN Value * Value
ENDFUNC
```

```
c22.bas:1:19: parse error, expected: FUNC 'SQUARE' needs 1 argument(s)
  Result = Square(2,  3)
  -------------------^
```

```basic
Result = DivMod(7, 2)

FUNC DivMod(A, B) Quot, Rest
  Quot = A / B
  Rest = A MOD B
ENDFUNC
```

```
c20.bas:1:9: FUNC 'DIVMOD' returns 2 values, use 'A, B = DIVMOD(...)'
  Result =  DivMod(7, 2)
  ---------^
```

A call needs its parentheses: `Area = RectArea` without `()` reads
`RectArea` as a variable and gives `variable name but got 'RECTAREA'`. An
integer argument passed to a floating point parameter is converted, so it
is not an error.

See [Functions](07-functions.md) for the rules.

## INCLUDE errors

`INCLUDE` lines are processed before compilation. Their errors start with
`exbasic:` and give the file and line, without a column.

| Message | Cause and fix |
|---------|---------------|
| `INCLUDE file 'name' not found` | the file is not in the folder of the including file, the `-I` folders or the compiler `include` folder |
| `invalid INCLUDE, use: INCLUDE "file"` | the name has no quotes, is empty, or other text follows on the line (only a `'` comment may follow) |
| `recursive INCLUDE of 'name'` | a file includes itself, directly or through other files |
| `too many nested INCLUDE files` | more than 16 levels of `INCLUDE` inside included files |
| `can't open input file 'name'` | the main source file does not exist |

```basic
INCLUDE "screen.bas"
```

```
exbasic: d01.bas:1: INCLUDE file 'screen.bas' not found
```

An error inside an included file is reported with the name and line of
that file (for example `./screen.bas:2:4: ...`).

Only one BASIC file can be given to the compiler. A second one stops with:

```
exbasic: only one BASIC file can be linked into a program, 'util.bas' is the second one; add it with INCLUDE "util.bas" instead
```

See [INCLUDE](17-program-organization.md#include).

## DATA errors

| Message | Cause and fix |
|---------|---------------|
| `can't open data file 'name'` | the file of `DATA ... FILE "name"` is not in the folder of the source or the current folder |
| `new label, got label already defined 'NAME'` | two `DATA` arrays with the same name |
| `segment name` (in the expected list) | `DATA Name() BYTE [segment]` without a valid segment name |

```basic
DATA Colors() BYTE = 1, 2
DATA Colors() BYTE = 3, 4
```

```
d05.bas:2:11: parse error, expected: new label, got label already defined 'COLORS'
  DATA Colors () BYTE = 3, 4
  -----------^
```

See [DATA](04-variables-and-data.md#data).

## COMMON errors

| Message | Cause and fix |
|---------|---------------|
| `variable 'X' is not integer` (or `string`, `floating point`) | the name in `COMMON` is already used by an array |
| `'NAME' is a FUNC, it can't be in COMMON` | a `FUNC` name in `COMMON` |
| `only one COMMON statement is allowed` | a second `COMMON` statement |
| `COMMON can't be inside a FUNC` | a `COMMON` statement inside a `FUNC` |

```basic
COMMON Score
COMMON Level
```

```
d07.bas:2:6: only one COMMON statement is allowed
  COMMON  Level
  ------^
```

Put all the shared variables in one `COMMON` line in the main program.
See [COMMON](17-program-organization.md#common).

## Assembler and linker errors

After the compiler, `ca65` and `ld65` can report errors. Their messages
are printed as is, followed by `exbasic: can't assemble file` or
`exbasic: can't link file`. Before giving up on a link error, the
compiler retries without extended instructions and without machine code
(see [Automatic retries](19-compiler-reference.md#automatic-retries)), so
the messages appear several times.

| Message | Cause and fix |
|---------|---------------|
| `Segment 'RUNTIME' overflows memory area 'MAIN' by N bytes` and `Cannot generate most of the files due to memory area overflow` | the program does not fit between the start address and `$BC20`. Use `-no-native`, `-no-ext`, a lower `-S` address, or make the program smaller |
| `Unresolved external 'NAME'` | an `@NAME` symbol not exported by any assembly module |
| `Missing memory area assignment for segment 'NAME'` | a `-s` segment or `DATA [segment]` not in the linker configuration |
| `Cannot find config file 'name'` | the file given with `-C` does not exist |

For a cartridge, the whole program must fit in 8 KB of ROM; an overflow
of the `ROM` area means the program is too big for a cartridge.

## Load errors

### Error 19: not loaded

A compiled `.xex` program checks the memory before it is loaded. Its
first block is a short check that DOS runs while loading the file, before
the rest of the program is read. When the program can't fit, it prints

```
ERROR- 19
```

and goes back to DOS without loading anything else, so the DOS and its
drivers are not overwritten. The check fails when:

- the DOS ends above the program's start: MEMLO (743, 744) is higher
  than the start address, $2000 unless compiled with `-S:`. This happens
  with a DOS that uses a lot of memory, or with many drivers loaded;
- the program ends above MEMTOP (741, 742), for example when the BASIC
  cartridge is on and the program is longer than about 30 KB.

In the first case, compile with a higher start address, for example
`exbasic -S:0x3000 game.bas` (see [-S](19-compiler-reference.md#-s)). In
the second, turn the BASIC cartridge off or make the program smaller.

The check also runs when a program is started by another one with `RUN`.
There the first program is already gone, so the message is followed by the
DOS, not by the first program.

## Runtime errors

A compiled program checks a few errors while it runs. When one happens,
the program prints `ERROR-` and the error number on the screen, as Atari
BASIC does, and ends: it closes the I/O channels 1 to 7, turns the sound
off and returns to DOS. There is no equivalent of `TRAP`. The short
message keeps the error code small: about 75 bytes for all the errors.

| Number | Error | Cause |
|-------:|-------|-------|
| 2 | Out of memory | `DIM`, the first assignment of a string, a hidden string for `+` or `PMGRAPHICS` needs more memory than is free |
| 9 | Bad DIM size | `DIM` with a size computed at run time that is out of range, for example negative |
| 10 | Stack overflow | recursion too deep (about 95 nested calls), too many nested `FOR` loops, or an expression too deep for the value stacks |
| 11 | Division by zero | a division or `MOD` by zero, with integers or floating point |
| 19 | Not loaded | the program does not fit in memory: printed by the load check, before the program starts (see [Error 19: not loaded](#error-19-not-loaded)) |
| 128 | Break | the user pressed the BREAK key |

The numbers are the ones of the same errors in Atari BASIC. 128 is also
the I/O error code of BREAK, which `ERR()` returns when the BREAK key
ends an `INPUT` or `GET` (see [I/O errors and ERR()](#io-errors-and-err)).

### Error 2: out of memory

```basic
' Ask for more memory than the Atari has
DIM Small(10000)
PRINT "20 KB allocated"
DIM Huge(15000)
PRINT "Not reached"
```

Output:

```
20 KB allocated

ERROR- 2
```

Each element of a word array takes two bytes, so `Small` takes 20 KB and
`Huge` would need 30 KB more. To avoid the error, measure the memory with
`FRE()` and reduce the arrays and strings (see [Performance and
Memory](20-performance-and-memory.md#measuring-memory)). After
`PMGRAPHICS`, arrays and strings can't grow into the P/M graphics memory:
they give error 2 too.

### Error 9: bad DIM size

```basic
' A negative DIM size stops the program
Size = -2
DIM Scores(Size)
PRINT "Not reached"
```

Output:

```

ERROR- 9
```

A constant size out of range is found by the compiler instead (see
[Array size out of range](#array-size-out-of-range)).

### Error 10: stack overflow

```basic
' Recursion without an end
PRINT "Start"
PRINT Deep(0)

' Calls itself forever
FUNC Deep(Level)
  RETURN Deep(Level + 1) + 1
ENDFUNC
```

Output:

```
Start

ERROR- 10
```

The same error appears when the 512-byte frame stack of the parameters
and locals of recursive functions is full, and when too many values wait
on the value stacks (see [Recursion](07-functions.md#recursion)).

### Error 11: division by zero

```basic
' Division by zero stops the program
Divisor = 0
PRINT "Before"
PRINT 10 / Divisor
PRINT "Not reached"
```

Output:

```
Before

ERROR- 11
```

Check the divisor before dividing when it can be 0:
`IF Divisor <> 0 THEN Result = Total / Divisor`.

### Error 128: break

Pressing the BREAK key stops any program, also the parts compiled to
machine code, with error 128 (`ERROR- 128`). An `INPUT` or `GET` that is
waiting for the keyboard ends with error 128 in `ERR()` instead.

### I/O errors and ERR()

Statements that use the Atari I/O system (`OPEN`, `CLOSE`, `PRINT #`,
`INPUT #`, `GET #`, `PUT #`, `BGET`, `BPUT`, `XIO`, `NOTE`, `POINT`,
`STATUS`, `RUN` and also `INPUT` and `GET` on the keyboard) do not stop
the program on errors. They store a result code that `ERR()` returns: 1
for success, 128 or more for an error. `ERR()` is 1 when the program
starts.

```basic
' Check the result of each I/O operation
OPEN #1, 4, 0, "D:NOFILE.DAT"
Result = ERR()
PRINT "OPEN D:NOFILE.DAT: "; Result
CLOSE #1

OPEN #2, 4, 0, "Q:"
Result = ERR()
PRINT "OPEN Q: "; Result
CLOSE #2

GET #3, Char
Result = ERR()
PRINT "GET from a closed channel: "; Result
```

Output:

```
OPEN D:NOFILE.DAT: 170
OPEN Q: 130
GET from a closed channel: 133
```

Printing on the screen does not change `ERR()`, so
`PRINT "Error: "; ERR()` shows the result of the operation before it:

```basic
' PRINT on the screen does not change ERR()
OPEN #1, 4, 0, "D:NOFILE.DAT"
PRINT "Error: "; ERR()
CLOSE #1
```

Output:

```
Error: 170
```

Still, save `ERR()` in a variable when you need it later: the next I/O
operation, conversion or floating point operation sets it again.

The most common codes:

| Code | Meaning |
|-----:|---------|
| 1    | success |
| 128  | BREAK key pressed during I/O |
| 130  | nonexistent device |
| 133  | channel not open |
| 136  | end of file |
| 138  | device does not respond (timeout) |
| 144  | device done error (for example, disk write protected) |
| 162  | disk full |
| 165  | bad file name |
| 170  | file not found |

See [Files and Devices](11-files-and-devices.md#err) for the complete
table, and [SERR()](18-sio-and-fujinet.md#serr) for the result of `SIO`.

### Errors that are not detected

| Situation | What happens |
|-----------|--------------|
| integer overflow | the result wraps around: `32767 + 1` is `-32768` |
| array index out of range | reads or writes memory outside the array, which can corrupt other data or crash |
| string longer than 255 characters | the string is cut at 255 characters |
| `RUN` of a missing file | the program continues with the next statement; `ERR()` gives the error code |

```basic
' Integer overflow does not stop the program
Big = 32767
PRINT Big + 1
```

Output:

```
-32768
```

Check array indexes yourself where bad values are possible.
