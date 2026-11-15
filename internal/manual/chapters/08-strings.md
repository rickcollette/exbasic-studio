# 8. Strings

A string is a sequence of up to 255 characters. This chapter describes
string constants, string variables and arrays, assignment and appending,
joining strings with `+`, taking parts of a string with `[ ]` or with
`LEFT$`, `RIGHT$` and `MID$`, searching with `INSTR`, changing the case
and removing spaces, comparing strings, the other string functions, and
how much memory strings use.

Characters are ATASCII codes from 0 to 255 (ATASCII is the Atari version
of ASCII; see [chapter 10](10-console-io.md)). Any code, including the end
of line character 155 (`$9B`), can be stored in a string.

## String constants

A string constant is text between double quotes:

```basic
PRINT "HELLO, WORLD"
```

Output:

```
HELLO, WORLD
```

Inside a constant:

- Two double quotes together (`""`) stand for one double quote character.
- `""` alone is the empty string.
- Any character can be added by its hexadecimal code: close the quotes
  and write `$` and two hex digits right after them, with no spaces. Several
  codes can follow each other, and a new quote continues the text.

```basic
' Quotes and character codes in constants
PRINT "SAY ""HELLO"""
PRINT "LINE 1"$9B"LINE 2"
PRINT "A"$42$43"D"
PRINT LEN("")
```

Output:

```
SAY "HELLO"
LINE 1
LINE 2
ABCD
0
```

The `$9B` code is the only way to put an end of line inside a constant.
The characters of the source file are copied as they are, so a constant
must not be broken across lines of the source.

A constant can have at most 255 characters. A longer constant gives the
compile error `string too long (max 255 characters)`. A missing closing
quote gives `unterminated string, missing '"'`, and a `$` after the quote
that is not followed by two hexadecimal digits gives
`bad character code in string, use $00 to $FF`.

## String variables

The name of a string variable ends with `$`: `Name$`, `Line1$`,
`Player_Name$`. A string variable can hold from 0 to 255 characters.

- A string variable that was never assigned is empty.
- The `$` is part of the name: `Count` and `Count$` are two different
  variables.
- Each string variable takes 2 bytes in the variable table. The first
  assignment also reserves 256 bytes of memory for its text (one length
  byte and 255 characters); the memory is kept until the program ends or
  executes `CLR`.

### Assignment

Copies a string into a string variable.

```
var$ = string
```

The string expression on the right is copied into the variable. The
variable can be a simple string variable or an element of a string array.

```basic
Greeting$ = "HELLO"
Copy$ = Greeting$
Greeting$ = "BYE"
PRINT Copy$; " "; Greeting$
```

Output:

```
HELLO BYE
```

### Appending with =+

Adds a string to the end of a string variable.

```
var$ =+ string
```

The string on the right is copied after the current text of the variable.
If the result would be longer than 255 characters, it is cut at 255
characters.

```basic
' Builds a line of text piece by piece
Message$ = "SCORE:"
Message$ =+ " "
Message$ =+ STR$(1200)
PRINT Message$
```

Output:

```
SCORE: 1200
```

## Concatenation with +

The `+` operator joins strings in any string expression:

```
string + string [+ string ...]
```

The terms can be constants, variables, array elements, string functions,
slices and function calls. The result is cut at 255 characters. Numbers
are not converted automatically: use `STR$` (adding a number to a string
is a compile error).

```basic
' Joining strings with +
Name$ = "ADA"
Level = 3
Title$ = "PLAYER " + Name$ + ", LEVEL " + STR$(Level) + "!"
PRINT Title$
PRINT "[" + Name$ + "]"
IF Name$ + "X" = "ADAX" THEN PRINT "COMPARE OK"
```

Output:

```
PLAYER ADA, LEVEL 3!
[ADA]
COMPARE OK
```

**Hidden strings.** The result of a `+` expression is built in a hidden
string variable. Each `+` expression in a statement uses its own hidden
string (so `IF A$ + B$ = C$ + D$` works), and the hidden strings are
reused by the next statements. The main code and each function have their
own hidden strings. Each hidden string uses a variable slot and 256 bytes
of memory, reserved the first time it is used.

