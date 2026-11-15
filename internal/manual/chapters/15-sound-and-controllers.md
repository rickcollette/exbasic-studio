# 15. Sound, Controllers and Time

This chapter covers the `SOUND` statement and the POKEY sound
registers, the game controller functions `STICK`, `STRIG`, `PADDLE`
and `PTRIG`, the keyboard function `KEY()`, the timer (`TIME`, `TIMER`,
`PAUSE`) and random numbers (`RAND`). It ends with tables of notes,
distortions and joystick directions, and a complete game loop.

## Sound

The Atari makes sound with the POKEY chip. POKEY has four independent
sound voices (channels), numbered 0 to 3 in exBASIC. Each voice has two
registers:

- `AUDF` (audio frequency): a divider from 0 to 255. The voice produces
  a tone whose frequency goes down as the value goes up: 0 is the
  highest pitch and 255 the lowest.
- `AUDC` (audio control): the distortion (high 4 bits) and the volume
  (low 4 bits).

The registers of voice _n_ are at $D200 + 2 × _n_ (`AUDF`) and
$D201 + 2 × _n_ (`AUDC`), that is 53760 to 53767. A sound keeps playing
until the program changes or stops it; `SOUND` does not wait.

When an exBASIC program starts, and again when it ends, all four voices
are silenced.

### SOUND

Plays a sound or stops one or all voices.

```
SOUND voice, pitch, distortion, volume
SOUND voice
SOUND
```
Abbreviation: `S.`

**`SOUND voice, pitch, distortion, volume`** starts a sound on _voice_
(0 to 3) and keeps it playing:

