# 10. Console Input and Output

This chapter covers text output to the screen and input from the
keyboard: `PRINT` and its separators (`,` `;` `TAB` `RTAB` `COLOR`),
`INPUT`, `GET`, `PUT`, `POSITION`, `CLS` and the `KEY()` function. It
also explains the parts of the Atari screen editor that a program sees,
inverse video and the ATASCII character set. Reading and writing files
and other devices with the same statements (`PRINT #`, `INPUT #`, ...)
is covered in [Files and Devices](11-files-and-devices.md).

## The screen and the keyboard

When an exBASIC program starts, the screen is in text mode (graphics
mode 0): 24 rows of 40 characters. The Atari operating system (OS)
manages it through the screen editor device, `E:`. The editor is always
open on I/O channel 0, and all the statements in this chapter use
channel 0 unless you give another channel with `#`.

A few things about the screen editor matter to programs:

- **Margins.** The left margin is column 2 and the right margin is
  column 39 (OS locations 82 and 83, `LMARGN` and `RMARGN`). Text
  starts at column 2, not at column 0, unless you change the margin
  with `POKE 82, 0`.
- **Cursor.** The cursor position is kept in `ROWCRS` (location 84)
  and `COLCRS` (locations 85 and 86). [POSITION](#position) changes
  them. The cursor is shown as an inverse block; `POKE 752, 1` hides
  it and `POKE 752, 0` shows it again (the change is seen the next
  time the cursor moves).
- **Logical lines.** The editor joins up to three screen lines into one
  logical line of up to 120 characters. `INPUT` from the keyboard reads
  one logical line.
- **Control characters.** Some character codes are commands to the
  editor instead of visible characters: 125 clears the screen, 155 ends
  the line, 28 to 31 move the cursor, and so on. See
  [Control characters](#control-characters).
- **Scrolling.** Printing past the last row scrolls the screen up.

The keyboard is read by the OS keyboard handler (`K:`). It converts key
presses to ATASCII codes, handles the CAPS and inverse video keys, and
makes the key click. [GET](#get) reads one key; [INPUT](#input) uses the
screen editor, so the user can type and edit a whole line before
pressing RETURN.

Pressing BREAK stops the program with error 128, `ERROR- 128` (see
[Runtime errors](B-error-messages.md#runtime-errors)). An input or output
operation that is waiting, like an `INPUT` from the keyboard, ends with
error 128 instead (see [ERR()](11-files-and-devices.md#err)).

## PRINT

Writes numbers and strings to the screen or to another device.

```
PRINT [#channel,] [item] [separator item ...] [separator]
? [#channel,] [item] [separator item ...] [separator]
```
Abbreviation: `PRI.` or `?`

Each _item_ is a numeric expression (integer or floating point) or a
string expression. A _separator_ is one of `,` `;` `TAB(n)` `RTAB(n)`
and `COLOR(n)`, described below.

- Integers are printed in decimal, with a minus sign when negative and
  no leading or trailing spaces: `PRINT 5` prints `5`.
- Comparisons and other boolean expressions print as `1` (true) or `0`
  (false).
- Floating point values use the format of
  [STR$](09-floating-point.md), for example `1.5`, `0.5`, `1.5E+10`.
- Strings are printed as they are, character by character.

After the last item, `PRINT` writes an end of line (ATASCII 155), so the
next output starts on a new line. If the statement ends with a
separator (`,`, `;`, `TAB(n)` or `COLOR(n)`), no end of line is written
and the next `PRINT` continues on the same line. `PRINT` with no items
prints only an end of line, giving an empty line.

```basic
Apples = 12
Pears = -7
PRINT "APPLES="; Apples; " PEARS="; Pears
PRINT "TOTAL", Apples + Pears
PRINT "TRUE IS "; 3 > 2
PRINT "NO NEWLINE ";
PRINT "HERE"
PRINT
PRINT "AFTER EMPTY LINE"
```

Output:

```
APPLES=12 PEARS=-7
TOTAL   5
TRUE IS 1
NO NEWLINE HERE

AFTER EMPTY LINE
```

(The output above, and in the rest of this chapter, is shown from the
left margin. On the screen it starts at column 2. The comma moved the
`5` to screen column 10; see [The comma](#the-comma).)

### Separators

| Separator | Effect |
| --------- | ------ |
| `;` | Nothing; the next item is printed right after the previous one. |
| `,` | Prints spaces up to the next column that is a multiple of 10. Same as `TAB(10)`. |
| `TAB(n)` | Prints spaces up to the next column that is a multiple of _n_. |
| `RTAB(n)` | Prints spaces so that the next item ends just before a column that is a multiple of _n_. |
| `COLOR(n)` | Changes the color (inverse video in text mode) of the rest of the statement. |

The separator can also be left out: two items written one after the
other are printed together, as if there was a `;` between them. Use this
with care, because a sign is read as part of an expression:

```basic
Count = 5
PRINT "COUNT=" Count " DOUBLE=" Count * 2
PRINT 10 -3
PRINT 10; -3
```

Output:

```
COUNT=5 DOUBLE=10
7
10-3
```

The second line printed `7` because `10 -3` is one expression, 10 minus
3. In normal programs, write the `;`.

### The comma

A comma moves the cursor to the next column that is a multiple of 10,
printing at least one space. It is the same as `TAB(10)`. Use it for
simple tables:

```basic
PRINT "NAME", "SCORE", "LEVEL"
PRINT "ANN", 1200, 3
PRINT "BOB", 75, 12
```

Output:

```
NAME    SCORE     LEVEL
ANN     1200      3
BOB     75        12
```

The tab stops are at the screen columns 10, 20, 30 and so on, measured
from the left edge of the screen, not from the margin. Because text
starts at column 2, the first column of the table above is 8 characters
wide and the others are 10. (Atari BASIC counts from the start of the
text, so there the first column is 10 characters wide too.)

The column is read from the OS cursor position (`COLCRS`). This makes
commas and `TAB` work after a [POSITION](#position), and when printing
to a graphics screen on channel #6. When printing to a file, a printer
or another device, the OS cursor does not move, so the number of spaces
depends on where the screen cursor is. Do not use `,`, `TAB` or `RTAB`
when printing to devices other than the screen.

### TAB

Advances to the next column that is a multiple of _n_.

```
PRINT ... TAB(n) ...
```
Abbreviation: `T.`

Prints spaces until the cursor column is a multiple of _n_. At least one
space is always printed, so if the cursor is already at a multiple of
_n_, it moves _n_ columns. `TAB(10)` is the same as a comma. As with the
comma, columns are counted from the left edge of the screen.

_n_ must be from 1 to 255; only its low 8 bits are used. `TAB(0)`
never ends: the program hangs. The same limits apply to `RTAB`.

```basic
' Price list aligned with TAB
PRINT "ITEM" TAB(16) "PRICE"
FOR Part = 1 TO 4
  PRINT "PART "; Part; TAB(16); "$"; Part * 37
NEXT Part
```

Output:

```
ITEM          PRICE
PART 1        $37
PART 2        $74
PART 3        $111
PART 4        $148
```

### RTAB

Right aligns the next item.

```
PRINT ... RTAB(n) item ...
```
Abbreviation: `RT.`

Prints spaces so that the item that follows ends just before the next
column that is a multiple of _n_. At least one space is printed. `RTAB`
must be followed directly by the item to align (a number or a string);
a separator can follow that item. Use it to align numbers in a column:

```basic
' Right-aligned numbers with RTAB
FOR I = 0 TO 10
  Value = I * (9 - 2 * I) * 134
  PRINT "Val:"; I; RTAB(16) Value
NEXT I
```

Output:

```
Val:0        0
Val:1      938
Val:2     1340
Val:3     1206
Val:4      536
Val:5     -670
Val:6    -2412
Val:7    -4690
Val:8    -7504
Val:9   -10854
Val:10  -14740
```

The numbers end just before screen column 16, so their last digit is
in column 15 (column 13 in the output above, which starts at the left
margin).

> **Note:** print a variable, a constant or a simple expression after
> `RTAB`. If the item uses `*`, `/`, `MOD` or a string slice
> `[ ]`, the computation overwrites the `RTAB` width and the item is
> not aligned correctly. Compute the value into a variable first, as
> the example above does with `Value`.

### COLOR

Changes the color of the text printed by the rest of the statement.

```
PRINT ... COLOR(n) ...
```
Abbreviation: `C.`

Every character printed after `COLOR(n)`, up to the end of the `PRINT`
statement, is combined with _n_ using an exclusive or (`EXOR`) before
it is written. A new `COLOR(n)` in the same statement replaces the
previous one, and the next statement starts with no color again. The
effect depends on the screen mode:

| Mode | _n_ | Result |
| ---- | --- | ------ |
| Text mode 0 | 0 | normal characters |
| Text mode 0 | 128 | inverse video |
| Modes 1 and 2 (`PRINT #6`) | 0, 32, 128, 160 | the four text colors |

In graphics modes 1 and 2, printing with `COLOR(32)` changes uppercase
letters to lowercase codes, which these modes show as the same letter
in a different color. See [Graphics](12-graphics.md).

```basic
PRINT "NORMAL "; COLOR(128) "INVERSE "; "STILL INVERSE"
PRINT "BACK TO NORMAL"
```

```basic
GRAPHICS 2
Score = 1234
PRINT #6, "SCORE: "; COLOR(32) Score
```

The `COLOR` separator in `PRINT` is not the same as the
[COLOR](12-graphics.md#color) statement, which sets the color used by
`PLOT` and `DRAWTO`.

### PRINT to other channels

`PRINT #channel, ...` sends the output to an open I/O channel instead
of the screen. After the channel number write a comma, or a semicolon as
in Atari BASIC (`PRINT #1; "TEXT"`). The channel is used for the whole
statement, then output returns to channel 0. `PRINT #6, ...` writes to the graphics screen opened by
[GRAPHICS](12-graphics.md#graphics); see
[PRINT #](11-files-and-devices.md#print-) for files.

`PRINT #6,` with nothing after the comma prints only an end of line.

### Errors

`PRINT` never stops the program. Printing on the screen (channel 0)
does not change [ERR()](11-files-and-devices.md#err), so a `PRINT` can
show the error of an earlier statement:

```basic
Number = VAL("ABC")
PRINT "ERR = "; ERR()
```

Output:

```
ERR = 18
```

`PRINT #` to another channel sets `ERR()` for each character written: 1
when it was written, or an error code. Save the value in a variable when
you need it later.

A [FUNC](07-functions.md) called from a `PRINT` item must only compute
its result. It must not print, and must not use another I/O channel:
the channel and the color belong to the `PRINT` statement in progress.

## PUT

Writes one character.

```
PUT [#channel,] code
```
Abbreviation: `PU.`

Writes the character with ATASCII code _code_ (0 to 255; only the low
8 bits are used) to the screen, or to the given channel. No end of line
is added. Control characters work as commands, so `PUT 125` clears the
screen and `PUT 155` starts a new line. With a channel other than 0 it
sets [ERR()](11-files-and-devices.md#err).

```basic
' Print ABCDEF and "!" with PUT
FOR Code = 65 TO 70
  PUT Code
NEXT Code
PUT 155       ' end of line
PUT #0, 33    ' "!" on channel 0, the screen
PUT 155
```

Output:

```
ABCDEF
!
```

`PUT` is faster and smaller than `PRINT CHR$(code);`.

## INPUT

Reads a line typed by the user into one or more variables.

```
INPUT var [, var ...]
INPUT "prompt"; var [, var ...]
INPUT "prompt", var [, var ...]
INPUT "prompt" TAB(n) var [, var ...]
INPUT ; var [, var ...]
INPUT #channel, var [, var ...]
```
Abbreviation: `IN.`

Before reading, `INPUT` prints a prompt:

| Form | Printed before the input |
| ---- | ------------------------ |
| `INPUT var` | `?` |
| `INPUT "prompt"; var` | the prompt |
| `INPUT "prompt", var` | the prompt, then spaces to the next column that is a multiple of 10 (like a comma in `PRINT`) |
| `INPUT "prompt" TAB(n) var` | the prompt, then spaces to the next multiple of _n_ |
| `INPUT ; var` | nothing |
| `INPUT #channel, var` | nothing (reads from the channel, see [INPUT #](11-files-and-devices.md#input-)) |

The prompt must be a constant string. No `?` is added after a prompt.

Then `INPUT` waits until the user types a line and presses RETURN. The
user can edit the line with the normal screen editor keys before
pressing RETURN. The line is stored in the variable:

- **String variable** (`A$`, or a string array element `A$(3)`): the
  whole line, without the end of line character. The line can be up to
  255 characters long (from the keyboard it is limited to one logical
  line of the editor).
- **Integer variable** (`A`, a word array element `A(3)` or a byte
  array element): the line is converted as by [VAL](08-strings.md):
  spaces at the start are skipped, then an optional `+` or `-` and the
  digits are read. Reading stops at the first character that is not a
  digit, so `-17XYZ` reads -17. Values from 32768 to 65535 are stored
  as negative numbers (65535 reads as -1). If there are no digits, or
  the number is larger than 65535, `ERR()` is set to 18 and the value
  stored is not meaningful (often 0, or the digits read before the
  error).
- **Floating point variable** (`A%`, or an element of a floating point
  array): the line is converted to a floating point number. `ERR()`
  is set to 18 if it is not a valid number.

With more than one variable, each variable reads a **separate line**.
The prompt is printed only once, before the first line. A comma typed in
the line does not separate values: typing `10,20` for
`INPUT A, B` stores 10 in `A` and then waits for another line for `B`.
This is different from Atari BASIC.

`INPUT` sets [ERR()](11-files-and-devices.md#err):

| `ERR()` | Meaning |
| ------- | ------- |
| 1 | A line was read. |
| 18 | The line could not be converted to a number. |
| 128 | The user pressed BREAK. |
| 136 | The user pressed CONTROL-3 (end of file). |

Read `ERR()` right after `INPUT` (see [Errors](#errors)):

```basic
' Ask for a name and a number
INPUT "YOUR NAME"; Name$
INPUT "YOUR AGE"; Age
InputError = ERR()
IF InputError = 18
  PRINT "That is not a number"
ELSE
  PRINT "Hello "; Name$; ", next year you will be "; Age + 1
ENDIF
```

The line is read into the OS line buffer at address $580 (1408). A line
longer than 128 characters (possible only from files) also uses the
first half of page 6 ($600 to $67F).

## GET

Waits for a key and returns its code.

```
GET var
GET #channel, var [, var ...]
```
Abbreviation: `GE.`

`GET var` waits until a key is pressed and stores its ATASCII code in
_var_. The key is not shown on the screen and RETURN is not needed.
_var_ is an integer variable or an element of a word or byte array; it
cannot be a string or floating point variable.

The key is read from the OS keyboard handler directly, without using an
I/O channel. The handler applies SHIFT, CONTROL, CAPS and the inverse
video key, so pressing A gives 65 or 97 depending on CAPS, RETURN gives
155 and ESC gives 27. The CAPS and inverse video keys only change the
mode and are not returned. The console keys (START, SELECT, OPTION),
SHIFT and CONTROL alone, HELP, BREAK and RESET are not keys for `GET`;
see [Other keys](#other-keys).

`ERR()` is 1 after a key, or 128 if BREAK was pressed.

```basic
' A simple menu using GET
DO
  CLS
  PRINT "1) NEW GAME"
  PRINT "2) OPTIONS"
  PRINT "3) QUIT"
  GET Choice
  IF Choice = ASC("1")
    PRINT "Starting..."
    PAUSE 30
  ELIF Choice = ASC("2")
    PRINT "No options yet"
    PAUSE 30
  ELIF Choice = ASC("3")
    EXIT
  ENDIF
LOOP
```

`GET` waits for a key. To check for a key without waiting, test
[KEY()](#key) first.

`GET #channel, var, ...` reads one byte from an I/O channel for each
variable; see [GET #](11-files-and-devices.md#get-). Note that
`GET #0, K` reads from the screen editor, which waits for a whole line
and RETURN; use `GET K` to read single keys.

## KEY

Tells if a key was pressed, without waiting.

```
KEY()
```
Abbreviation: `K.`

Returns 0 when no key is waiting, or a nonzero value when a key was
pressed and has not been read yet. The value is the internal keyboard
code of the key (OS location `CH`, 764) exclusive-or 255, so it is the
same as `PEEK(764) EXOR 255`. To get the internal key code itself, use
`KEY() EXOR 255` or `PEEK(764)`.

The key stays waiting, and `KEY()` stays nonzero, until it is read by
`GET` (or `INPUT`), or until the program clears it with `POKE 764, 255`.

```basic
' Wait for a key without stopping the program
LastKey = 764              ' OS location CH
PRINT "Press any key"
POKE LastKey, 255          ' forget any key pressed before
REPEAT
  ' ... other work here ...
  PAUSE 0
UNTIL KEY()
GET Key
PRINT "You pressed "; CHR$(Key); " (ATASCII "; Key; ")"
```

Internal key codes are not ATASCII codes. Some common ones:

| Key | Internal code (`PEEK(764)`) | `KEY()` |
| --- | --------------------------- | ------- |
| A | 63 | 192 |
| SPACE | 33 | 222 |
| RETURN | 12 | 243 |
| ESC | 28 | 227 |

SHIFT adds 64 to the internal code and CONTROL adds 128. The cursor
keys are CONTROL with `-` (up, 142), `=` (down, 143), `+` (left, 134)
and `*` (right, 135).

## POSITION

Moves the cursor.

```
POSITION column, row
```
Abbreviation: `POS.`

Sets the OS cursor to _column_ and _row_ (both counted from 0), so the
next `PRINT` or `PUT` writes there. In text mode 0 the columns are 0 to
39 and the rows 0 to 23. Remember that the left margin is column 2.

`POSITION` only stores the values; they are checked when the next
character is written. If they are out of range, that output fails with
error 141 (cursor out of range).

In graphics modes, `POSITION` sets the cursor of the graphics screen
(channel #6), used by `PRINT #6`, `PLOT`, `DRAWTO` and `LOCATE`. It does
not move the cursor of the text window at the bottom of split screen
modes; that cursor is in locations 656 (row) and 657 (column). See
[Graphics](12-graphics.md).

```basic
' Draw a box with POSITION and PUT
CursorInhibit = 752        ' OS location CRSINH
HorizontalLine = 18        ' ATASCII line characters
VerticalLine = 124

CLS
POKE CursorInhibit, 1      ' hide the cursor
FOR X = 10 TO 30
  POSITION X, 5
  PUT HorizontalLine
  POSITION X, 15
  PUT HorizontalLine
NEXT X
FOR Y = 6 TO 14
  POSITION 10, Y
  PUT VerticalLine
  POSITION 30, Y
  PUT VerticalLine
NEXT Y
POSITION 15, 10
PRINT "HELLO, ATARI"

POSITION 2, 22
POKE CursorInhibit, 0      ' show the cursor again
```

(Character 18 is a horizontal line and 124 a vertical bar.)

## CLS

Clears the screen.

```
CLS
CLS #channel
```
Abbreviation: none

`CLS` writes character 125 (clear screen) to channel 0, so it is the
same as `PUT 125`. The screen is cleared and the cursor goes to the top
left corner (row 0, left margin).

`CLS #channel` writes character 125 to the given channel. `CLS #6`
clears the graphics screen opened by `GRAPHICS`.

```basic
CLS
PRINT "A FRESH SCREEN"
```

## Other keys

These keys are not read with `GET` or `KEY()`. Read the hardware
registers with [PEEK](16-memory-and-machine-code.md):

| Keys | How to read | Value |
| ---- | ----------- | ----- |
| START, SELECT, OPTION | `PEEK(53279)` (`CONSOL`) | bit 0 START, bit 1 SELECT, bit 2 OPTION; a bit is 0 while the key is pressed (7 = none) |
| SHIFT | `PEEK(53775)` (`SKSTAT`) | bit 3 is 0 while SHIFT is pressed |
| HELP (XL/XE) | `PEEK(53769)` (`KBCODE`) | 17 while HELP is pressed |

```basic
' Wait for START, SELECT or OPTION
Consol = 53279             ' console keys register
PRINT "Press START, SELECT or OPTION"
REPEAT
  Keys = PEEK(Consol) & 7
UNTIL Keys <> 7            ' 7 = no key pressed

IF Keys & 1 = 0
  PRINT "START"
ENDIF
IF Keys & 2 = 0
  PRINT "SELECT"
ENDIF
IF Keys & 4 = 0
  PRINT "OPTION"
ENDIF
```

Other useful OS locations for keyboard and screen:

| Location | Name | Use |
| -------- | ---- | --- |
| 82 | `LMARGN` | Left margin (default 2) |
| 83 | `RMARGN` | Right margin (default 39) |
| 84 | `ROWCRS` | Cursor row |
| 85, 86 | `COLCRS` | Cursor column |
| 88, 89 | `SAVMSC` | Address of screen memory: `DPEEK(88)` |
| 656 | `TXTROW` | Text window cursor row |
| 657, 658 | `TXTCOL` | Text window cursor column |
| 694 | `INVFLG` | 128 when the inverse video key is on |
| 702 | `SHFLOK` | 0 lowercase, 64 CAPS, 128 CONTROL lock |
| 752 | `CRSINH` | 1 hides the cursor |
| 764 | `CH` | Last key pressed (internal code), 255 for none |

## ATASCII

The Atari uses its own character set, ATASCII (ATari ASCII). Codes 32
to 122 are the same as ASCII: space, punctuation, digits, uppercase and
lowercase letters. The other codes differ:

| Codes | Characters |
| ----- | ---------- |
| 0 to 31 | Graphics characters (typed with CONTROL and a letter or `,` `.` `;`) |
| 32 to 95 | Space, punctuation, digits, `@`, uppercase letters, `[ \ ] ^ _` |
| 96 | Diamond (graphics character) |
| 97 to 122 | Lowercase letters |
| 123 | Spade (graphics character) |
| 124 | `\|` vertical bar |
| 125 to 127 | Clear screen, backspace and tab (see below) |
| 128 to 255 | Inverse video versions of codes 0 to 127 |

There are no ASCII carriage return (13) or line feed (10) line endings:
the end of a line is ATASCII 155 (`$9B`), called EOL. Text files on
Atari disks use EOL too.

### Inverse video

Characters with codes 128 to 255 show as the inverse of codes 0 to
127 (light on dark becomes dark on light). On the keyboard, the inverse
video key (the Atari logo key, or the inverse key on XL/XE models)
switches typing of inverse characters on and off.

To print inverse text, use `COLOR(128)` in `PRINT`, add 128 to the code
in `PUT`, or write the codes in a string constant. A string constant
can be followed by character codes in hexadecimal (see
[Strings](08-strings.md)):

```basic
' Inverse characters in a string constant: codes 128 + ATASCII
PRINT "PRESS "$D3$D4$C1$D2$D4" TO PLAY"
```

### Control characters

When written to the screen (channel 0, or channel 6 in text modes),
these codes are commands to the screen editor:

| Code | Hex | Key | Action |
| ---- | --- | --- | ------ |
| 27 | $1B | ESC | Print the next character as a symbol, not as a command |
| 28 | $1C | CONTROL - | Cursor up |
| 29 | $1D | CONTROL = | Cursor down |
| 30 | $1E | CONTROL + | Cursor left |
| 31 | $1F | CONTROL * | Cursor right |
| 125 | $7D | SHIFT CLEAR | Clear the screen |
| 126 | $7E | BACK S | Backspace (delete the character to the left) |
| 127 | $7F | TAB | Move to the next tab stop |
| 155 | $9B | RETURN | End of line (EOL) |
| 156 | $9C | SHIFT BACK S | Delete line |
| 157 | $9D | SHIFT INSERT | Insert line |
| 158 | $9E | CONTROL TAB | Clear tab stop |
| 159 | $9F | SHIFT TAB | Set tab stop |
| 253 | $FD | CONTROL 2 | Sound the buzzer |
| 254 | $FE | CONTROL BACK S | Delete character |
| 255 | $FF | CONTROL INSERT | Insert character |

To show the symbol of a control character instead of running the
command, print an ESC (27) before it:

```basic
' Show control characters as symbols, then sound the buzzer
Esc = 27
PRINT CHR$(Esc); CHR$(28); CHR$(Esc); CHR$(29)   ' up and down arrows
PRINT CHR$(Esc); CHR$(125)                       ' clear screen symbol
PUT 253                                          ' buzzer
```

### Screen codes

Screen memory does not store ATASCII codes but "internal" codes, in a
different order. This matters only when you `POKE` characters directly
into screen memory (at `DPEEK(88)`):

| ATASCII | Internal code |
| ------- | ------------- |
| 0 to 31 | 64 to 95 |
| 32 to 95 | 0 to 63 |
| 96 to 127 | 96 to 127 (unchanged) |

Inverse characters (128 to 255) follow the same rule, plus 128. For
example, the letter A is ATASCII 65 and internal code 33:

```basic
ScreenMemory = DPEEK(88)        ' SAVMSC: start of screen memory
POKE ScreenMemory + 2, 33       ' internal code 33 is "A"
```

puts an A at row 0, column 2.
