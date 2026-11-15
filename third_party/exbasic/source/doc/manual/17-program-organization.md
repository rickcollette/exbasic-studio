# 17. Program Organization

This chapter is about the shape of whole programs: what happens when a
program starts and ends, the order of things in the source, splitting a
program into several files with `INCLUDE`, chaining programs with `RUN`
and passing variables between them with `COMMON`, clearing memory with
`CLR`, and advice for structuring big programs.

## Program startup

A compiled program (`.XEX` file) is loaded by DOS, or by the `RUN`
statement of another program. Before the first statement runs, the
exBASIC runtime prepares a clean state:

| Item | State at startup |
|------|------------------|
| Integer and floating point variables | 0 |
| String variables | empty |
| Arrays | none; they are created by `DIM` and `DATA` |
| I/O channels #1 to #7 | closed (#0 is the screen editor, `E:`) |
| `ERR()` | 1 |
| `PRINT COLOR` | 0 |
| Sound | all voices off |
| Trigonometric functions | radians (only in programs that use floating point) |

The screen is not cleared and the graphics mode is not changed: the
program starts with the screen that DOS (or the previous program) left.
Use `GRAPHICS 0` or `CLS` if you need a clean screen (see
[chapter 10](10-console-io.md) and [chapter 12](12-graphics.md)).

The program then executes the main code from the first line. `FUNC`
definitions in the main code are skipped (see
[chapter 7](07-functions.md)), and `DATA` statements don't execute
anything.

## Program end

A program ends when:

