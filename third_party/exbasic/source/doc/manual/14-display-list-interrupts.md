# 14. Display List Interrupts

This chapter covers display list interrupts (DLIs): small routines that
run at chosen lines of the screen to change colors, player positions or
other hardware registers while the picture is drawn. It explains the
ANTIC display list, the `DLI SET`, `DLI name` and `DLI` statements, the
timing limits, and gives examples. This is an advanced topic: read
[chapter 12](12-graphics.md) first.

## The display list

The ANTIC chip draws the screen following a program in memory called the
*display list*. Each instruction is one byte (some are followed by an
address) and describes one horizontal band of the screen, from top to
bottom. The OS builds a display list for each graphics mode; its address
is in SDLSTL, `DPEEK(560)`.

| Instruction byte   | Meaning                                                           |
| ------------------ | ----------------------------------------------------------------- |
| $00, $10, ... $70  | 1 to 8 blank scan lines (`$70` is 8 lines)                        |
| $01                | Jump to the address in the next two bytes                         |
| $41                | Jump to the address and wait for the vertical blank (end of list) |
| $02 to $0F         | One *mode line* of ANTIC mode 2 to 15 (see the table)             |
| + $40 (LMS)        | Load Memory Scan: the next two bytes are the screen memory address |
| + $20              | Vertical fine scrolling for this line                             |
| + $10              | Horizontal fine scrolling for this line                           |
| + $80              | Display list interrupt at the end of this line                    |

A *mode line* is one row of text or one row of graphics pixels. It is
several *scan lines* (the horizontal lines of the TV picture) tall:

| ANTIC mode | GRAPHICS mode | Scan lines per mode line |
| ---------- | ------------- | ------------------------ |
| 2          | 0             | 8                        |
| 4          | 12            | 8                        |
| 5          | 13            | 16                       |
| 6          | 1             | 8                        |
| 7          | 2             | 16                       |
| 8          | 3             | 8                        |
| 9          | 4             | 4                        |
| 10 ($A)    | 5             | 4                        |
| 11 ($B)    | 6             | 2                        |
| 12 ($C)    | 14            | 1                        |
| 13 ($D)    | 7             | 2                        |
| 14 ($E)    | 15            | 1                        |
| 15 ($F)    | 8, 9, 10, 11  | 1                        |

The normal screen has 192 scan lines, after 24 blank lines at the top.

### The GRAPHICS 0 display list

| Offset from `DPEEK(560)` | Contents       | Meaning                                  |
| ------------------------ | -------------- | ---------------------------------------- |
| 0, 1, 2                  | $70 $70 $70    | 24 blank scan lines                      |
| 3                        | $42            | Text row 0, mode 2 with LMS              |
| 4, 5                     | address        | Screen memory address                    |
| 6 to 28                  | $02            | Text rows 1 to 23 (row `r` at `r + 5`)   |
| 29                       | $41            | Jump and wait for vertical blank         |
| 30, 31                   | address        | Address of the display list              |

So `POKE DPEEK(560) + 16, 130` changes text row 11 into mode 2 with the
DLI bit (2 + 128 = 130).

In other modes, the first mode line has LMS (bit 6), and modes with a
text window have a second LMS instruction ($42) for the text window. Modes
that use more than 4 KB have another LMS in the middle. This FUNC finds
the instruction of any mode line, skipping the address bytes:

```basic
' Find the display list instruction of a mode line
GRAPHICS 7
Line = DLLine(40)
IF Line
  POKE Line, PEEK(Line) ! 128      ' set the DLI bit
ENDIF
END

' Return the address of the display list instruction for
' mode line N (the first is 0), or 0 if there is none
FUNC DLLine(N)
  LOCAL Address, Code
  Address = DPEEK(560)
  DO
    Code = PEEK(Address)
    IF Code & 15 > 1
      ' A mode line: stop at line N
      IF N = 0 THEN EXIT
      DEC N
      ' With LMS (bit 6) two address bytes follow
      IF Code & 64 THEN Address = Address + 2
    ELIF Code & 15 = 1
      ' A jump: the end of the display list
      RETURN 0
    ENDIF
    INC Address
  LOOP
  RETURN Address
ENDFUNC
```

A `GRAPHICS` statement builds a new display list, so set the DLI bits
again after each `GRAPHICS`.

## What a DLI does

When ANTIC reaches a mode line with the DLI bit, it interrupts the 6502 at
the start of the *last* scan line of that mode line. The OS jumps to the
routine whose address is in VDSLST (512 and 513), if DLIs are enabled in
NMIEN ($D40E).

A DLI routine is machine code that must run in very little time: a scan
line lasts 114 CPU cycles, and ANTIC takes part of them to read the
screen. To make a change exactly at the start of a scan line, the routine
writes to WSYNC ($D40A). Any write to WSYNC stops the CPU until the end of
the current scan line, the *horizontal blank*. A register written just
after it changes from the start of the next scan line.

