# 16. Memory and Machine Code

This chapter covers direct access to memory and machine code: `PEEK`,
`POKE`, `DPEEK`, `DPOKE`, `ADR`, `MOVE`, `-MOVE`, `MSET`, `FRE()` and
`USR`, linking assembly modules with the compiler, and the memory map of a
compiled program (XEX and cartridge), including the zero page and the
areas you can use.

## Addresses and values

The 6502 processor addresses 64 KB of memory, from 0 to 65535 ($0000 to
$FFFF). RAM, ROM and the hardware registers of the custom chips all live
in this space. Each address holds one byte, a value from 0 to 255.

exBASIC integers are 16-bit signed numbers, from -32768 to 32767, but the
memory statements treat an address as a 16-bit unsigned value. Addresses
from 32768 to 65535 can be written in hexadecimal (`$D01A`), or as the
negative number with the same 16 bits (`-1` is `$FFFF`). This is why
addresses above 32767 print as negative numbers:

| Address | Decimal | As a signed integer |
| ------- | ------- | ------------------- |
| $7FFF   | 32767   | 32767               |
| $8000   | 32768   | -32768              |
| $BC1F   | 48159   | -17377              |
| $D01A   | 53274   | -12262              |
| $FFFF   | 65535   | -1                  |

A 16-bit value in memory (a *word*) uses two bytes, with the low byte
first: the value `$1234` is stored as `$34`, `$12`.

## Reading and writing memory

### PEEK
Reads one byte from memory.

```
PEEK(address)
```
Abbreviation: `P.`

