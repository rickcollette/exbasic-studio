# 9. Floating Point

Integer numbers are the default in exBASIC, and they are fast. When a
program needs fractions, very large numbers or functions like `SIN` and
`SQR`, it can use floating point numbers. This chapter describes floating
point variables and constants, their precision, how the compiler decides
whether an expression is integer or floating point, the operators and
functions, conversions, the printing format, errors and speed.

## Overview

exBASIC uses the floating point format of the Atari operating system
(OS): binary coded decimal (BCD), 6 bytes per number. The arithmetic is
done by the floating point routines in the Atari OS ROM (the "math
pack"), plus exBASIC routines for `SQR`, `SIN`, `COS` and `^`.

| Property | Value |
|----------|-------|
| Size | 6 bytes |
| Precision | 9 or 10 significant decimal digits |
| Largest value | about ±9.99999999E+97 |
| Smallest nonzero value | about ±1E-98 |
| Decimal fractions like 0.1 | exact |

Floating point support is included in a program automatically, only when
the program uses floating point numbers. Programs that use only integers
don't contain or initialize it.

Because the arithmetic uses the OS routines, floating point programs use
the OS floating point work areas in page zero (`$D4` to `$FF`) and the
line buffer at `$0580`. Don't use these addresses for your own data.

## Floating point variables and arrays

The name of a floating point variable ends with `%`: `Speed%`, `Angle%`,
`X1%`. A variable that was never assigned is 0.

- The `%` is part of the name: `Size` and `Size%` are two different
  variables.
- Each floating point variable takes 6 bytes (3 slots of the variable
  table).

Floating point arrays are created with `DIM` or `DATA`, with a `%` after
the name; each element uses 6 bytes (see
[chapter 4](04-variables-and-data.md)):

```basic
' Floating point arrays
DIM Halves%(3)
FOR Index = 0 TO 3
  Halves%(Index) = Index / 2.0
NEXT Index
PRINT Halves%(1); " "; Halves%(3)

DATA Weights%() = 1.5, 2.25, -3.0
PRINT Weights%(0) + Weights%(1) + Weights%(2)
```

Output:

```
0.5 1.5
0.75
```

`%(address)` reads or writes a floating point number stored at any memory
address, see [chapter 16](16-memory-and-machine-code.md).

## Constants

A floating point constant is a number with a decimal point, an exponent,
or both:

| Written | Value |
|---------|-------|
| `1.5`, `-0.25`, `.5`, `5.` | decimal fractions |
| `1.5E3`, `1.5E+3` | 1500 |
| `2.5E-3` | 0.0025 |
| `1E3` | 1000 |

A number with an exponent is always floating point, also without a
decimal point. The exponent has at most two digits. A constant out of the
range of the format (like `1.0E98`) is a compile error. Constants are
rounded to the precision of the format when the program is compiled.

A number without a decimal point and without an exponent is an integer
constant, from 0 to 65535. Where a floating point value is expected it is
converted, so `Half% = 1 / 2` stores 0.5. A larger number, like `100000`,
is an error (`integer constant out of range (max 65535), write 100000.0
for a floating point number`): write `100000.0`.

## Precision

The BCD format stores decimal digits, so decimal fractions like 0.1 or
0.25 are exact, and `0.1 + 0.2 = 0.3` is true. Other fractions, like 1/3,
are rounded to 9 or 10 digits, and results can be slightly off:

```basic
' Exact and inexact fractions
PRINT 0.1 + 0.2 = 0.3
Third% = 1.0 / 3
PRINT Third%
PRINT Third% * 3 = 1
```

Output:

```
1
0.3333333333
0
```

When comparing results of calculations, compare with a small tolerance
instead of `=`, for example `IF ABS(A% - B%) < 0.000001`.

## Integer or floating point expressions

Every expression is either integer or floating point. The compiler
decides from the place where the expression is used and from the values
in it.

**Assignment to a floating point variable.** The whole expression is
floating point. Each integer value in it (variables, constants, integer
functions) is converted to floating point first, so all the arithmetic is
done in floating point and can't overflow:

```basic
' In a floating point assignment, all arithmetic is floating point
Count = 1000
Big% = Count * Count + 1.2
PRINT Big%
Big% = 32767 + 1
PRINT Big%
```

Output:

```
1000001.2
32768
```

Only the arguments of integer functions (like `PEEK`, `RAND` or `LEN`)
are still integer expressions.

**Assignment to an integer variable.** The expression must be integer. A
floating point value is an error; convert it with `INT` (see
[Conversions](#conversions)).

**PRINT and conditions.** In `PRINT` and in the conditions of `IF`,
`ELIF`, `WHILE`, `UNTIL` and the other loops, an expression with only
integer values is an integer expression, and an expression with at least
one floating point value is calculated in floating point, all of it:

| Expression | Result |
|------------|--------|
| `PRINT 1.0 / 3` | 0.3333333333 |
| `PRINT 1 / 3.0` | 0.3333333333 |
| `PRINT 2 * Speed%` | floating point |
| `PRINT 7 / 2` | 3, integer division |
| `PRINT 7 / 2 + 0.5` | 4, the division is done in floating point |
| `IF Count * 1.5 > 2` | floating point comparison |
| `PRINT INT(Value% * 100) / 100` | integer division: `INT` gives an integer |

`MOD`, `&`, `!` and `EXOR` work only on integers: in a floating point
expression, put them in parentheses, as in `(7 MOD 3) + 1.5`.

**Function arguments** for integer parameters, and the arguments of
integer functions like `PEEK` and `CHR$`, must be integer expressions;
use `INT` to convert a floating point value.

**Comparisons** between an integer and a floating point value are done in
floating point: `IF Speed% > 2` and `IF 2 < Speed%` both work. A floating
point value alone is not a condition: write `IF Speed% <> 0`.

**Floating point loops.** `FOR`, `INC` and `DEC` work only with integer
variables. To step through floating point values, use an integer counter
and compute the value from it.

## Operators

| Operator | Meaning | Precedence |
|----------|---------|------------|
| `-` `+` (unary) | negative, positive | highest |
| `^` | power | |
| `*` `/` | multiply, divide | |
| `+` `-` | add, subtract | lowest |
| `=` `<>` `<` `>` `<=` `>=` | comparison, result 1 or 0 | |

Operators of the same precedence are applied from left to right.

- `/` is always a real division: `7.0 / 2` is 3.5. `MOD` and the bitwise
  operators (`&`, `!`, `EXOR`) are integer only.
- The unary minus binds tighter than `^`, so `-2.0 ^ 2` is 4. Write
  `-(2.0 ^ 2)` for -4.
- `^` with an integer exponent (a constant, an integer variable or an
  expression in parentheses) is computed by repeated multiplication and
  is exact for whole results: `2.0 ^ 10` is 1024, `(-2.0) ^ 3` is -8,
  `10.0 ^ -2` is 0.01.
- `^` with a floating point exponent is computed with logarithms
  (`EXP10(LOG10(x) * y)`): it is less precise, and the base must be
  positive.

```basic
' Powers
PRINT 2.0 ^ 10; " "; (-2.0) ^ 3; " "; 10.0 ^ -2
PRINT 2.0 ^ 0.5; " "; SQR(2.0); " "; -2.0 ^ 2
```

Output:

```
1024 -8 0.01
1.41421357 1.41421356 4
```

## Functions

The functions in this section take floating point arguments; integer
arguments are converted. Trigonometric functions use radians unless
`DEG` is active.

### ABS

Returns the absolute value.

```
ABS(number)
```
Abbreviation: `A.`

`ABS(-3.25)` is 3.25. With an integer argument in an integer expression,
`ABS` is the integer function (see [chapter 5](05-expressions.md)).

### ATN

Returns the arc tangent.

```
ATN(number)
```
Abbreviation: `AT.`

The result is an angle from -π/2 to π/2 radians (or -90 to 90 degrees
after `DEG`). `ATN(1.0) * 4` is π.

### COS

Returns the cosine of an angle.

```
COS(angle)
```
Abbreviation: `CO.`

The angle is in radians, or in degrees after `DEG`.

### EXP

Returns e (2.71828183) raised to a power.

```
EXP(number)
```
(no abbreviation)

`EXP(1.0)` is 2.71828184. A result too large for the format sets
`ERR()` to 3.

### EXP10

Returns 10 raised to a power.

```
EXP10(number)
```
Abbreviation: `EX.`

`EXP10(2.5)` is 316.227767.

### INT

Converts a floating point number to an integer.

```
INT(number)
```
Abbreviation: `I.`

`INT` rounds to the nearest integer; halves are rounded away from zero:
`INT(2.5)` is 3, `INT(-2.5)` is -3, `INT(1.7)` is 2, `INT(2.4999)` is 2.
The result is an integer from -32768 to 32767. A value outside that range
gives the nearest end of the range: `INT(1E10)` is 32767 and
`INT(-1E10)` is -32768, and `ERR()` is set to 3. A successful `INT` sets
`ERR()` to 1. See [Conversions](#conversions).

### LOG

Returns the natural logarithm (base e).

```
LOG(number)
```
(no abbreviation)

The number must be greater than zero; otherwise the result is invalid
and `ERR()` is set to 3.

### LOG10

Returns the decimal logarithm (base 10).

```
LOG10(number)
```
Abbreviation: `LO.`

`LOG10(1000.0)` is 3. The number must be greater than zero.

### RND

Returns a random number.

```
RND()
```
Abbreviation: `RN.`

The result is a floating point number from 0 up to, but not including,
1. The parentheses are required (unless abbreviated) and take no
argument. For random integers, use `RAND(n)` (see
[chapter 15](15-sound-and-controllers.md)).

```basic
' A random number from 1.0 to 6.0 (not included)
Roll% = 1 + RND() * 5
PRINT (Roll% >= 1) AND (Roll% < 6)
```

Output:

```
1
```

### SGN

Returns the sign of a number.

```
SGN(number)
```
Abbreviation: `SG.`

The result is -1 for negative numbers, 0 for zero and 1 for positive
numbers. With a floating point argument the result is floating point.

### SIN

Returns the sine of an angle.

```
SIN(angle)
```
Abbreviation: `SI.`

The angle is in radians, or in degrees after `DEG`.

### SQR

Returns the square root.

```
SQR(number)
```
Abbreviation: `SQ.`

The number must not be negative. For a negative number `ERR()` is set to
3 and the result is invalid. `SQR` is more precise than `^ 0.5`.

### VAL

Converts the text of a number into a floating point number.

```
VAL(string)
```
Abbreviation: `V.`

In a floating point expression, `VAL` reads a floating point number:
optional spaces, a sign, digits, a decimal point and an exponent, like
`"  -2.25"` or `"1.5E3"`. Reading stops at the first character that
can't be part of the number. If the text does not start with a number,
the result is 0 and `ERR()` is set to 18; otherwise `ERR()` is set to 1.

In an integer expression, `VAL` reads an integer (see
[chapter 8](08-strings.md#val)). Assign the result to a floating point
variable, or use it in a floating point expression, to get the floating
point `VAL`:

```basic
Price% = VAL("1.5E3")
PRINT Price%
Price% = VAL("  -2.25 EUR")
PRINT Price%
```

Output:

```
1500
-2.25
```

### STR$

Returns the text of a floating point number.

```
STR$(number)
```
(no abbreviation)

The text is the same that `PRINT` shows (see
[Printing format](#printing-format)). The result is in the temporary
buffer described in [chapter 8](08-strings.md#slicing-strings).

```basic
Ratio$ = STR$(2.0 / 3)
PRINT Ratio$; " HAS "; LEN(Ratio$); " CHARACTERS"
```

Output:

```
0.6666666667 HAS 12 CHARACTERS
```

### %TIME

Returns the system clock as a floating point number.

```
%TIME
```
Abbreviation: `%T.`

The value is the number of "jiffies" (TV frames: 60 per second on NTSC
machines, 50 on PAL) counted by the 24-bit OS clock since the computer was
turned on. Unlike the integer `TIME`, it does not become negative after
9 minutes: it counts for more than 3 days before it wraps. Don't use the
`TIMER` statement in programs that use `%TIME`: `TIMER` resets only the
low part of the clock, and `%TIME` then gives wrong values. See
[chapter 15](15-sound-and-controllers.md) for `TIME` and `TIMER`.

```basic
' Measures how long a loop takes, in seconds (NTSC)
Start% = %TIME
FOR I = 1 TO 1000
NEXT I
Seconds% = (%TIME - Start%) / 60
PRINT Seconds% >= 0
```

Output:

```
1
```

### DEG

Makes the trigonometric functions use degrees.

```
DEG
```
(no abbreviation)

After `DEG`, `SIN`, `COS` and `ATN` use angles in degrees: a full circle
is 360. The results are slightly less precise than in radians.

### RAD

Makes the trigonometric functions use radians.

```
RAD
```
Abbreviation: `RA.`

After `RAD`, `SIN`, `COS` and `ATN` use angles in radians: a full circle
is 2π. Radians are the default when a program starts.

```basic
' Degrees and radians
DEG
PRINT SIN(30.0); " "; COS(60.0); " "; ATN(1.0)
RAD
PRINT SIN(1.0); " "; ATN(1.0) * 4
```

Output:

```
0.5000000033 0.5000000133 44.9999998
0.8414709755 3.14159264
```

## Conversions

**Integer to floating point** is automatic wherever a floating point
value is expected: in floating point assignments, in function arguments
for `%` parameters, in comparisons with floating point values and in
floating point functions.

**Floating point to integer** needs `INT`, which rounds to the nearest
integer. There is no function that cuts off the fraction; for positive
numbers you can round down with `INT(Value% - 0.5)`. The result is
limited to the integer range, -32768 to 32767.

```basic
' Rounding to integers
Value% = 3.7
Rounded = INT(Value%)
PRINT Rounded; " "; INT(-3.7); " "; INT(2.5); " "; INT(-2.5)
```

Output:

```
4 -4 3 -3
```

To round to a number of decimals, multiply, round and divide in a
floating point assignment:

```basic
Price% = 2.0 / 3
Price% = INT(Price% * 100) / 100
PRINT Price%
```

Output:

```
0.67
```

## Printing format

`PRINT` and `STR$` show floating point numbers in the format of the
Atari OS:

- Up to 10 significant digits, without trailing zeros and without a
  decimal point for whole numbers.
- Numbers from 0.01 up to 9999999999 are shown as decimals.
- Smaller and larger numbers are shown with an exponent: `E`, a sign and
  two digits. The OS sometimes adds `.0` to the first digit.
- No space is printed before or after the number.

```basic
' How floating point numbers are printed
PRINT 2.5
PRINT 100000.0
PRINT 9999999999.0
PRINT 12345678901.0
PRINT 0.01
PRINT 0.001
PRINT 0.0001
PRINT 0.000012345
PRINT -1.5E20
PRINT 3.14159265358979
```

Output:

```
2.5
100000
9999999999
1.23456789E+10
0.01
1.0E-03
1E-04
1.2345E-05
-1.5E+20
3.14159265
```

The last line shows that a number has 9 or 10 significant digits,
depending on its value.

## Errors

A division by zero stops the program with error 11 (`ERROR- 11`). Other
floating point errors don't stop the program.
The operation gives an invalid or limited result, and `ERR()` is set to
3:

| Cause | Example |
|-------|---------|
| Result too large | `9.0E97 * 100` |
| Logarithm of zero or a negative number | `LOG(0.0)` |
| Square root of a negative number | `SQR(-4.0)` |
| `INT` out of the integer range | `INT(40000.0)` gives 32767 |

A result too small for the format (underflow) becomes 0 without an error.
A power of a negative number with an exponent that is not an integer,
like `(-8.0) ^ (1 / 3.0)`, gives a wrong result without an error.

Successful floating point operations, `INT`, `VAL` and `^` set `ERR()` to
1, so set the value aside right after the operation you want to check.
Printing on the screen does not change `ERR()`.

```basic
' Checking for a floating point error
Result% = 9.0E97 * 100
ErrorCode = ERR()
IF ErrorCode = 3 THEN PRINT "OVERFLOW"
Result% = 2.0 * 3
PRINT ERR()
```

Output:

```
OVERFLOW
1
```

## Floating point in functions

Functions can have floating point parameters, results and `LOCAL`
variables, and a function whose name ends with `%` returns a floating
point value. Integer arguments are converted for `%` parameters. See
[chapter 7](07-functions.md).

```basic
' Distance between two points
PRINT Distance%(0, 0, 3, 4)

' Length of the line from (X1%, Y1%) to (X2%, Y2%)
FUNC Distance%(X1%, Y1%, X2%, Y2%)
  LOCAL DeltaX%, DeltaY%
  DeltaX% = X2% - X1%
  DeltaY% = Y2% - Y1%
  RETURN SQR(DeltaX% * DeltaX% + DeltaY% * DeltaY%)
ENDFUNC
```

Output:

```
5
```

## Limits

The interpreter keeps the floating point values that wait while an
expression is evaluated on a floating point stack with room for 16
values. Very deeply nested floating point expressions, or recursive
functions that leave floating point values waiting during the recursive
call, fill it, and the program stops with error 10 (`ERROR- 10`).
For example, a recursive factorial written as
`RETURN N% * Factorial%(N% - 1)` works up to 15! and stops at 16!. Store
intermediate values in `LOCAL` variables to keep the stack short.

## Speed

Floating point is much slower than integer arithmetic. The OS routines
work on decimal digits, and the native code compiler (see
[chapter 20](20-performance-and-memory.md)) does not translate floating
point operations to machine code. As an example, this loop takes 0.09
seconds with an integer `Sum` and 2.4 seconds with a floating point
`Sum%`, more than 25 times slower:

```basic
Sum% = 0
FOR I = 1 TO 1000
  Sum% = Sum% + I * 3
NEXT I
```

Use integers wherever you can. Many calculations that seem to need
fractions can use integers scaled by 10 or 100 (for example, money in
cents or positions in 1/16 pixels).

## Putting it together

```basic
' Prints a table of the sine function in degrees, with a bar
DEG
FOR Angle = 0 TO 90 STEP 15
  Value% = SIN(Angle)
  PRINT Pad$(STR$(Angle), 3); " "; Bar$(Value%); " "; Value%
NEXT Angle

' Returns Text$ with spaces on the left, Width characters long
FUNC Pad$(Text$, Width)
  LOCAL Result$
  Result$ = Text$
  WHILE LEN(Result$) < Width
    Result$ = " " + Result$
  WEND
  RETURN Result$
ENDFUNC

' A bar of up to 10 stars for a value from 0 to 1
FUNC Bar$(Fraction%)
  LOCAL Stars, Result$
  Stars = INT(Fraction% * 10)
  Result$ = ""
  WHILE LEN(Result$) < 10
    IF LEN(Result$) < Stars
      Result$ =+ "*"
    ELSE
      Result$ =+ "."
    ENDIF
  WEND
  RETURN Result$
ENDFUNC
```

Output:

```
  0 .......... 0
 15 ***....... 0.2588190467
 30 *****..... 0.5000000033
 45 *******... 0.70710678
 60 *********. 0.8660254
 75 ********** 0.9659258333
 90 ********** 1
```