The changes last until something writes the register again. At each
vertical blank the OS copies the shadow color registers (704 to 712) to
the hardware, so a color changed by a DLI returns to its normal value at
the top of the next frame. The hardware registers without a shadow (P/M
positions and sizes) keep the value set by the last DLI.

exBASIC builds DLI routines from a simple description, so you don't have
to write machine code.

## DLI statements

### DLI SET
Defines a new DLI routine.

```
DLI SET name = op [op ...] [, op ...]
DLI = op [op ...] [, op ...]
```
Abbreviation: `DL. S.` (also `DLIS.`)

`DLI SET` creates a machine code routine called `name`. It does not
activate it; use [DLI name](#dli-name) for that. Each *op* writes one
value to one or more addresses:

```
value INTO address [INTO address ...]
value WSYNC [WSYNC ...] INTO address [INTO address ...]
```

- `value` is either a constant byte (0 to 255) or the name of a
  `DATA ... BYTE` array.
- `INTO address` writes the value to `address`, a constant from 0 to
  65535. Repeat `INTO` to write the same value to several addresses.
- `WSYNC` waits for the start of the next scan line before the following
  writes. Repeat it to wait more lines. It must be followed by `INTO` or
  another `WSYNC`.
- Separate the ops with commas. They run in order.

| Keyword | Abbreviation |
| ------- | ------------ |
| `SET`   | `S.`         |
| `INTO`  | `I.`         |
| `WSYNC` | `W.`         |

The first op always waits for the next scan line (an implicit `WSYNC`)
before its first write, so the first change starts at the first scan line
of the mode line after the one with the DLI bit. A `WSYNC` written in the
first op waits one more line.

When `value` is a `DATA` array, each DLI in a frame uses the next element:
the first DLI of the frame uses element 0, the second element 1, and so
on. All the arrays of one routine use the same element number. The array
needs at least as many elements as there are DLI lines on the screen.
Because the routine reads the array each time, the program can change the
values while the DLI runs.

The array must be a `DATA ... BYTE` array (see
[chapter 4](04-variables-and-data.md)), not one created with `DIM`. The
addresses and constant values must be numbers, not variables: write
`INTO $D01A`, not `INTO ColBk`. The address can also be an assembler
symbol written `@NAME` (see
[chapter 16](16-memory-and-machine-code.md#using-assembler-symbols-from-basic)).

A long definition can continue on the next lines. End the line with a
comma and start the next line with `DLI =`:

```basic
' A long DLI split over three lines
DATA Colors() BYTE = $24, $46, $68
DLI SET Split = Colors INTO $D01A,
DLI = $8A INTO $D018,
DLI = $00 WSYNC INTO $D018
```

Other examples of definitions:

```basic
DLI SET Red = $24 INTO $D01A          ' COLBK: background color
```

```basic
DATA Colors() BYTE = $24, $46, $68
DLI SET Bands = Colors INTO $D01A INTO $D018
```

```basic
' Two scan lines with different colors from one DLI
DLI SET Stripe = $0E INTO $D01A, $00 WSYNC INTO $D01A
```

The name of a DLI is a label, like the name of a `DATA` array: it must be
unique, and the DLI must be defined in the source before the `DLI name`
statement that uses it. The routine is stored with the program data; the
`DLI SET` line itself does nothing when it runs, so it can be anywhere
before its use.

Errors:

| Message                                    | Cause                                          |
| ------------------------------------------ | ---------------------------------------------- |
| `expected: DLI source data`                | Value over 255, a variable, or not a `DATA BYTE` array |
| `expected: 'INTO', 'WSYNC', DLI destination` | `WSYNC` not followed by `INTO`, or a variable after `INTO` |
| `expected: new label, got label already defined` | The name is already used                 |

### DLI name
Activates a DLI routine.

```
DLI name
```
Abbreviation: `DL.`

`DLI name` waits for the next vertical blank (like `PAUSE 0`), stores the
address of the routine `name` in VDSLST (512) and enables display list
interrupts by writing 192 to NMIEN ($D40E). Waiting for the vertical
blank makes the switch happen between two frames.

The DLI runs at each mode line with the DLI bit ($80) in the display list.
Set those bits with `POKE` before or after `DLI name`; without them
nothing happens.

`name` can be a routine made with `DLI SET`, or a `DATA ... BYTE` array
with your own machine code. A custom routine must save every register it
changes and end with `RTI` ($40). For example:

```basic
' A DLI written in machine code:
'   PHA / LDA #$24 / STA WSYNC / STA COLBK / PLA / RTI
DATA MyDli() BYTE = $48, $A9, $24, $8D, $0A, $D4,
DATA BYTE = $8D, $1A, $D0, $68, $40

GRAPHICS 0
POKE DPEEK(560) + 16, 130
DLI MyDli
PRINT "Press a key"
GET Key
DLI
```

Only one DLI routine is active at a time. Activating another one replaces
it.

### DLI
Turns off display list interrupts.

```
DLI
```
Abbreviation: `DL.`

`DLI` alone writes 64 to NMIEN ($D40E), which leaves only the vertical
blank interrupt enabled. The DLI bits in the display list stay, but no
routine runs. It does not wait for the vertical blank, so the current
frame may show the colors of the DLI in its upper part.

Turn off the DLIs before the program ends: after the program ends the
memory of the routine can be reused by other programs. `RUN "D:FILE"`
turns them off by itself.

## How the routine works

This is the machine code that `DLI SET` builds, with the cycles each
instruction takes:

```
        PHA                 ; 3  save A
        TXA                 ; 2
        PHA                 ; 3  save X
        LDX COLRSH          ; 3  X = number of DLIs since the vertical blank
        LDA #value          ; 2  (or LDA array,X: 4)
        STA WSYNC           ; 4  first op: wait for the next scan line
        STA address         ; 4  for each INTO
        ...                 ;    next ops: LDA, [STA WSYNC ...], STA address ...
        INC COLRSH          ; 5  next DLI uses the next element
        PLA                 ; 4
        TAX                 ; 2
        PLA                 ; 4
        RTI                 ; 6
```

COLRSH (address 79) is an OS location that the vertical blank sets to 0 on
each frame. The DLI uses it as its counter.

Limits:

- After a `WSYNC`, only a few writes fit in the horizontal blank before
  ANTIC starts drawing the next line: about 3 writes. Later writes happen
  while the line is drawn and the change shows partway across it. Put
  writes that must be exact first, and use `WSYNC` to spread the rest
  over the next lines.
- A DLI must finish before the next DLI line. With DLIs on consecutive
  mode lines of 1 or 2 scan lines, keep the routines very short.
- While a DLI waits on `WSYNC`, the main program is stopped. Many DLIs
  with many `WSYNC` make the program slower.
- In *attract mode* (the color cycling that starts after about 9 minutes
  without a key press), the OS sets COLRSH to a changing value instead of
  0, so array data is read from the wrong place. A program that uses DLIs
  with arrays should write 0 to ATRACT (77) from time to time, as the
  example below does.

## Useful registers

Registers often changed in a DLI. Write to the hardware addresses (the
shadow registers in RAM are only copied at the vertical blank).

| Address         | Name     | Contents                                    |
| --------------- | -------- | ------------------------------------------- |
| $D000 to $D003  | HPOSP0-3 | Player 0 to 3 horizontal position           |
| $D004 to $D007  | HPOSM0-3 | Missile 0 to 3 horizontal position          |
| $D008 to $D00B  | SIZEP0-3 | Player 0 to 3 width                         |
| $D012 to $D015  | COLPM0-3 | Color of player/missile 0 to 3              |
| $D016 to $D019  | COLPF0-3 | Color registers 0 to 3 (`SETCOLOR 0` to 3)  |
| $D01A           | COLBK    | Background and border color (`SETCOLOR 4`)  |
| $D01B           | PRIOR    | Priority and GTIA mode                      |
| $D404           | HSCROL   | Horizontal fine scroll                      |
| $D405           | VSCROL   | Vertical fine scroll                        |
| $D409           | CHBASE   | Page of the character set                   |
| $D401           | CHACTL   | Character options (inverse, upside down)    |

Other locations used by DLIs:

| Address   | Name    | Contents                                                      |
| --------- | ------- | ------------------------------------------------------------- |
| 77        | ATRACT  | Attract mode timer; write 0 to stop attract mode              |
| 79        | COLRSH  | Color shift; exBASIC DLIs use it as their counter             |
| 512, 513  | VDSLST  | Address of the DLI routine                                    |
| 560, 561  | SDLSTL  | Address of the display list                                   |
| $D40A     | WSYNC   | Any write waits for the end of the scan line                  |
| $D40B     | VCOUNT  | Current scan line divided by 2 (read)                         |
| $D40E     | NMIEN   | 64: vertical blank interrupt only; 192: also DLIs             |

`PEEK($D40B)` (VCOUNT) tells where ANTIC is drawing: 0 to about 130 on
NTSC and 155 on PAL. It is useful to wait for a screen position from
BASIC, although BASIC is too slow to change colors at an exact line.

## Examples

### A color change in the middle of the screen

```basic
' Change the background color from text row 12 down
DLI SET Red = $24 INTO $D01A        ' $24: dark red, $D01A: COLBK

GRAPHICS 0
DisplayList = DPEEK(560)            ' SDLSTL: display list address
POKE DisplayList + 16, 130          ' row 11: mode 2 + 128 (DLI bit)
DLI Red

PRINT "Press a key"
GET Key
DLI
```

The DLI runs at the last scan line of text row 11. After the implicit
`WSYNC`, the border and background change at the first scan line of row
12, and stay red down to the bottom of the screen.

### Several values in each DLI

```basic
' Three color bands, each with its own background color
DATA Colors() BYTE = $24, $46, $68

' Each DLI sets the background (COLBK) from the array and the
' text background (COLPF2) to $8A, then black one line later
DLI SET Bands = Colors INTO $D01A,
DLI = $8A INTO $D018,
DLI = $00 WSYNC INTO $D018

GRAPHICS 0
DisplayList = DPEEK(560)
POKE DisplayList + 13, 130          ' DLI on rows 8, 11 and 14
POKE DisplayList + 16, 130
POKE DisplayList + 19, 130
DLI Bands

PRINT "Press a key"
GET Key
DLI
```

### A color for each text row

```basic
' A different background color on each text row
Attract = 77                ' ATRACT: attract mode timer
DATA Bars() BYTE = 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0

' Hue 1 to 22 (the hue wraps after 15), luminance 6
FOR I = 0 TO 21
  Bars(I) = I * 16 + 22
NEXT I
DLI SET Rows = Bars INTO $D018 INTO $D01A

' DLI on text rows 1 to 22 (display list bytes 6 to 27),
' so the colors change from row 2 down to row 23
GRAPHICS 0
DisplayList = DPEEK(560)
FOR I = 6 TO 27
  POKE DisplayList + I, PEEK(DisplayList + I) ! 128
NEXT I
DLI Rows

PRINT "Press a key"
REPEAT
  POKE Attract, 0           ' keep attract mode off
UNTIL KEY()
DLI
GRAPHICS 0
```

The `DATA` array gives the DLI a place for its 22 values; the `FOR` loop
fills it. Assigning to a byte array element stores only the low 8 bits,
so the hue wraps around.

### Several scan lines from one DLI

```basic
' One DLI that changes the color on 8 consecutive scan lines
DLI SET Rainbow = $30 INTO $D01A,
DLI = $32 WSYNC INTO $D01A, $34 WSYNC INTO $D01A,
DLI = $36 WSYNC INTO $D01A, $38 WSYNC INTO $D01A,
DLI = $3A WSYNC INTO $D01A, $3C WSYNC INTO $D01A,
DLI = $3E WSYNC INTO $D01A, $00 WSYNC INTO $D01A

GRAPHICS 0
POKE DPEEK(560) + 15, $82       ' row 10: mode 2 + DLI bit
DLI Rainbow
GET Key
DLI
```

The border shows 8 lines of increasing brightness of hue 3 under text row
10, then black to the bottom of the screen.

### Reusing players at several heights

A player is as tall as the screen, but a DLI can move it and change its
color between parts of the screen. This shows players 0 and 1 four times,
each copy at its own position and with its own colors.

```basic
' Reuse players 0 and 1 at four heights with one DLI
DATA Shape1() BYTE = $E7, $81, $81, $E7
DATA Shape2() BYTE = $18, $3C, $3C, $18
DATA XPos() BYTE = $40, $60, $80, $A0
DATA Color1() BYTE = $28, $88, $C8, $08
DATA Color2() BYTE = $2E, $80, $CE, $06

' Write the position to HPOSP0 and HPOSP1 ($D000, $D001)
' and the colors to COLPM0 and COLPM1 ($D012, $D013)
DLI SET Multi = XPos INTO $D000 INTO $D001,
DLI = Color1 INTO $D012, Color2 INTO $D013

GRAPHICS 0
PMGRAPHICS 2
MSET PMADR(-1), 640, 0
DisplayList = DPEEK(560)

' A DLI on rows 3, 7, 11 and 15, and a copy of the shapes
' a little below each one
FOR Offset = 8 TO 20 STEP 4
  POKE DisplayList + Offset, 130
  MOVE ADR(Shape1), PMADR(0) + Offset * 4 + 5, 4
  MOVE ADR(Shape2), PMADR(1) + Offset * 4 + 5, 4
NEXT Offset
DLI Multi

PRINT "Press a key"
REPEAT
  PAUSE
  XPos(0) = XPos(0) + 2
  XPos(1) = XPos(1) + 1
  XPos(2) = XPos(2) - 1
  XPos(3) = XPos(3) - 2
UNTIL KEY()
DLI
GRAPHICS 0
```

The main loop changes the `XPos` array, and the DLI uses the new values on
the next frame. The same technique can reuse players many times on the
screen, as long as the copies don't overlap vertically.

The program `samples/dli.bas` contains these examples.
