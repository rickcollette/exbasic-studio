# 13. Player/Missile Graphics

This chapter covers Player/Missile graphics (P/M graphics, often called
sprites): the `PMGRAPHICS` statement, the `PMADR` function and the `PMHPOS`
statement, the P/M memory layout, the GTIA registers for colors, sizes,
priority and collisions, and how to draw and move players. The color
registers and `SETCOLOR` are described in
[chapter 12](12-graphics.md#setcolor).

## What players and missiles are

The GTIA chip can draw 4 *players* and 4 *missiles* over (or under) the
normal screen, the *playfield*. They are independent of the graphics mode
and of the screen memory.

- A **player** is a vertical strip 8 pixels wide and as tall as the whole
  screen. Each byte of the player data is one line of the strip: bit 7 is
  the leftmost pixel. A player has one color.
- A **missile** is the same, but only 2 pixels wide. The 4 missiles share
  one block of data: missile 0 uses bits 0 and 1 of each byte, missile 1
  bits 2 and 3, missile 2 bits 4 and 5, and missile 3 bits 6 and 7. A
  missile has the color of the player with the same number.

ANTIC reads the data of the players and missiles from memory on each
frame (this is the P/M *DMA*) and GTIA draws them. To draw a shape you
write its bytes in the player data at the vertical position you want. To
move a shape up or down you move the bytes in memory. To move it left or
right you change a horizontal position register, which is very fast.

There are two vertical resolutions:

- **Double line**: each byte of a player covers 2 scan lines. A player has
  128 bytes.
- **Single line**: each byte covers 1 scan line. A player has 256 bytes.

The *P/M base* (register PMBASE) is the address of a 1 KB (double line) or
2 KB (single line) block of memory with the data of all the players and
missiles. exBASIC finds a place for this block with `PMGRAPHICS` and gives
you the addresses with `PMADR`.

### P/M memory layout

Offsets from the P/M base:

| Data        | Double line (1 KB block) | Single line (2 KB block) |
| ----------- | ------------------------ | ------------------------ |
| (unused)    | 0 to 383                 | 0 to 767                 |
| Missiles    | 384 to 511               | 768 to 1023              |
| Player 0    | 512 to 639               | 1024 to 1279             |
| Player 1    | 640 to 767               | 1280 to 1535             |
| Player 2    | 768 to 895               | 1536 to 1791             |
| Player 3    | 896 to 1023              | 1792 to 2047             |

The missiles and the 4 players are consecutive, so one `MSET` from
`PMADR(-1)` clears all of them: 640 bytes in double line mode, 1280 bytes
in single line mode.

### Coordinates

The **horizontal position** is measured in *color clocks*, the width of
one pixel of `GRAPHICS 7`. Position 48 is the left edge of the normal
playfield and 208 is just past its right edge, so a playfield pixel `x` of
`GRAPHICS 7` (0 to 159) is at position `48 + x`. Positions from 0 to about
40 and from about 215 to 255 are off the visible screen; use them to hide a
player. A player of normal width is 8 color clocks wide.

The **vertical position** is the offset of the byte in the player data.
The normal screen (192 scan lines, as in `GRAPHICS 0`, 7 or 8) starts at
scan line 32:

| Resolution  | First visible byte | Last visible byte | Text row `r` of GRAPHICS 0 starts at |
| ----------- | ------------------ | ----------------- | ------------------------------------ |
| Double line | 16                 | 111               | byte `16 + r * 4`                    |
| Single line | 32                 | 223               | byte `32 + r * 8`                    |

## Statements and functions

### PMGRAPHICS
Turns Player/Missile graphics on or off.

```
PMGRAPHICS mode
```
Abbreviation: `PMG.`

| Mode | Effect                                               |
| ---- | ---------------------------------------------------- |
| 0    | P/M graphics off                                     |
| 1    | Single line resolution, 256 bytes per player, 2 KB   |
| 2    | Double line resolution, 128 bytes per player, 1 KB   |

Only the two low bits of `mode` are used, so 4 is the same as 0, 5 as 1
and 6 as 2. Mode 3 is the same as 2.

With mode 1 or 2, `PMGRAPHICS`:

- Places the P/M block at the highest 2 KB (mode 1) or 1 KB (mode 2)
  boundary that leaves the block below the top of free memory, MEMTOP.
  The base page is `(PEEK(742) & $F8) - 8` in mode 1 and
  `(PEEK(742) & $FC) - 4` in mode 2. For example, with MEMTOP at $BC1F
  (`GRAPHICS 0` on a 48 KB machine), the block is at $B000 in mode 1 and
  at $B800 in mode 2.
- Checks that the block is above the arrays and strings already in use.
  If they reach the block, the program stops with error 2 (`ERROR- 2`).
- Writes the base to PMBASE ($D407).
- Turns on player and missile DMA in SDMCTL (559) with the chosen
  resolution, and enables players and missiles in GRACTL ($D01D).
- Sets the priority register GPRIOR (623) to 1 (players in front of the
  playfield). It keeps bits 6 and 7, which hold the GTIA mode of
  `GRAPHICS 9`, 10 and 11, and clears the other options.
- Sets the horizontal positions, the sizes and the graphics registers of
  all players and missiles ($D000 to $D011) to 0. Position 0 is off the
  left side of the screen.

`PMGRAPHICS 0` turns off the P/M DMA in SDMCTL, sets GRACTL to 0 and
clears the same position and size registers. The P/M memory is not
changed.

Important:

- `PMGRAPHICS` does **not** clear the P/M memory. Clear it with `MSET`
  before showing anything, or old memory contents appear as garbage.
- The P/M block is protected. Arrays created with `DIM` after
  `PMGRAPHICS`, and string variables assigned for the first time after it
  (each one takes 256 bytes), can't grow into the block: when there is no
  room left below it, the program stops with error 2 (`ERROR- 2`) instead of
  overwriting the shapes. `FRE()` does not count the P/M block, so it
  shows more memory than the arrays can use. Create the arrays and assign
  the strings first, or leave enough free memory (see
  [FRE()](16-memory-and-machine-code.md#fre)). `PMGRAPHICS 0` (and `GRAPHICS`,
  which turns P/M graphics off) removes the protection.
- [GRAPHICS](12-graphics.md#graphics) turns P/M graphics off and moves
  MEMTOP. Always use `PMGRAPHICS` after `GRAPHICS`.
- Writes to GPRIOR (623) must come after `PMGRAPHICS`, which resets it.
- Before the program ends, use `PMGRAPHICS 0` or `GRAPHICS 0`, or the
  players stay on the screen.

```basic
GRAPHICS 0
PMGRAPHICS 2
MSET PMADR(-1), 640, 0      ' clear the missiles and the 4 players
```

### PMADR
Returns the address of the data of a player or of the missiles.

```
PMADR(n)
```
Abbreviation: `PM.`

`n` is 0 to 3 for players 0 to 3, and -1 for the missiles. The result is
`PMBASE * 256 + (n + 4) * 128` in double line mode and
`PMBASE * 256 + (n + 4) * 256` in single line mode. Other values of `n`
are not checked.

`PMADR` is only valid after `PMGRAPHICS 1` or `PMGRAPHICS 2`. When P/M
graphics are off it returns a meaningless value.

The addresses are usually above 32767, so they print as negative numbers.
This does not matter for `PEEK`, `POKE`, `MOVE` and `MSET`, which take the
16-bit value as an address.

```basic
' Show a solid block with player 0
PMGRAPHICS 2
Player0 = PMADR(0)
MSET Player0, 128, 0        ' clear player 0
MSET Player0 + 50, 8, $FF   ' a solid block 8 bytes tall
PMHPOS 0, 120
SETCOLOR -4, 4, 8
```

### PMHPOS
Sets the horizontal position of a player or missile.

```
PMHPOS n, position
```
Abbreviation: `PM.`

`n` is 0 to 3 for players 0 to 3 and 4 to 7 for missiles 0 to 3.
`position` is 0 to 255 (see [Coordinates](#coordinates)); only the low 8
bits are used.

`PMHPOS n, x` is the same as `POKE $D000 + n, x`. The value of `n` is not
checked: values 8 to 17 write the size and graphics registers that follow
(for example `PMHPOS 8, 1` makes player 0 double width).

The position registers are hardware registers without an OS shadow. The
change is immediate, even in the middle of the screen, and it stays until
the next `PMHPOS`, `PMGRAPHICS` or `GRAPHICS`. Reading these addresses
does not give the positions back (they are collision registers when read),
so keep the positions in variables. To avoid flicker, move players just
after [PAUSE](15-sound-and-controllers.md#pause), during the vertical
blank.

```basic
PMHPOS 0, 100     ' player 0
PMHPOS 4, 104     ' missile 0
```

### Player colors: SETCOLOR -4 to -1

`SETCOLOR -4, hue, lum` sets the color of player 0 and missile 0,
`SETCOLOR -3` player 1, `SETCOLOR -2` player 2 and `SETCOLOR -1` player 3.
They are the shadow registers PCOLR0 to PCOLR3 at 704 to 707. See
[SETCOLOR](12-graphics.md#setcolor).

## P/M registers

All these registers are in the GTIA chip, except PMBASE and DMACTL in
ANTIC. Most of them are write-only: reading the same address gives a
different register.

### Registers you write

| Address           | Name        | Shadow       | Purpose                                                    |
| ----------------- | ----------- | ------------ | ---------------------------------------------------------- |
| $D000 to $D003    | HPOSP0-3    | (none)       | Horizontal position of players 0 to 3 (`PMHPOS 0` to 3)     |
| $D004 to $D007    | HPOSM0-3    | (none)       | Horizontal position of missiles 0 to 3 (`PMHPOS 4` to 7)    |
| $D008 to $D00B    | SIZEP0-3    | (none)       | Width of players 0 to 3: 0 normal, 1 double, 3 quadruple    |
| $D00C             | SIZEM       | (none)       | Width of missiles, 2 bits each (missile 0 in bits 0-1)      |
| $D00D to $D010    | GRAFP0-3    | (none)       | Player data when DMA is off                                 |
| $D011             | GRAFM       | (none)       | Missile data when DMA is off                                |
| $D012 to $D015    | COLPM0-3    | 704 to 707   | Player/missile colors (`SETCOLOR -4` to -1)                 |
| $D019             | COLPF3      | 711          | Color of the fifth player (`SETCOLOR 3`)                    |
| $D01B             | PRIOR       | 623 (GPRIOR) | Priority and options, see below                             |
| $D01C             | VDELAY      | (none)       | Delay a double line player or missile by one scan line      |
| $D01D             | GRACTL      | (none)       | 1 enables missiles, 2 enables players                       |
| $D01E             | HITCLR      | (none)       | Any write clears all collision registers                    |
| $D400             | DMACTL      | 559 (SDMCTL) | DMA control, see below                                      |
| $D407             | PMBASE      | (none)       | Page of the P/M block                                       |

Write the shadow register in RAM, not the hardware address, when there is
one: the OS copies the shadows to the hardware on each vertical blank and
overwrites a direct write.

### SDMCTL (559)

| Bit value | Meaning                                         |
| --------- | ----------------------------------------------- |
| 1, 2, 3   | Playfield width: narrow, normal, wide           |
| 4         | Missile DMA on                                  |
| 8         | Player DMA on                                   |
| 16        | Single line resolution (off: double line)       |
| 32        | Display list DMA on (the screen is shown)       |

The normal value without P/M is 34. `PMGRAPHICS 2` makes it 46 and
`PMGRAPHICS 1` makes it 62. `POKE 559, 0` turns off the screen, which
makes programs run faster, because ANTIC stops
using memory cycles to read the screen.

### GPRIOR (623)

| Value | Meaning                                                                    |
| ----- | -------------------------------------------------------------------------- |
| 1     | All players in front of all playfield colors (set by `PMGRAPHICS`)         |
| 2     | Players 0 and 1 in front of the playfield, players 2 and 3 behind it       |
| 4     | All playfield colors in front of all players                               |
| 8     | Playfield 0 and 1 in front of the players, playfield 2 and 3 behind them   |
| 16    | Fifth player: the 4 missiles use the color of `SETCOLOR 3` (711)          |
| 32    | Multicolor: where players 0 and 1 (or 2 and 3) overlap, their colors mix   |
| 64, 128 | GTIA mode bits used by `GRAPHICS 9`, 10, 11; keep them unchanged        |

Use one of 1, 2, 4 or 8, plus 16 and 32 if needed. For example
`POKE 623, 1 + 16` gives a fifth player. In `GRAPHICS 9` to 11 keep the top
bits: `POKE 623, PEEK(623) & $C0 ! 17`.

To use the missiles as a fifth player, also place the four missiles side
by side: `PMHPOS 4, X : PMHPOS 5, X + 2 : PMHPOS 6, X + 4 : PMHPOS 7, X + 6`.

### Collision registers (read)

GTIA records when a player or missile overlaps something. Each register
holds 4 bits; bit 0 is playfield 0 or player 0, bit 1 is playfield 1 or
player 1, and so on. The bits stay set until you write to HITCLR.

| Address          | Name       | Bits set when                                   |
| ---------------- | ---------- | ----------------------------------------------- |
| $D000 to $D003   | M0PF-M3PF  | Missile 0 to 3 overlaps playfield color 0 to 3  |
| $D004 to $D007   | P0PF-P3PF  | Player 0 to 3 overlaps playfield color 0 to 3   |
| $D008 to $D00B   | M0PL-M3PL  | Missile 0 to 3 overlaps player 0 to 3           |
| $D00C to $D00F   | P0PL-P3PL  | Player 0 to 3 overlaps player 0 to 3            |

Playfield color 0 to 3 are the registers of `SETCOLOR 0` to 3, so in
`GRAPHICS 3`, 5 and 7, `COLOR 1` points set bit 0, `COLOR 2` points bit 1
and `COLOR 3` points bit 2.

Collisions are detected while the picture is drawn. Read them after
`PAUSE` to get the collisions of the last frame, then clear them with
`POKE $D01E, 0`.

## Drawing and moving players

To draw a player, copy its shape to the player data with `MOVE`, at the
vertical position you want:

```basic
DATA Ship() BYTE = $18, $3C, $7E, $FF
MOVE ADR(Ship), PMADR(0) + 60, 4
```

To move it vertically, erase the old shape (with `MSET`) and copy it at the
new position. A simpler way: put an empty byte at the top and at the bottom
of the shape. When the shape moves one byte up or down, copying it also
erases the old line that is left over.

To move it horizontally, use `PMHPOS`. Do both just after `PAUSE`, so the
change happens while the screen is not drawn.

```basic
' Move a player with the joystick; the button doubles its width.
' The shape has an empty row at the top and the bottom,
' so moving it by one line also erases the old image.
DATA Ship() BYTE = 0, $18, $3C, $7E, $FF, $DB, $18, $24, 0
SizeP0 = $D008              ' width of player 0
CursorOff = 752             ' 1 hides the text cursor

GRAPHICS 0
PMGRAPHICS 2
MSET PMADR(-1), 640, 0      ' clear missiles and players 0 to 3
SETCOLOR -4, 12, 10         ' player 0: green
POKE CursorOff, 1
PRINT "Joystick moves, fire changes size"

X = 120
Y = 60
Player0 = PMADR(0)
REPEAT
  Joy = STICK(0)
  IF Joy & 4 = 0 AND X > 48 THEN DEC X
  IF Joy & 8 = 0 AND X < 200 THEN INC X
  IF Joy & 1 = 0 AND Y > 16 THEN DEC Y
  IF Joy & 2 = 0 AND Y < 104 THEN INC Y
  PAUSE
  POKE SizeP0, 1 - STRIG(0)   ' 0 = normal, 1 = double width
  PMHPOS 0, X
  MOVE ADR(Ship), Player0 + Y, 9
UNTIL KEY()
PMGRAPHICS 0
```

`STICK(0)` returns the joystick bits: bit 0 is 0 when pushed up, bit 1
down, bit 2 left and bit 3 right. `STRIG(0)` is 0 while the button is
pressed. See [chapter 15](15-sound-and-controllers.md#stick).

### Missiles

A missile is 2 bits of each byte of the missile data. To draw missile 0
without changing the others, change only its bits:

```basic
' Fire a missile upward from the bottom of the screen
GRAPHICS 0
PMGRAPHICS 2
Missiles = PMADR(-1)
MSET Missiles, 128, 0
SETCOLOR -4, 3, 12          ' missile 0 has the color of player 0
PMHPOS 4, 124               ' missile 0 horizontal position

FOR Y = 108 TO 16 STEP -2
  PAUSE
  POKE Missiles + Y + 2, 0  ' erase the old position
  POKE Missiles + Y, 3      ' missile 0 uses bits 0 and 1
NEXT Y
POKE Missiles + 16, 0
```

When several missiles are in use, combine the bits:
`POKE Missiles + Y, PEEK(Missiles + Y) ! 3` sets missile 0 and
`POKE Missiles + Y, PEEK(Missiles + Y) & $FC` clears it.

### Collisions

```basic
' Player 0 moves right until it touches a wall
P0Pf = $D004                ' collisions of player 0 with playfield
HitClr = $D01E              ' any write clears all collisions

' The wall: COLOR 1 uses playfield color 0
GRAPHICS 3 + 16
COLOR 1
PLOT 30, 0
DRAWTO 30, 23

' A solid 8 x 16 block as player 0
PMGRAPHICS 2
MSET PMADR(-1), 640, 0
MSET PMADR(0) + 56, 8, $FF
SETCOLOR -4, 4, 8
X = 60
PMHPOS 0, X
PAUSE
POKE HitClr, 0

REPEAT
  PAUSE
  Hit = PEEK(P0Pf) & 1      ' bit 0: touched playfield color 0
  IF Hit = 0
    INC X
    PMHPOS 0, X
  ENDIF
UNTIL Hit OR X > 220

GRAPHICS 0
PRINT "Stopped at X ="; X
```

In `GRAPHICS 3` each pixel is 4 color clocks wide, so the wall at column
30 starts at position 48 + 30 * 4 = 168. The block stops when its right
edge reaches the wall, at X = 161.

## Example: a bouncing ball

This example is based on the sample `samples/pmtest.bas`. The ball
position uses fixed point numbers (128 units per pixel) to move smoothly,
and `FUNC MoveBall` draws the ball at its new place.

```basic
' Bouncing ball with player 0 (based on samples/pmtest.bas)
' Positions use 128 units per pixel, to move smoothly.
ColBk = $D01A               ' hardware background color register

GRAPHICS 0
PMGRAPHICS 2
Player0 = PMADR(0)
OldPos = Player0
MSET Player0, 128, 0
SETCOLOR -4, 1, 15

DATA Ball() BYTE = $38, $44, $54, $44, $38

XPos = 6400
YPos = 2560
XSpeed = 64
YSpeed = 0

REPEAT
  XPos = XPos + XSpeed
  YPos = YPos + YSpeed
  YSpeed = YSpeed + 2           ' gravity
  IF YSpeed > 0 AND YPos > 12800
    ' Bounce on the floor, with a random horizontal speed
    YSpeed = -YSpeed
    XSpeed = RAND(512) - 256
  ENDIF
  IF XSpeed > 0 AND XPos > 25600
    XSpeed = -XSpeed
  ELIF XSpeed < 0 AND XPos < 6400
    XSpeed = -XSpeed
  ENDIF
  MoveBall(XPos / 128, Player0 + YPos / 128)
UNTIL KEY()

GRAPHICS 0
END

' Draw the ball at a horizontal position and a player address
FUNC MoveBall(X, Address)
  POKE ColBk, $74           ' show the CPU time used (see text)
  PAUSE
  PMHPOS 0, X
  MSET OldPos, 5, 0         ' clear the old ball
  MOVE ADR(Ball), Address, 5
  OldPos = Address
ENDFUNC
```

The `POKE ColBk, $74` writes the background color directly to the
hardware register COLBK just before `PAUSE`. The OS restores it from the
shadow register 712 at the vertical blank, so the part of the screen drawn
while the program computes the next position shows in blue. This shows how
much of each frame the program uses.

See also: [MOVE and MSET](16-memory-and-machine-code.md#move),
[display list interrupts](14-display-list-interrupts.md) to reuse players
at several heights.
