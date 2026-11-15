# 4. Variables and Data

This chapter describes the three types of variables (integer, floating point
and string), how variables are created and assigned, arrays and the `DIM`
statement, constant arrays defined with `DATA`, the `INC`, `DEC` and `CLR`
statements, and how much memory each kind of data uses.

## Variable types

| Type | Name suffix | Values | Memory | Initial value |
| --- | --- | --- | --- | --- |
| Integer | none | -32768 to 32767 | 2 bytes | 0 |
| Floating point | `%` | about ±1E-98 to ±1E+98, 9 to 10 digits | 6 bytes | 0 |
| String | `$` | 0 to 255 characters | 2 bytes, plus 256 bytes once assigned | empty string |

The suffix is part of the name and fixes the type of the variable for the
whole program. `Score`, `Score$` and `Score%` are three different
variables, and `Score()`, `Score$()` and `Score%()` three different
arrays. A variable and an array with the same name and suffix can't both
exist (see [Names](03-language-basics.md#names)).

### Integer variables

Integer variables hold whole numbers from -32768 to 32767, stored in 16 bits
(two bytes). They are the default type and the fastest one: use them for
counters, coordinates, scores, addresses and anything else that fits.

Integer arithmetic wraps around: adding 1 to 32767 gives -32768, with no
error (see [Integer arithmetic](05-expressions.md#integer-arithmetic)).

Addresses above 32767 are stored as negative numbers with the same bits.
`PEEK`, `POKE` and the other memory statements treat the 16 bits as an
address from 0 to 65535, so this works without any special care:

```basic
' Addresses above 32767 are negative but work normally
Consol = $D01F    ' console keys register, 53279
PRINT Consol
PRINT PEEK(Consol) = PEEK(53279)
```

Output:

```
-12257
1
```

### Floating point variables

Floating point variables have names ending in `%`. They hold numbers with a
fractional part and a large range, in the 6-byte binary coded decimal (BCD)
format of the Atari operating system. They are much slower than integers;
use them only when you need fractions or large values.

```basic
Radius% = 2.5
Area% = 3.14159 * Radius% * Radius%
PRINT Area%
```

Output:

```
19.6349375
```

[Chapter 9](09-floating-point.md) describes the format, the operators and
the functions.

### String variables

String variables have names ending in `$`. They hold text of 0 to 255
characters. The first assignment to a string variable reserves a 256-byte
buffer for it (one byte for the length and 255 for the characters); later
assignments reuse the same buffer, whatever the length of the text.

```basic
Greeting$ = "HELLO"
Greeting$ =+ ", WORLD"
PRINT Greeting$; " has "; LEN(Greeting$); " characters"
```

Output:

```
HELLO, WORLD has 12 characters
```

[Chapter 8](08-strings.md) describes strings in detail.

## Creating variables

A variable is created by the first statement in the source file that
assigns it a value: an assignment, `FOR`, `INPUT`, `GET`, `INC`, `DEC`, a
multiple assignment from a function, or `DIM`. From that line on, the
variable can be used in expressions.

"First" refers to the position in the source file, not to the order of
execution. A variable used in an expression before any line that creates it
is a compile time error:

```
PRINT Total
Total = 5
```

```
prog.bas:1:11: parse error, expected: variable name but got 'TOTAL'
```

A variable that is created by a line that never runs still exists, with its
initial value:

```basic
' Total is created by the IF line, even if the
' assignment never runs
Debug = 0
IF Debug THEN Total = 100
PRINT Total
```

Output:

```
0
```

When the program starts, all integer and floating point variables are 0 and
all string variables are empty.

To create a variable without assigning it, list it in a `DIM` statement (see
[DIM](#dim)). This is useful when the first use of a variable in the source
file reads it, for example in a function written before the code that sets
it.

### Global and private variables

All variables are global: they can be used anywhere in the program,
including inside functions. The exceptions are the parameters, named results
and `LOCAL` variables of a function, which are private to that function (see
[chapter 7](07-functions.md)).

### Limit on the number of variables

All variables are kept in one area of at most 512 bytes. Each integer
variable, string variable and array uses 2 bytes, and each floating point
variable uses 6 bytes, so a program can have up to 256 integer variables, or
fewer if it uses floating point variables. Function parameters, results and
local variables count too. The contents of strings and arrays are kept
elsewhere and do not count.

If the program has too many variables, the compiler stops with:

```
exbasic: prog.bas: too many variables, they use 600 bytes and the limit is 512 (integer and string variables use 2 bytes, floating point 6); use arrays for big tables
```

Use arrays to hold large groups of values.

## Assignment

### Assignment
Gives a new value to a variable or array element.

```
var = expression
var$ = text$
var$ =+ text$
var1, var2, ... = function(args)
```
Abbreviation: none (there is no `LET` keyword)

The variable on the left gets the value of the expression on the right. If
the variable does not exist yet, the assignment creates it with the type
given by its suffix.

| Target | Allowed values | Notes |
| --- | --- | --- |
| Integer variable or word array element | Integer expression | A floating point value is an error; convert it with `INT` (rounds to the nearest integer). |
| Byte array element | Integer expression | Only the low 8 bits are stored: 300 is stored as 44, -1 as 255. |
| Floating point variable or array element | Floating point or integer expression | Integers are converted to floating point. |
| `%(addr)` | Floating point expression | Stores 6 bytes at address `addr` (see [chapter 9](09-floating-point.md)). |
| String variable or array element | String expression | Text longer than 255 characters is cut. |

The form `var$ =+ text$` appends the text to the end of the string variable,
like `var$ = var$ + text$` (see [chapter 8](08-strings.md)).

The form `var1, var2, ... = function(args)` assigns the results of a
function that returns several values (see [chapter 7](07-functions.md)).

An expression with `=` on the right side of an assignment is a comparison,
not a second assignment: `Ok = Count = 3` sets `Ok` to 1 if `Count` is 3,
and to 0 otherwise.

```basic
' The assignment forms
DIM Small(2) BYTE, Values(2)
Count = 3
Small(0) = 300            ' only the low byte is kept
Values(1) = Count * 2
Ratio% = Count / 2.0
Rounded = INT(Ratio%)     ' 1.5 rounds to 2
Name$ = "ATARI"
Name$ =+ " 800"
Ok = Count = 3            ' a comparison: 1 means true
PRINT Small(0); " "; Values(1); " "; Ratio%; " "; Rounded
PRINT Name$; " "; Ok
```

Output:

```
44 6 1.5 2
ATARI 800 1
```

### INC
Adds 1 to an integer variable or word array element.

```
INC var
```
Abbreviation: none

`INC var` is the same as `var = var + 1`, but shorter and faster. `var` must
be an integer variable or an element of a word array; byte arrays and
floating point variables are not allowed. Like other arithmetic, `INC`
wraps around: `INC` on 32767 gives -32768. If the variable does not exist
yet, `INC` creates it, so it becomes 1.

```basic
' Count the multiples of 3 below 20
Multiples = 0
FOR I = 1 TO 19
  IF I MOD 3 = 0 THEN INC Multiples
NEXT I
PRINT Multiples
```

Output:

```
6
```

### DEC
Subtracts 1 from an integer variable or word array element.

```
DEC var
```
Abbreviation: `DE.`

`DEC var` is the same as `var = var - 1`, but shorter and faster. The same
rules as `INC` apply: `var` must be an integer variable or a word array
element, and `DEC` on -32768 gives 32767.

```basic
DIM Lives(1)
Lives(0) = 3
Lives(1) = 3
DEC Lives(1)
PRINT "PLAYER 1: "; Lives(0); "  PLAYER 2: "; Lives(1)
```

Output:

```
PLAYER 1: 3  PLAYER 2: 2
```

## Arrays

An array is a numbered group of values with one name. Each value is an
element, selected by an index written in parentheses after the name:
`Scores(3)`. Arrays must be created with `DIM` (or `DATA`) before they are
used.

Indexes start at 0. `DIM Scores(10)` creates 11 elements, `Scores(0)` to
`Scores(10)`: the number in `DIM` is the highest index, not the number of
elements.

An index can be any integer expression. Floating point indexes are not
allowed.

Array elements can be used wherever a variable of the same type can be used,
including as the target of an assignment, `INC`, `DEC`, `INPUT`, `GET` and
`FOR`.

### Kinds of arrays

| Kind | How to create | Values | Memory per element |
| --- | --- | --- | --- |
| Word | `DIM A(n)` or `DIM A(n) WORD` | -32768 to 32767 | 2 bytes |
| Byte | `DIM A(n) BYTE` | 0 to 255 | 1 byte |
| Floating point | `DIM A%(n)` | Floating point | 6 bytes |
| String | `DIM A$(n)` | 0 to 255 characters | 2 bytes, plus 256 bytes for each element once assigned |

Byte arrays use half the memory of word arrays and are a good choice for
flags, small counters, screen data and tables of bytes. When you store a
value in a byte array element, only the low 8 bits are kept, and reading it
always gives a value from 0 to 255.

### No bounds checking

exBASIC does not check array indexes, for speed. An index below 0 or above
the size given in `DIM` reads or writes the memory before or after the array,
which can contain other arrays, strings or the program itself. Writing out of
bounds can make the program misbehave or crash. Check indexes yourself when
they come from calculations or from the user.

### Arrays in memory

The elements of an array are stored one after the other: element `I` of a
word array is at `ADR(A) + 2 * I`, element `I` of a byte array at
`ADR(A) + I`, element `I` of a floating point array at `ADR(A%) + 6 * I`.
`ADR(A)` (or `&A`) gives the address of element 0 (see
[chapter 16](16-memory-and-machine-code.md)).

Arrays and strings are allocated at run time, when `DIM` executes, from the
free memory after the program. If there is not enough memory, the program
prints error 2 (`ERROR- 2`) and ends.

### DIM
Creates arrays and declares variables.

```
DIM name(size) [WORD | BYTE], ...
DIM name%(size), ...
DIM name$(size), ...
DIM name, name%, name$, ...
```
Abbreviation: `DI.`

Each item in the list creates one array or declares one variable. Items are
separated by commas and can be of different kinds.

- `name(size)` creates a word array with elements 0 to `size`. The word
  `WORD` (abbreviation `W.`) can be written after it but is not needed.
- `name(size) BYTE` creates a byte array (abbreviation `B.`).
- `name%(size)` creates a floating point array.
- `name$(size)` creates a string array. `BYTE` and `WORD` cannot be used
  with `%` and `$` arrays.
- `name`, `name%` or `name$` without a size declares a variable of that
  type. If the variable does not exist, it is created with its initial value
  (0 or empty); if it exists with the same type, nothing happens.

`size` is an integer expression evaluated when `DIM` runs, so the size of an
array can be computed by the program. The new array is filled with zeros (or
empty strings).

Rules and limits:

- Each array name can appear in only one `DIM` statement in the source file,
  and cannot also be the name of a variable or a `DATA` array.
- The memory needed is `(size + 1)` times the element size. If it is more
  than the free memory, the program prints error 2 (`ERROR- 2`) and ends. On a
  computer with 48 KB, a few tens of kilobytes are usually free; see
  [FRE()](05-expressions.md#fre).
- `size` must be from 0 to a maximum that depends on the kind of array:

  | Kind | Largest size |
  | --- | --- |
  | Byte array | 32767 |
  | Word array, string array | 16383 |
  | Floating point array | 5375 |

  A constant size out of range is a compile error, for example
  `array size out of range (0 to 16383)` for `DIM Table(20000)`. A
  computed size out of range, such as a negative value, stops the program
  with error 9 (`ERROR- 9`).
- When a `DIM` statement runs again (in a loop or a function), it allocates a
  new, empty array and the memory of the old one is not reused until `CLR`.

```basic
' Create arrays of each kind
NumPlayers = 4
DIM Scores(NumPlayers), Flags(NumPlayers) BYTE
DIM LapTime%(NumPlayers), Names$(NumPlayers)
Scores(1) = 1200
Flags(1) = 1
LapTime%(1) = 12.5
Names$(1) = "ANA"
PRINT Names$(1); " "; Scores(1); " "; Flags(1); " "; LapTime%(1)
PRINT "UNUSED: "; Scores(2); " ["; Names$(2); "]"
```

Output:

```
ANA 1200 1 12.5
UNUSED: 0 []
```

The memory used by each kind of array, measured with `FRE()`:

```basic
' Bytes used by arrays of 10 elements (0 to 9)
Before = FRE()
DIM Words(9)
PRINT "WORD:  "; Before - FRE()
Before = FRE()
DIM Bytes(9) BYTE
PRINT "BYTE:  "; Before - FRE()
Before = FRE()
DIM Reals%(9)
PRINT "FLOAT: "; Before - FRE()
Before = FRE()
DIM Texts$(9)
PRINT "STRING: "; Before - FRE()
Before = FRE()
Texts$(0) = "X"
PRINT "ONE STRING ASSIGNED: "; Before - FRE()
```

Output:

```
WORD:  20
BYTE:  10
FLOAT: 60
STRING: 20
ONE STRING ASSIGNED: 256
```

## Constant arrays with DATA

### DATA
Defines an array with initial values, stored in the program.

```
DATA name() [WORD] [ROM | [segment]] = value, value, ...
DATA name() BYTE [ROM | [segment]] = item, item, ...
DATA name%() [ROM | [segment]] = fp, fp, ...
DATA name() BYTEFILE [ROM | [segment]] "file"
DATA name() [WORD]FILE [ROM | [segment]] "file"
```
Abbreviation: `DA.`

`DATA` creates an array whose values are written into the program file by
the compiler. Nothing happens when execution reaches a `DATA` statement: the
values are already in memory when the program starts. A `DATA` array can be
used like an array created with `DIM`: its elements can be read and also
changed (except in ROM, see below).

The parts of the statement:

| Part | Meaning |
| --- | --- |
| `name()` | The name of the new array, followed by `()`. Use a name ending in `%` for floating point values. The name must not be used by another `DATA` array or a function. |
| `WORD` (`W.`) | Each value is a 16-bit word (-32768 to 32767, or 0 to 65535). This is the default. |
| `BYTE` (`B.`) | Each value is a byte (0 to 255). |
| `ROM` | Place the data with the program code (see [Data in ROM](#data-in-rom-and-other-segments)). |
| `[segment]` | Place the data in the named linker segment. |
| `= values` | The list of values, separated by commas. |
| `FILE "file"` | Read the values from a binary file at compile time (see [Data from files](#data-from-files)). |

The size of the array is the number of values: `DATA Days() = 31, 28, 31`
creates `Days(0)` to `Days(2)`. There is no bounds checking.

Values must be constants. Word values are decimal or hexadecimal numbers,
optionally negative. Byte values are numbers from 0 to 255 (negative numbers
are not allowed), and can also be:

- a string constant, which stores all its bytes including the first one,
  the length (`"AB"` stores 2, 65, 66);
- the name of another byte `DATA` array defined earlier, which stores the
  address of that array as two bytes (low byte first).

Floating point values go in an array whose name ends in `%` (see
[Floating point DATA](#floating-point-data)).

```basic
' Days of each month, and their names in a byte array
DATA DaysIn() = 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31
DATA Initials() BYTE = "JFMAMJJASOND"
Total = 0
FOR Month = 0 TO 11
  Total = Total + DaysIn(Month)
NEXT Month
PRINT "DAYS IN A YEAR: "; Total
' Element 0 is the length of the string, the letters follow
PRINT "LETTERS: "; Initials(0); ", FIRST: "; CHR$(Initials(1))
```

Output:

```
DAYS IN A YEAR: 365
LETTERS: 12, FIRST: J
```

#### Continuation lines

A long list can be split over several lines. End the line with a comma, and
continue on the next line with another `DATA` statement without a name,
starting with the type and `=`:

```basic
' A table split over several lines
DATA Powers() WORD = 1, 2, 4, 8, 16, 32, 64, 128,
DATA          WORD = 256, 512, 1024, 2048, 4096
DATA Codes() BYTE = $41, $42,
DATA         BYTE = $43
PRINT Powers(12); " "; Codes(2)
```

Output:

```
4096 67
```

For word data the type can be left out on the continuation line
(`DATA = 256, 512`). The continuation must follow immediately; a `DATA` line
that ends with a comma and is not followed by a continuation line gives the
error `unclosed FUNC/DATA block`. The compiler does not check that the
continuation uses the same type as the first line: use the same type on all
the lines of one array.

#### Floating point DATA

A floating point `DATA` array has a name ending in `%`. All its values are
stored as floating point numbers, also the ones written without a decimal
point, like `10`:

```basic
DATA Prices%() = 1.5, 2.25, 10, 0.99
PRINT Prices%(1); " "; Prices%(2); " "; Prices%(3)
```

Output:

```
2.25 10 0.99
```

A long floating point array continues on the next line in the same way as
the other arrays: end the line with a comma and write `DATA =` followed by
more values:

```basic
DATA Prices%() = 1.5, 2.25,
DATA = 10, 0.99
```

A `DATA` statement whose name has no `%` can't contain floating point values
(`DATA Rate() = 1.5` is an error).

#### Data from files

`BYTEFILE` and `WORDFILE` read the contents of a binary file into the array
when the program is compiled:

```
DATA Picture() BYTEFILE "picture.bin"
DATA Table() WORDFILE "table.bin"
```

With `BYTEFILE`, each byte of the file is one element. With `WORDFILE`,
each pair of bytes is one element (low byte first). Up to 65536 bytes are
read. The words can also be written separately (`BYTE FILE`, `WORD FILE`)
and abbreviated: `B.F.` for `BYTEFILE`, `F.` or `W.F.` for `WORDFILE`.

The compiler searches the file first in the folder of the main source file
(the file given on the command line, also when the `DATA` line is in a file
read with `INCLUDE`), then in the current folder. The `-I:` folders are not
searched. If the file is not found, the compiler stops with
`can't open data file`.

#### Data in ROM and other segments

By default, `DATA` arrays are stored in the `DATA` segment, in RAM. In a
cartridge, the values are stored in the cartridge and copied to RAM when the
program starts, so they use both cartridge space and RAM.

The word `ROM` after the type stores the array with the program code instead.
In an executable this makes no difference. In a cartridge, the array stays
in the cartridge ROM: it uses no RAM, but it cannot be changed.

```
DATA Font() BYTE ROM = $00, $18, $3C, $66, $7E, $66, $66, $00
```

A segment name between brackets places the array in any segment of the
linker configuration, for example `DATA Charset() BYTE [ALIGNDATA] = ...`
(the `ALIGNDATA` segment starts at a page boundary, a multiple of 256). The
segment must exist in the linker configuration; see
[chapter 16](16-memory-and-machine-code.md) and
[chapter 19](19-compiler-reference.md).

#### Rules for DATA

- `DATA` can appear anywhere, also inside a function, and on a line with
  other statements. The array can be used anywhere in the program after the
  `DATA` line.
- A byte array name used inside a byte `DATA` list must be defined in an
  earlier line.
- `DATA` arrays are not cleared by `CLR`, and do not count in the limit of
  variables.
- `DATA` arrays cannot be resized and do not use `DIM`.

### CLR
Clears all variables and frees the memory of arrays and strings.

```
CLR
```
Abbreviation: none

`CLR` sets all integer and floating point variables to 0, makes all string
variables empty, and releases all the arrays created with `DIM` and the
buffers of all strings. After `CLR`, an array must be created again by
running its `DIM` statement before it is used; using it before that accesses
invalid memory.

`DATA` arrays keep their current values.

```basic
' CLR clears variables but not DATA arrays
DATA Level() = 5
Score = 1200
Name$ = "ANA"
DIM Board(100)
Level(0) = 6
CLR
PRINT Score; " ["; Name$; "] "; ADR(Board); " "; Level(0)
```

Output:

```
0 [] 0 6
```

Since `CLR` also clears the variables that hold the sizes and the loop
counters, use it at a point where the program starts again from a known
state, for example at the start of a new game.

## Memory used by each kind of data

| Data | Memory |
| --- | --- |
| Integer variable | 2 bytes in the variables area |
| Floating point variable | 6 bytes in the variables area |
| String variable | 2 bytes in the variables area, plus 256 bytes of free memory after the first assignment |
| Array (`DIM`) | 2 bytes in the variables area, plus the elements in free memory |
| Word array element | 2 bytes |
| Byte array element | 1 byte |
| Floating point array element | 6 bytes |
| String array element | 2 bytes, plus 256 bytes after the first assignment to that element |
| `DATA` array | the size of its values, in the program file |

`FRE()` returns the free memory in bytes (see [FRE()](05-expressions.md#fre)).
[Chapter 20](20-performance-and-memory.md) explains how to plan the memory of
a large program.
