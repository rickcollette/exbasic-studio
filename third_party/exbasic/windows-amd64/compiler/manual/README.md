# exBASIC Programmer's Manual

exBASIC 1.0.0 is a compiled BASIC for the Atari 8-bit home computers
(400, 800, XL and XE).  You write programs on a modern computer with any text
editor, and the `exbasic` compiler turns them into Atari executables (`.XEX`)
or cartridge images (`.ROM`).  exBASIC is based on FastBasic by Daniel
Serpell.

This manual describes the whole language, the Atari-specific statements and
the compiler.  Chapters 1 to 5 introduce the language and are best read in
order; the rest of the manual is a reference that you can read in any order.

## Contents

### Learning exBASIC

1. [Introduction](01-introduction.md): what exBASIC is, how it works, and the
   conventions used in this manual.
2. [Getting Started](02-getting-started.md): building the compiler, your first
   program, and running it in an emulator or on a real Atari.
3. [Language Basics](03-language-basics.md): program structure, statements,
   comments, names, keywords and abbreviations.
4. [Variables and Data](04-variables-and-data.md): integer, floating point
   and string variables, arrays and `DATA`.
5. [Expressions](05-expressions.md): operators, precedence, integer and
   boolean arithmetic, and the list of all functions.

### Programming

6. [Control Flow](06-control-flow.md): `IF`, `FOR`, `WHILE`, `REPEAT`,
   `DO`/`LOOP`, `EXIT` and `END`.
7. [Functions](07-functions.md): `FUNC`, parameters, results, `LOCAL`,
   `RETURN` and recursion.
8. [Strings](08-strings.md): string constants and variables, joining,
   slicing, searching, comparing and converting strings, and the string
   functions.
9. [Floating Point](09-floating-point.md): floating point variables,
   precision and the mathematical functions.

### The Atari

10. [Console Input and Output](10-console-io.md): `PRINT`, `INPUT`, `GET`,
    `PUT` and the screen editor.
11. [Files and Devices](11-files-and-devices.md): I/O channels, disk files,
    `OPEN`, `XIO`, `NOTE`, `POINT`, `STATUS` and error codes.
12. [Graphics](12-graphics.md): graphics modes, drawing and colors.
13. [Player/Missile Graphics](13-player-missile-graphics.md): hardware
    sprites.
14. [Display List Interrupts](14-display-list-interrupts.md): changing
    colors and registers in the middle of the screen.
15. [Sound and Controllers](15-sound-and-controllers.md): `SOUND`, joysticks,
    paddles, the keyboard and timing.
16. [Memory and Machine Code](16-memory-and-machine-code.md): `PEEK`, `POKE`,
    `MOVE`, `USR`, assembly modules and the memory map.
17. [Program Organization](17-program-organization.md): `INCLUDE`, `COMMON`
    and `RUN`, and structuring big programs.
18. [SIO and FujiNet](18-sio-and-fujinet.md): direct serial I/O and network
    access.

### The Compiler

19. [Compiler Reference](19-compiler-reference.md): every command line
    option, targets, output files and the build steps.
20. [Performance and Memory](20-performance-and-memory.md): how programs run,
    native code, and how to make programs fast and small.

### Appendices

- [Appendix A. Keyword Reference](A-keyword-reference.md): every keyword with
  its abbreviation and where it is documented.
- [Appendix B. Error Messages](B-error-messages.md): compiler and runtime
  errors, their causes and fixes.
- [Appendix C. Differences from Other BASICs](C-differences.md): Atari BASIC,
  Turbo BASIC XL and FastBasic, with porting tips.
