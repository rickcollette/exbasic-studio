# 7. Functions

A function (`FUNC`) is a named block of code with its own parameters and
private variables. Functions replace the subroutines (`GOSUB`) and
procedures of other BASICs: they can return nothing, one value or several
values, and they can call themselves. This chapter is the complete
reference for defining and calling functions, for `RETURN` and `LOCAL`,
for the scope of variables, and for recursion and its limits.

## A first example

```basic
' Prints a framed title and the area of a rectangle
ShowTitle("ROOMS")
PRINT "KITCHEN: "; Area(4, 3)
PRINT "HALL: "; Area(2, 6)

' Prints Text$ between two lines of dashes
FUNC ShowTitle(Text$)
  PRINT "----------"
  PRINT Text$
  PRINT "----------"
ENDFUNC

' Returns the area of a Width x Height rectangle
FUNC Area(Width, Height)
  RETURN Width * Height
ENDFUNC
```

Output:

```
----------
ROOMS
----------
KITCHEN: 12
HALL: 12
```

The main program calls the functions before their definitions. That is
allowed: the compiler reads all function headers before it compiles the
program. The function bodies are not executed when the program reaches
them, so the definitions can be at the end, at the start or between parts
of the main code.

## Defining functions

### FUNC

Starts the definition of a function.

```
FUNC Name[$|%]( [param[$|%], ...] ) [result[$|%], ...]
  statements
ENDFUNC
```
(no abbreviation)

- _Name_ is the function name. Its type suffix gives the type of the
  function's value: no suffix for an integer, `$` for a string, `%` for a
  floating point number.
- _param_ are the parameters, separated by commas, each one with its own
  type suffix. The parentheses are required, also when there are no
  parameters: `FUNC Beep()`.
