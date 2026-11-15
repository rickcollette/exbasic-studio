# 20. Performance and Memory

This chapter explains how a compiled exBASIC program runs, what is fast and
what is slow, how the compiler decides which code becomes 6502 machine
code and how you can influence it. It then shows how to measure the
memory a program uses, what each kind of data costs, and how to profile a
program to find where its time goes. It ends with benchmark figures and a
list of practical tips.

## How a program runs

The compiler translates a program to *bytecode*: a compact list of simple
instructions, such as "load variable", "add", "print string" or "jump if
false". At run time, a small interpreter reads the instructions one by one
and calls the 6502 routine of each. The interpreter loop lives in zero
page and each instruction is dispatched through a jump table, so the
overhead per instruction is small, but it is still there: an empty
`FOR` loop iteration costs about 140 cycles as bytecode.

The compiler also translates the integer parts of the program to 6502
machine code ("native code"). Machine code does the same work without the
interpreter in between, and also keeps values in the CPU registers between
operations. The same empty loop iteration costs about 40 cycles.

A program can mix both freely. When the machine code reaches a statement
that has no machine code version, such as `PRINT`, it switches to the
interpreter, which runs the bytecode of that statement and switches back.
Each switch costs about as much as two bytecode instructions, so the
compiler does not switch for very short runs of machine code.

The integer code that can be machine code is:

- loading and storing integer variables and constants,
- `+`, `-`, `*`, `/`, `MOD`, unary `-`, `ABS`,
- the bitwise operators `&`, `!` and `EXOR`,
- comparisons and the logical operators `AND`, `OR` and `NOT`,
- `PEEK`, `POKE`, `DPEEK`, `DPOKE`,
- reading and writing elements of `WORD` and `BYTE` arrays,
- `INC` and `DEC`,
- the control of `IF`, `FOR`/`NEXT`, `WHILE`, `REPEAT`, `DO`/`LOOP` and
  `EXIT`,
- calls of a `FUNC` and `RETURN`.

Everything else stays bytecode: printing and input, strings, floating
point, file I/O, graphics, sound, `USR`, `MOVE`, `MSET` and the other
statements that call the operating system or bigger runtime routines.
These are slow because of the work they do, not because of the
interpreter, so machine code would gain little for them.

## What is fast and what is slow

The following table shows the approximate cost in 6502 cycles of some
statements, measured inside a `FOR` loop with `exbprof` (the cost of the
loop itself is subtracted). The operands are `X = 123`, `Y = 45`,
`P% = 1.5`, `Q% = 2.5` and `S$ = "HELLO"`. The "machine code" column is
the default build; the "bytecode" column is `-no-native`.

| Statement             | Machine code | Bytecode |
|-----------------------|-------------:|---------:|
| (empty loop: `FOR`/`NEXT`) |      40 |      141 |
| `INC Z` or `Z = Z + 1`|            9 |       79 |
| `POKE 1536, X`        |           14 |      148 |
| `Z = PEEK(1536)`      |           27 |      186 |
| `Z = X + Y`           |           32 |      212 |
| `Z = LEN(S$)`         |           31 |      208 |
| `BB(5) = X` (byte array) |        44 |      282 |
| `IF X > Y THEN Z = 1` |           46 |      396 |
| `W(5) = X` (word array) |         54 |      297 |
| `Z = X / 4`           |           86 |      290 |
| `Z = X / Y`           |          130 |      612 |
| `Z = X MOD Y`         |          134 |      608 |
| `Z = F(X)` (a `FUNC` returning `N + 1`) | 156 | 748 |
| `T$ = S$`             |          221 |      305 |
| `Z = X * Y`           |          279 |      525 |
| `T$ = STR$(X)`        |          527 |      609 |
| `T$ = S$ + S$`        |          961 |    1,045 |
| `R% = P% + Q%`        |        1,092 |    1,162 |
| `R% = X` (integer to floating point) | 1,309 | 1,393 |
| `R% = P% * Q%`        |        2,789 |    2,859 |
| `R% = P% / Q%`        |        6,044 |    6,114 |

