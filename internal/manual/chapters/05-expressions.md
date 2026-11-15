# 5. Expressions

This chapter describes how expressions are written and evaluated: the four
kinds of expressions, all the operators and their precedence, the exact rules
of integer arithmetic (overflow, division, `MOD`), logical and bitwise
operators, comparisons, how integers and floating point numbers mix,
conversions between types, and the standard functions `ABS`, `SGN`, `RAND`
and `FRE()`. It ends with a list of all the functions of the language.

## Kinds of expressions

An expression computes a value. exBASIC has four kinds of expressions:

| Kind | Values | Examples |
| --- | --- | --- |
| Integer | -32768 to 32767 | `Count + 1`, `PEEK(764)`, `X MOD 8` |
| Floating point | Atari BCD numbers | `Radius% * 2.0`, `SIN(Angle%)` |
| String | Text of 0 to 255 characters | `Name$ + "!"`, `STR$(Score)` |
| Logical | True (1) or false (0) | `X > 10`, `Ready AND NOT Done` |

Integers are the default. An expression is floating point when it contains
floating point variables, constants with a decimal point or an exponent,
or floating point functions (see [Mixing integers and floating point](#mixing-integers-and-floating-point)).

A logical expression is the result of a comparison or of the operators
`NOT`, `AND` and `OR`. It can be used as a number: true is 1 and false is 0.
Where a condition is expected (`IF`, `WHILE`, `UNTIL`, and the operands of
`AND`, `OR` and `NOT`), any integer can be used: 0 is false and any other
value is true.

## Operator precedence

When an expression has several operators, the ones with higher precedence
are applied first. Operators with the same precedence are applied from left
to right. Parentheses change the order: the expression inside them is
evaluated first.

Integer and logical operators, from highest to lowest precedence:

| Precedence | Operators | Meaning |
| --- | --- | --- |
| 1 (highest) | `-` `+` (unary) | Negation, plus sign |
| 2 | `&` `!` `EXOR` | Bitwise AND, OR, exclusive OR |
| 3 | `*` `/` `MOD` | Multiplication, division, remainder |
| 4 | `+` `-` | Addition, subtraction |
| 5 | `=` `<>` `<` `>` `<=` `>=` | Comparisons |
| 6 | `NOT` | Logical NOT |
| 7 | `AND` | Logical AND |
| 8 (lowest) | `OR` | Logical OR |

Floating point operators, from highest to lowest precedence:

| Precedence | Operators | Meaning |
| --- | --- | --- |
| 1 (highest) | `-` `+` (unary) | Negation, plus sign |
| 2 | `^` | Power |
| 3 | `*` `/` | Multiplication, division |
| 4 | `+` `-` | Addition, subtraction |

Comparisons, `NOT`, `AND` and `OR` work with floating point values in the
same way as with integers.

String operators: the slice `[ ]` applies to one string term, then `+`
joins strings, then comparisons compare them.

Note two differences from many other languages:

- The bitwise operators `&`, `!` and `EXOR` have higher precedence than
  multiplication: `2 + 3 & 1` is `2 + (3 & 1)`, which is 3.
- Unary minus has higher precedence than everything else, also than `^`:
  `-2.0 ^ 2` is `(-2.0) ^ 2`, which is 4.

```basic
' Precedence examples
PRINT 1 + 2 * 3 - 4 * -5
PRINT (2 + 5) * 10
PRINT 2 + 3 & 1
PRINT 7 & 3 = 3
PRINT NOT 1 = 0
PRINT 1 OR 0 AND 0
```

Output:

```
27
70
3
1
1
1
```

The first line is evaluated as `1 + (2 * 3) - (4 * (-5))`. In the fourth,
`7 & 3` is computed before the comparison. In the fifth, `NOT` applies to the
comparison `1 = 0`. In the last, `AND` is applied before `OR`.

## Integer arithmetic

### + - * / MOD
Integer addition, subtraction, multiplication, division and remainder.

```
a + b
a - b
a * b
a / b
a MOD b
```
Abbreviation: `M.` for `MOD`

All integer operations work on 16-bit signed numbers and give a 16-bit
result. The only one that can stop the program is a division by zero.

**Overflow.** Results that do not fit in -32768 to 32767 wrap around: only
the low 16 bits of the result are kept. `32767 + 1` is -32768 and
`300 * 300` is 24464 (90000 - 65536). Use floating point when values can be
larger.

**Division** gives the quotient truncated toward zero: `7 / 2` is 3 and
`-7 / 2` is -3.

**`MOD`** gives the remainder of the division. Its sign is the sign of the
first operand: `-7 MOD 3` is -1 and `7 MOD -3` is 1. For any `a` and `b`
(with `b` not 0), `(a / b) * b + a MOD b` equals `a`.

**Division by zero** stops the program: `a / 0` and `a MOD 0` print
`ERROR- 11` (error 11) and end the program. Check the divisor first
when it can be 0.

**Special cases** with -32768, the only value without a positive
counterpart: `-(-32768)`, `ABS(-32768)` and `-32768 / -1` all give -32768;
`-32768 MOD -1` gives 0.

These rules are the same in bytecode and in machine code (see
[chapter 20](20-performance-and-memory.md)).

```basic
' Integer arithmetic rules
Big = 32767
Divisor = 0
PRINT Big + 1; " "; 300 * 300
PRINT 7 / 2; " "; -7 / 2; " "; 7 / -2; " "; -7 / -2
PRINT 7 MOD 3; " "; -7 MOD 3; " "; 7 MOD -3; " "; -7 MOD -3
IF Divisor <> 0 THEN PRINT 5 / Divisor ELSE PRINT "CAN'T DIVIDE BY 0"
```

Output:

```
-32768 24464
3 -3 -3 3
1 -1 1 -1
CAN'T DIVIDE BY 0
```

To divide rounding down (toward minus infinity) instead of toward zero, or to
get a remainder that is never negative, correct the result when the dividend
is negative:

```basic
' Remainder from 0 to Divisor - 1, also for negative values
Divisor = 8
FOR Value = -3 TO 3
  Remainder = Value MOD Divisor
  IF Remainder < 0 THEN Remainder = Remainder + Divisor
  PRINT Remainder; " ";
NEXT Value
PRINT
```

Output:

```
5 6 7 0 1 2 3 
```

### Unary - and +
Negates a value, or leaves it unchanged.

```
-a
+a
```
Abbreviation: none

Unary minus changes the sign of the value that follows. Because it has the
highest precedence, `-X * 3` is `(-X) * 3` and `-X MOD 4` is `(-X) MOD 4`.
Unary plus has no effect.

## Bitwise operators

### & ! EXOR
Bitwise AND, OR and exclusive OR of two integers.

```
a & b
a ! b
a EXOR b
```
Abbreviation: `E.` for `EXOR`

These operators combine the 16 bits of the two operands one by one:

| Operator | Result bit is 1 when |
| --- | --- |
| `&` | both bits are 1 |
| `!` | at least one bit is 1 |
| `EXOR` | the two bits are different |

Use `&` to test or clear bits, `!` to set bits and `EXOR` to invert bits.
There is no bitwise NOT operator: `X EXOR -1` (or `X EXOR $FFFF`) inverts
all the bits of `X`. There are no shift operators: multiply or divide by
powers of two instead (`X * 4`, `X / 16`), keeping in mind that division of
negative numbers truncates toward zero.

Remember that these operators have higher precedence than `*` and `+`. Use
parentheses when you combine them with arithmetic.

```basic
' Test, set and invert bits
Flags = 12            ' bits 2 and 3 set
PRINT Flags & 4; " "; Flags & 1
PRINT Flags ! 1; " "; Flags EXOR 15
PRINT 3281 ! 2928; " "; 3281 & 2928; " "; 3281 EXOR 2928
PRINT $FF & -2; " "; 0 EXOR -1
```

Output:

```
4 0
13 3
4081 2128 1953
254 -1
```

## Comparison operators

### = <> < > <= >=
Compare two values and give true (1) or false (0).

```
a = b
a <> b
a < b
a > b
a <= b
a >= b
```
Abbreviation: none

| Operator | True when |
| --- | --- |
| `=` | `a` is equal to `b` |
| `<>` | `a` is not equal to `b` |
| `<` | `a` is less than `b` |
| `>` | `a` is greater than `b` |
| `<=` | `a` is less than or equal to `b` |
| `>=` | `a` is greater than or equal to `b` |

Both operands must be numbers (integer or floating point, in any
combination) or both must be strings. Integer comparisons are signed and
exact over the whole range: `-20000 < 15000` is true. When an integer is
compared with a floating point value, the integer is converted to floating
point first.

Strings are compared character by character using the ATASCII codes; a
string that is the beginning of a longer one is smaller (see
[chapter 8](08-strings.md)).

The result is a logical value that can be used as a number. Several
comparisons in a row are applied from left to right: `3 = 3 = 3` compares
`(3 = 3)`, which is 1, with 3, and gives 0. Use `AND` to combine conditions.

```basic
' Comparisons give 1 or 0
Count = 1
PRINT 1 < 2; " "; 2 < 1; " "; Count = 1.5; " "; Count < 1.5
PRINT "ABC" < "ABD"; " "; "AB" = "AB"; " "; "" < "A"
Steps = (Count > 0) + 1
PRINT Steps
```

Output:

```
1 0 0 1
1 1 1
2
```

## Logical operators

### NOT AND OR
Combine conditions.

```
NOT a
a AND b
a OR b
```
Abbreviation: `N.` for `NOT`, `A.` for `AND`, `O.` for `OR`

The operands are conditions: any integer counts as false when it is 0 and as
true otherwise. The result is 1 (true) or 0 (false).

| Operator | Result is true when |
| --- | --- |
| `NOT a` | `a` is false |
| `a AND b` | both `a` and `b` are true |
| `a OR b` | `a`, `b` or both are true |

These are logical operators, not bitwise: `5 AND 3` is 1 and `NOT 7` is 0.
For bitwise operations use `&`, `!` and `EXOR`.

`NOT` has lower precedence than comparisons, so `NOT X = 1` means
`NOT (X = 1)`. `AND` has higher precedence than `OR`.

`AND` and `OR` are short-circuit operators: the right side is evaluated
only when it is needed. In `a AND b`, if `a` is false the result is false
and `b` is not evaluated; in `a OR b`, if `a` is true the result is true
and `b` is not evaluated. A function on the right side is then not
called. This makes it safe to check a value before using it:

```basic
' The index is checked before the array is read
DIM Values(4)
Index = 9
IF Index <= 4 AND Values(Index) = 0 THEN PRINT "EMPTY" ELSE PRINT "NO SUCH ELEMENT"
```

Output:

```
NO SUCH ELEMENT
```

```basic
' Logical operators
Lives = 2
GameOver = 0
PRINT 5 AND 3; " "; 0 OR 7; " "; NOT 7
IF Lives > 0 AND NOT GameOver
  PRINT "PLAYING"
ENDIF
```

Output:

```
1 1 0
PLAYING
```

## Floating point operators

### + - * / ^ (floating point)
Floating point arithmetic.

```
a + b
a - b
a * b
a / b
a ^ b
```
Abbreviation: none

The usual arithmetic on floating point values. `a ^ b` raises `a` to the
power `b`; when `b` is an integer the result is calculated by repeated
multiplication, otherwise with logarithms. Several `^` are applied from left
to right: `2.0 ^ 3 ^ 2` is `(2.0 ^ 3) ^ 2`, which is 64.

There is no `MOD`, `&`, `!` or `EXOR` for floating point values.

A floating point division by zero stops the program with error 3
(`ERROR- 11`), like an integer one. Other floating point errors, such
as an overflow, do not stop the program: the result is not valid and
`ERR()` returns 3. [Chapter 9](09-floating-point.md) describes floating
point numbers in detail.

```basic
' Floating point operators
Side% = 1.5
PRINT Side% * Side%; " "; 7.0 / 2; " "; 2.0 ^ 10; " "; 2.0 ^ 0.5
```

Output:

```
2.25 3.5 1024 1.41421357
```

## String operators

| Operator | Meaning | Example |
| --- | --- | --- |
| `+` | Joins two strings | `"HI " + Name$` |
| `[start]` | Characters from position `start` to the end | `Name$[2]` |
| `[start, len]` | At most `len` characters from position `start` | `Name$[1, 3]` |
| `=` `<>` `<` `>` `<=` `>=` | Compare two strings | `Name$ = "ANA"` |

Positions in a string start at 1. [Chapter 8](08-strings.md) describes the
string operators and their limits.

```basic
First$ = "ATARI"
PRINT First$ + " " + "800"; " "; First$[2, 3]
```

Output:

```
ATARI 800 TAR
```

## Mixing integers and floating point

exBASIC decides the kind of each expression while it compiles the program.

An expression with only integer values is an integer expression: `7 / 2`
is 3 and `300 * 300` wraps around to 24464.

When an expression in `PRINT`, in a condition (`IF`, `WHILE`, `UNTIL`
and the other loops) or on the right side of an assignment to a floating
point variable or array element has at least one floating point value,
the whole expression is calculated in floating point. Each integer value
(constant, variable, array element or integer function) is converted to
floating point before it is used, so there is no integer overflow and
division is not truncated:

```basic
Count = 300
Result% = Count * Count
Half% = Count / 7
PRINT Result%; " "; Half%
PRINT 7 / 2; " "; 7 / 2 * 1.0; " "; 1 + 0.5
IF Count * 1.5 > 400 THEN PRINT "MORE THAN 400"
```

Output:

```
90000 42.85714286
3 3.5 1.5
MORE THAN 400
```

The integer-only operators (`MOD`, `&`, `!`, `EXOR`) must be put in
parentheses when they are used in a floating point expression; the part
in parentheses is calculated as an integer:

```basic
Count = 300
Result% = (Count MOD 7) * 1.5
PRINT Result%; " "; (7 MOD 3) + 1.5
```

Output:

```
9 2.5
```

Assigning to an integer variable needs an integer expression: `Total =
Count * 1.5` is an error. Convert the value with `INT` (see
[Conversions](#conversions)). Function arguments for integer parameters,
array indexes and the arguments of integer functions (like `PEEK`, `RAND`
or `LEN`) are always integer expressions.

A comparison between an integer and a floating point value works in any
order: `Count < 1.5` and `1.5 > Count` are both correct. Floating point
comparisons can be combined with `AND` and `OR` anywhere, also in
assignments and in `PRINT`:

```basic
Speed% = 1.5
Moving = Speed% > 1.0 AND Speed% < 2.0
PRINT Moving
```

Output:

```
1
```

## Conversions

| From | To | How |
| --- | --- | --- |
| Integer | Floating point | Automatic in floating point expressions and assignments. |
| Floating point | Integer | `INT(x)` rounds to the nearest integer; halves round away from zero. |
| Logical | Integer | Automatic: true is 1, false is 0. |
| Integer | Logical | Automatic in conditions: 0 is false, other values are true. |
| Number | String | `STR$(x)` |
| String | Number | `VAL(text$)`, integer or floating point depending on the expression. |
| Character code | String | `CHR$(code)` |
| String | Character code | `ASC(text$)` |

`INT` does not truncate: `INT(2.5)` is 3, `INT(-2.5)` is -3 and `INT(2.4)`
is 2. To truncate, subtract 0.5 from positive values (or add it to negative
ones) before calling `INT`. If the value does not fit in -32768 to 32767,
the result is the nearest end of the range: `INT(1E10)` is 32767 and
`INT(-1E10)` is -32768, and `ERR()` returns 3.

Assigning a floating point value to an integer variable without `INT` is a
compile error.

`VAL` reads the number at the start of the string and ignores what follows
(`VAL("12AB")` is 12). If the string does not start with a number, the
result is 0 and `ERR()` returns 18; an integer `VAL` larger than 65535 also
sets `ERR()` to 18. A successful `VAL` sets `ERR()` to 1. `VAL` does not
accept hexadecimal (`$`) numbers.

`ERR()` is also changed by floating point operations and by input/output
operations on files and devices, so read it right after the conversion.
Printing on the screen does not change it:

```basic
' Read ERR() right after the operation
Value = VAL("12AB")
Status = ERR()
PRINT Value; " "; Status
Value = VAL("HELLO")
Status = ERR()
PRINT Value; " "; Status
Rounded = INT(40000.5)
Status = ERR()
PRINT Status
PRINT INT(2.5); " "; INT(-2.5); " "; INT(2.4)
```

Output:

```
12 1
0 18
3
3 -3 2
```

`STR$`, `VAL`, `ASC` and `CHR$` are described in
[chapter 8](08-strings.md), and `INT` in [chapter 9](09-floating-point.md).

## Calling functions

Functions take their arguments in parentheses: `ABS(X)`, `PEEK(764)`.

Functions with one argument can also be written without parentheses, followed
by a single value: `ABS X`, `RAND 6`, `LEN Name$`. Without parentheses the
function applies only to the next value (a constant, a variable, a function
call or an expression in parentheses), so `ABS X * 2` means `ABS(X) * 2`.
This does not work with `USR` and `ADR`, which always need parentheses.

Functions without arguments are written with an empty pair of parentheses:
`KEY()`, `ERR()`, `RND()`. Their abbreviation includes the parentheses:
`E.` is `ERR()`. `TIME` and `%TIME` are written without parentheses, and
`FRE` can be written with or without them.

Functions you define with `FUNC` are called by name with their arguments in
parentheses (see [chapter 7](07-functions.md)).

## Standard functions

### ABS
Returns the absolute value of a number.

```
ABS(x)
```
Abbreviation: `A.`

Returns `x` if it is positive or zero, and `-x` if it is negative. It works
with integers and with floating point values; the result has the same type
as the argument. `ABS(-32768)` is -32768, because 32768 does not fit in an
integer.

```basic
Distance = -7
PRINT ABS(Distance); " "; ABS(5); " "; ABS(-2.5)
```

Output:

```
7 5 2.5
```

### SGN
Returns the sign of a number.

```
SGN(x)
```
Abbreviation: `SG.`

Returns 1 if `x` is positive, -1 if it is negative and 0 if it is zero. It
works with integers and with floating point values; the result has the same
type as the argument.

```basic
Speed = -300
PRINT SGN(Speed); " "; SGN(0); " "; SGN(32767); " "; SGN(-0.5)
```

Output:

```
-1 0 1 -1
```

### RAND
Returns a random integer.

```
RAND(n)
```
Abbreviation: `R.`

Returns a random integer from 0 to `n - 1`, for `n` from 1 to 32767. For
example, `RAND(6) + 1` gives a dice roll from 1 to 6. `RAND(1)` and
`RAND(0)` always return 0. Negative values of `n` give results from a wider
range and should not be used.

The numbers come from the random number generator of the POKEY sound chip,
so they are different each time the program runs; there is no seed to set.
For random floating point numbers, use `RND()` (see
[chapter 9](09-floating-point.md)).

```basic
' Roll a die 1000 times and check the range
Lowest = 99
Highest = 0
FOR Roll = 1 TO 1000
  Die = RAND(6) + 1
  IF Die < Lowest THEN Lowest = Die
  IF Die > Highest THEN Highest = Die
NEXT Roll
PRINT "FROM "; Lowest; " TO "; Highest
```

Output:

```
FROM 1 TO 6
```

### FRE()
Returns the free memory in bytes.

```
FRE
FRE()
FRE(x)
```
Abbreviation: `F.`

Returns the number of bytes between the end of the memory used by arrays and
strings and the top of free memory (`MEMTOP`, the address stored at
locations 741 and 742). This is the memory still available for `DIM` and for
new strings. The three forms are the same; the argument `x`, as in
`FRE(0)` of Atari BASIC, is ignored.

The free memory is often more than 32767 bytes, which does not fit in an
integer. exBASIC handles this so that the value is never shown negative:

- `PRINT FRE()`, `STR$(FRE())` and an assignment to a floating point
  variable (`Free% = FRE()`) give the real value, from 0 to 65535.
- Comparisons written with `FRE()` itself, like `IF FRE() > 1000` or
  `IF 0 < FRE()`, compare the real value.
- In integer arithmetic the 16-bit value is used, so differences are
  right: `Before - FRE()` gives the memory used by an operation.
- An integer variable keeps only 16 bits: after `Free = FRE()`, `Free`
  prints as a negative number when more than 32767 bytes are free. Use a
  floating point variable to keep the exact value.

```basic
' Show the free memory and the memory used by an array
PRINT "FREE BYTES: "; FRE()
IF FRE() > 1000 THEN PRINT "ENOUGH MEMORY"
Before = FRE()
DIM Buffer(99)
PRINT "THE ARRAY USES "; Before - FRE(); " BYTES"
```

Output:

```
FREE BYTES: 39573
ENOUGH MEMORY
THE ARRAY USES 200 BYTES
```

The free memory depends on the computer, the DOS and the graphics mode, so
the first number varies. Opening a graphics mode with `GRAPHICS` moves
`MEMTOP` and changes the result (see [chapter 12](12-graphics.md)).

## List of all functions

The table lists every function of the language with the chapter that
describes it. Functions you define with `FUNC` are described in
[chapter 7](07-functions.md).

| Function | Result | Description | Chapter |
| --- | --- | --- | --- |
| `ABS(x)` | Integer or FP | Absolute value | [5](#abs) |
| `ADR(name)`, `&name` | Integer | Address of a variable, array or string | [16](16-memory-and-machine-code.md) |
| `ASC(text$)` | Integer | ATASCII code of the first character | [8](08-strings.md) |
| `ATN(x)` | FP | Arc tangent | [9](09-floating-point.md) |
| `CHR$(code)` | String | One-character string | [8](08-strings.md) |
| `COS(x)` | FP | Cosine | [9](09-floating-point.md) |
| `DPEEK(addr)` | Integer | Reads a 16-bit word from memory | [16](16-memory-and-machine-code.md) |
| `ERR()` | Integer | Last input/output or conversion error code | [11](11-files-and-devices.md) |
| `EXP(x)` | FP | e raised to `x` | [9](09-floating-point.md) |
| `EXP10(x)` | FP | 10 raised to `x` | [9](09-floating-point.md) |
| `FRE()` | Integer | Free memory in bytes, never negative when printed | [5](#fre) |
| `INSTR(text$, find$ [, start])` | Integer | Position of `find$` in `text$`, or 0 | [8](08-strings.md#instr) |
| `INT(x)` | Integer | Floating point rounded to the nearest integer | [9](09-floating-point.md) |
| `KEY()` | Integer | Code of the last key pressed, or 0 | [10](10-console-io.md) |
| `LCASE$(text$)` | String | Text in lowercase | [8](08-strings.md#ucase-and-lcase) |
| `LEFT$(text$, n)` | String | The first `n` characters | [8](08-strings.md#left) |
| `LEN(text$)` | Integer | Length of a string | [8](08-strings.md) |
| `LOG(x)` | FP | Natural logarithm | [9](09-floating-point.md) |
| `LOG10(x)` | FP | Decimal logarithm | [9](09-floating-point.md) |
| `LTRIM$(text$)` | String | Text without spaces at the start | [8](08-strings.md#ltrim-and-rtrim) |
| `MID$(text$, start [, n])` | String | Characters from position `start` | [8](08-strings.md#mid) |
| `PADDLE(n)` | Integer | Position of paddle controller `n` | [15](15-sound-and-controllers.md) |
| `PEEK(addr)` | Integer | Reads a byte from memory | [16](16-memory-and-machine-code.md) |
| `PMADR(n)` | Integer | Address of the memory of player or missiles | [13](13-player-missile-graphics.md) |
| `PTRIG(n)` | Integer | Button of paddle controller `n` | [15](15-sound-and-controllers.md) |
| `RAND(n)` | Integer | Random integer from 0 to `n - 1` | [5](#rand) |
| `RIGHT$(text$, n)` | String | The last `n` characters | [8](08-strings.md#right) |
| `RND()` | FP | Random number from 0 to less than 1 | [9](09-floating-point.md) |
| `RTRIM$(text$)` | String | Text without spaces at the end | [8](08-strings.md#ltrim-and-rtrim) |
| `SERR()` | Integer | Status of the last `SIO` operation | [18](18-sio-and-fujinet.md) |
| `SGN(x)` | Integer or FP | Sign: -1, 0 or 1 | [5](#sgn) |
| `SIN(x)` | FP | Sine | [9](09-floating-point.md) |
| `SQR(x)` | FP | Square root | [9](09-floating-point.md) |
| `STICK(n)` | Integer | Direction of joystick `n` | [15](15-sound-and-controllers.md) |
| `STR$(x)` | String | Number converted to text | [8](08-strings.md) |
| `STRIG(n)` | Integer | Button of joystick `n` | [15](15-sound-and-controllers.md) |
| `TIME` | Integer | Time in frames (jiffies) | [15](15-sound-and-controllers.md) |
| `%TIME` | FP | Time in frames, without the integer limit | [15](15-sound-and-controllers.md) |
| `UCASE$(text$)` | String | Text in uppercase | [8](08-strings.md#ucase-and-lcase) |
| `USR(addr, ...)` | Integer | Calls a machine code routine | [16](16-memory-and-machine-code.md) |
| `VAL(text$)` | Integer or FP | Number read from a string | [8](08-strings.md) |
| `$(addr)` | String | The string stored at an address | [8](08-strings.md) |
| `%(addr)` | FP | The floating point number stored at an address | [9](09-floating-point.md) |

FP means floating point. Functions that give "Integer or FP" return the type
of their argument (`ABS`, `SGN`) or the type of the expression where they are
used (`VAL`).