**Appending in place.** When a variable is extended with itself at the
start, as in `Text$ = Text$ + A$ + B$`, the compiler appends in place,
like `Text$ =+ A$` followed by `Text$ =+ B$`. This makes building a
string in a loop fast and does not use a hidden string.

**In recursive functions** a `+` expression can call the function it is
in. See [Recursion](07-functions.md#recursion).

## Slicing strings

Square brackets after a string take a part of it (a substring).

```
string[start]
string[start, length]
```

- _start_ is the position of the first character to take. The first
  character of the string is at position 1.
- With one number, the slice goes from _start_ to the end of the string.
- With two numbers, the slice has at most _length_ characters; it is
  shorter if the string ends first.

Brackets can follow any string: a variable, an array element, a constant,
a string function, a function call, a `+` expression in parentheses, or
another slice.

```basic
' Slices of a string
Text$ = "HELLO WORLD"
PRINT Text$[7]
PRINT Text$[1, 5]
PRINT Text$[7, 100]
PRINT Text$[2][2][2, 3]
PRINT "ABCDEF"[3, 2]
PRINT STR$(1234)[2]
PRINT ("<" + Text$ + ">")[1, 3]
```

Output:

```
WORLD
HELLO
WORLD
LO 
CD
234
<HE
```

The exact rules for unusual values:

| Values | Result |
|--------|--------|
| _start_ from 1 to the length | the characters from _start_ |
| _start_ greater than the length | empty string |
| _start_ = 0, negative, or greater than 255 | empty string |
| _length_ = 0 or negative | empty string |
| _length_ greater than 255 | treated as 255 |

Brackets can't be used on the left of `=`: to change characters, use the
[MID$ statement](#mid-statement), as in `MID$(Text$, 3, 1) = "X"`.

**Temporary results.** A slice, like the result of `STR$`, is placed in a
temporary buffer (the system line buffer at address `$0580`) that is
reused by the next slice, `STR$` or `INPUT`. Use it right away or assign
it to a variable. In particular, two slices or two `STR$` results compare
each other's same buffer, so this comparison is always true:

```basic
' Two temporary results can't be compared directly
Text$ = "LONG STRING"
IF Text$[2, 1] = Text$[3, 1] THEN PRINT "WRONG: ALWAYS EQUAL"

' Assign one of them to a variable first
Second$ = Text$[2, 1]
IF Second$ <> Text$[3, 1] THEN PRINT "CORRECT: DIFFERENT"
```

Output:

```
WRONG: ALWAYS EQUAL
CORRECT: DIFFERENT
```

The same happens with two `CHR$` results. A `+` expression is built in
its own hidden string, so `IF "A" + "" = "B" + ""` compares correctly. The
functions `LEFT$`, `RIGHT$`, `MID$`, `UCASE$`, `LCASE$`, `LTRIM$` and
`RTRIM$` also build their result in a hidden string, so
`IF LEFT$(A$, 1) = RIGHT$(A$, 1)` compares correctly too.

## Comparing strings

Strings are compared with the operators `=`, `<>`, `<`, `>`, `<=` and
`>=`. The result is 1 (true) or 0 (false).

Two strings are compared character by character, by ATASCII code, from
the first character. The first different character decides. If one string
is the start of the other, the shorter one is smaller. The comparison is
case sensitive: all uppercase letters (65 to 90) come before the
lowercase letters (97 to 122).

```basic
' String comparison
IF "ABC" < "ABD" THEN PRINT "ABC < ABD"
IF "AB" < "ABC" THEN PRINT "AB < ABC"
IF "a" > "Z" THEN PRINT "a > Z"
IF "" < "A" THEN PRINT "EMPTY < A"
IsEqual = "ATARI" = "ATARI"
PRINT IsEqual
```

Output:

```
ABC < ABD
AB < ABC
a > Z
EMPTY < A
1
```

A string can only be compared with another string. A string alone is not
a condition: write `IF Name$ <> ""` or `IF LEN(Name$)`.

## String functions

### LEN

Returns the length of a string.

```
LEN(string)
```
Abbreviation: `L.`

Returns the number of characters, from 0 to 255.

```basic
Name$ = "ATARI"
PRINT LEN(Name$); " "; LEN(""); " "; LEN(Name$ + "XL")
```

Output:

```
5 0 7
```

### ASC

Returns the ATASCII code of the first character of a string.

```
ASC(string)
```
Abbreviation: `AS.`

The result is from 0 to 255. For an empty string the result is 0.

```basic
PRINT ASC("A"); " "; ASC("ABC"); " "; ASC(CHR$(155))
```

Output:

```
65 65 155
```

### CHR$

Returns a one-character string with the given ATASCII code.

```
CHR$(code)
```
(no abbreviation)

Only the low byte of _code_ is used, so `CHR$(321)` is the same as
`CHR$(65)`. The result is kept in a small buffer that the next `CHR$`
overwrites (see [Slicing strings](#slicing-strings)).

```basic
' Prints the letters A to E
FOR Code = 65 TO 69
  PRINT CHR$(Code);
NEXT Code
PRINT
```

Output:

```
ABCDE
```

### STR$

Returns the text of a number.

```
STR$(number)
```
(no abbreviation)

For an integer, the result has the digits of the number, with a `-` sign
for negative numbers and no spaces: `STR$(-32768)` is `"-32768"`. For a
floating point number, the result is the same text that `PRINT` shows
(see [chapter 9](09-floating-point.md#printing-format)).

The result is placed in the temporary buffer described in
[Slicing strings](#slicing-strings).

```basic
Lives = 3
Text$ = "LIVES:" + STR$(Lives)
PRINT Text$; " "; LEN(STR$(-123)); " "; STR$(2.5)
```

Output:

```
LIVES:3 4 2.5
```

### VAL

Converts the text of a number into a number.

```
VAL(string)
```
Abbreviation: `V.`

In an integer expression, `VAL` reads an integer:

- Leading spaces are skipped, then an optional `+` or `-` sign, then the
  digits. Reading stops at the first character that is not a digit, so
  `VAL("12AB")` is 12 and `VAL("1.9")` is 1.
- Values from 32768 to 65535 wrap around to negative numbers, like all
  integers.
- If there are no digits, or the number is larger than 65535, `VAL`
  returns the digits read so far (0 if none) and sets `ERR()` to 18. On
  success it sets `ERR()` to 1.

In a floating point expression, `VAL` reads a floating point number, like
`"1.5E3"` or `"-0.25"` (see [VAL in chapter 9](09-floating-point.md#val)).

```basic
' Converting text to integers
Value = VAL("  -45")
PRINT Value
Value = VAL("12AB")
PRINT Value
Value = VAL("ABC")
ErrorCode = ERR()
PRINT Value; " ERROR "; ErrorCode
```

Output:

```
-45
12
0 ERROR 18
```

Read `ERR()` into a variable right after `VAL`: other operations, like
reading a file, change the value of `ERR()` again.

### LEFT$

Returns the first characters of a string.

```
LEFT$(text$, count)
```
(no abbreviation)

Returns at most _count_ characters from the start of _text$_. If _count_
is 0 or negative the result is empty; if it is longer than the string,
the result is the whole string.

### RIGHT$

Returns the last characters of a string.

```
RIGHT$(text$, count)
```
(no abbreviation)

Returns at most _count_ characters from the end of _text$_, with the same
rules as `LEFT$`.

### MID$

Returns a part of a string.

```
MID$(text$, start [, count])
```
(no abbreviation)

Returns at most _count_ characters of _text$_, from position _start_. The
first character is at position 1. Without _count_, the result goes to the
end of the string. A _start_ of 0 or less is taken as 1; a _start_ after
the end of the string, or a _count_ of 0 or less, gives an empty string.
`MID$(Text$, Start, Count)` is the same as `Text$[Start, Count]`.

```basic
' Parts of a string
Text$ = "HELLO, WORLD"
PRINT LEFT$(Text$, 5); "|"; RIGHT$(Text$, 5); "|"; MID$(Text$, 8, 3); "|"
PRINT MID$(Text$, 8); "|"; LEFT$(Text$, 99); "|"; RIGHT$(Text$, 0); "|"
```

Output:

```
HELLO|WORLD|WOR|
WORLD|HELLO, WORLD||
```

### MID$ statement

Replaces characters inside a string variable.

```
MID$(var$, start [, count]) = text$
```
(no abbreviation)

Copies the characters of _text$_ over the characters of _var$_, starting
at position _start_. At most _count_ characters are copied (without
_count_, all of _text$_). The length of _var$_ never changes: characters
that would go past its end are dropped, and if _text$_ is shorter, only
its characters are replaced. A _start_ after the end of the string changes
nothing. _var$_ can be a string variable or an element of a string array.

```basic
' Change characters without changing the length
Text$ = "HELLO, WORLD"
MID$(Text$, 8, 5) = "ATARI!!"
PRINT Text$; " ("; LEN(Text$); ")"
MID$(Text$, 1) = "J"
PRINT Text$
```

Output:

```
HELLO, ATARI (12)
JELLO, ATARI
```

### INSTR

Searches a string inside another.

```
INSTR(text$, find$ [, start])
```
(no abbreviation)

Returns the position of the first place where _find$_ appears in _text$_,
searching from position _start_ (1 if not given). The result is 0 if
_find$_ is not found. If _find$_ is empty, the result is _start_. A
_start_ of 0 or less is taken as 1; a _start_ after the end of _text$_
gives 0. The search is case sensitive; use `UCASE$` on both strings to
ignore case.

```basic
' Find all the commas of a list
List$ = "RED,GREEN,BLUE"
Position = INSTR(List$, ",")
WHILE Position > 0
  PRINT "COMMA AT "; Position
  Position = INSTR(List$, ",", Position + 1)
WEND
PRINT INSTR(List$, "BLUE"); " "; INSTR(List$, "blue")
```

Output:

```
COMMA AT 4
COMMA AT 10
11 0
```

### UCASE$ and LCASE$

Change the letters of a string to uppercase or lowercase.

```
UCASE$(text$)
LCASE$(text$)
```
(no abbreviation)

`UCASE$` changes the letters `a` to `z` to `A` to `Z`, and `LCASE$` does
the opposite. Other characters are not changed.

### LTRIM$ and RTRIM$

Remove spaces from the start or the end of a string.

```
LTRIM$(text$)
RTRIM$(text$)
```
(no abbreviation)

`LTRIM$` removes the spaces at the start of _text$_, and `RTRIM$` the
spaces at the end. To remove both, use `LTRIM$(RTRIM$(Text$))`.

```basic
' Clean up a name typed with spaces and mixed case
Typed$ = "   ada Lovelace  "
Name$ = UCASE$(LTRIM$(RTRIM$(Typed$)))
PRINT "["; Name$; "] ["; LCASE$(Name$); "]"
PRINT "["; Typed$; "]"
```

Output:

```
[ADA LOVELACE] [ada lovelace]
[   ada Lovelace  ]
```

None of these functions changes its argument: `UCASE$(Name$)` returns a
new string and `Name$` keeps its text. Positions and counts out of range
never stop the program; they are limited to the string as described above.

### $( )

Returns the string stored at a memory address.

```
$(address)
```
(no abbreviation)

The byte at _address_ is the length, and the characters follow it. This
is the format of every exBASIC string in memory, and `$()` is the inverse
of `ADR()`. The string is not copied: `$()` is a reference to the memory.

```basic
' A string built from the bytes of a DATA array
' Length 5, then the ATASCII codes of H, E, L, L, O
DATA Message() BYTE = 5, $48, $45, $4C, $4C, $4F
PRINT $(ADR(Message))
```

Output:

```
HELLO
```

A string constant in a `BYTE` `DATA` array is stored with its length
byte, so it can be read back with `$()`:

```basic
DATA Words() BYTE = "FIRST", "SECOND"
WordsStart = ADR(Words)
PRINT $(WordsStart); " "; $(WordsStart + 6)
```

Output:

```
FIRST SECOND
```

### ADR

Returns the memory address of a string.

```
ADR(string)
&string
```
(no abbreviation)

For a string, `ADR` returns the address of the length byte; the
characters follow it. This differs from Atari BASIC and Turbo BASIC XL,
where the address is that of the first character. `&` is a short form of
`ADR`. `ADR` of a string variable that was never assigned is 0, and
address 0 always holds 0, so it reads as an empty string.

```basic
' Reading and changing a string in memory
Word$ = "HELLO"
WordAddress = ADR(Word$)
PRINT PEEK(WordAddress); " "; PEEK(WordAddress + 1)
POKE WordAddress + 1, ASC("J")   ' change the first character
PRINT Word$
```

Output:

```
5 72
JELLO
```

`ADR` of a string constant gives the address of the constant in the
program. Saving that address and using `$()` later uses less memory than
copying the text to a variable. See
[chapter 16](16-memory-and-machine-code.md) for `ADR` of arrays and
variables.

## String arrays

A string array holds many strings, each up to 255 characters:

```basic
' A small table of names
DIM Names$(3)
Names$(0) = "ZERO"
Names$(3) = "THREE"
Names$(1) = Names$(0) + "/" + Names$(3)
FOR Index = 0 TO 3
  PRINT Index; ": "; Names$(Index); " ("; LEN(Names$(Index)); ")"
NEXT Index
```

Output:

```
0: ZERO (4)
1: ZERO/THREE (10)
2:  (0)
3: THREE (5)
```

`DIM Names$(3)` creates 4 elements, numbered 0 to 3, all empty. Each
element uses 2 bytes, plus 256 bytes when it is assigned for the first
time. Elements can be used anywhere a string variable can: in
assignments, `=+`, slices, comparisons and function arguments. See
[chapter 4](04-variables-and-data.md) for `DIM`.

## Strings in functions

Functions can take string parameters (`FUNC Show(Text$)`), return a
string (`FUNC Upper$(Text$)`) and have string results and `LOCAL`
strings. A string argument is copied into the parameter. See
[chapter 7](07-functions.md), and the notes on strings in
[recursive functions](07-functions.md#recursion).

## Limits and memory

| Item | Limit or size |
|------|---------------|
| Length of a string | 0 to 255 characters |
| String constant | up to 255 characters |
| String variable | 2 bytes, plus 256 bytes from the first assignment |
| String array of N elements | 2 × (N + 1) bytes, plus 256 bytes for each assigned element |
| Hidden string of `+` | 1 variable slot and 256 bytes, one per `+` expression in a statement, in the main code and in each function |
| Result of a slice, `STR$` | temporary, in the buffer at `$0580` |
| Result of `CHR$` | temporary, 2-byte buffer |
| Result of `LEFT$`, `RIGHT$`, `MID$`, `UCASE$`, `LCASE$`, `LTRIM$`, `RTRIM$` | a hidden string, like `+` |

The memory of strings is reserved from the free memory after the program,
like arrays, and it is never freed except by `CLR` (see
[chapter 17](17-program-organization.md#clr)). Use `FRE()` to see how
much memory is left (see [chapter 16](16-memory-and-machine-code.md)).

## Putting it together

```basic
' Centers words in a 20-character field and shows their initials
DIM Words$(2)
Words$(0) = "ATARI"
Words$(1) = "EXBASIC"
Words$(2) = "STRINGS"

Initials$ = ""
FOR Index = 0 TO 2
  PRINT "|"; Center$(Words$(Index), 20); "|"
  Initials$ =+ Words$(Index)[1, 1]
NEXT Index
PRINT "INITIALS: "; Initials$

' Returns Text$ centered in a field of Width characters
FUNC Center$(Text$, Width)
  LOCAL Left, Result$
  Left = (Width - LEN(Text$)) / 2
  Result$ = ""
  WHILE LEN(Result$) < Left
    Result$ =+ " "
  WEND
  Result$ =+ Text$
  WHILE LEN(Result$) < Width
    Result$ =+ " "
  WEND
  RETURN Result$
ENDFUNC
```

Output:

```
|       ATARI        |
|      EXBASIC       |
|      STRINGS       |
INITIALS: AES
```