The Atari CPU runs at about 1.79 MHz, so one screen frame (1/60 s on NTSC
machines) has about 30,000 cycles, fewer when the display is on. Some
conclusions:

- Integer code in machine code is 3 to 10 times faster than as bytecode.
- Multiplication and division are much slower than addition on the 6502,
  which has no instructions for them. Division by a constant power of two
  (`X / 4`) is faster than a general division.
- Floating point is 30 to 200 times slower than integer arithmetic, and
  machine code does not help it. Even converting an integer to floating
  point costs more than a floating point addition.
- String operations copy whole strings and take hundreds of cycles.
  Joining strings with `+` builds the result in a hidden string first.
- A `FUNC` call costs about as much as a division.

## Integer versus floating point

Integer variables are the default because they are fast and small: 2
bytes each, and arithmetic in tens of cycles. Floating point variables
(names ending in `%`) use the 6-byte BCD format of the Atari floating
point package, and each operation takes thousands of cycles. Use floating
point only where you need fractions or numbers outside -32768 to 32767,
and keep the conversions between the two types out of inner loops.

The floating point code is only included in the program when the program
uses floating point, so an integer program saves the memory of the
floating point runtime as well. See [Floating Point](09-floating-point.md).

## Native code

### How the compiler selects the hot code

With `-native:hot`, the compiler estimates how many times each
instruction of the program runs, without running it:

1. Each loop multiplies the count of the instructions inside it by its
   estimated number of iterations:
   - a `FOR` loop with constant start, limit and step runs
     (limit - start) / step + 1 times (a negative constant step, such as
     `STEP -1`, counts as constant);
   - a `FOR` loop up to the variable of a `FOR` loop around it, as
     `FOR J = 0 TO I` inside `FOR I = 198 TO 0 STEP -1`, runs half as
     many times as that loop, on average;
   - any other `FOR` loop, and any `WHILE`, `REPEAT` or `DO` loop that
     has a way out (a condition or an `EXIT`), is estimated at 16
     iterations;
   - a loop without any way out, such as the `DO`/`LOOP` main loop of a
     game, is estimated at 1000 iterations.
2. The code of a `FUNC` is multiplied by the number of times it is
   called: the highest count among the statements that call it. A
   recursive `FUNC` is counted as called 1000 times for each call from
   outside.
3. The highest count in the program is the "hottest" code. The compiler
   compiles to machine code the instructions whose count is at least 64
   and at least 1/64 of the hottest count.

Then it removes runs of fewer than 3 machine code instructions without a
jump, as the switch to and from the interpreter would cost more than they
gain, and it keeps a `FUNC` call as bytecode when the `FUNC` itself is not
hot.

You can see the estimates with the `EXBASIC_HOT_DEBUG` environment
variable. For `samples/sieve.bas`, shown here as distributed with the
compiler (the line numbers below refer to this file):

```basic
? "Starting!"
NumIter = 10
sTime = TIME
' Arrays are initialized to 0
DIM A(8190) Byte
FOR Iter= 1 TO NumIter
  MSET Adr(A), 8190, 0
  Count = 0
  FOR I = 0 TO 8190
    IF NOT A(I)
      Prime = I + I + 3
      FOR K = I + Prime TO 8190 STEP Prime
        A(K) = 1
      NEXT K
      INC Count
    ENDIF
  NEXT I
NEXT Iter

eTime = TIME
? "End."
? "Elapsed time: "; eTime-sTime; " in "; NumIter; " iterations."
? "Found "; Count; " primes."
```

```sh
EXBASIC_HOT_DEBUG=1 exbasic -native:hot sieve.bas
```

Output (first lines):

```
func  called 1 times
loop at line 12: 16 trips, weight 2.0969e+06
loop at line 9: 8191 trips, weight 131056
loop at line 6: 16 trips, weight 16
max weight 2.0969e+06, compiling from 32764
```

The outer loop (line 6) has a variable limit, so it is estimated at 16
iterations. The loop at line 9 has constant limits: 8191 iterations,
16 x 8191 = 131056 executions. The inner loop (line 12) has a variable
start and step: 16 more iterations, about two million executions. The
limit is 1/64 of that, 32764, so the two inner loops are compiled and the
setup code is not. (The real program runs the inner loop fewer times; the
estimate only needs to find which code is hot relative to the rest.)