- it executes `END` (see [END](06-control-flow.md#end)), or
- execution reaches the end of the main code.

The program closes the I/O channels 1 to 7 and turns the sound off. An
`.XEX` program then returns to DOS. A runtime error (see
[Appendix B](B-error-messages.md#runtime-errors)) ends the program in the
same way, after printing its message.

A program can also load and start another program with `RUN`, see
[Chaining programs](#chaining-programs).

## Order of the source

exBASIC reads the source from top to bottom, and some rules depend on the
order of the text, not on the order in which the statements run:

- A variable is created by its first assignment (or by `DIM`, `INPUT`,
  `FOR`, and similar statements). It can't be read in a line above that
  place, even if the program executes the assignment first.
- An array must be created by `DIM` or `DATA` above the lines that use it.
- Functions are different: they can be called above their definition,
  because the compiler reads all the function headers first.

A function defined near the top that uses global variables needs those
variables to exist before its definition. The clearest way is to declare
all the global variables with `DIM` at the start of the program. `DIM`
with plain names creates variables without assigning them:

```basic
' Global state of the program
DIM Score, Lives, PlayerName$, Speed%
DIM Board(8) BYTE

' Shows the status line
FUNC ShowStatus()
  PRINT PlayerName$; ": SCORE "; Score; ", LIVES "; Lives
ENDFUNC

' Main program
PlayerName$ = "ADA"
Lives = 3
Score = 120
ShowStatus()
```

Output:

```
ADA: SCORE 120, LIVES 3
```

A good order for a program file is:

1. A header comment that says what the program does.
2. `INCLUDE` lines for the libraries it uses.
3. `COMMON`, if the program receives variables from another program.
4. `DIM` of the global variables and arrays, and `DATA` arrays.
5. The main code.
6. The function definitions, each with a one-line comment.

## Splitting programs into files

### INCLUDE

Inserts another source file at this place.

```
INCLUDE "file"
```
(no abbreviation)

The compiler replaces the `INCLUDE` line with the full text of _file_,
as if it were written there. Use it to share functions between programs
and to split big programs into several files.

- `INCLUDE` must be alone on its line, as the first statement. A comment
  with `'` can follow it. The file name must be a constant between
  double quotes.
- `INCLUDE` is processed before the program is compiled, so it can't be
  made conditional with `IF`: the file is always included.
- The included file can include other files, up to 16 levels deep. A file
  that includes itself, directly or through other files, is an error.
- There is no protection against including the same file twice. If a
  library is included twice, its functions are defined twice, which is an
  error. Include each library once, in the main file.
- `INCLUDE` is the only way to build one program from several BASIC
  files. Giving two BASIC files to the compiler stops with the error
  `only one BASIC file can be linked into a program, 'util.bas' is the
  second one; add it with INCLUDE "util.bas" instead` (see
  [chapter 19](19-compiler-reference.md)).

**Where files are searched.** A file name with an absolute path is used as
it is. Other names are searched, in order:

1. in the folder of the file that contains the `INCLUDE` line,
2. in each folder given with the `-I:` compiler option, in the order of
   the options (see [chapter 19](19-compiler-reference.md)),
3. in the `include` folder of the compiler installation, if there is one.

**Errors.** Errors in an included file are reported with the name of that
file and the line in it:

```
./lib/broken.bas:2:4: missing ')' for this '('
```

The `INCLUDE` errors are:

| Error | Cause |
|-------|-------|
| `INCLUDE file 'name' not found` | The file is not in any of the folders searched. |
| `invalid INCLUDE, use: INCLUDE "file"` | The name is missing, not quoted, or followed by other statements. |
| `recursive INCLUDE of 'name'` | The file includes itself. |
| `too many nested INCLUDE files` | More than 16 levels of `INCLUDE`. |

An `INCLUDE` after other statements on the same line (like
`PRINT 1 : INCLUDE "x.bas"`) is not recognized and gives a parse error.

**Example.** A program in three files. The main file:

```basic
' GAME.BAS: main program
INCLUDE "lib/text.bas"
INCLUDE "lib/mathlib.bas"

PRINT Pad$("7", 3); " "; Square(9)
```

The file `lib/text.bas`:

```basic
' TEXT.BAS: text helpers

' Returns Text$ with zeros on the left, Width characters long
FUNC Pad$(Text$, Width)
  LOCAL Result$
  Result$ = Text$
  WHILE LEN(Result$) < Width
    Result$ = "0" + Result$
  WEND
  RETURN Result$
ENDFUNC
```

The file `lib/mathlib.bas`:

```basic
' MATHLIB.BAS: small math helpers

' Returns Value squared
FUNC Square(Value)
  RETURN Value * Value
ENDFUNC
```

Compiled with `exbasic game.bas`, the program prints:

```
007 81
```

If the libraries are in another folder, for example `/home/ada/atari/lib`, write
`INCLUDE "text.bas"` and give the folder to the compiler:

```sh
exbasic -I:/home/ada/atari/lib game.bas
```

## Chaining programs

A program that is too big for the memory of the Atari can be split into
several programs that load each other: for example a title screen, the
game and the high score table. `RUN` loads and starts the next program,
and `COMMON` passes variables to it.

### RUN

Loads and runs another program.

```
RUN "device:file"
RUN string
```
(no abbreviation)

_string_ is the file name with its device, like `"D:GAME.XEX"` or
`"D2:LEVEL2.XEX"`. Any string expression can be used.

`RUN` works like this:

1. It closes channel #7 and opens the file on channel #7 for reading.
2. If the file can't be opened, `RUN` returns and the program continues
   with the next statement. `ERR()` gives the error code (for example
   170, file not found). See [chapter 11](11-files-and-devices.md) for
   the error codes.
3. If the program has a `COMMON` statement, the current values of the
   `COMMON` variables are saved in a buffer just below `MEMTOP` (the top
   of the free memory).
4. Display list interrupts are turned off (their code would be
   overwritten).
5. A small loader is copied to page 6 (`$0600`-`$06FF`) and reads the new
   program over the old one. Initialization segments of the file are
   called as DOS does. When the file is loaded, the new program starts.

The loaded file can be any Atari binary executable, not only an exBASIC
program, but it must not load data into page 6 or into the `COMMON`
buffer. If it does, or if there is an error while reading the file, the
loader stops and returns to DOS: the old program is already gone at that
point.

The new program starts as described in [Program startup](#program-startup):
all its variables are cleared, except the ones it receives with `COMMON`.
When the new program ends, it returns to DOS, not to the old program.

```basic
' MENU.BAS: loads the game, or reports an error
RUN "D:GAME.XEX"
ErrorCode = ERR()
PRINT "CAN'T LOAD GAME.XEX, ERROR "; ErrorCode
```

### COMMON

Declares the variables that are passed to and from other programs.

```
COMMON var[$|%] [, var[$|%] ...]
```
(no abbreviation)

_var_ are integer, string (`$`) or floating point (`%`) variables. Arrays
can't be passed.

`COMMON` has two jobs:

- When the program executes `RUN`, the current values of the `COMMON`
  variables are saved for the next program.
- When the program executes the `COMMON` statement, the variables get the
  values saved by the previous program, if there are any and the two
  `COMMON` statements match. Otherwise the variables keep their values.

The values are passed **by position**: the first variable of the new
program gets the value of the first variable of the old one, and so on.
The names don't need to match, but the number of variables and the type
of each one must be the same. The programs check this with a signature
computed from the count and the types; if the signatures differ, no value
is passed. Using the same `COMMON` line in all the programs avoids
mistakes: put it in a file and `INCLUDE` it.

The saved values can be received only once: the first `COMMON` statement
executed after the program starts takes them.

Rules:

- A program can have only one `COMMON` statement, and it can't be inside a
  `FUNC`.
- Put `COMMON` at the start of the program, before any `DIM` or string
  assignment. The values wait in the buffer below `MEMTOP` until `COMMON`
  executes, and arrays or strings created before that can overwrite them.
- A string variable that was never assigned is passed as an empty string.

Memory used by the buffer: 2 bytes for each integer, 6 bytes for each
floating point variable and 256 bytes for each string. The loaded program
must not use that memory, which is no problem for exBASIC programs that
execute `COMMON` first.

**Example.** The first program sets the values and runs the second:

```basic
' FIRST.BAS: sets up the player and starts the game
COMMON Level, Score%, Player$
Level = 1
Score% = 0
Player$ = "ADA"
PRINT "LOADING THE GAME..."
RUN "D:GAME.XEX"
ErrorCode = ERR()
PRINT "CAN'T LOAD GAME.XEX, ERROR "; ErrorCode
```

The second program, compiled to `GAME.XEX` on the same disk, receives
them:

```basic
' GAME.BAS: receives the values from FIRST.BAS
COMMON Level, Score%, Player$
PRINT "PLAYER "; Player$; ", LEVEL "; Level; ", SCORE "; Score%
```

Running `FIRST.XEX` prints:

```
LOADING THE GAME...
PLAYER ADA, LEVEL 1, SCORE 0
```

The second program can pass the values on (or back to the first program)
with its own `RUN`.

**Errors.**

| Error | Cause |
|-------|-------|
| `variable 'A' is not integer` | A `COMMON` name is already used by an array. |
| `'F' is a FUNC, it can't be in COMMON` | A `COMMON` name is a function. |
| `only one COMMON statement is allowed` | A second `COMMON` statement. |
| `COMMON can't be inside a FUNC` | A `COMMON` statement inside a function. |

A `COMMON` without variables gives a general parse error at the `COMMON`
keyword.

## Clearing memory

### CLR

Clears all variables and frees the memory of arrays and strings.

```
CLR
```
(no abbreviation)

After `CLR`:

- all integer and floating point variables are 0, including the counters
  of `FOR` loops and the private variables of functions;
- all string variables are empty, and the 256 bytes of each one are
  freed;
- all arrays created with `DIM` are freed; they can't be used until their
  `DIM` statement executes again.

`DATA` arrays are part of the program and are not changed. Open channels,
the screen and the graphics mode are not changed either.

Because a `DIM` for the same array can appear only once in the source,
put the `DIM` statements in a function to create the arrays again after
`CLR`:

```basic
' Creates the arrays of the program
FUNC CreateArrays()
  DIM Map(1000)
ENDFUNC

' Main program
Score = 5
CreateArrays()
Map(1) = 7
CLR
PRINT "SCORE="; Score
CreateArrays()
PRINT "MAP(1)="; Map(1)
```

Output:

```
SCORE=0
MAP(1)=0
```

Don't use `CLR` inside a `FOR` loop or a function that must continue
after it: the loop counter and the function's variables are cleared too.

### END

Ends the program. See [END](06-control-flow.md#end) in chapter 6.

## Structuring big programs

- **Main loop and functions.** Keep the main code short: initialization
  followed by a main loop that calls one function per task, like
  `ReadJoystick()`, `MovePlayer()`, `DrawScreen()`.
- **Global state at the top.** Declare the global variables and arrays
  with `DIM` at the start of the program, so every function can use them,
  and so a reader sees the state of the program in one place.
- **Libraries.** Move tested, general functions (text formatting,
  drawing helpers, sound effects) to separate files and `INCLUDE` them.
  Keep the files of a project in one folder, or give the library folder
  with `-I:`.
- **Variable slots.** All the variables of a program, including the
  parameters, named results and `LOCAL` variables of functions and the
  hidden strings of `+`, share one table of about 256 slots (a floating
  point variable takes 3). A very big program can run out of slots; the
  error then comes from the linker (`too many variables`). Reuse work
  variables and prefer integer variables.
- **Memory.** Check `FRE()` while developing. Strings use 256 bytes each,
  and string arrays 256 bytes per assigned element. See
  [chapter 20](20-performance-and-memory.md).
- **Chaining.** When a program no longer fits in memory, split it into
  parts that load each other with `RUN`, and pass the shared state with
  one `COMMON` line kept in an included file.

## Putting it together

A menu program that chains to two other programs and gets the result
back. The shared variables are in one file:

```basic
' SHARED.BAS: the variables shared by all the programs
COMMON Credits, LastScore, Player$
```

The menu (compiled to `MENU.XEX`):

```basic
' MENU.BAS: start of the game collection
INCLUDE "shared.bas"

IF Player$ = ""
  Player$ = "PLAYER 1"
  Credits = 3
ENDIF
PRINT "WELCOME, "; Player$
PRINT "CREDITS: "; Credits; ", LAST SCORE: "; LastScore
IF Credits > 0
  Credits = Credits - 1
  RUN "D:GAME.XEX"
  ErrorCode = ERR()
  PRINT "ERROR "; ErrorCode; " LOADING THE GAME"
ENDIF
```

The game (compiled to `GAME.XEX`), which goes back to the menu:

```basic
' GAME.BAS: a very short game
INCLUDE "shared.bas"

PRINT "PLAYING, "; Player$
LastScore = 100 + Credits * 10
RUN "D:MENU.XEX"
```

Running `MENU.XEX` shows the menu and the game in turn until the credits
are used up:

```
WELCOME, PLAYER 1
CREDITS: 3, LAST SCORE: 0
PLAYING, PLAYER 1
WELCOME, PLAYER 1
CREDITS: 2, LAST SCORE: 120
PLAYING, PLAYER 1
WELCOME, PLAYER 1
CREDITS: 1, LAST SCORE: 110
PLAYING, PLAYER 1
WELCOME, PLAYER 1
CREDITS: 0, LAST SCORE: 100
```