- _pitch_ (0 to 255) goes to the `AUDF` register of the voice: a
  smaller value gives a higher note. See the [note table](#note-table).
- _distortion_ (0 to 14, even values) selects the kind of sound: 10 is a
  pure tone, the others are noises of different kinds. See the
  [distortion table](#distortion-table).
- _volume_ (0 to 15): 0 is silent, 15 the loudest. When several voices
  play at the same time, keep the sum of the volumes at 32 or less, or
  the sound distorts.

The statement writes _pitch_ to `AUDF` and _distortion_ × 16 +
_volume_ to `AUDC`. It also writes 0 to `AUDCTL` ($D208, 53768) and 3
to `SKCTL` ($D20F, 53775), which sets POKEY to the normal mode: 8-bit
voices with the 64 kHz clock, as in Atari BASIC.

The values are not checked:

- a _pitch_ above 255 is added into `AUDC`, changing the volume or
  distortion;
- a _volume_ above 15 changes the distortion;
- an odd _distortion_ (1, 3, ... 15) sets the "volume only" bit of
  `AUDC`: the voice does not oscillate and the output stays at a fixed
  level given by the volume (this is used to play digitized samples by
  changing the volume quickly);
- a _voice_ above 3 writes into other POKEY registers.

**`SOUND voice`** silences one voice: it writes 0 to its `AUDF` and
`AUDC` registers.

**`SOUND`** with no parameters silences all four voices. It does not
change `AUDCTL`.

```basic
' Play the C major scale, one note every 15 jiffies
PureTone = 10
DATA Notes() BYTE = 121, 108, 96, 91, 81, 72, 64, 60
FOR I = 0 TO 7
  SOUND 0, Notes(I), PureTone, 8
  PAUSE 15
NEXT I
SOUND
```

```basic
' A C major chord on three voices
PureTone = 10
SOUND 0, 121, PureTone, 6        ' C
SOUND 1, 96, PureTone, 6         ' E
SOUND 2, 81, PureTone, 6         ' G
PAUSE 60
SOUND 1                          ' stop voice 1 only
PAUSE 30
SOUND                            ' stop all voices
```

```basic
' Explosion: noise with decaying volume
WhiteNoise = 8
FOR Volume = 15 TO 0 STEP -1
  SOUND 0, 60, WhiteNoise, Volume
  PAUSE 3
NEXT Volume
SOUND 0
```

TurboBASIC XL has a `DSOUND` statement for 16-bit voices; exBASIC does
not. You can still use the other POKEY modes by writing the registers
with [POKE](16-memory-and-machine-code.md) after the last `SOUND` that
plays a note, because each `SOUND voice, pitch, distortion, volume`
resets `AUDCTL` to 0.

### Distortion table

The distortion selects how POKEY combines the voice divider with its
polynomial counters (pseudo-random bit generators of 4, 5 and 17 bits).

| Distortion | `AUDC` bits | Sound | Typical use |
| ---------- | ----------- | ----- | ----------- |
| 0 | 5-bit and 17-bit poly | Irregular noise | Engines, rumble, steam |
| 2 | 5-bit poly | Buzzy, rough tone | Engines, motors |
| 4 | 5-bit and 4-bit poly | Coarse buzz | Machine noise |
| 6 | 5-bit poly | Same as 2 | |
| 8 | 17-bit poly | White noise | Explosions, gunshots, waves, wind |
| 10 | Pure tone | Square wave | Music, beeps |
| 12 | 4-bit poly | Buzzer | Alarms, low buzz |
| 14 | Pure tone | Same as 10 | |

With the noise distortions the pitch changes the color of the noise:
low values give a hiss, high values a rumble. The sound of distortions
0, 2, 4, 6 and 12 also depends strongly on the pitch value, so try
several values.

### Note table

With distortion 10 the frequency of the tone is

    frequency = 63921 / (2 × (pitch + 1))   Hz on NTSC machines
    frequency = 63337 / (2 × (pitch + 1))   Hz on PAL machines

so the same pitch value plays about 1% lower on a PAL machine. The
table gives the classic Atari values for three octaves. Middle C is 121.

| Note | Low octave | Middle octave | High octave |
| ---- | ---------- | ------------- | ----------- |
| C | 243 | 121 | 60 |
| C# | 230 | 114 | 57 |
| D | 217 | 108 | 53 |
| D# | 204 | 102 | 50 |
| E | 193 | 96 | 47 |
| F | 182 | 91 | 45 |
| F# | 173 | 85 | 42 |
| G | 162 | 81 | 40 |
| G# | 153 | 76 | 37 |
| A | 144 | 72 | 35 |
| A# | 136 | 68 | 33 |
| B | 128 | 64 | 31 |

The next C above the high octave is 29. Higher notes are less in tune,
because the steps between pitch values become large compared to the
distance between notes.

## Controllers

The Atari 400 and 800 have four controller ports; the XL and XE models
have two. A port can hold a joystick or a pair of paddles. The OS reads
the controllers 50 or 60 times per second (in the vertical blank) and
keeps the values in memory; the functions below read those values.

### STICK

Returns the position of a joystick.

```
STICK(n)
```
Abbreviation: `S.`

Returns the direction of joystick _n_ (0 for the first port). _n_ is
taken modulo 4, so `STICK(4)` is the same as `STICK(0)`. On the XL and
XE models only joysticks 0 and 1 exist.

The value has one bit per direction, and a bit is 0 while the stick is
pushed that way:

| Bit | Value | Direction |
| --- | ----- | --------- |
| 0 | 1 | Up |
| 1 | 2 | Down |
| 2 | 4 | Left |
| 3 | 8 | Right |

So the value is 15 when the stick is centered. The nine positions are:

```
          up
     10   14    6
left 11   15    7  right
      9   13    5
         down
```

| Value | Direction | Value | Direction |
| ----- | --------- | ----- | --------- |
| 15 | Center | | |
| 14 | Up | 13 | Down |
| 11 | Left | 7 | Right |
| 10 | Up and left | 6 | Up and right |
| 9 | Down and left | 5 | Down and right |

Testing the bits gives the movement in each axis directly, because a
comparison is 1 when true and 0 when false:

```basic
Joystick = STICK(0)
MoveX = (Joystick & 8 = 0) - (Joystick & 4 = 0)
MoveY = (Joystick & 2 = 0) - (Joystick & 1 = 0)
```

`MoveX` is -1 (left), 0 or 1 (right), and `MoveY` is -1 (up), 0 or 1
(down).

```basic
' Move a "*" around the text screen with joystick 0
CursorInhibit = 752        ' OS location CRSINH
Space = 32
Star = 42

X = 20
Y = 12
CLS
POKE CursorInhibit, 1      ' hide the cursor
DO
  Joystick = STICK(0)
  MoveX = (Joystick & 8 = 0) - (Joystick & 4 = 0)
  MoveY = (Joystick & 2 = 0) - (Joystick & 1 = 0)
  IF MoveX OR MoveY
    ' Erase the star and move it, staying on the screen
    POSITION X, Y
    PUT Space
    X = X + MoveX
    Y = Y + MoveY
    IF X < 2 THEN X = 2
    IF X > 39 THEN X = 39
    IF Y < 0 THEN Y = 0
    IF Y > 22 THEN Y = 22
  ENDIF
  POSITION X, Y
  PUT Star
  PAUSE 2
LOOP UNTIL STRIG(0) = 0    ' until the button is pressed
POKE CursorInhibit, 0      ' show the cursor again
```

`STICK(n)` is the same as `PEEK(632 + n)` (OS shadow registers
`STICK0` to `STICK3`).

### STRIG

Returns the state of a joystick button.

```
STRIG(n)
```
Abbreviation: `STR.`

Returns 0 while the button of joystick _n_ is pressed and 1 when it is
not. _n_ is taken modulo 4. Note that the value is 0 for **pressed**;
`IF STRIG(0) = 0` tests for the button.

`STRIG(n)` is the same as `PEEK(644 + n)`.

```basic
PRINT "Press the fire button"
REPEAT
UNTIL STRIG(0) = 0
PRINT "Fire!"
```

### PADDLE

Returns the position of a paddle controller.

```
PADDLE(n)
```
Abbreviation: `PA.`

Returns the position of paddle _n_ (0 to 7; _n_ is taken modulo 8), a
value from 1 (turned fully clockwise) to 228 (fully counterclockwise).
Paddles come in pairs: paddles 0 and 1 use port 1, 2 and 3 port 2, and
so on. The XL and XE models have only paddles 0 to 3.

`PADDLE(n)` is the same as `PEEK(624 + n)`.

```basic
' Show paddle 0 until its button is pressed
REPEAT
  POSITION 2, 5
  PRINT "PADDLE: "; PADDLE(0); "   "
  PAUSE 0
UNTIL PTRIG(0) = 0
```

### PTRIG

Returns the state of a paddle button.

```
PTRIG(n)
```
Abbreviation: `PT.`

Returns 0 while the button of paddle _n_ (0 to 7, taken modulo 8) is
pressed, 1 otherwise. `PTRIG(n)` is the same as `PEEK(636 + n)`.

### KEY

Tells if a key is waiting, without waiting for it.

```
KEY()
```
Abbreviation: `K.`

Returns 0 when no key was pressed, and a nonzero value (the internal
key code exclusive-or 255) when a key is waiting to be read. Read the
key with `GET`, or clear it with `POKE 764, 255`. This is the way to
read the keyboard in a game loop without stopping it. See
[KEY](10-console-io.md#key) for the details and a table of key codes,
and [Other keys](10-console-io.md#other-keys) for the START, SELECT and
OPTION keys.

```basic
' Game loop that ends when ESC is pressed
LastKey = 764              ' OS location CH
EscKey = 27
POKE LastKey, 255          ' forget any key pressed before
DO
  ' ... move things ...
  IF KEY()
    GET Key
    IF Key = EscKey
      EXIT
    ENDIF
  ENDIF
  PAUSE 0
LOOP
```

## Time

The OS counts the frames shown on the TV in a three byte counter,
`RTCLOK`, at locations 18, 19 and 20. One count is a **jiffy**: 1/60
second on NTSC machines and 1/50 second on PAL machines.

### TIME

Returns the frame counter.

```
TIME
```
Abbreviation: `T.`

Returns the number of jiffies counted by the OS, from the low two bytes
of `RTCLOK` (locations 19 and 20). It counts from 0 to 65535 and then
starts again at 0. As exBASIC integers are signed, the value goes from
32767 to -32768 after about 9 minutes (NTSC) or 11 minutes (PAL), and
becomes positive again after the next 9 or 11 minutes. `TIME` has no
parentheses.

To measure a time interval, subtract two values of `TIME`: the result
is correct, even across the change from 32767 to -32768, as long as the
interval is less than 32768 jiffies.

For longer times, use the floating point `%TIME`, which uses all three
bytes and goes up to more than 3 days. See
[Floating Point](09-floating-point.md).

```basic
' Time a loop in jiffies
TIMER
Sum = 0
FOR I = 1 TO 10000
  Sum = Sum + I
NEXT I
Elapsed = TIME
PRINT "Loop took "; Elapsed; " jiffies"
```

### TIMER

Sets the frame counter to 0.

```
TIMER
```
Abbreviation: `T.`

Sets locations 19 and 20 to 0, so `TIME` counts again from 0. The
highest byte (location 18) is not changed, so do not use `TIMER` in
programs that use `%TIME`.

### PAUSE

Waits for a number of frames.

```
PAUSE [jiffies]
```
Abbreviation: `PA.`

Stops the program until the frame counter has changed _jiffies_ + 1
times. As the first change can come at any moment, the wait is between
_jiffies_ and _jiffies_ + 1 frames: `PAUSE 60` waits about one second
on NTSC machines. _jiffies_ is taken as an unsigned number from 0 to
65535.

`PAUSE` alone, or `PAUSE 0`, waits until the next vertical blank, the
moment the TV beam returns to the top of the screen. Use it in a game
loop to run once per frame and to change the screen without flicker.

```basic
' One step per frame
DO
  ' ... read the joystick, move the players ...
  PAUSE 0
LOOP
```

`PAUSE` depends on the OS vertical blank interrupt: if a program
disables it, `PAUSE` never returns.

## Random numbers

### RAND

Returns a random integer.

```
RAND(n)
```
Abbreviation: `R.`

Returns a random number from 0 to _n_ - 1, for _n_ from 1 to 32767:
`RAND(6)` returns 0, 1, 2, 3, 4 or 5. All values are equally likely.
`RAND(0)` and `RAND(1)` return 0. A negative _n_ is taken as a number
from 32768 to 65535 and gives results that can be negative, so do not
use it.

The numbers come from POKEY's hardware random number generator
(location 53770, `RANDOM`), so they are different on each run and there
is no seed to set.

```basic
' Roll two dice ten times
FOR Roll = 1 TO 10
  Die1 = RAND(6) + 1
  Die2 = RAND(6) + 1
  PRINT Die1; "+"; Die2; "="; Die1 + Die2
NEXT Roll
```

Output (one run; the numbers change every time):

```
5+4=9
3+1=4
3+4=7
2+6=8
3+4=7
2+4=6
3+2=5
5+4=9
4+4=8
3+3=6
```

For a random floating point number from 0 to 1, use `RND()` (see
[Floating Point](09-floating-point.md)).

## Example: a game loop

This complete program uses most of this chapter: the joystick moves a
`*` on the text screen to catch `$` signs placed at random, a beep
plays for 4 jiffies on each catch, and the game ends after 30 seconds.

```basic
' CATCH: move the "*" with joystick 0 and catch the "$"
' signs. The game lasts 30 seconds (NTSC).

CursorInhibit = 752        ' OS location CRSINH
GameTime = 30 * 60         ' 30 seconds of 60 jiffies
BeepLength = 4             ' jiffies
Space = 32
Star = ASC("*")

CLS
POKE CursorInhibit, 1      ' hide the cursor
PRINT "CATCH!  SCORE: 0"
X = 20
Y = 12
Score = 0
BeepEnd = 0
TargetX, TargetY = PlaceTarget()
TIMER

DO
  ' Read the joystick and move the player
  Joystick = STICK(0)
  MoveX = (Joystick & 8 = 0) - (Joystick & 4 = 0)
  MoveY = (Joystick & 2 = 0) - (Joystick & 1 = 0)
  IF MoveX OR MoveY
    POSITION X, Y
    PUT Space
    X = X + MoveX
    Y = Y + MoveY
    IF X < 2 THEN X = 2
    IF X > 39 THEN X = 39
    IF Y < 2 THEN Y = 2
    IF Y > 22 THEN Y = 22
  ENDIF
  POSITION X, Y
  PUT Star

  ' Caught the target?
  IF X = TargetX AND Y = TargetY
    INC Score
    POSITION 16, 0
    PRINT Score
    BeepEnd = StartBeep(60, BeepLength)
    TargetX, TargetY = PlaceTarget()
  ENDIF

  ' Stop the beep when its time is over
  IF BeepEnd AND TIME > BeepEnd
    SOUND 0
    BeepEnd = 0
  ENDIF

  ' The fire button speeds up the player
  IF STRIG(0)
    PAUSE 3
  ELSE
    PAUSE 1
  ENDIF
LOOP UNTIL TIME > GameTime

SOUND
POSITION 2, 23
PRINT "GAME OVER - SCORE "; Score;
POKE CursorInhibit, 0      ' show the cursor again
GET Key

' Draw a "$" at a random place, return its position
FUNC PlaceTarget() TargetX, TargetY
  TargetX = RAND(38) + 2
  TargetY = RAND(21) + 2
  POSITION TargetX, TargetY
  PUT ASC("$")
ENDFUNC

' Start a pure tone on voice 0, return the time to stop it
FUNC StartBeep(Pitch, Length)
  SOUND 0, Pitch, 10, 10
  RETURN TIME + Length
ENDFUNC
```

Notes on the program:

- `PlaceTarget` returns two results, the position of the new target,
  and `StartBeep` returns the time when the beep must stop. See
  [Functions](07-functions.md).
- `PAUSE` sets the speed of the game and keeps it the same on every
  machine of the same TV standard; the loop itself takes much less than
  a frame.
- The beep is stopped by comparing with `TIME` instead of waiting, so
  the game does not stop while the sound plays.