### Loops that gain little

Machine code only speeds up the integer instructions. In a loop where
most of the time goes to floating point, strings, `PRINT` or graphics,
which stay bytecode, the machine code saves little time and adds its
size, and each switch between machine code and bytecode costs time too.

So the compiler also makes a third, "lean" selection: from the hot code,
it leaves in bytecode each loop where the machine code would save less
than 10% of the loop's time. It estimates the time of each instruction:
about 40 cycles for most bytecode instructions, and from hundreds to
tens of thousands of cycles for floating point operations, string
operations and input and output. It saves about 28 cycles for each
instruction compiled to machine code, less the cost of each switch to
bytecode. When what is left would save less than 10% of the time of the
whole program, the lean selection has no machine code at all, so the
program doesn't include the support routines of the machine code either.

With `EXBASIC_HOT_DEBUG`, the compiler shows this decision for each
loop, for example for Ahl's benchmark (`samples/ahlbench.bas`):

```
loop at line 6: saves 50000 of 33080000 cycles: bytecode
```

### The default mode

The best choice depends on the program, so by default the compiler builds
the program six times: with `-native:all`, with `-native:hot` and with
the lean selection (the hot code without the loops that gain little),
each with and without the extended instructions (see
[-no-ext](19-compiler-reference.md#-no-ext)). It keeps the smallest
executable, and on a tie the faster one (see [Native code
modes](19-compiler-reference.md#native-code-modes)). Memory is the first
concern: the build kept can be slower than another one that is only a
little bigger. The `-native:all` build is usually the fastest, and it
wins when it is also the smallest or ties. This happens more often than you may expect:
machine code does not need the interpreter routines of the instructions
it replaces, so in a small program where most of the code is in loops,
all machine code can be the smallest form.

| Program        | Default          | `-native:hot` | `-native:all` | `-no-native` |
|----------------|------------------|--------------:|--------------:|-------------:|
| `sieve.bas`    | all, 1434 bytes  |          1570 |          1434 |         1443 |
| `pi.bas`       | hot without extended instructions, 3904 bytes |          4156 |          5467 |         2761 |
| `carrera3d.bas`| lean without extended instructions, 2995 bytes |          4234 |          5071 |         3151 |
| `joyas.bas`    | lean without extended instructions, 3040 bytes |          4112 |          5028 |         3167 |

(Sizes of the `.xex` files of the samples. The `-native:hot` and
`-native:all` columns are with the extended instructions.)

### Choosing a mode

- Use the default for most programs. It keeps the smallest of the six
  builds; integer loops that run often are still machine code.
- Use `-native:hot` or `-native:all` when speed matters more than memory.
  For `pi.bas` the default leaves out the extended instructions:
  `-native:hot` is 21% faster for 252 bytes more, and `-native:all` 26%
  faster for 1563 bytes more.
- Use `-no-native -no-ext` for the smallest program of all, when speed
  does not matter: `sieve.bas` is then 97 bytes smaller and 3.7 times
  slower. For programs that spend little time in integer loops, such as
  the games `carrera3d.bas` and `joyas.bas`, the default is already this
  build.

If the program does not fit in memory, the compiler retries without the
extended instructions and then without machine code (see [Automatic
retries](19-compiler-reference.md#automatic-retries)).

### Helping the selection

The estimate is simple, so the way you write loops affects which code
becomes machine code:

- Give `FOR` loops constant limits when you can. `FOR I = 0 TO 199` is
  known to run 200 times; `FOR I = 0 TO N` is assumed to run 16 times
  (unless `N` is the variable of a `FOR` loop around it). A loop with a
  constant start, limit and step that does not change its variable or
  call a `FUNC` also gets shorter and faster machine code: its `NEXT`
  compares with the limit directly and needs no overflow check.
- Put the work in loops, not in long sequences of straight-line code.
  Code outside loops runs once and stays bytecode, which is what you want
  for setup code.
- A `FUNC` called from a hot loop becomes hot too. Small helper functions
  called in inner loops are compiled to machine code with the loop, and
  the call itself is then machine code.
- Statements that stay bytecode (`PRINT`, strings, floating point) in the
  middle of a hot loop force switches between machine code and the
  interpreter. Move them out of the inner loop when possible. A loop where
  they take most of the time stays bytecode in the default mode (see
  [Loops that gain little](#loops-that-gain-little)).

The environment variables `EXBASIC_HOT_RATIO` and `EXBASIC_HOT_MIN`
change the 1/64 ratio and the minimum of 64 for experiments. For example,
`EXBASIC_HOT_RATIO=16` compiles less code. They are not needed for normal
use.

## Benchmarks

The following figures compare exBASIC with the original FastBasic
compiler on the same programs, run in the 6502 simulator of the test
suite. "Cycles" are 6502 cycles to run the program (in millions); "RAM"
is the memory used by the program in bytes, from the start address to
`EXB_FREE` (code, runtime and variables, before arrays and strings).

| Program                          | FastBasic cycles | FastBasic RAM | `-no-native` cycles | `-no-native` RAM | Default cycles | Default RAM | Speed vs FastBasic |
|----------------------------------|-----------------:|------:|------:|------:|------:|------:|-----:|
| Sieve of Eratosthenes            | 132.7 | 1298 | 100.5 | 1339 | 27.1 | 1330 | 4.9x |
| Pi to 254 digits                 | 138.8 | 2340 | 100.7 | 2683 | 47.0 | 3826 | 3.0x |
| Ahl's benchmark (floating point) |  50.9 | 1820 |  50.8 | 1866 | 50.9 | 1787 | 1.0x |
| Bubble sort, 200 numbers         |  32.5 | 1290 |  31.6 | 1293 |  6.5 | 1449 | 5.0x |
| Integer Mandelbrot               |  41.5 | 1298 |  30.6 | 1446 | 16.3 | 1245 | 2.5x |
| Number to text (`STR$`)          |   6.4 | 1030 |   5.7 | 1224 |  4.2 |  978 | 1.5x |
| `PEEK` / `POKE`                  |   5.9 | 1032 |   4.6 | 1180 |  1.0 | 1003 | 6.1x |
| `FUNC` calls and recursion       |     - |    -  |  17.5 | 1993 |  8.0 | 1990 |  -   |

The `FUNC` benchmark has no FastBasic figure because FastBasic has no
functions with parameters and results. The floating point benchmark
shows no gain: its time goes into the floating point routines.

Even without machine code, exBASIC is faster than FastBasic for integer
code, thanks to the extended instructions. With the default machine code
selection, integer programs run 1.5 to 6.1 times faster than with
FastBasic. Most of them use about as much memory as with FastBasic or
less: the error checks (division by zero, stack overflow, the BREAK key)
take some memory, but the machine code needs no interpreter routines for
the instructions it replaces. The pi program is the exception: its
machine code takes 1490 bytes more than FastBasic.

Machine code makes bigger programs bigger. The RAM used by the sample
games and applications:

| Program                  | FastBasic | `-no-native` | Default |
|--------------------------|----------:|-------------:|--------:|
| Carrera 3D (game)        |      2858 |         3073 |    2917 |
| Joyas (game)             |      2854 |         3085 |    2958 |
| Fedora (game)            |      2630 |         2705 |    2626 |
| Mastodon client          |      2062 |         2033 |    2288 |
| Network client           |      2322 |         2300 |    2448 |
| Display list demo        |      1798 |         2002 |    1916 |

Compared with the first exBASIC 1.0 compiler, the default selection
(loops that gain little stay bytecode, extended instructions only when
they make the program smaller), the shorter machine code of `FOR` loops
and the smaller runtime make these programs 120 to 1590 bytes smaller:
Carrera 3D went from 4506 to 2917 bytes.

### Against Atari BASIC, Altirra BASIC and FastBasic

These figures compare four BASICs on an emulated Atari 800XL (Altirra
4.40). The test machine has 64 KB of RAM, NTSC video, the Atari XL OS
rev. 2 with its floating point routines, and no DOS. The interpreters are
Atari BASIC rev. C and Altirra BASIC (the one built into the emulator).
The compilers are FastBasic 4.7, with its integer target except for Ahl's
benchmark, and exBASIC with its default options. Each program is written
the usual way for each BASIC with the same algorithm, and computes a check
value. All four BASICs give the same check value (except the accuracy
figure of Ahl's benchmark, which depends on the floating point code).
Each program times itself with the frame counter. The screen is on, as
in normal use.

Time in seconds:

| Program                                  | Atari BASIC | Altirra BASIC | FastBasic | exBASIC | exBASIC vs Atari BASIC |
|------------------------------------------|------:|------:|------:|------:|-----:|
| Sieve of Eratosthenes, 8191 flags | 210.9 | 129.8 | 11.92 | 2.39 | 88x |
| Bubble sort, 200 numbers | 449.1 | 215.7 | 28.37 | 5.96 | 75x |
| GCD of 1600 pairs (`MOD`, loops) | 137.0 | 91.6 | 7.71 | 2.29 | 60x |
| 3000 subroutine calls | 24.5 | 14.1 | 1.02 | 0.60 | 41x |
| `POKE` and `PEEK` 4096 bytes | 69.4 | 43.6 | 4.21 | 0.65 | 107x |
| Build and scan 100-char strings, 20 times | 72.2 | 52.2 | 3.89 | 2.42 | 30x |
| Ahl's benchmark (floating point) | 404.6 | 296.8 | 79.25 | 79.30 | 5.1x |
| `PRINT` 300 lines to the screen | 16.6 | 15.6 | 15.17 | 15.17 | 1.1x |
| 160 `DRAWTO` lines in `GRAPHICS 8` | 20.8 | 20.4 | 19.64 | 19.64 | 1.1x |

Overall speed against Atari BASIC (geometric mean of the ratios):

| Programs                        | Atari BASIC | Altirra BASIC | FastBasic | exBASIC |
|---------------------------------|-----:|-----:|------:|------:|
| All nine                        | 1.0x | 1.5x |  8.4x | 18.9x |
| The seven without screen output | 1.0x | 1.6x | 15.2x | 42.8x |

How to read these figures:

- **Integer code:** exBASIC is 30 to 107 times faster than Atari BASIC,
  22 to 67 times faster than Altirra BASIC, and 1.6 to 6.5 times faster
  than FastBasic.
- **Floating point (Ahl's benchmark):** the time goes into the floating
  point routines of the OS. The two compilers are 5 times faster than
  Atari BASIC because they don't interpret the program text, but exBASIC
  is not faster than FastBasic. Altirra BASIC is faster than Atari BASIC
  here because it has its own faster floating point code.
- **Screen output (`PRINT`, graphics):** the OS draws the screen in all
  four BASICs, so they take nearly the same time.

Memory used by the running program, in bytes. For the interpreters this
is the program, variables, arrays and strings (`LOMEM` to the top of
BASIC's memory). For the compilers it is the program, its runtime,
variables, arrays and strings, from the start address (`$2000`) to the
first free byte:

| Program                                  | Atari BASIC | Altirra BASIC | FastBasic | exBASIC |
|------------------------------------------|-----:|-----:|-----:|-----:|
| Sieve of Eratosthenes, 8191 flags | 9072 | 9072 | 9487 | 9583 |
| Bubble sort, 200 numbers | 2193 | 2192 | 1954 | 2319 |
| GCD of 1600 pairs | 785 | 785 | 1298 | 1475 |
| 3000 subroutine calls | 679 | 679 | 1290 | 1211 |
| `POKE` and `PEEK` 4096 bytes | 4830 | 4830 | 5390 | 5378 |
| Build and scan strings | 911 | 911 | 2062 | 2191 |
| Ahl's benchmark | 854 | 854 | 1822 | 1893 |
| `PRINT` 300 lines | 674 | 674 | 1290 | 1147 |
| 160 `DRAWTO` lines | 763 | 763 | 1292 | 1442 |

Free memory while the program runs (`FRE`), in bytes:

| Program                                  | Atari BASIC | Altirra BASIC | FastBasic | exBASIC |
|------------------------------------------|------:|------:|------:|------:|
| Sieve of Eratosthenes, 8191 flags | 29103 | 29103 | 30480 | 30384 |
| Bubble sort, 200 numbers | 35982 | 35983 | 38013 | 37648 |
| GCD of 1600 pairs | 37390 | 37390 | 38669 | 38492 |
| 3000 subroutine calls | 37496 | 37496 | 38677 | 38756 |
| `POKE` and `PEEK` 4096 bytes | 33345 | 33345 | 34577 | 34589 |
| Build and scan strings | 37264 | 37264 | 37905 | 37776 |
| Ahl's benchmark | 37321 | 37321 | 38145 | 38074 |
| `PRINT` 300 lines | 37501 | 37501 | 38677 | 38820 |
| 160 `DRAWTO` lines | 37412 | 37412 | 38675 | 38525 |

How these memory figures compare:

- **Program size:** a compiled program carries its own runtime, so a
  small program uses about 130 to 1280 bytes more in exBASIC than in
  Atari BASIC. Against FastBasic, exBASIC uses from 143 bytes less to 365
  bytes more: the machine code of the integer loops and the error checks
  cost memory, while the smaller runtime saves it.
- **Free memory:** even so, every program has more free memory compiled
  than in Atari BASIC: 510 to 1670 bytes more with exBASIC, and 600 to
  2000 bytes more with FastBasic. With no BASIC cartridge, the 8 KB from
  `$A000` to `$BFFF` is RAM, which makes up for the runtime.
- **With a DOS:** the figures above are without a DOS. A DOS takes its
  memory from the interpreters' free memory, but not from a compiled
  program's while `MEMLO` stays below `$2000`, which adds 5 to 6 KB more
  to the compilers' advantage.
- **File size:** a saved Atari BASIC program (`SAVE`) is 440 to 760
  bytes. A FastBasic executable is 1100 to 1700 bytes, and an exBASIC
  executable 1260 to 2040 bytes.

To run these benchmarks again, use `make bench` in the source tree. It
needs wine, Xvfb, Altirra, and the Atari XL OS and Atari BASIC ROM images
(see `bench/bench.py`). The programs are in `bench/atari` (Atari BASIC
and Altirra BASIC), `bench/fastbasic` and `bench/exbasic`.

## Memory

### Where the memory goes

An executable is loaded at the start address, `$2000` by default (see
[-S](19-compiler-reference.md#-s)). From there up, memory holds:

1. the interpreter jump table (2 bytes for each bytecode instruction the
   program uses) and, in the same page if they fit, the variables;
2. the runtime: the routines of the instructions and statements the
   program uses, and nothing else;
3. the bytecode and machine code of the program, including the `DATA`
   arrays;
4. runtime data such as the `FUNC` frame stack (only in programs with
   recursive functions);
5. the variables, when they did not fit in the jump table page;
6. then the free memory, from the label `EXB_FREE` up to `MEMTOP`.

Arrays and strings are allocated at run time in the free memory, from
`EXB_FREE` up, by `DIM` and by the first assignment to each string.
`MEMTOP` is set by the operating system below the screen memory, so it
depends on the graphics mode: a `GRAPHICS 8` screen takes about 8 KB more
than the text screen. Player/missile graphics also take memory below
`MEMTOP`. See [Memory and Machine Code](16-memory-and-machine-code.md)
for the full memory map.

### Measuring memory

To measure the memory used by the program itself, compile it with `-g`
and look at the label file. The label `EXB_FREE` is the first free byte,
and `__STARTADDRESS__` is the start address:

```sh
exbasic -g sieve.bas
grep -E 'EXB_FREE|__STARTADDRESS__' sieve.lbl
```

Output:

```
al 002507 .EXB_FREE
al 002000 .__STARTADDRESS__
al 002507 .EXB_FREE
```

The program uses `$2507 - $2000 = $507` = 1287 bytes. (Some labels appear
twice in the file.) The size of the `.xex` file is close to this number,
plus a few bytes of headers and the zero page interpreter.

To measure the memory left at run time, use `FRE()`. It returns the bytes
between the top of the allocated arrays and strings and `MEMTOP`, so it
goes down as the program allocates them. It is never shown negative:
`PRINT FRE()` prints the real value, also above 32767 (see
[FRE()](05-expressions.md#fre)):

```basic
' Show the free memory
PRINT "Free memory: "; FRE(); " bytes"
```

Output (in the simulator, with `MEMTOP` at `$BC00`):

```
Free memory: 40214 bytes
```

You can also measure how much memory a part of the program allocates, by
comparing `FRE()` before and after it, as in the next example.

### What each kind of data uses

| Item                          | Memory                                                        |
|-------------------------------|---------------------------------------------------------------|
| integer variable              | 2 bytes in the variable area                                  |
| floating point variable       | 6 bytes in the variable area                                  |
| string variable               | 2 bytes, plus 256 bytes allocated at its first assignment     |
| `DIM A(N)` (word array)       | 2 bytes, plus 2 x (N + 1) bytes allocated by `DIM`            |
| `DIM A(N) BYTE`               | 2 bytes, plus N + 1 bytes                                     |
| `DIM A%(N)` (floating point)  | 2 bytes, plus 6 x (N + 1) bytes                               |
| `DIM A$(N)` (string array)    | 2 bytes, plus 2 x (N + 1) bytes, plus 256 bytes for each element at its first assignment |
| `DATA` array                  | its data, inside the program                                  |
| `FUNC` parameters and `LOCAL` variables | like other variables of the same type               |
| hidden string for `+`         | 256 bytes at its first use                                    |
| recursion frame stack         | 512 bytes, only in programs with recursive `FUNC`s that have parameters or locals |

These figures were measured with `FRE()`:

```basic
' Measure the memory taken by arrays and strings
Before = FRE()
DIM Scores(99)
PRINT "DIM Scores(99):         "; Before - FRE()

Before = FRE()
DIM Flags(99) BYTE
PRINT "DIM Flags(99) BYTE:     "; Before - FRE()

Before = FRE()
DIM Speeds%(99)
PRINT "DIM Speeds%(99):        "; Before - FRE()

Before = FRE()
DIM Names$(9)
PRINT "DIM Names$(9):          "; Before - FRE()

Before = FRE()
Names$(1) = "ANA"
PRINT "Names$(1) = ""ANA"":     "; Before - FRE()

Before = FRE()
Title$ = "HELLO"
PRINT "Title$ = ""HELLO"":      "; Before - FRE()

Before = FRE()
Title$ = "WORLD"
PRINT "Title$ = ""WORLD"":      "; Before - FRE()
```

Output:

```
DIM Scores(99):         200
DIM Flags(99) BYTE:     100
DIM Speeds%(99):        600
DIM Names$(9):          20
Names$(1) = "ANA":     256
Title$ = "HELLO":      256
Title$ = "WORLD":      0
```

Notes:

- A string always takes 256 bytes once assigned, whatever its length. A
  program with many string variables can run out of memory quickly; a
  `BYTE` array or a `DATA` array is much smaller for fixed text or tables.
- Each chain of `+` in a statement builds its result in a hidden 256-byte
  string. The hidden strings are reused from one statement to the next,
  but the main program and each `FUNC` that uses `+` have their own. The
  first `C$ = A$ + B$` of a program allocates 512 bytes: 256 for `C$`
  and 256 for the hidden string.
- The variable area has room for 256 two-byte slots (512 bytes). Each
  integer, string or array variable takes one slot and each floating point
  variable three. The parameters and local variables of every `FUNC` and
  the hidden strings also take slots. A program with too many variables
  gives the compile error `too many variables, they use N bytes and the
  limit is 512 ...`.
- Recursion saves the parameters and locals of the recursive `FUNC` in
  the 512-byte frame stack: 2 bytes for each integer, 6 for each floating
  point value and the length of the text plus 1 for each string, per
  level. The 6502 stack limits the depth to about 95 levels; deeper
  recursion stops with error 10 (`ERROR- 10`). See [Functions](07-functions.md).
- When an allocation does not fit below `MEMTOP`, the program stops with
  error 2, `ERROR- 2` (see [Runtime errors](B-error-messages.md#runtime-errors)).

### Getting more memory

- Compile with `-no-native` (and `-no-ext`) if the program is big and does
  not spend its time in integer loops.
- Use `BYTE` arrays for values from 0 to 255, and `DATA` arrays for
  constant tables: they are part of the program and are not copied.
- Reuse string variables, and avoid string arrays with many elements.
- Avoid floating point in programs that don't need it: the floating point
  runtime and 6-byte variables are only included when used.
- If the program runs without DOS, a lower start address (`-S`) gives
  more free memory. A cartridge (`-t:atari-cart`) keeps the code in ROM,
  leaving the RAM for data.

## Profiling with exbprof

The test suite includes `exbprof`, a profiler that runs a compiled
program in a 6502 simulator with an emulation of the Atari operating
system, and reports where the time goes. It is built with `make profile`
in the source distribution, as `build/bin/exbprof`. The simulator has no
keyboard, graphics display or sound: it is suited to programs that compute
and print.

```sh
exbasic -g prog.bas
exbprof [-q] [-a] [-c max_cycles] [-d folder] prog.xex prog.lbl
```

| Option | Meaning                                                            |
|--------|--------------------------------------------------------------------|
| `-q`   | don't show the output of the program                               |
| `-a`   | use the original Atari floating point package (slower, as on a real Atari) |
| `-c`   | stop after the given number of cycles, for programs that never end |
| `-d`   | folder used as the `D:` disk                                       |

The profiler prints the output of the program, the total number of
cycles, and then the cycles spent in each runtime routine and the most
executed bytecode instructions and pairs of instructions. For
`sieve.bas`:

```
Starting!
End.
Elapsed time: 959 in 10 iterations.
Found 1899 primes.

sieve.xex: 26548849 cycles (14.83 s at 1.79MHz, without DMA)
49 instructions dispatched, 541813.2 cycles per instruction

Cycles per runtime routine:
   91.28%  (machine code of loops)
    4.94%  pushAX
    3.75%  alloc_array
```

Here 91% of the time is in machine code: the program is already as fast
as exBASIC can make it, and only a better algorithm would help. When a
program spends most of its time in bytecode routines such as `EXE_PRINT_STR`
or floating point routines, the profile shows which ones. The time in
seconds does not count the cycles taken by the display ("DMA"), so a real
Atari is slower with the screen on: the video chip takes memory cycles
from the CPU to read the screen.

To compare versions of a program, compile each with `-g` and compare the
cycle counts. This is how the tables in this chapter were measured.

## Tips

For speed:

1. Use integers. Use floating point only when you need it, and convert
   between the types outside loops.
2. Give `FOR` loops constant limits, and keep the work of inner loops in
   integer statements, so the compiler compiles them to machine code.
3. Use `INC X` and `DEC X`, or `X = X + 1`: both are a single
   instruction.
4. Prefer `FOR` loops to `WHILE` loops that count: `NEXT` is one
   combined instruction.
5. Divide by constant powers of two (`X / 2`, `X / 256`) instead of
   variables when you can. To get both the quotient and the remainder of
   the same division in a `FOR` loop, write `Q = X / D` and `R = X MOD D`
   with the same operands: the division is done once.
6. Use `BYTE` arrays for small values: they are smaller and faster than
   word arrays.
7. Move `PRINT`, string operations and floating point out of inner loops.
8. Use `MOVE`, `-MOVE` and `MSET` to copy or fill memory instead of
   loops of `POKE`.
9. Use the screen accelerator `EFAST` of the disk image when the program
   prints a lot of text (see [Getting Started](02-getting-started.md)).
10. Measure. Use `exbprof` on the parts of the program that compute, and
    `TIME` on the real machine.

For memory:

1. Compile with `-g` and watch `EXB_FREE` as the program grows.
2. Try `-no-native` on big programs, and compare the size and speed.
3. Avoid many string variables and big string arrays; each string uses
   256 bytes.
4. Keep constant data in `DATA` arrays.
5. Store `FRE()` in a floating point variable to keep its exact value:
   an integer variable prints values above 32767 as negative numbers.
