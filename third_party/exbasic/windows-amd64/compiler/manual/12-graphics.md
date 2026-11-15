# 12. Graphics

This chapter covers the graphics statements of exBASIC: `GRAPHICS`,
`COLOR`, `FCOLOR`, `PLOT`, `DRAWTO`, `FILLTO`, `LOCATE`, `POSITION` and
`SETCOLOR`. It explains the Atari graphics modes, the color registers and
how hue and luminance make a color, and how to work with screen memory
directly. Player/Missile graphics are in
[chapter 13](13-player-missile-graphics.md) and display list interrupts in
[chapter 14](14-display-list-interrupts.md).

## How Atari graphics work

The Atari 8-bit computers draw the screen with two chips:

- **ANTIC** reads a small program called the *display list* and the screen
  memory, and generates the picture line by line. The display list says
  which kind of line (text or graphics, and its resolution) goes at each
  vertical position. See
  [chapter 14](14-display-list-interrupts.md#the-display-list).
- **GTIA** turns the data from ANTIC into colors, using the *color
  registers*. It also draws Player/Missile graphics and detects collisions.

You do not program ANTIC directly for normal use. The operating system
(OS) screen handler, the `S:` device, builds the display list and the
screen memory when you select a graphics mode. exBASIC talks to `S:`
through I/O channel 6 (IOCB #6): `GRAPHICS` opens it, and `PLOT`,
`DRAWTO`, `FILLTO` and `LOCATE` send commands to it. Text printed with
`PRINT #6` also goes to the graphics screen. For I/O channels in general
see [chapter 11](11-files-and-devices.md).

Pixels are not colors. A pixel holds a small number, the *color number*
(selected with `COLOR`), and that number selects one of the color
registers. The register holds the actual color. Changing a register with
`SETCOLOR` instantly changes every pixel drawn with that color number.

Coordinates start at 0,0 in the top left corner. X grows to the right and
Y grows down.

## Graphics modes

`GRAPHICS n` selects one of the OS graphics modes. The table lists the
full screen resolution, the resolution when the mode has a 4 line text
window at the bottom (the default), the number of colors and the screen
memory used.

| Mode | Kind     | Full screen | With text window | Colors          | Bytes per line | ANTIC mode | Approx. RAM used |
| ---- | -------- | ----------- | ---------------- | --------------- | -------------- | ---------- | ---------------- |
| 0    | text     | 40 x 24     | (none)           | 1 hue, 2 lums   | 40             | 2          | 1 KB             |
| 1    | text     | 20 x 24     | 20 x 20          | 5               | 20             | 6          | 0.7 KB           |
| 2    | text     | 20 x 12     | 20 x 10          | 5               | 20             | 7          | 0.4 KB           |
| 3    | graphics | 40 x 24     | 40 x 20          | 4               | 10             | 8          | 0.4 KB           |
| 4    | graphics | 80 x 48     | 80 x 40          | 2               | 10             | 9          | 0.7 KB           |
| 5    | graphics | 80 x 48     | 80 x 40          | 4               | 20             | 10 ($A)    | 1.2 KB           |
| 6    | graphics | 160 x 96    | 160 x 80         | 2               | 20             | 11 ($B)    | 2.2 KB           |
| 7    | graphics | 160 x 96    | 160 x 80         | 4               | 40             | 13 ($D)    | 4.2 KB           |
| 8    | graphics | 320 x 192   | 320 x 160        | 1 hue, 2 lums   | 40             | 15 ($F)    | 8 KB             |
| 9    | graphics | 80 x 192    | (none)           | 16 lums, 1 hue  | 40             | 15 + GTIA  | 8 KB             |
| 10   | graphics | 80 x 192    | (none)           | 9               | 40             | 15 + GTIA  | 8 KB             |
| 11   | graphics | 80 x 192    | (none)           | 16 hues, 1 lum  | 40             | 15 + GTIA  | 8 KB             |
| 12   | text     | 40 x 24     | 40 x 20          | 5               | 40             | 4          | 1.1 KB           |
| 13   | text     | 40 x 12     | 40 x 10          | 5               | 40             | 5          | 0.7 KB           |
| 14   | graphics | 160 x 192   | 160 x 160        | 2               | 20             | 12 ($C)    | 4.2 KB           |
| 15   | graphics | 160 x 192   | 160 x 160        | 4               | 40             | 14 ($E)    | 8 KB             |

Notes:

- Modes 12 to 15 need the XL/XE operating system. The original 400/800 OS
  only has modes 0 to 11.
- Modes 9, 10 and 11 are the *GTIA modes*. ANTIC draws a normal mode 8
  screen and GTIA joins each group of 4 bits into one wide pixel. They
  never have a text window.
- The text window is a 40 x 4 `GRAPHICS 0` area. `PRINT` without a channel
  writes there.
- Mode 0 and the text modes 1, 2, 12 and 13 show characters. `PLOT` and
  `DRAWTO` in these modes write characters: the `COLOR` value is the
  ATASCII code of the character.

### Which color register each color number uses

| Mode            | COLOR 0                  | COLOR 1            | COLOR 2           | COLOR 3           | Border       |
| --------------- | ------------------------ | ------------------ | ----------------- | ----------------- | ------------ |
| 3, 5, 7, 15     | SETCOLOR 4 (background)  | SETCOLOR 0         | SETCOLOR 1        | SETCOLOR 2        | SETCOLOR 4   |
| 4, 6, 14        | SETCOLOR 4 (background)  | SETCOLOR 0         | (none)            | (none)            | SETCOLOR 4   |
| 8               | SETCOLOR 2 (background)  | luminance of SETCOLOR 1, hue of SETCOLOR 2 | (none) | (none) | SETCOLOR 4 |
| 0 (text)        | background is SETCOLOR 2, characters use the luminance of SETCOLOR 1 | | | | SETCOLOR 4 |

The text modes select the color from the character code:

- **Modes 1 and 2** show 64 different character shapes. The two upper bits
  of the character pick the color: uppercase letters, digits and
  punctuation use SETCOLOR 0, lowercase letters use SETCOLOR 1, inverse
  uppercase uses SETCOLOR 2 and inverse lowercase uses SETCOLOR 3. The
  background is SETCOLOR 4. Lowercase letters show as uppercase in another
  color. With `PRINT #6`, use `COLOR(32)`, `COLOR(128)` and `COLOR(160)`
  to switch a string to the other colors (see
  [PRINT](10-console-io.md#print)).
- **Modes 12 and 13** use characters 4 pixels wide, where each pair of bits
  of the character shape is one pixel: `00` is SETCOLOR 4, `01` is SETCOLOR
  0, `10` is SETCOLOR 1 and `11` is SETCOLOR 2, or SETCOLOR 3 when the
  character is inverse (code 128 or more). The ROM character set looks
  garbled in these modes, so they are used with a custom character set.
- **Mode 9** uses `COLOR 0` to `COLOR 15` as 16 luminances of the hue set
  with `SETCOLOR 4, hue, 0`.
- **Mode 10** uses `COLOR 0` to `COLOR 8`, and `COLOR n` uses the register
  of `SETCOLOR n - 4`: `COLOR 0` is `SETCOLOR -4` (address 704) and
  `COLOR 8` is `SETCOLOR 4` (address 712).
- **Mode 11** uses `COLOR 0` to `COLOR 15` as 16 hues, all with the
  luminance set with `SETCOLOR 4, 0, lum`. `COLOR 0` is the background.

## Colors: hue and luminance

Each color register holds one byte: the upper 4 bits are the *hue* (0 to
15) and the lower 4 bits the *luminance* or brightness (0 to 15). The
value is `hue * 16 + luminance`. On most machines only the even
luminances are different (0, 2, 4, ... 14 give 8 levels), except in
`GRAPHICS 9`, which shows all 16.

| Hue | Color (NTSC)   | Hue | Color (NTSC)  |
| --- | -------------- | --- | ------------- |
| 0   | gray           | 8   | blue          |
| 1   | light orange   | 9   | light blue    |
| 2   | orange         | 10  | turquoise     |
| 3   | red-orange     | 11  | green-blue    |
| 4   | pink           | 12  | green         |
| 5   | purple         | 13  | yellow-green  |
| 6   | purple-blue    | 14  | orange-green  |
| 7   | blue           | 15  | light orange  |

PAL machines show somewhat different colors for the same hue numbers.
Luminance 0 is the darkest and 14 the brightest. Hue 0 gives black, grays
and white.

### The color registers

The OS keeps *shadow* copies of the color registers in RAM and copies
them to the GTIA chip 50 or 60 times per second, during the vertical
blank. `SETCOLOR` and `POKE` to the shadow addresses are the normal way
to change colors. Writing directly to the hardware addresses lasts only
until the next vertical blank, which is what display list interrupts use.

| SETCOLOR | Shadow address | Hardware address | Default value      | Used for                          |
| -------- | -------------- | ---------------- | ------------------ | --------------------------------- |
| -4       | 704 ($2C0)     | $D012 (COLPM0)   | 0                  | player/missile 0                  |
| -3       | 705 ($2C1)     | $D013 (COLPM1)   | 0                  | player/missile 1                  |
| -2       | 706 ($2C2)     | $D014 (COLPM2)   | 0                  | player/missile 2                  |
| -1       | 707 ($2C3)     | $D015 (COLPM3)   | 0                  | player/missile 3                  |
| 0        | 708 ($2C4)     | $D016 (COLPF0)   | 40 (hue 2, lum 8)  | playfield 0                       |
| 1        | 709 ($2C5)     | $D017 (COLPF1)   | 202 (hue 12, lum 10) | playfield 1, text luminance     |
| 2        | 710 ($2C6)     | $D018 (COLPF2)   | 148 (hue 9, lum 4) | playfield 2, text background      |
| 3        | 711 ($2C7)     | $D019 (COLPF3)   | 70 (hue 4, lum 6)  | playfield 3, fifth player         |
| 4        | 712 ($2C8)     | $D01A (COLBK)    | 0 (black)          | background and border             |

The default values are set by the OS when the computer starts. Opening the
screen with `GRAPHICS` resets them.

## Graphics statements

### GRAPHICS
Selects a graphics mode.

```
GRAPHICS mode
```
Abbreviation: `G.`

`GRAPHICS` closes I/O channel 6 and opens it again on the `S:` device in
the given mode. The OS builds a new display list and screen memory just
below the top of memory and clears the screen. `mode` is an integer
expression; the low 4 bits select the mode (0 to 15, see the table above)
and these values can be added:

| Add | Effect                                                         |
| --- | -------------------------------------------------------------- |
| 16  | No text window: the graphics area uses the whole screen.        |
| 32  | Do not clear the screen memory.                                 |

For example `GRAPHICS 8 + 16` is a full screen 320 x 192 mode and
`GRAPHICS 7 + 32` keeps the old contents of memory. Modes 0, 9, 10 and 11
never have a text window.

Before opening the screen, `GRAPHICS` also turns Player/Missile graphics
off, exactly like [PMGRAPHICS 0](13-player-missile-graphics.md#pmgraphics):
P/M DMA is disabled and the positions and sizes of all players and
missiles are set to 0.

`GRAPHICS` does not stop the program on errors. If the mode can't be
opened (for example, not enough memory, error 147, or a mode the OS does
not support) the error number is stored in
[ERR()](11-files-and-devices.md#err). After `GRAPHICS`, the top of free
memory (MEMTOP, address 741) is below the new screen, so
[FRE()](16-memory-and-machine-code.md#fre) changes.

The OS places the screen at the top of memory. exBASIC tells the OS where
the arrays and strings of your program end, so the screen can't be put
over them: if there is not enough free memory for the new mode,
`GRAPHICS` fails with error 147 in `ERR()` and the arrays are kept. Call
`GRAPHICS` with the biggest mode your program uses before creating big
arrays with `DIM`, or make sure
[FRE()](16-memory-and-machine-code.md#fre) leaves room for the screen.

When the program starts, channel 6 is closed. `PLOT`, `DRAWTO`, `FILLTO`
and `LOCATE` need a `GRAPHICS` statement first, even if it is only
`GRAPHICS 0`. Without it they fail with error 133 (channel not open) in
`ERR()`.

```basic
GRAPHICS 7
COLOR 1
PLOT 0, 0
DRAWTO 159, 79
COLOR 2
PLOT 159, 0
DRAWTO 0, 79
PRINT "GRAPHICS 7: 160 x 80 with a text window"
```

`PRINT` without a channel writes in the text window. To write in the
graphics area of a text mode (1, 2, 12, 13), use `PRINT #6`:

```basic
' Big text in GRAPHICS 2, then wait for START
Consol = 53279              ' console keys register (CONSOL)

GRAPHICS 2 + 16
POSITION 5, 4
PRINT #6, "GAME OVER"
POSITION 4, 6
PRINT #6, COLOR(32) "PRESS START"

REPEAT
UNTIL PEEK(Consol) = 6      ' 6: only START is pressed
```

`PEEK(Consol)` reads the console keys (CONSOL, 53279); it is 6 while START is
pressed. To clear the graphics screen without changing the mode use
`CLS #6` (see [CLS](10-console-io.md#cls)).

See also: [SETCOLOR](#setcolor), [Screen memory](#screen-memory).

### COLOR
Sets the color number used to draw.

```
COLOR number
```
Abbreviation: `C.`

`COLOR` sets the color number that `PLOT`, `DRAWTO` and `FILLTO` (for its
line) use. The value is stored in one byte, so only the low 8 bits are
used. The meaning depends on the mode:

- In the graphics modes, it is the color number: 0 to 1 in two-color
  modes, 0 to 3 in four-color modes, 0 to 15 in modes 9 and 11 and 0 to 8
  in mode 10. Higher values are reduced by the OS to the valid range.
- In the text modes (0, 1, 2, 12, 13), it is the ATASCII code of the
  character to draw.

The color number stays until the next `COLOR`. Its value at program start
is not defined, so always use `COLOR` before drawing.

```basic
GRAPHICS 0
PRINT "Text in GRAPHICS 0"
COLOR ASC("*")
PLOT 10, 5
DRAWTO 30, 5
POSITION 2, 10
PRINT "POSITION moves the cursor"
```

This draws a line of `*` characters on the text screen.

Note: `COLOR(n)` inside a `PRINT` statement is a different thing; it
changes the characters printed. See [PRINT](10-console-io.md#print).

### FCOLOR
Sets the color number used to fill by `FILLTO`.

```
FCOLOR number
```
Abbreviation: `FC.`

`FCOLOR` stores the fill color in the OS location FILDAT (765). `FILLTO`
paints the area to the right of its line with this color number. The
value is one byte. Like `COLOR`, its value at program start is not defined.

`FCOLOR n` is the same as `POKE 765, n`.

```basic
FCOLOR 2      ' fill with color number 2
```

See [FILLTO](#fillto) for a full example.

### PLOT
Plots a single point.

```
PLOT x, y
```
Abbreviation: `PL.`

`PLOT` draws one point at column `x` and row `y` with the current `COLOR`.
It is the same as `POSITION x, y` followed by `PUT #6, COLOR`. In the text
modes it writes the character with ATASCII code `COLOR` at that position.

`x` can go from 0 to the width of the mode minus 1 (up to 319 in
`GRAPHICS 8`) and `y` from 0 to the height minus 1. The parameters are
integers; for floating point values use [INT](09-floating-point.md#int).

Errors do not stop the program. A point outside the screen gives error 141
(cursor out of range) in [ERR()](11-files-and-devices.md#err), and nothing
is drawn.

After `PLOT`, the point is the start of the next `DRAWTO` or `FILLTO`.

```basic
' A dotted line across the middle of the screen
GRAPHICS 8 + 16
COLOR 1
FOR X = 0 TO 319 STEP 4
  PLOT X, 96
NEXT X
GET Key
```

### DRAWTO
Draws a line.

```
DRAWTO x, y
```
Abbreviation: `DR.`

`DRAWTO` draws a straight line with the current `COLOR` from the last
point drawn (by `PLOT`, `DRAWTO` or `FILLTO`) to `x`, `y`. The end point
becomes the start of the next line, so a series of `DRAWTO` draws
connected lines.

`POSITION` does not change the start of the line: use `PLOT` to start a
line at a new place.

The OS does the drawing with the special command 17 on channel 6. Errors
are stored in `ERR()`; a point outside the screen gives error 141.

```basic
' Concentric boxes in GRAPHICS 3 + 16 (40 x 24, 4 colors)
GRAPHICS 3 + 16
FOR I = 0 TO 11
  COLOR I MOD 3 + 1
  PLOT I, I
  DRAWTO 39 - I, I
  DRAWTO 39 - I, 23 - I
  DRAWTO I, 23 - I
  DRAWTO I, I
NEXT I
GET Key
```

### FILLTO
Draws a line and fills the area to its right.

```
FILLTO x, y
```
Abbreviation: `FI.`

`FILLTO` works like `DRAWTO`: it draws a line from the last point drawn to
`x`, `y` with the current `COLOR`. For each point of the line, the OS also
paints the pixels to its right with the `FCOLOR` color number, and stops
at the first pixel that is not color 0. Draw the right border of the
shape first, so that the fill stops there. If there is no border, the fill
continues to the right edge of the screen and wraps around to the left
side of the same row.

This is the Atari BASIC `XIO 18` fill command. In Atari BASIC you write
`POSITION x, y : POKE 765, f : XIO 18, #6, 0, 0, "S:"`.

```basic
' A filled triangle
GRAPHICS 7 + 16
COLOR 1
PLOT 120, 10
DRAWTO 120, 80
DRAWTO 20, 80
FCOLOR 2
FILLTO 120, 10
GET Key
```

The program draws the right side and the bottom, then fills from the
bottom left corner up to the top. Each row is filled with color 2 from the
diagonal line to the right side.

### LOCATE
Reads the color of a point.

```
LOCATE x, y, var [, var ...]
```
Abbreviation: `LOC.`

`LOCATE` moves the graphics cursor to `x`, `y` and reads the color number
of the pixel there into the variable `var`. In the text modes it reads the
ATASCII code of the character. `var` can be an integer variable or an
element of an integer or byte array, like `A(3)`.

With more than one variable, each one reads the next point. The OS
advances the cursor one position to the right after each read, so
`LOCATE 9, 10, First, Second, Third` reads the points at columns 9, 10
and 11 of row 10.

If the point is outside the screen, the error (141) is stored in `ERR()`
and the value stored in the variable is not valid.

```basic
' Read back the colors of a line
GRAPHICS 5
COLOR 2
PLOT 10, 10
DRAWTO 30, 10

LOCATE 20, 10, ColorAt
LOCATE 9, 10, First, Second, Third
PRINT "Color at 20,10:"; ColorAt
PRINT "From 9,10:"; First; Second; Third
```

On an Atari the first line shows `2`, the color of the point on the line,
and the second line shows `022` for the three points starting at column 9
(`;` prints the numbers without spaces).

`LOCATE` changes the cursor, so after it `PRINT #6` writes at the new
position. It does not change the start point of `DRAWTO`.

### POSITION
Moves the cursor.

```
POSITION x, y
```
Abbreviation: `POS.`

`POSITION` sets the OS cursor column (COLCRS, addresses 85 and 86) to `x`
and the cursor row (ROWCRS, address 84) to `y`. The next `PRINT #6`,
`PUT #6` or `GET #6` uses this position. In `GRAPHICS 0` it is also the
position of the next `PRINT` to the screen.

In modes with a text window, `POSITION` moves the graphics cursor. The
text window has its own cursor (TXTROW at 656 and TXTCOL at 657), so
`PRINT` to the text window is not affected by `POSITION`.

The position is not checked by `POSITION`; the next output on channel 6
stores error 141 in `ERR()` if it is outside the screen.

See [POSITION](10-console-io.md#position) for printing text.

```basic
GRAPHICS 1 + 16
POSITION 7, 11
PRINT #6, "BOTTOM";
GET Key
```

### SETCOLOR
Changes a color register.

```
SETCOLOR register, hue, luminance
```
Abbreviation: `SE.`

`SETCOLOR` stores `hue * 16 + luminance` in the shadow color register
`register`, at address `708 + register`. See the tables in
[Colors: hue and luminance](#colors-hue-and-luminance).

- `register` is 0 to 4 for the playfield colors and the background, and
  -4 to -1 for the colors of players 0 to 3 (addresses 704 to 707).
- `hue` is 0 to 15.
- `luminance` is 0 to 15. Only the even values are different, except in
  `GRAPHICS 9`.

The values are not checked. A luminance of 16 or more adds to the hue, and
a register number outside -4 to 4 writes other OS locations. Only the
lower 8 bits of the result are stored.

`SETCOLOR r, h, l` is the same as `POKE 708 + r, h * 16 + l`. The change
is visible at the next vertical blank, and it stays until the next
`GRAPHICS` statement, which resets the colors to the defaults.

```basic
' Cycle the background through all the hues
GRAPHICS 0
FOR Hue = 0 TO 15
  SETCOLOR 4, Hue, 6        ' border
  SETCOLOR 2, Hue, 2        ' text background
  PAUSE 10
NEXT Hue

SETCOLOR 0, 3, 6
PRINT PEEK(708)             ' shadow of color register 0
```

The last line prints `54` (3 * 16 + 6).

## Screen memory

Each graphics mode stores its picture in a block of memory, the screen
memory. Its address is in SAVMSC, `DPEEK(88)`. You can read and write it
with `PEEK`, `POKE`, `MOVE` and `MSET` (see
[chapter 16](16-memory-and-machine-code.md)), which is much faster than
plotting point by point.

- Each line uses the number of bytes given in the modes table. The byte of
  a point is `DPEEK(88) + y * bytes_per_line + x / points_per_byte`.
- In the 2-color modes (4, 6, 8, 14) each byte has 8 points, one bit each,
  the leftmost point in bit 7.
- In the 4-color modes (3, 5, 7, 15) each byte has 4 points, 2 bits each.
- In the GTIA modes (9, 10, 11) each byte has 2 points, 4 bits each.
- In the text modes each byte is one character, in *internal code*, not in
  ATASCII. ATASCII 32 to 95 (space, digits, uppercase) are internal codes
  0 to 63, ATASCII 0 to 31 are 64 to 95, and ATASCII 96 to 127 do not
  change. Add 128 for inverse video.
- The text window starts at TXTMSC, `DPEEK(660)`.

```basic
' Fill the GRAPHICS 8 screen with a pattern, then clear half
GRAPHICS 8 + 16
Screen = DPEEK(88)          ' SAVMSC: address of screen memory
BytesPerLine = 40

MSET Screen, 192 * BytesPerLine, $AA     ' every other pixel on
MSET Screen, 96 * BytesPerLine, 0        ' clear the top half
GET Key
```

In `GRAPHICS 0`, `POKE DPEEK(88), 33` puts an `A` (internal code 33) in
the top left corner of the screen.

Some screens are bigger than 4 KB. ANTIC can't read screen memory across a
4 KB boundary without help, so the display list of these modes (8, 9, 10,
11, 15) has a second memory address (an LMS instruction) at the line where
the screen crosses the boundary. The OS places the screen so that the lines
are still consecutive in memory, and the formula above works for all
lines. See [chapter 14](14-display-list-interrupts.md#the-display-list).

### Redefining characters

The character set is 1024 bytes, 8 bytes per character, in internal code
order. The ROM set is at $E000. CHBAS (756) holds the page of the
character set in use. A character set must start at a 1 KB boundary (in
modes 1 and 2, which use only 64 characters, a 512 byte boundary).

```basic
' Copy the ROM character set to RAM and redefine the letter "A"
Chbas = 756                 ' page of the character set in use
RomFont = $E000             ' the character set in ROM
DIM Mem(2047) BYTE

' A character set must start at a 1 KB boundary
Font = (ADR(Mem) + 1023) & $FC00
MOVE RomFont, Font, 1024

' "A" is internal code 33, and each character uses 8 bytes
DATA Smiley() BYTE = $3C, $42, $A5, $81, $A5, $99, $42, $3C
MOVE ADR(Smiley), Font + 33 * 8, 8

POKE Chbas, Font / 256
PRINT "AAAA"
```

After a `GRAPHICS` statement the OS sets CHBAS back to the ROM set.

## Useful screen locations

| Address      | Name    | Contents                                               |
| ------------ | ------- | ------------------------------------------------------ |
| 84           | ROWCRS  | Cursor row (`POSITION` sets it)                        |
| 85, 86       | COLCRS  | Cursor column, 2 bytes                                 |
| 87           | DINDEX  | Current OS screen mode                                 |
| 88, 89       | SAVMSC  | Address of the screen memory                           |
| 90, 91, 92   | OLDROW, OLDCOL | Start point of the next `DRAWTO`/`FILLTO`       |
| 559          | SDMCTL  | ANTIC DMA control (0 turns the screen off, 34 is normal) |
| 560, 561     | SDLSTL  | Address of the display list                            |
| 623          | GPRIOR  | GTIA priority and GTIA mode                            |
| 656          | TXTROW  | Text window cursor row                                 |
| 657, 658     | TXTCOL  | Text window cursor column                              |
| 660, 661     | TXTMSC  | Address of the text window memory                      |
| 704 to 712   | PCOLR0..COLOR4 | Color registers (see `SETCOLOR`)               |
| 741, 742     | MEMTOP  | Top of free memory, just below the display list        |
| 752          | CRSINH  | 1 hides the text cursor, 0 shows it                    |
| 755          | CHACT   | Text options: 2 is normal, 0 shows inverse characters as normal, add 4 for upside-down characters |
| 756          | CHBAS   | Page of the character set                              |
| 763          | ATACHR  | Color used by the last line drawn                      |
| 765          | FILDAT  | Fill color (`FCOLOR`)                                  |

## Example: a star field

```basic
' Random stars over a striped ground in GRAPHICS 7,
' with the stars blinking
GRAPHICS 7 + 16
SETCOLOR 4, 0, 0            ' background: black
SETCOLOR 0, 0, 14           ' COLOR 1: stars, white
SETCOLOR 1, 8, 6            ' COLOR 2: blue
SETCOLOR 2, 12, 4           ' COLOR 3: green

' The stars
COLOR 1
FOR I = 1 TO 150
  PLOT RAND(160), RAND(60)
NEXT I

' The ground, alternating colors 2 and 3
FOR Y = 60 TO 95
  COLOR 2 + (Y & 1)
  PLOT 0, Y
  DRAWTO 159, Y
NEXT Y

' Blink the stars until a key is pressed
DO
  FOR Lum = 0 TO 15 STEP 2
    SETCOLOR 0, 0, Lum
    PAUSE 4
  NEXT Lum
LOOP UNTIL KEY()
GRAPHICS 0
```
