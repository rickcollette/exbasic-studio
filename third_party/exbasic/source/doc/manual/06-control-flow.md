# 6. Control Flow

exBASIC has no line numbers and no `GOTO` or `GOSUB`. Programs are built
from structured blocks: `IF` for decisions, and `FOR`, `WHILE`, `REPEAT` and
`DO` for loops, with `EXIT` to leave a loop early. This chapter describes
every control statement, the exact rules of each loop, how blocks nest, and
the errors the compiler reports when blocks don't match. Functions (`FUNC`,
`RETURN`) are described in [chapter 7](07-functions.md).

## Conditions

`IF`, `ELIF`, `WHILE`, `UNTIL`, `DO WHILE`, `DO UNTIL`, `LOOP WHILE` and
`LOOP UNTIL` take a condition. A condition is any integer expression, a
comparison or a logical combination of them (see
[chapter 5](05-expressions.md)):

- An integer expression is true when it is not zero: `IF N THEN ...` is the
  same as `IF N <> 0 THEN ...`.
- Comparisons (`=`, `<>`, `<`, `>`, `<=`, `>=`) of integers, floating point
  values or strings give true (1) or false (0).
- `AND`, `OR` and `NOT` combine conditions.

A floating point value or a string alone is not a condition: write
`IF X% <> 0` and `IF A$ <> ""` (or `IF LEN(A$)`).

## Decisions

### IF

Executes statements only when a condition is true. `IF` has two forms.

```
IF condition THEN statements [ELSE statements]
IF condition
  statements
[ELIF condition
  statements] ...
[ELSE
  statements]
ENDIF
```
Abbreviations: `I.` for `IF`, `T.` for `THEN`

**Single-line form.** With `THEN`, all the statements after `THEN` up to
the end of the line are executed when the condition is true, also the
statements after a `:`. This is the same as in Atari BASIC and Turbo
BASIC XL. An optional `ELSE` on the same line starts the statements that
are executed when the condition is false; they also go up to the end of
the line.

```basic
' Everything after THEN, up to the end of the line, is conditional
Count = 1
IF Count = 0 THEN PRINT "ZERO" : PRINT "THE END"
IF Count THEN PRINT "COUNT IS "; : PRINT "NOT ZERO"
IF Count = 1 THEN IF Count > 0 THEN PRINT "BOTH TRUE"
IF Count = 2 THEN PRINT "TWO" ELSE PRINT "NOT TWO" : PRINT "ALSO ELSE"
```

Output:

```
COUNT IS NOT ZERO
BOTH TRUE
NOT TWO
ALSO ELSE
```

The statements after `THEN` can be any statements: assignments, function
calls, `EXIT`, `RETURN`, `END`, or another `IF ... THEN`. In a nested
one-line `IF`, the `ELSE` belongs to the last `IF`. A block opened after
`THEN`, like a `FOR` loop, must also end on the same line:

```basic
Count = 1
IF Count = 1 THEN FOR I = 1 TO 3 : PRINT I; : NEXT I : PRINT
```

Output:

```
123
```

Otherwise the compiler reports `blocks started after THEN must end in the
same line`. `THEN` must be followed by a statement: `IF Count = 1 THEN`
alone gives the error `expected a statement after THEN`. For longer code,
use the block form.

**Block form.** Without `THEN`, all the statements that follow the `IF` up
to the matching `ELIF`, `ELSE` or `ENDIF` are executed when the condition is
true. If the condition is false, the program tests the conditions of the
`ELIF` parts in order and executes the first part whose condition is true.
If none is true, the statements after `ELSE` are executed, if there is an
`ELSE`. Execution then continues after `ENDIF`.

```basic
' Names the numbers from 1 to 4
FOR Number = 1 TO 4
  IF Number = 1
    PRINT "ONE"
  ELIF Number = 2
    PRINT "TWO"
  ELIF Number = 3
    PRINT "THREE"
  ELSE
    PRINT "MANY"
  ENDIF
NEXT Number
```