Returns the byte at `address`, a value from 0 to 255. Reading a hardware
register can have side effects or return a different register than the
one written at the same address (see
[chapter 13](13-player-missile-graphics.md#pm-registers)).

Byte arrays are faster for your own data: `Flags(I)` is the same as
`PEEK(ADR(Flags) + I)`.

```basic
' Bytes hold 0 to 255: POKE keeps the low 8 bits
Buffer = $0680              ' free bytes in page 6
POKE Buffer, 65
PRINT PEEK(Buffer)
POKE Buffer, 300            ' 300 - 256 = 44
PRINT PEEK(Buffer)
POKE Buffer, -1             ' -1 is $FFFF, the low byte is 255
PRINT PEEK(Buffer)
```

Output:

```
65
44
255
```

### POKE
Writes one byte to memory.

```
POKE address, value
```
Abbreviation: `P.`

Writes the low 8 bits of `value` (that is, `value` modulo 256) to
`address`. See the example of [PEEK](#peek).

`POKE` can write anywhere in RAM, including the memory used by the
program, its variables and the OS. A wrong address can crash the computer.
See [The memory map](#the-memory-map) for the areas that are safe to use.

### DPEEK
Reads a 16-bit word from memory.

```
DPEEK(address)
```
Abbreviation: `D.`

Returns `PEEK(address) + 256 * PEEK(address + 1)` as a signed integer.
The OS stores most addresses this way, for example the screen memory
address in `DPEEK(88)` and the display list in `DPEEK(560)`.

```basic
' A 16-bit value is stored low byte first
Buffer = $0680              ' free bytes in page 6
DPOKE Buffer, $1234
PRINT PEEK(Buffer), PEEK(Buffer + 1)
DPOKE Buffer, 40000
PRINT DPEEK(Buffer)
```

Output:

```
52      18
-25536
```

40000 does not fit in a signed integer, so it reads back as
40000 - 65536 = -25536. The 16 bits are the same.

### DPOKE
Writes a 16-bit word to memory.

```
DPOKE address, value
```
Abbreviation: `D.`

Writes the low byte of `value` to `address` and the high byte to
`address + 1`. See the example of [DPEEK](#dpeek).

When a hardware or OS location is two bytes (like an address), use
`DPOKE`, so both bytes change together.

### ADR
Returns the address of a variable, an array or a string.

```
ADR(name)
&name
ADR(string)
&string
```
No abbreviation; `&` is the short form.

| Argument                        | Result                                              |
| ------------------------------- | --------------------------------------------------- |
| Integer variable `X`            | Address of its 2 bytes                              |
| Floating point variable `X%`    | Address of its 6 bytes                              |
| Array `A` or `A%` (from `DIM`)  | Address of element 0; the elements follow in order  |
| `DATA` array `T`                | Address of element 0                                |
| String expression (`S$`, `"ABC"`, `S$(2)`) | Address of the string (see below)        |

The array name is written without parentheses: `ADR(Table)`. Word arrays
use 2 bytes per element, byte arrays 1 byte and floating point arrays 6
bytes.

A string in memory starts with its length byte, followed by the
characters. So `PEEK(ADR(S$))` is the length and `PEEK(ADR(S$) + 1)` the
first character. This is different from Atari BASIC and Turbo BASIC XL,
where `ADR` points to the first character. A string variable that was
never assigned has address 0. The address of a substring like
`ADR(S$[2, 3])` points to a temporary copy in the buffer at $057F, which
the next string operation overwrites. String arrays (`ADR(S$)` for a
`DIM S$(10)`) are not accepted. See also `$(address)`, the reverse of
`ADR`, in [chapter 8](08-strings.md).

```basic
' Addresses of variables, arrays and strings
DIM Table(9), Flags(9) BYTE
DATA Primes() BYTE = 2, 3, 5, 7
Count = 1234
Name$ = "ATARI"

POKE ADR(Flags) + 2, 99
PRINT Flags(2)
PRINT DPEEK(ADR(Count))
PRINT PEEK(ADR(Name$)); " characters"
PRINT CHR$(PEEK(&Name$ + 1))
PRINT PEEK(ADR(Primes) + 3)
PRINT ADR(Table) = &Table
```

Output:

```
99
1234
5 characters
A
7
1
```

Variables and arrays stay at the same address while the program runs.
A string variable gets its 256 byte buffer the first time it is assigned
and keeps it.

## Moving and filling memory

### MOVE
Copies a block of memory.

```
MOVE from, to, length
```
Abbreviation: `M.`

Copies `length` bytes from address `from` to address `to`. The copy goes
from the lowest address to the highest. `length` is an unsigned 16-bit
value; 0 copies nothing. `MOVE` is a machine code loop that takes about 15 CPU cycles
per byte.

`MOVE Source, Target, Length` gives the same result as this FUNC, only
much faster:

```basic
' The same as MOVE Source, Target, Length
FUNC SlowMove(Source, Target, Length)
  FOR I = 0 TO Length - 1
    POKE Target + I, PEEK(Source + I)
  NEXT I
ENDFUNC
```

When the two blocks overlap, the direction matters:

| Blocks           | Use      |
| ---------------- | -------- |
| Don't overlap    | `MOVE` or `-MOVE` |
| `to` below `from` (copy down) | `MOVE`  |
| `to` above `from` (copy up)   | `-MOVE` |

`MOVE` up over an overlapping block repeats the first bytes instead of
copying.

```basic
' MOVE and -MOVE with overlapping memory
DIM Bytes(9) BYTE

FillBytes()
MOVE ADR(Bytes) + 1, ADR(Bytes), 9     ' down by one: correct
ShowBytes()

FillBytes()
-MOVE ADR(Bytes), ADR(Bytes) + 1, 9    ' up by one: correct
ShowBytes()

FillBytes()
MOVE ADR(Bytes), ADR(Bytes) + 1, 9     ' up with MOVE: wrong
ShowBytes()
END

' Fill the array with 0 to 9
FUNC FillBytes()
  FOR I = 0 TO 9
    Bytes(I) = I
  NEXT I
ENDFUNC

' Print the 10 bytes of the array in one line
FUNC ShowBytes()
  FOR I = 0 TO 9
    PRINT Bytes(I); " ";
  NEXT I
  PRINT
ENDFUNC
```

Output:

```
1 2 3 4 5 6 7 8 9 9 
0 0 1 2 3 4 5 6 7 8 
0 0 0 0 0 0 0 0 0 0 
```

`MOVE` is useful to copy shapes to Player/Missile memory, to copy the
character set from ROM (see [chapter 12](12-graphics.md#redefining-characters)),
to scroll screen memory and to copy arrays.

### -MOVE
Copies a block of memory, from the end.

```
-MOVE from, to, length
```
Abbreviation: `-.`

Like `MOVE`, but copies from the highest address to the lowest. Use it
when the blocks overlap and `to` is above `from`. `-MOVE Source, Target, Length`
gives the same result as:

```basic
' The same as -MOVE Source, Target, Length
FUNC SlowMoveDown(Source, Target, Length)
  FOR I = Length - 1 TO 0 STEP -1
    POKE Target + I, PEEK(Source + I)
  NEXT I
ENDFUNC
```

See the example of [MOVE](#move).

### MSET
Fills a block of memory with a value.

```
MSET address, length, value
```
Abbreviation: `MS.`

Writes the low byte of `value` to `length` bytes starting at `address`.
`length` is an unsigned 16-bit value; 0 writes nothing. `MSET Address, Length, Value`
gives the same result as:

```basic
' The same as MSET Address, Length, Value
FUNC SlowSet(Address, Length, Value)
  FOR I = 0 TO Length - 1
    POKE Address + I, Value
  NEXT I
ENDFUNC
```

`MSET` is the fast way to clear screen memory, Player/Missile data or
arrays.

```basic
' Fill memory with a value
DIM Row(19) BYTE
MSET ADR(Row), 20, ASC("-")
MSET ADR(Row) + 5, 10, ASC("*")
FOR I = 0 TO 19
  PUT Row(I)
NEXT I
PRINT
```

Output:

```
-----**********-----
```

### FRE
Returns the free memory.

```
FRE
FRE()
FRE(x)
```
Abbreviation: `F.`

Returns the number of bytes between the top of the memory used by arrays
and strings and MEMTOP (address 741), the top of free memory set by the
OS. Arrays take memory when `DIM` runs, and each string variable takes 256
bytes the first time it is assigned. The value changes after `GRAPHICS`,
because the screen memory sits just below MEMTOP.

```basic
' Measure the memory used by an array and a string
Before = FRE()
DIM Big(9999) BYTE
PRINT "The array used "; Before - FRE(); " bytes"

Before = FRE()
Text$ = "A"
PRINT "The string used "; Before - FRE(); " bytes"
```

Output:

```
The array used 10000 bytes
The string used 256 bytes
```

A small program on a 48 KB or bigger machine has more than 32767 bytes
free. `PRINT FRE()`, `STR$(FRE())`, comparisons like `IF FRE() > 1000`
and assignments to a floating point variable use the real value, so it
is never shown negative. Only an integer variable keeps 16 bits and
prints a negative number; use a floating point variable to keep the value
(see [FRE() in chapter 5](05-expressions.md#fre)):

```basic
' Keep the free memory in a floating point variable
FreeBytes% = FRE()
PRINT "Free bytes: "; FreeBytes%
IF FRE() < 1000 THEN PRINT "Low memory"
```

`FRE()` does not count the memory used by `PMGRAPHICS` (see
[chapter 13](13-player-missile-graphics.md#pmgraphics)). When there is no
memory for a new array or string, the program stops with
error 2 (`ERROR- 2`).

## Calling machine code

### USR
Calls a machine code routine.

```
USR(address [, param ...])
```
No abbreviation. Write the `(` right after `USR`: with a space, the
compiler reports `FUNC 'USR' is not defined`.

`USR` calls the 6502 machine code routine at `address` and returns the
value the routine leaves in the A (low byte) and X (high byte) registers,
as a signed integer. `address` is any integer expression: the address of
a `DATA` array with the code, a ROM address, or a symbol of an assembly
module (`@NAME`, see [Linking assembly modules](#linking-assembly-modules)).

The parameters are integer expressions (0 or more). Floating point values
are not accepted; convert them with `INT()`. To pass a string, pass its
address with `ADR()`.

#### Calling convention

1. exBASIC evaluates `address` and pushes its own return address on the
   CPU stack.
2. It evaluates each parameter, from first to last, and pushes it on the
   CPU stack: first the low byte, then the high byte.
3. It jumps to `address`.

So the routine finds the **last** parameter on top of the stack, and the
first `PLA` gets its high byte:

| `PLA` number | Gets                        |
| ------------ | --------------------------- |
| 1            | High byte of the last parameter |
| 2            | Low byte of the last parameter  |
| 3            | High byte of the parameter before it |
| ...          | ...                         |

The routine must:

- Pull **all** the parameters, 2 bytes each, before its final `RTS`.
  With the wrong number of `PLA`, the `RTS` jumps to a wrong address and
  the computer crashes.
- End with `RTS`, with the result in A (low byte) and X (high byte). If
  the result is not needed, A and X can hold anything.
- Leave the decimal flag clear (`CLD` if it used `SED`).
- Not change the zero page locations used by exBASIC (see
  [Zero page](#zero-page)) and the interpreter stack at $0480.

It can change A, X, Y and the other flags. `USR` is a function, so to call
a routine for its effect, assign the result: `Dummy = USR(ADR(Code))`.

A `USR` call can be used inside the parameters of another `USR`, like
`USR(R1, USR(R2, X))`: the inner call runs first and its result is
passed to the outer routine.

```basic
' Invert all the bits of a number with machine code
' (the result is -Value - 1):
'   PLA        get the high byte of the parameter
'   EOR #$FF   invert it
'   TAX        high byte of the result in X
'   PLA        get the low byte
'   EOR #$FF   invert it (A is the low byte of the result)
'   RTS
DATA Invert() BYTE = $68, $49, $FF, $AA, $68, $49, $FF, $60

FOR Value = 0 TO 1000 STEP 250
  PRINT Value, USR(ADR(Invert), Value)
NEXT Value
```

Output:

```
0       -1
250     -251
500     -501
750     -751
1000    -1001
```

With two parameters, the routine pulls the second one first:

```basic
' A routine with two parameters: USR(Sub, A, B) returns A - B
'   PLA : STA $CC   high byte of B
'   PLA : STA $CB   low byte of B
'   PLA : TAX       high byte of A
'   PLA             low byte of A
'   SEC : SBC $CB   low byte of A - B
'   PHA
'   TXA : SBC $CC   high byte of A - B
'   TAX
'   PLA
'   RTS
DATA Subtract() BYTE = $68, $85, $CC, $68, $85, $CB, $68, $AA,
DATA BYTE = $68, $38, $E5, $CB, $48, $8A, $E5, $CC, $AA, $68, $60

PRINT USR(ADR(Subtract), 1000, 1)
PRINT USR(ADR(Subtract), 5, 8)
```

Output:

```
999
-3
```

The routine uses $CB and $CC as temporary storage; they are free in the
zero page of a normal program (see [Zero page](#zero-page)). For longer
routines, write them in assembly and link them with the program, as the
next section shows. The code in a `DATA` array must not depend on its own
address, because the address of the array is chosen by the linker.

## Linking assembly modules

The compiler can assemble and link your own assembly files (for the CA65
assembler of the CC65 tools, included with exBASIC) together with the
program. Pass them on the command line after the BASIC file:

```sh
exbasic game.bas mylib.asm
```

You can pass several `.asm` files, and also `.o` object files already
assembled. Each one is assembled with CA65 and linked with LD65.

Important: the compiler writes the BASIC program translated to assembly
to a file with the same name as the BASIC file and the `.asm` extension
(`game.asm` for `game.bas`). An assembly module with that name would be
overwritten: give your modules different names.

### Exporting symbols

The module makes its routines and data visible with `.export`. The names
must be in uppercase, because exBASIC converts the names you write to
uppercase. This module has two routines, a table and a zero page byte:

```
; mylib.asm - assembly module for exBASIC
        .export ADD_10, DOUBLE_SCORE, TABLE, MYZP
        .import fb_var_SCORE

        .zeropage
MYZP:   .res 1

        .code
; USR(@ADD_10, n) returns n + 10
.proc ADD_10
        pla             ; high byte of n
        tax
        pla             ; low byte of n
        clc
        adc #10
        bcc :+
        inx
:       rts             ; result in A (low) and X (high)
.endproc

; USR(@DOUBLE_SCORE) doubles the BASIC variable SCORE in place
.proc DOUBLE_SCORE
        asl fb_var_SCORE
        rol fb_var_SCORE+1
        lda fb_var_SCORE
        ldx fb_var_SCORE+1
        rts
.endproc

; Tables can go in CODE (there is no RODATA segment)
TABLE:  .byte 1, 2, 4, 8, 16, 32, 64, 128
```

### Using assembler symbols from BASIC

In the BASIC program, `@NAME` is the 16-bit value of the symbol `NAME`
(an address or a constant), and `@@NAME` is the value of a zero page
symbol, one byte. They can be used anywhere a number can: in expressions,
as the address of `USR`, and in `DLI SET ... INTO @NAME`. The name is not
case sensitive in BASIC (`@Add_10` is `ADD_10`), but it must not be
followed by `(`, which would be a FUNC call.

```basic
' Call routines from an assembly module (mylib.asm)
Score = 21
PRINT USR(@ADD_10, 25)
PRINT USR(@DOUBLE_SCORE), Score
PRINT PEEK(@TABLE + 3)
POKE @@MYZP, 99
PRINT PEEK(@@MYZP)
```

Compiled with `exbasic game.bas mylib.asm`, the output is:

```
35
42      42
8
99
```

A symbol that no module exports gives a link error:

```
Unresolved external 'LOWSYM' referenced in:
  prog.asm(35)
ld65: Error: 1 unresolved external(s) found - cannot create output file
```

### Symbols exported by the program

The compiled program exports these symbols, which a module can `.import`:

| Symbol             | Meaning                                                          |
| ------------------ | ---------------------------------------------------------------- |
| `fb_var_NAME`      | The variable `NAME` (uppercase, without `%` or `$`)               |
| `fb_lbl_NAME`      | The `DATA` array or DLI `NAME`                                   |
| `EXB_VARS`, `EXB_VARS_END` | Start and end of the variables                           |
| `EXB_FREE`         | Start of the free memory for arrays and strings                  |
| `bytecode_start`   | Start of the compiled program                                    |

The variable symbol is the address of the value:

| Variable type      | At `fb_var_NAME`                                               |
| ------------------ | -------------------------------------------------------------- |
| Integer            | 2 bytes, low byte first                                        |
| Floating point     | 6 bytes, Atari BCD format                                      |
| String             | 2 bytes: address of the string (length byte, then characters), 0 if never assigned |
| Array (`DIM`)      | 2 bytes: address of element 0                                  |

The module above uses `fb_var_SCORE` to change the BASIC variable
`Score`. If the BASIC program does not use the variable, the link fails
with an unresolved external.

### Segments

Put the code and data of a module in these segments:

| Segment     | In a XEX program               | In a cartridge                       |
| ----------- | ------------------------------ | ------------------------------------ |
| `CODE`      | RAM, loaded with the program   | ROM (read only)                      |
| `DATA`      | RAM, loaded with the program   | Stored in ROM, copied to RAM at start |
| `BSS`       | RAM, not initialized           | RAM, not initialized                 |
| `ZEROPAGE`  | Zero page, not initialized     | Zero page, not initialized           |
| `ALIGNDATA` | RAM, starts at a page boundary | ROM, starts at a page boundary       |

There is no `RODATA` segment: a module that uses it fails with
`Missing memory area assignment for segment 'RODATA'`. Writable variables
of a module that must work in a cartridge go in `BSS` or `DATA`.

### Compiler options for assembly

| Option              | Effect                                                           |
| ------------------- | ---------------------------------------------------------------- |
| `-X:option`         | Pass an option to CA65, for example `-X:-Iinclude` or `-X:-DDEBUG` |
| `-C:file.cfg`       | Use another LD65 configuration file                              |
| `-S:address`        | Start address of the program, default $2000                      |
| `-DL:symbol=value`  | Define a linker symbol                                           |
| `-g`                | Write the label file (`.lbl`) and assembly listings               |
| `-keep`             | Keep the intermediate `.asm` and `.o` files                       |

`-X:` passes one word: use several `-X:` for options with spaces. See
[chapter 19](19-compiler-reference.md) for all the options. The
`-S:` option also works for cartridges, where it moves the RAM used for
variables (default $2000).

With `-g`, the label file lists every symbol with its address, one per
line, like `al 002060 .EXB_VARS`. Use it to find where things are:

```sh
exbasic -g game.bas
grep -E 'EXB_|__ZP_LAST__' game.lbl
```

## The memory map

### XEX programs

A compiled `.xex` program loads at $2000 (change it with `-S:`). The
blocks follow one another in this order:

| Block (segment)     | Contents                                                           |
| ------------------- | ------------------------------------------------------------------ |
| `LDCHK`             | The load-time memory check, at $80 to $CB, run by DOS before the rest is loaded (see below) |
| `JUMPTAB`           | Table of the interpreter instructions used (2 bytes each)          |
| `HEAP`              | The variables, in the same page as the jump table when they fit     |
| `RUNTIME`           | The interpreter routines used by the program                       |
| `DATA`              | `DATA` arrays, DLI routines and interpreter data                   |
| `BYTECODE`          | The compiled program (bytecode and machine code)                   |
| `CODE`              | Code of linked assembly modules                                    |
| `ALIGNDATA`         | Page aligned data of assembly modules                              |
| `BSS`               | P/M state, the 512 byte FUNC frame stack (if recursion is used)    |
| `HEAPA`             | The variables, when they don't fit after the jump table            |
| `EXB_FREE`          | Start of the free memory                                           |

Variables are addressed as "page + 2 * number", so all the variables must
fit in 512 bytes from the start of their page: integers, strings and
arrays use 2 bytes each and floating point variables 6 bytes. A program
with more fails to assemble with the message `too many variables`.

Above `EXB_FREE` the memory is used while the program runs:

| From            | To              | Contents                                                     |
| --------------- | --------------- | ------------------------------------------------------------ |
| `EXB_FREE`      | grows up        | Arrays (at `DIM`) and strings (at first assignment, 256 bytes each) |
| (free)          |                 | `FRE()` bytes                                                |
| P/M base        | MEMTOP area     | Player/Missile data, if `PMGRAPHICS` is used                 |
| MEMTOP + 1      | RAMTOP          | Display list and screen memory of the current mode           |

For example, a small program compiled with `-g` gave this label file
excerpt:

```
al 002000 .__JUMPTAB_RUN__
al 002060 .EXB_VARS
al 002064 .__RUNTIME_RUN__
al 0024AF .__BYTECODE_RUN__
al 00250F .EXB_FREE
```

The program, with its runtime, uses $2000 to $250E: about 1.3 KB.

The program must end below $BC20, the bottom of the `GRAPHICS 0` screen
on a 48 KB machine, or the linker reports an error. On a machine with
48 KB or more and no cartridge, MEMTOP is $BC1F in `GRAPHICS 0`, so the
free memory is roughly $BC1F - `EXB_FREE` bytes. A `GRAPHICS 8` screen
takes about 8 KB more.

The DOS loaded before the program uses the memory up to MEMLO (743, 744).
Most DOS versions end below $2000. If yours ends higher, compile with a
higher start address, for example `-S:$2800`.

The first block of the file is a check that DOS runs (through INITAD,
738-739) before it loads the rest: if MEMLO is above the start address, or
`EXB_FREE` is above MEMTOP + 1, it prints `ERROR- 19` and goes
back to DOS, before anything is overwritten (see [Appendix
B](B-error-messages.md#error-6-not-loaded)). The check is in page zero,
where the interpreter loop is loaded next. A linker configuration of your
own (`-C:`) needs the `LDCHK` memory area and `initad` line of
`exbasic.cfg`.

### Cartridge programs

With `-t:atari-cart` the program is an 8 KB cartridge image (`.rom`) for
the address $A000 to $BFFF:

| Address         | Contents                                                           |
| --------------- | ------------------------------------------------------------------ |
| $A000           | `JUMPTAB`, `RUNTIME`, `BYTECODE`, `CODE`, `ALIGNDATA`              |
| (in ROM)        | A copy of the interpreter loop and of the `DATA` segment           |
| $BFFA to $BFFF  | Cartridge header (start address and flags)                         |
| $2000 (RAM)     | `HEAP` and `HEAPA` (variables), `DATA` (copied from ROM at start), `BSS` |
| `EXB_FREE`      | Free memory for arrays and strings, up to MEMTOP                   |

With the cartridge inserted, RAM ends at $9FFF, so MEMTOP is about $9C1F
in `GRAPHICS 0`. The RAM start can be moved with `-S:`. The cartridge
flags byte (default 5: boot the disk first, then start the cartridge) can
be changed with `-DL:__CARTFLAGS__=4`, which starts without booting the
disk.

### Zero page

The zero page ($0000 to $00FF) is the fastest memory of the 6502.

| Address         | Used by                                                          |
| --------------- | ---------------------------------------------------------------- |
| $00             | Set to 0 at start, used as the empty string                      |
| $01 to $7F      | The OS                                                           |
| $80 to $9B      | The interpreter main loop and the `MOVE` loop (code)              |
| $9C to `__ZP_LAST__` - 1 | exBASIC variables (stack pointer, temporaries, `COLOR`, I/O channel, error, ...) and the zero page of linked modules |
| up to $D3       | Not used by the program                                          |
| $D4 to $FF      | The OS floating point routines                                   |

`__ZP_LAST__` is in the label file; it is usually between $AB and $B8.
The bytes from there to $D3 are free, for example $CB to $D1, the area
that Atari BASIC also leaves free. In an assembly module, reserve zero
page with `.zeropage` and `.res` instead of choosing fixed addresses: the
linker then places it after the exBASIC variables (up to $D3).

Don't use $D4 to $FF in your own code: the OS floating point routines
use them, and exBASIC calls those routines for floating point numbers and
some number conversions.

### Other low memory

| Address         | Used by                                                          |
| --------------- | ---------------------------------------------------------------- |
| $0100 to $01FF  | The 6502 stack (cartridges also keep the `USR` address at $0100) |
| $0200 to $047F  | The OS                                                           |
| $0480 to $04CF  | The exBASIC integer stack (40 values)                            |
| $04D0 to $057D  | Not used by exBASIC                                              |
| $057E to $05FF  | Line buffer: `INPUT`, number conversion, file names, substrings  |
| $0600 to $067F  | Long substrings (`S$[I, N]` up to 255 characters)                |
| $0680 to $06EF  | Free                                                             |
| $06F0 to $06F5  | `RUN` and `COMMON` information (see [chapter 17](17-program-organization.md)) |
| $0600 to $06FF  | All of page 6 is overwritten while `RUN` loads the next program  |

Page 6 is the traditional free page for machine code in Atari BASIC. In
exBASIC only $0680 to $06EF is safe at all times; the lower half is safe
if the program does not take substrings longer than 128 characters.

## Useful OS and hardware locations

| Address        | Name     | Contents                                                     |
| -------------- | -------- | ------------------------------------------------------------ |
| 18, 19, 20     | RTCLOK   | Frame counter (see `TIME` in [chapter 15](15-sound-and-controllers.md)) |
| 77             | ATRACT   | Attract mode timer; write 0 to stop the color cycling        |
| 88, 89         | SAVMSC   | Address of the screen memory                                 |
| 106            | RAMTOP   | Top of RAM, in pages (160 with an 8 KB cartridge)            |
| 559            | SDMCTL   | DMA control; 0 turns the screen off                          |
| 560, 561       | SDLSTL   | Address of the display list                                  |
| 623            | GPRIOR   | P/M priority and GTIA mode                                   |
| 704 to 712     | PCOLR0..COLOR4 | Color registers                                        |
| 741, 742       | MEMTOP   | Top of free memory                                           |
| 743, 744       | MEMLO    | Bottom of free memory after DOS                              |
| 752            | CRSINH   | 1 hides the text cursor                                      |
| 756            | CHBAS    | Page of the character set                                    |
| 764            | CH       | Last key pressed, 255 if none                                |
| $D01F (53279)  | CONSOL   | Console keys: bit 0 START, bit 1 SELECT, bit 2 OPTION (0 = pressed) |
| $D20A (53770)  | RANDOM   | A random byte on each read                                   |
| $D40A (54282)  | WSYNC    | Wait for the end of the scan line                            |
| $D40B (54283)  | VCOUNT   | Current scan line / 2                                        |
| $E000 (57344)  | (ROM)    | The ROM character set                                        |

See also the register tables in [chapter 12](12-graphics.md#useful-screen-locations),
[chapter 13](13-player-missile-graphics.md#pm-registers) and
[chapter 14](14-display-list-interrupts.md#useful-registers).