- _result_ are optional names for the results, see
  [Named results](#named-results).

A function is one of three kinds, decided by its header and its `RETURN`
statements:

| Kind | Header | How it returns | Call |
|------|--------|----------------|------|
| No value (a procedure) | `FUNC Show(X)` | `RETURN` without values, or `ENDFUNC` | as a statement: `Show(5)` |
| One value | `FUNC Square(X)` | `RETURN value` | in expressions: `Y = Square(5)` |
| Named results | `FUNC DivMod(A, B) Quot, Rest` | assign the results, then `ENDFUNC` or `RETURN` | `Q, R = DivMod(17, 5)` |

A function without named results returns a value when its body contains a
`RETURN` followed by a value (also after `THEN`). A function whose name
ends in `$` or `%` must return a value.

Rules for the definition:

- `FUNC` must be the first statement of its line.
- A function can't be defined inside another function.
- Each name can be defined only once. A function name can't also be used
  as a variable, an array or a `DATA` array name, with any suffix.
- Parameter and result names must be different from each other.

### ENDFUNC

Ends the definition of a function.

```
ENDFUNC
```
(no abbreviation)

When execution reaches `ENDFUNC`, the function returns to the caller. A
function with named results returns their current values. A one-value
function that reaches `ENDFUNC` without executing a `RETURN` with a value
returns an undefined value (in practice, the value of the previous call),
so make sure every path ends in a `RETURN`.

## Parameters

Parameters are private variables of the function that receive the values
of the arguments of each call:

- Integer parameters (`X`) receive integer values.
- String parameters (`Name$`) receive a copy of the string.
- Floating point parameters (`Size%`) receive floating point values.
  An integer argument is converted to floating point automatically.

Arguments are passed by value: assigning a parameter inside the function
does not change the variable used in the call. A parameter with the same
name as a global variable is a different variable, so the global is not
changed either.

```basic
' Parameters are copies of the arguments
Total = 100
PRINT Twice(Total); " "; Total
PRINT Half%(5)

' Doubles Total; only the parameter changes
FUNC Twice(Total)
  Total = Total * 2
  RETURN Total
ENDFUNC

' Half of a floating point value
FUNC Half%(Value%)
  RETURN Value% / 2
ENDFUNC
```

Output:

```
200 100
2.5
```

The number and the types of the arguments are checked when the program is
compiled. A floating point argument for an integer parameter is an error:
use `INT()` to convert it (see
[chapter 9](09-floating-point.md#conversions)).

## Results

### RETURN

Returns from a function, optionally giving its values.

```
RETURN
RETURN value [, value ...]
```
Abbreviation: `RET.`

- `RETURN value` returns one value from a one-value function. The value
  must have the type of the function name; an integer value is converted
  when the function returns floating point.
- `RETURN value, value, ...` gives a value for every named result, in
  order. The count and the types must match the header.
- `RETURN` without values leaves a function with no value, or a function
  with named results, which returns the current values of its results.
  It can't be used in a one-value function.

`RETURN` can be used anywhere in the function body, also inside loops: the
open `FOR` loops of the function are closed correctly. `RETURN` outside a
function is an error.

```basic
' Linear search in a small table
DATA Primes() = 2, 3, 5, 7, 11, 13
PRINT FindIndex(7); " "; FindIndex(8)

' Index of Value in Primes(), or -1 if it isn't there
FUNC FindIndex(Value)
  FOR Index = 0 TO 5
    IF Primes(Index) = Value THEN RETURN Index
  NEXT Index
  RETURN -1
ENDFUNC
```

Output:

```
3 -1
```

`EXIT` outside any loop also returns from a function, like `RETURN`
without values (see [EXIT](06-control-flow.md#exit)).

### Named results

Names written after the parameter list are the function's results. They
are private variables that start as zero (or an empty string) on each
call. The function assigns them, and the caller receives their values
when the function returns at `ENDFUNC`, at `EXIT` or at a `RETURN`
without values.

```basic
' Integer division with quotient and remainder
Quotient, Rest = DivMod(17, 5)
PRINT Quotient; " "; Rest

' Returns A / B and A MOD B
FUNC DivMod(A, B) Quot, Remain
  Quot = A / B
  Remain = A MOD B
ENDFUNC
```

Output:

```
3 2
```

The type of the function name must be the type of its first result: a
function whose first result is a string needs a `$` name.

```basic
' Results of different types
Initials$, Size = Describe$("ATARI")
PRINT Initials$; " "; Size

' First two letters and the length of Text$
FUNC Describe$(Text$) Start$, Length
  Start$ = Text$[1, 2]
  Length = LEN(Text$)
ENDFUNC
```

Output:

```
AT 5
```

A function with named results can also return with `RETURN` and a list of
values; then the values in the list are returned instead of the named
variables.

A function with exactly one named result works like a one-value function
and can be used in expressions.

## Calling functions

A function is called by its name followed by the arguments in
parentheses. The parentheses are required even when there are no
arguments: `Beep()`.

| Form | Use |
|------|-----|
| `Name(args)` as a statement | calls the function and discards its results |
| `Variable = Name(args)` or any expression | uses the value of a one-value function |
| `Var1, Var2, ... = Name(args)` | assigns all the results of a function with several results |

A function with several results can't be used in an expression. When
assigning several results, the number of variables must be the number of
results, and each variable must have the type of its result. Write `_`
in place of a variable to discard that result. The targets must be simple
variables, not array elements.

```basic
' Calling forms
Low, High = MinMax(7, 3)
PRINT Low; " "; High
_, High = MinMax(10, 20)
PRINT High
PRINT Square(3) + Square(4)
Square(5)

' Returns the smaller and the larger of A and B
FUNC MinMax(A, B) Smaller, Larger
  IF A < B THEN RETURN A, B
  RETURN B, A
ENDFUNC

' Prints a message and returns Value squared
FUNC Square(Value)
  PRINT "SQUARE OF "; Value
  RETURN Value * Value
ENDFUNC
```

Output:

```
3 7
20
SQUARE OF 3
SQUARE OF 4
25
SQUARE OF 5
```

A string function can be sliced like any string, for example
`Name$()[1, 3]`, and function calls can be nested: `Twice(Twice(3))`.

## Local variables

### LOCAL

Declares private variables of a function.

```
LOCAL var[$|%] [, var[$|%] ...]
```
(no abbreviation)

Each variable listed is private to the function and is set to zero (or to
an empty string) when the `LOCAL` statement is executed. `LOCAL` can only
be used inside a `FUNC`, and a name can't be declared twice in the same
function (parameters and named results count as declared).

The variable is private from the `LOCAL` statement to the end of the
function. Before the `LOCAL` line, the same name refers to the global
variable. Put `LOCAL` at the start of the function to avoid confusion.

```basic
' A LOCAL hides the global variable with the same name
Count = 7
CountDown(3)
PRINT "GLOBAL COUNT="; Count

' Prints From, From-1, ..., 1
FUNC CountDown(From)
  LOCAL Count
  FOR Count = From TO 1 STEP -1
    PRINT Count;
  NEXT Count
  PRINT
ENDFUNC
```

Output:

```
321
GLOBAL COUNT=7
```

A `LOCAL` inside a loop sets its variables to zero again at each
iteration.

## Scope of variables

The scope of a variable is the part of the program where its name refers
to it.

- **Private variables**: the parameters, the named results and the
  `LOCAL` variables of a function. Each function has its own; they can't
  be seen from the main code or from other functions, and they don't
  conflict with globals or with private variables of other functions that
  have the same name. They keep their value between calls, but don't rely
  on that: `LOCAL` and named results are cleared on each call.
- **Global variables**: all the other variables, in the main code and
  inside functions. A variable assigned for the first time inside a
  function is global.

As in the rest of the program, a global variable must be created (by an
assignment or a `DIM`) before the place where it is read, counting
positions in the source text, not in execution order. A function defined
near the start of the program can't read a global that is first assigned
later in the text. Declare such globals with `DIM` at the start of the
program (see
[chapter 17](17-program-organization.md#order-of-the-source)):

```basic
' Globals declared before the functions that use them
DIM Score, Player$

' Adds Points to the score of the player
FUNC AddPoints(Points)
  Score = Score + Points
ENDFUNC

Player$ = "ADA"
AddPoints(10)
AddPoints(25)
PRINT Player$; ": "; Score
```

Output:

```
ADA: 35
```

Arrays and `DATA` arrays are always global, also when they are created
inside a function.

## Recursion

A function is recursive when it calls itself, directly or through other
functions. The compiler finds the recursive functions from the calls in
the program. On each call of a recursive function, the current values of
its private variables (parameters, named results and `LOCAL` variables)
are saved, and they are restored when the call returns, so each active
call has its own values. Functions that are not recursive don't pay this
cost.

```basic
' Recursive functions
PRINT Factorial(7)
PRINT Fibonacci(15)

' N! = N * (N-1) * ... * 1
FUNC Factorial(N)
  IF N <= 1 THEN RETURN 1
  RETURN Factorial(N - 1) * N
ENDFUNC

' Fibonacci numbers, uses a LOCAL after the recursive call
FUNC Fibonacci(N)
  LOCAL Previous
  IF N < 2 THEN RETURN N
  Previous = Fibonacci(N - 1)
  RETURN Previous + Fibonacci(N - 2)
ENDFUNC
```

Output:

```
5040
610
```

Recursion has limits. The interpreter checks all of them: when one is
full, the program stops with error 10 (`ERROR- 10`) instead of
giving wrong results.

| Resource | Size | Used by |
|----------|------|---------|
| Frame stack | 512 bytes | saved private variables, on every call of a recursive function: 2 bytes for each integer, 6 for each floating point variable, and the length of the text plus 1 for each string |
| Return stack (6502 CPU stack) | 256 bytes, shared with the system | each active call of any function; calls nest up to about 95 levels |
| Value stack | 63 values | values waiting in an expression while a call runs, 3 values per active `FOR` loop, arguments |
| Floating point stack | 16 values | floating point values waiting in an expression while a call runs |

For example, a recursive function with 10 integer private variables uses
20 bytes of the frame stack per call, so it can be nested at most 25
levels.

The order of an expression matters for the value stacks. In
`RETURN 1 + Depth(N - 1)` the 1 waits on the value stack during the call,
so each level of recursion uses one more value, and the program stops
after about 60 levels. Writing `RETURN Depth(N - 1) + 1`, or storing the
result in a `LOCAL` first, keeps the stack empty during the call.

String parameters and `LOCAL` strings are saved with their text, so each
active call has its own strings, and a `+` expression can call the same
function again. A function can return one of its private strings
(`RETURN Result$`): the caller gets the text. Recursive calls don't lose
memory, so a recursive function with strings can be called any number of
times.

```basic
' Recursion with strings
PrintReversed("ABC")
PRINT
PRINT Countdown$(3)
PRINT Reverse$("HELLO")

' Prints Text$ backwards
FUNC PrintReversed(Text$)
  IF LEN(Text$) = 0 THEN RETURN
  PrintReversed(Text$[2])
  PRINT Text$[1, 1];
ENDFUNC

' Returns "N(N-1)...1"
FUNC Countdown$(N)
  IF N = 0 THEN RETURN ""
  RETURN STR$(N) + Countdown$(N - 1)
ENDFUNC

' Returns Text$ backwards
FUNC Reverse$(Text$)
  LOCAL Result$
  IF LEN(Text$) <= 1 THEN RETURN Text$
  Result$ = Reverse$(Text$[2]) + Text$[1, 1]
  RETURN Result$
ENDFUNC
```

Output:

```
CBA
321
OLLEH
```

## Compiler errors

The compiler checks the definitions and the calls. Errors in a function
header are reported with the file and line; errors in calls and statements
also show the column.

| Error | Cause |
|-------|-------|
| `expected FUNC name` | `FUNC` without a name. |
| `expected '(' after FUNC name` | The parameter list is missing; write `()` for none. |
| `expected ')' after parameters`, `expected parameter name` | Malformed parameter list. |
| `expected result name`, `expected end of FUNC definition` | Malformed list of named results. |
| `FUNC 'F' already defined at line N` | Two functions with the same name. |
| `repeated parameter name 'A'`, `repeated result name 'A'` | A name used twice in the header. |
| `FUNC name 'F$' must have the type of its first result 'N'` | The suffix of the name doesn't match the first named result. |
| `FUNC 'F$' must return a value` | A `$` or `%` function without a `RETURN` value. |
| `FUNC definition must be in its own line` | `FUNC` after other statements on the same line. |
| `FUNC can't be inside another FUNC` | A `FUNC` before the `ENDFUNC` of the previous one. |
| `FUNC name 'F' already used by DATA`, `'F' is a FUNC` | A `DATA` array with the name of a function. |
| `'F' is a FUNC, it can't be used as a variable` | A variable with the name of a function. |
| `FUNC 'F' is not defined` | A call to a name that is not a function, a variable or an array. |
| `FUNC 'F' needs 2 argument(s), got 1` | Wrong number of arguments. |
| `argument 1 of 'F' must be integer` | An argument of the wrong type (also `string`, `floating point`). |
| `FUNC 'F' does not return a value` | A function without a value used in an expression. |
| `FUNC 'F' returns 2 values, use 'A, B = F(...)'` | A function with several results used in an expression. |
| `FUNC 'F' returns 3 value(s), but there are 2 variables` | Wrong number of variables in a multiple assignment. |
| `result 2 of 'F' is integer, but 'B$' is string` | A variable of the wrong type in a multiple assignment. |
| `FUNC 'F' returns 1 value(s)` | `RETURN` with the wrong number of values. |
| `RETURN value 1 must be integer` | `RETURN` with a value of the wrong type. |
| `RETURN needs a value in FUNC 'F'` | `RETURN` without values in a one-value function. |
| `RETURN outside FUNC` | `RETURN` in the main code. |
| `LOCAL can only be used inside a FUNC` | `LOCAL` in the main code. |
| `'A' is already a local variable` | A `LOCAL` name already declared in the function. |
| `ENDFUNC without FUNC` | `ENDFUNC` in the main code. |
| `too many local variables in FUNC 'F'` | A recursive function with more than 255 private variables. |

## Design tips

- Give each function one job and a name that says what it does:
  `DrawBoard()`, `ReadScore()`, `IsFree(X, Y)`.
- Pass values as parameters and return results, instead of sharing
  global variables. Use globals for the state that the whole program
  shares, and declare them with `DIM` at the start.
- Declare the work variables of a function with `LOCAL`, so they don't
  change globals by accident.
- Use named results to return several values instead of setting globals.
- Every parameter, named result and `LOCAL` variable takes a slot in the
  program's variable table, like a global variable. The table holds about
  256 integer or string variables (a floating point variable takes 3
  slots), see [chapter 4](04-variables-and-data.md).
- Prefer loops to deep recursion on the Atari: keep recursion depth well
  below 95 calls.
- Calling a function costs some time. In the innermost loops of a
  program that must be fast, consider writing small computations inline
  (see [chapter 20](20-performance-and-memory.md)).

## Putting it together

```basic
' Prints a small multiplication table using several functions
Size = 4
PrintHeader(Size)
FOR Row = 1 TO Size
  PrintRow(Row, Size)
NEXT Row
Smallest, Largest = Range(Size)
PRINT "PRODUCTS FROM "; Smallest; " TO "; Largest

' Prints a number right-aligned in 4 columns
FUNC PrintCell(Value)
  PRINT Pad$(STR$(Value), 4);
ENDFUNC

' Returns Text$ with spaces on the left, Width characters long
FUNC Pad$(Text$, Width)
  LOCAL Result$
  Result$ = Text$
  WHILE LEN(Result$) < Width
    Result$ = " " + Result$
  WEND
  RETURN Result$
ENDFUNC

' Prints the column numbers
FUNC PrintHeader(Columns)
  PRINT "   *";
  FOR Column = 1 TO Columns
    PrintCell(Column)
  NEXT Column
  PRINT
ENDFUNC

' Prints one row of the table
FUNC PrintRow(Row, Columns)
  PrintCell(Row)
  FOR Column = 1 TO Columns
    PrintCell(Row * Column)
  NEXT Column
  PRINT
ENDFUNC

' Smallest and largest product of the table
FUNC Range(Columns) Low, High
  Low = 1
  High = Columns * Columns
ENDFUNC
```

Output:

```
   *   1   2   3   4
   1   1   2   3   4
   2   2   4   6   8
   3   3   6   9  12
   4   4   8  12  16
PRODUCTS FROM 1 TO 16
```