Output:

```
ONE
TWO
THREE
MANY
```

A block `IF` can be written on one line, separating the statements with
`:`:

```basic
Lives = 1
IF Lives = 1 : PRINT "LAST LIFE" : ENDIF
```

Block `IF` statements can be nested to any depth. Each `IF` needs its own
`ENDIF`.

See also: [ELIF](#elif), [ELSE](#else), [ENDIF](#endif).

### ELIF

Tests another condition inside a block `IF`.

```
ELIF condition
```
Abbreviation: `ELI.`

`ELIF` is only valid inside a block `IF`, before the `ELSE` (if any). Its
statements run when all the previous conditions of the same `IF` were false
and this condition is true. A block `IF` can have any number of `ELIF`
parts. See the example in [IF](#if).

### ELSE

Starts the part of a block `IF` that runs when no condition was true.

```
ELSE
```
Abbreviation: `EL.`

`ELSE` must be the last part of a block `IF`, after all the `ELIF` parts,
and must be followed by the end of the statement (a new line, `:` or a
comment). The single-line `IF ... THEN` form has its own `ELSE` on the same
line (see [IF](#if)).

```basic
Number = 7
IF Number MOD 2 = 0
  PRINT Number; " IS EVEN"
ELSE
  PRINT Number; " IS ODD"
ENDIF
```

Output:

```
7 IS ODD
```

### ENDIF

Ends a block `IF`.

```
ENDIF
```
Abbreviation: `E.`

Every block `IF` must be closed with `ENDIF`. Note that `E.` is `ENDIF`,
not `END`.

## Loops

exBASIC has four kinds of loops. They differ in where the condition is
tested:

| Loop | Condition tested | Runs at least once | Ends when |
|------|------------------|--------------------|-----------|
| `FOR` ... `NEXT` | at the start | no | the counter passes the limit |
| `WHILE` ... `WEND` | at the start | no | the condition is false |
| `REPEAT` ... `UNTIL` | at the end | yes | the condition is true |
| `DO` ... `LOOP` | at the start, the end, both or never | depends | depends on the form, or only with `EXIT` |

Any loop can be ended early with [EXIT](#exit).

### FOR

Repeats a block of statements while a counter variable goes from a start
value to a limit.

```
FOR var = start TO limit [STEP step]
  statements
NEXT [var]
```
Abbreviations: `F.` for `FOR`, `T.` for `TO`, `S.` for `STEP`

_var_ is the counter: an integer variable or an element of a `WORD` array,
like `A(2)`. It can't be a floating point variable or a `BYTE` array
element. _start_, _limit_ and _step_ are integer expressions; if `STEP` is
not given, the step is 1.

A `FOR` loop works like this:

1. _start_ is stored in _var_.
2. _limit_ and _step_ are evaluated **once**, and their values are kept
   for the whole loop. Changing the variables used in these expressions
   inside the loop does not change the number of iterations.
3. The counter is tested **before** each iteration, including the first.
   If _step_ is zero or positive, the loop ends when _var_ is greater than
   _limit_. If _step_ is negative, the loop ends when _var_ is less than
   _limit_.
4. The statements of the loop are executed.
5. At `NEXT`, _step_ is added to _var_ and the program goes back to the
   test of step 3.

Some consequences:

- If _start_ is already past _limit_, the statements are never executed
  (an empty loop), and _var_ keeps the value _start_.
- After a loop that ends normally, _var_ holds the first value past the
  limit: after `FOR I = 1 TO 10`, `I` is 11.
- After an `EXIT`, _var_ keeps the value it had when `EXIT` was executed.
- The loop uses the current value of _var_ at each `NEXT`, so assigning
  _var_ inside the loop changes the following iterations.
- With `STEP 0` the loop never ends (unless you use `EXIT`).

```basic
' The exact rules of FOR
' An empty loop: the body never runs
FOR I = 10 TO 1
  PRINT "NEVER PRINTED"
NEXT I
PRINT "EMPTY LOOP, I="; I

' A negative step, and the counter after the loop
FOR I = 10 TO 1 STEP -3
  PRINT I; " ";
NEXT I
PRINT
PRINT "AFTER THE LOOP, I="; I

' The limit is read only once
Last = 3
FOR I = 1 TO Last
  Last = 10
  PRINT I;
NEXT I
PRINT
PRINT "LIMIT READ ONCE, I="; I

' Changing the counter changes the next iterations
FOR I = 1 TO 6
  I = I + 1
  PRINT I;
NEXT I
PRINT

' An array element as the counter
DIM Table(3)
FOR Table(2) = 5 TO 7
  PRINT Table(2);
NEXT
PRINT
```

Output:

```
EMPTY LOOP, I=10
10 7 4 1 
AFTER THE LOOP, I=-2
123
LIMIT READ ONCE, I=4
246
567
```

**Limits near the ends of the integer range.** The counter is a 16-bit
signed integer (-32768 to 32767). Loops whose limit is at the end of the
range work correctly: `FOR I = 32765 TO 32767` runs 3 times, and
`FOR I = 0 TO -32768 STEP -16384` runs with 0, -16384 and -32768.

**Nesting.** `FOR` loops can be nested; each one needs its own `NEXT`, and
the inner loop must end before the outer one. Each active `FOR` loop keeps
three values (the counter address, the limit and the step) on the
interpreter's value stack, which has room for 63 values and is also used
to evaluate expressions and pass function arguments. About 20 nested
`FOR` loops (counting also the loops that are active in the functions
that called the current one) fill it, and the program stops with the
error 10 (`ERROR- 10`). See [Recursion](07-functions.md#recursion) for
loops inside recursive functions.

See also: [NEXT](#next), [EXIT](#exit).

### NEXT

Ends a `FOR` loop.

```
NEXT [var]
```
Abbreviation: `N.`

`NEXT` adds the step to the counter of the innermost open `FOR` loop and
jumps back to the test at the start of the loop.

The variable name after `NEXT` is optional. When it is given, the
compiler checks that it is the counter of the innermost open `FOR`:
`NEXT J` after `FOR I` gives the error `NEXT J does not match FOR I`.
When the counter is an array element, write `NEXT` alone.

```basic
FOR Row = 1 TO 2
  FOR Column = 1 TO 3
    PRINT Row * 10 + Column; " ";
  NEXT Column
NEXT Row
PRINT
```

Output:

```
11 12 13 21 22 23 
```

### WHILE

Repeats a block of statements while a condition is true, testing the
condition before each iteration.

```
WHILE condition
  statements
WEND
```
Abbreviations: `W.` for `WHILE`, `WE.` for `WEND`

The condition is tested first. If it is false, the program skips to the
statement after `WEND` and the loop body is never executed. If it is true,
the statements are executed, and `WEND` jumps back to test the condition
again.

```basic
' First power of 3 that is 100 or more
Power = 1
WHILE Power < 100
  Power = Power * 3
WEND
PRINT Power
```

Output:

```
243
```

### WEND

Ends a `WHILE` loop.

```
WEND
```
Abbreviation: `WE.`

`WEND` jumps back to the `WHILE` that opened the loop. Nothing may follow
`WEND` in the same statement.

### REPEAT

Repeats a block of statements until a condition becomes true, testing the
condition after each iteration.

```
REPEAT
  statements
UNTIL condition
```
Abbreviations: `R.` for `REPEAT`, `U.` for `UNTIL`

The statements are always executed at least once. Then the condition is
tested: if it is false, the loop repeats; if it is true, the loop ends and
the program continues after `UNTIL`.

```basic
Count = 10
REPEAT
  PRINT Count
  INC Count
UNTIL Count > 5
```

Output:

```
10
```

The loop ran once even though `Count` was already greater than 5.

### UNTIL

Ends a `REPEAT` loop.

```
UNTIL condition
```
Abbreviation: `U.`

If the condition is false, the program jumps back to the `REPEAT`. If it is
true, the loop ends. A common use is waiting for an event:

```basic
' Wait until a key is pressed
REPEAT : UNTIL KEY()
```

`UNTIL` is also used after `DO` and `LOOP`, see [DO](#do).

### DO

Starts a loop that can test a condition at the start, at the end, at both
places or nowhere.

```
DO [WHILE condition | UNTIL condition]
  statements
LOOP [WHILE condition | UNTIL condition]
```
Abbreviations: `L.` for `LOOP`, `W.` for `WHILE`, `U.` for `UNTIL`
(`DO` has no abbreviation)

- `DO` alone does not test anything.
- `DO WHILE condition` tests the condition at the start of each iteration
  and ends the loop when it is false.
- `DO UNTIL condition` tests the condition at the start of each iteration
  and ends the loop when it is true.
- `LOOP` alone always jumps back to the `DO`.
- `LOOP WHILE condition` jumps back to the `DO` only if the condition is
  true.
- `LOOP UNTIL condition` jumps back to the `DO` only if the condition is
  false.

Any `DO` form can be combined with any `LOOP` form. When the loop jumps
back from `LOOP`, the condition of `DO` (if any) is tested again. A loop
with `DO` and `LOOP` and no conditions is endless: the only way to leave
it is `EXIT` (or `RETURN` or `END`).

```basic
' The forms of DO ... LOOP
Count = 0
DO WHILE Count < 3
  PRINT "W"; Count;
  INC Count
LOOP
PRINT

DO
  PRINT "U"; Count;
  DEC Count
LOOP UNTIL Count = 0
PRINT

' An endless loop, left with EXIT
DO
  INC Count
  IF Count = 4 THEN EXIT
LOOP
PRINT "EXIT AT COUNT="; Count

' Conditions at both ends
Count = 0
DO WHILE Count < 3
  INC Count
LOOP WHILE Count < 2
PRINT "BOTH TESTS, COUNT="; Count
```

Output:

```
W0W1W2
U3U2U1
EXIT AT COUNT=4
BOTH TESTS, COUNT=2
```

`DO WHILE` and `WHILE` ... `WEND` do the same thing, as do `DO` ...
`LOOP UNTIL` and `REPEAT` ... `UNTIL`. Use the form you find clearer.

### LOOP

Ends a `DO` loop.

```
LOOP [WHILE condition | UNTIL condition]
```
Abbreviation: `L.`

See [DO](#do) for the meaning of each form.

### EXIT

Leaves the innermost loop, or returns from a function.

```
EXIT
```
Abbreviation: `EX.`

Inside a loop, `EXIT` jumps to the statement after the end of the
innermost `FOR`, `WHILE`, `REPEAT` or `DO` loop that contains it. Block
`IF` statements are not loops: an `EXIT` inside an `IF` leaves the loop
that contains the `IF`.

`EXIT` leaves only one loop. To leave two nested loops, set a variable and
test it in the outer loop, or put the loops in a function and use
`RETURN`.

```basic
' EXIT leaves only the innermost loop
FOR I = 1 TO 3
  FOR J = 1 TO 3
    IF J = 2 THEN EXIT
    PRINT I; J; " ";
  NEXT J
NEXT I
PRINT
PRINT "I="; I; " J="; J

REPEAT
  EXIT
  PRINT "NEVER PRINTED"
UNTIL 0
PRINT "OUT OF REPEAT"
```

Output:

```
11 21 31 
I=4 J=2
OUT OF REPEAT
```

Inside a `FUNC` but outside any loop, `EXIT` returns from the function,
like a `RETURN` without values: functions with named results return the
current values of the results (see [chapter 7](07-functions.md#return)).

`EXIT` outside any loop and outside a function is an error
(`EXIT without loop`).

### END

Ends the program.

```
END
```
(no abbreviation)

`END` stops the program immediately. It closes the I/O channels 1 to 7
and turns the sound off. A compiled `.XEX` program returns to DOS. `END`
can appear anywhere: inside loops, `IF` blocks or functions.
A program also ends when execution reaches the last statement of the main
code, so `END` is only needed to stop before that.

```basic
FOR I = 1 TO 5
  PRINT I
  IF I = 2 THEN END
NEXT I
PRINT "NEVER PRINTED"
```

Output:

```
1
2
```

Function definitions are not executed when the program reaches them (see
[chapter 7](07-functions.md)), so you don't need an `END` before the
functions at the end of a program.

## Nesting rules

Blocks must be properly nested: a block that starts inside another block
must end inside it too. The compiler matches each closing statement with
the last block that is still open:

| Opening | Closing |
|---------|---------|
| `IF` (without `THEN`) | `ENDIF` (with optional `ELIF` and `ELSE` parts before it) |
| `FOR` | `NEXT` |
| `WHILE` | `WEND` |
| `REPEAT` | `UNTIL` |
| `DO` | `LOOP` |
| `FUNC` | `ENDFUNC` |

For example, this is invalid because the `IF` is still open when the
`NEXT` arrives:

```
FOR I = 1 TO 10
  IF I > 5
    PRINT I
NEXT I
ENDIF
```

Blocks can be nested to any depth, with the limit on active `FOR` loops
described in [FOR](#for). A `FUNC` can't be defined inside another
`FUNC`.

## Errors

These are the errors the compiler reports for control statements. Errors
found while reading a line show the file, line and column, followed by the
line and a marker under the error position:

```
prog.bas:3:6: NEXT J does not match FOR I
  NEXT J 
  ------^
```

When a closing statement doesn't match the last open block, the message
names the statement and the block it needs, and also the block that is
still open, as in `NEXT without FOR loop (an IF is still open)`. Errors
found at the end of the program show only the file and the line where the
open block started.

| Error | Cause |
|-------|-------|
| `ELIF without IF`, `ELSE without IF`, `ENDIF without IF` | No open block `IF`, a second `ELSE` (or an `ELIF` after the `ELSE`) in the same `IF`, or another block is still open inside the `IF`. |
| `NEXT without FOR loop`, `NEXT J without FOR` | `NEXT` without an open `FOR`, or another block is still open inside the `FOR`. |
| `NEXT J does not match FOR I` | The variable after `NEXT` is not the counter of the innermost `FOR`. |
| `WEND without WHILE loop` | `WEND` without an open `WHILE`, or another block is still open inside it. |
| `UNTIL without REPEAT loop` | `UNTIL` without an open `REPEAT`, or another block is still open inside it. |
| `LOOP without DO loop` | `LOOP` without an open `DO`, or another block is still open inside it. |
| `ENDFUNC without FUNC` | `ENDFUNC` outside a function, or a block is still open inside the function. |
| `(an IF is still open)` | Added to the messages above when another block is open; it names that block: `IF`, `ELSE`, `FOR loop`, `WHILE loop`, `REPEAT loop` or `DO loop`. |
| `blocks started after THEN must end in the same line` | A loop or block `IF` started in a one-line `IF ... THEN` is not closed on that line. |
| `expected a statement after THEN` | `THEN` at the end of the line. |
| `EXIT without loop` | `EXIT` outside any loop and outside a function. |
| `unclosed IF` | The program ends with an `IF` block still open; the same message names `FOR loop`, `WHILE loop`, `REPEAT loop`, `DO loop` or `FUNC`. |

When the closing statement is abbreviated (`N.`, `E.`), the message has the
form `missing FOR loop` or `missing IF`.

See [Appendix B](B-error-messages.md) for the complete list of compiler
error messages.
