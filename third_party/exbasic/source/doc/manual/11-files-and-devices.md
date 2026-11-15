# 11. Files and Devices

This chapter explains how exBASIC programs use disk files, the printer
and the other Atari devices through the operating system's Central
Input/Output (CIO) system: the I/O channels, `OPEN`, `CLOSE`,
`PRINT #`, `INPUT #`, `GET #`, `PUT #`, `BGET`, `BPUT`, `XIO`, `NOTE`,
`POINT`, `STATUS` and the `ERR()` function. It ends with reference tables of open modes,
`XIO` commands and error codes, and complete examples. The screen and
keyboard statements without a channel are in
[Console Input and Output](10-console-io.md).

## The Atari I/O system

All input and output on the Atari goes through CIO, a part of the
operating system. A program does not talk to a disk drive or printer
directly. It opens an **I/O channel** on a **device**, and then reads
or writes bytes or lines through that channel. CIO passes the request
to the **handler** of the device, which does the work.

### Channels

There are 8 channels, numbered 0 to 7. Each one has an I/O control
block (IOCB) of 16 bytes in memory, at address $340 + 16 × channel
(832 + 16 × channel), that holds the device, the command, the buffer and
the auxiliary bytes. In exBASIC statements the channel is written with
`#`: `#1`, `#Ch`, `#(N + 1)`.

| Channel | Use in exBASIC |
| ------- | -------------- |
| #0 | Screen editor `E:`, opened by the OS. Used by `PRINT`, `INPUT` and `PUT` without a channel. Do not close it. |
| #1 to #5 | Free for your program. |
| #6 | Opened by [GRAPHICS](12-graphics.md#graphics) on the screen device `S:`; used by `PLOT`, `DRAWTO`, `FILLTO`, `LOCATE`. Free if the program never uses `GRAPHICS`. |
| #7 | Used by [RUN](17-program-organization.md) to load the next program (`RUN` closes it first). Free otherwise. |

When an exBASIC program starts, channels 1 to 7 are closed. When the
program ends, with `END`, at the end of the main code or with a runtime
error, channels 1 to 7 are closed too, so the data written to disk files
is saved. It is still good style to close each file as soon as you have
finished with it, and to check `ERR()` after closing a written file.

### Devices

A device is named by a letter and a colon. Disk devices also take a
drive number and a file name.

| Device | Name | Notes |
| ------ | ---- | ----- |
| `E:` | Screen editor | Keyboard input with line editing, and text output to the screen. Open on channel 0. |
| `S:` | Screen | Text and graphics screen; opened by `GRAPHICS` on channel 6. |
| `K:` | Keyboard | Reads single keys, without echo. |
| `P:` | Printer | Output only. |
| `C:` | Cassette | Program recorder, read or write. |
| `D:` | Disk | Needs DOS. `D:NAME.EXT` is drive 1; `D2:NAME.EXT` is drive 2. Names have up to 8 characters plus a 3 character extension. |
| `R:` | RS-232 | Serial port of the 850 interface (or compatible); needs its handler loaded. |
| `N:` | Network | FujiNet network device; needs the FujiNet `N:` handler loaded. exBASIC can also use FujiNet without the handler, see [SIO and FujiNet](18-sio-and-fujinet.md). |

The device name is given as a string: a constant like `"D:SCORES.DAT"`
or a string variable or expression. It is copied to a buffer of 128
bytes, so it can have up to 127 characters.

### Errors and ERR()

exBASIC has no `TRAP` and never stops on an I/O error. Every I/O
statement stores a status code that you read with [ERR()](#err): 1
means success, and 128 or more is an error (see
[Error codes](#error-codes)). Check `ERR()` after `OPEN`, after reading
data, and after `CLOSE` of a file that was written.

## Statements

### OPEN

Opens an I/O channel on a device.

```
OPEN #channel, mode, aux2, device$
```
Abbreviation: `O.`

Opens a channel on a device or file:

- _channel_: the channel number, 0 to 7.
- _mode_: the first auxiliary byte (AUX1), 0 to 255. It says what the
  channel will be used for: 4 to read, 8 to write, 12 for both, and
  some device specific values (see [Open modes](#open-modes)).
- _aux2_: the second auxiliary byte (AUX2), 0 to 255. Most devices
  ignore it; give 0.
- _device$_: the device or file name, for example `"D:SCORES.DAT"`.

`ERR()` is 1 if the channel was opened. Common errors are 129 (channel
already open), 130 (no such device), 170 (file not found), 165 (bad
file name) and 167 (file locked).

The channel must be closed before it is opened again. **This includes
a failed `OPEN`**: if `OPEN` fails, the channel stays reserved and the
next `OPEN` on it gives error 129 until you `CLOSE` it. Closing a
channel that is not open does no harm, so a good habit is to `CLOSE`
before `OPEN`:

```basic
' Open files safely, checking the error code
Status = OpenRead(1, "D:MISSING.TXT")
IF Status <> 1
  PRINT "MISSING.TXT: error "; Status
ENDIF

IF OpenRead(1, "D:NAMES.TXT") = 1
  INPUT #1, FirstLine$
  PRINT "First line: "; FirstLine$
  CLOSE #1
ENDIF

' Open a file for reading (mode 4), return the error code
FUNC OpenRead(Channel, FileName$)
  LOCAL Status
  CLOSE #Channel
  OPEN #Channel, 4, 0, FileName$
  Status = ERR()
  IF Status <> 1
    CLOSE #Channel
  ENDIF
  RETURN Status
ENDFUNC
```

Output (with the file `NAMES.TXT` written by the
[first example](#writing-and-reading-a-text-file)):

```
MISSING.TXT: error 170
First line: ADA
```

The order of the parameters is the same as Atari BASIC
(`OPEN #1, 4, 0, "D:FILE"`). `OPEN` is the same as
`XIO 3, #channel, mode, aux2, device$`.

### CLOSE

Closes an I/O channel.

```
CLOSE #channel
```
Abbreviation: `CL.`

Ends the use of _channel_. For a file opened to write, the data still
in the DOS buffer is written to the disk and the directory is updated,
so check `ERR()` after closing a written file to be sure the data is
on the disk. Closing a channel that is not open is allowed and gives
`ERR()` = 1.

```basic
OPEN #1, 8, 0, "D:LOG.TXT"     ' 8 = write
PRINT #1, "STARTED"
CLOSE #1
IF ERR() <> 1
  PRINT "Disk error"
ENDIF
```

Do not close channel 0 (the screen editor). Closing channel 6 in a
graphics mode is allowed, but `PLOT` and `DRAWTO` stop working until
the next `GRAPHICS`.

### PRINT #

Writes text to a channel.

```
PRINT #channel, [item] [separator item ...] [separator]
? #channel, ...
```
Abbreviation: `PRI.` or `?`

Works like [PRINT](10-console-io.md#print), but writes to _channel_.
After the channel number write a comma, or a semicolon as in Atari BASIC
(`PRINT #1; "TEXT"`). Each
`PRINT #` line written to a file ends with an EOL (155), unless the
statement ends with `;` or `,`. `PRINT #1,` alone writes only an EOL.

Numbers are written as text, the same as on the screen, so a file
written with `PRINT #` can be read back with `INPUT #`.

`ERR()` holds the status of the last character written.

The comma, `TAB` and `RTAB` separators count columns using the screen
cursor, not the position in the file, so they do not give the expected
spacing in files. Use `;` and print the spaces yourself.

### INPUT #

Reads lines from a channel.

```
INPUT #channel, var [, var ...]
```
Abbreviation: `IN.`

Reads one line (up to the next EOL, character 155) from _channel_ for
each variable. No prompt is printed. As in
[INPUT](10-console-io.md#input), a string variable gets the whole line
without the EOL, an integer variable gets the number at the start of the
line, and a floating point variable gets the line converted to a
floating point number. The variable can also be an array element.

| `ERR()` | Meaning |
| ------- | ------- |
| 1 | A line was read. |
| 18 | The line was read but is not a valid number (numeric variable). |
| 136 | End of file: there was nothing more to read. The variable gets an empty string or a meaningless number. |
| 137 | The line was longer than 255 characters. The variable gets the first 255 characters. |

If the last line of a file has no EOL, `INPUT #` returns that line
**and** `ERR()` = 136. A loop that stops as soon as `ERR()` is not 1
loses that line. When a file may end without an EOL, use the string
before testing the error:

```basic
' Read a file whose last line has no EOL
OPEN #1, 8, 0, "D:NOEOL.TXT"     ' 8 = write
PRINT #1, "LINE 1"
PRINT #1, "LAST";
CLOSE #1

OPEN #1, 4, 0, "D:NOEOL.TXT"     ' 4 = read
DO
  INPUT #1, Line$
  Status = ERR()
  PRINT Status; " ["; Line$; "]"
LOOP UNTIL Status <> 1
CLOSE #1
```

Output:

```
1 [LINE 1]
136 [LAST]
```

The line is read into the OS line buffer at address $580. Lines longer
than 128 characters also use the first half of page 6 ($600 to $67F).

```basic
' Save and load numbers as text, one per line
DIM Score(4), Loaded(4)
FOR I = 0 TO 4
  Score(I) = I * I * 100
NEXT I

OPEN #1, 8, 0, "D:SCORES.TXT"    ' 8 = write
FOR I = 0 TO 4
  PRINT #1, Score(I)
NEXT I
CLOSE #1

OPEN #1, 4, 0, "D:SCORES.TXT"    ' 4 = read
FOR I = 0 TO 4
  INPUT #1, Loaded(I)
NEXT I
CLOSE #1

FOR I = 0 TO 4
  PRINT Loaded(I); " ";
NEXT I
PRINT
```

Output:

```
0 100 400 900 1600 
```

### GET #

Reads bytes from a channel.

```
GET #channel, var [, var ...]
```
Abbreviation: `GE.`

Reads one byte (0 to 255) from _channel_ for each variable. A variable
can be an integer variable or an element of a word or byte array. EOL
characters are read as normal bytes (155).

`ERR()` is 1 if the byte was read. At the end of the file it is 136 and
the variable gets a meaningless value. With several variables, `ERR()`
is the result of the last one.

```basic
' Count the bytes and lines of a file, one byte at a time
Eol = 155
OPEN #1, 4, 0, "D:NAMES.TXT"     ' 4 = read
ByteCount = 0
LineCount = 0
DO
  GET #1, Byte
  IF ERR() <> 1
    EXIT
  ENDIF
  INC ByteCount
  IF Byte = Eol
    INC LineCount
  ENDIF
LOOP
CLOSE #1
PRINT ByteCount; " bytes, "; LineCount; " lines"
```

Output (with the file `NAMES.TXT` made by the examples
[Writing and reading a text file](#writing-and-reading-a-text-file) and
[Appending to a file](#appending-to-a-file)):

```
29 bytes, 4 lines
```

`GET #` on the keyboard device `K:` reads keys:

```basic
' Read keys through the K: device on channel 1
EscKey = 27
OPEN #1, 4, 0, "K:"              ' 4 = read
PRINT "Type keys, ESC ends"
DO
  GET #1, Key
  IF Key = EscKey
    EXIT
  ENDIF
  PUT Key
LOOP
CLOSE #1
```

(The statement `GET K` without a channel reads a key without opening
any channel; see [GET](10-console-io.md#get).)

### PUT #

Writes one byte to a channel.

```
PUT #channel, value
```
Abbreviation: `PU.`

Writes the byte _value_ (only its low 8 bits are used) to _channel_.
Sets `ERR()`.

```basic
' Write the 256 possible byte values to a file
OPEN #1, 8, 0, "D:BYTES.DAT"     ' 8 = write
FOR Value = 0 TO 255
  PUT #1, Value
NEXT Value
CLOSE #1
```

### BGET

Reads a block of bytes from a channel into memory.

```
BGET #channel, address, length
```
Abbreviation: `BG.`

Reads _length_ bytes from _channel_ and stores them in memory starting
at _address_. It is much faster than reading the bytes one by one with
`GET #`. Use [ADR()](16-memory-and-machine-code.md) to get the address
of an array: `BGET #1, ADR(Table), 200`.

_length_ is from 1 to 65535 (values above 32767 are written as negative
numbers or in hexadecimal). Make sure the memory area is large enough:
`BGET` does not check, and writes over whatever is there.

Do not use a _length_ of 0. CIO treats a length of 0 as a request for a
single byte, which `BGET` then discards.

`ERR()` is 1 if all the bytes were read. If the file ends first, `ERR()`
is 136 and only part of the block was read. The number of bytes read is
stored by CIO in the IOCB of the channel; read it with
`DPEEK($348 + 16 * channel)`.

To read into a string, skip the length byte: `BGET #1, ADR(A$) + 1, 10`
reads 10 bytes into the characters of `A$` (the string length is not
changed).

### BPUT

Writes a block of bytes from memory to a channel.

```
BPUT #channel, address, length
```
Abbreviation: `BP.`

Writes _length_ bytes from memory, starting at _address_, to
_channel_. `ERR()` is 1 if all the bytes were written. The number of
bytes written is in `DPEEK($348 + 16 * channel)`.

As with `BGET`, do not use a _length_ of 0: one undefined byte is
written.

```basic
' Save a word array as binary data and load it back
DIM Table(99), Copy(99)
FOR I = 0 TO 99
  Table(I) = I * 300
NEXT I

OPEN #1, 8, 0, "D:TABLE.DAT"     ' 8 = write
BPUT #1, ADR(Table), 200         ' 100 words = 200 bytes
Status = ERR()
CLOSE #1
PRINT "Write: "; Status

OPEN #1, 4, 0, "D:TABLE.DAT"     ' 4 = read
BGET #1, ADR(Copy), 200
Status = ERR()
CLOSE #1
PRINT "Read: "; Status; ", Copy(99) = "; Copy(99)
```

Output:

```
Write: 1
Read: 1, Copy(99) = 29700
```

A word array of 100 elements (0 to 99) uses 200 bytes; a byte array
uses one byte per element. See [Variables and Data](04-variables-and-data.md).

`BPUT #0, address, length` writes the bytes to the screen.

### XIO

Sends any CIO command to a channel.

```
XIO command, #channel, aux1, aux2, device$
```
Abbreviation: `X.`

Calls CIO with any command. It is used for the special commands of
each device, such as renaming or deleting disk files. The order of the
parameters is the same as in Atari BASIC.

- _command_: the CIO command number; see [XIO commands](#xio-commands).
- _channel_: the channel number, 0 to 7.
- _aux1_, _aux2_: the auxiliary bytes, 0 to 255.
- _device$_: a string passed as the buffer, usually the device or file
  name.

`ERR()` gets the result.

The common command codes:

| Command | Operation |
| ------- | --------- |
| 3 | Open (same as `OPEN`) |
| 5 | Get record (a line) |
| 7 | Get characters |
| 9 | Put record |
| 11 | Put characters |
| 12 | Close (same as `CLOSE`) |
| 13 | Status (same as `STATUS`) |
| 17 | Draw line (same as `DRAWTO`) |
| 18 | Fill area (same as `FILLTO`) |
| 32 | Rename a file |
| 33 | Delete a file |
| 35 | Lock a file |
| 36 | Unlock a file |
| 37 | POINT (same as `POINT`) |
| 38 | NOTE (same as `NOTE`) |
| 254 | Format a disk |

The full list, with the devices, is in [XIO commands](#xio-commands).

The channel used for disk commands like rename and delete must be
closed: CIO opens it, runs the command and closes it again.

```basic
' Rename, lock, unlock and delete a file with XIO
XIO 32, #1, 0, 0, "D:OLD.TXT,NEW.TXT"    ' 32 = rename
Status = ERR()
IF Status <> 1
  PRINT "Rename error "; Status
ENDIF

XIO 35, #1, 0, 0, "D:NEW.TXT"    ' 35 = lock
XIO 36, #1, 0, 0, "D:NEW.TXT"    ' 36 = unlock
XIO 33, #1, 0, 0, "D:NEW.TXT"    ' 33 = delete
IF ERR() = 170
  PRINT "File not found"
ENDIF
```

`XIO` copies _device$_ to a 128 byte buffer, adds an EOL and gives CIO
a buffer length of 255. For this reason, use the normal statements for
reading and writing (commands 5, 7, 9 and 11): `INPUT #`, `BGET`,
`PRINT #` and `BPUT`.

On a channel that is already open, `XIO` stores _aux1_ and _aux2_ in
the IOCB. For commands that do not use them, give the same mode used in
the `OPEN` as _aux1_.

### NOTE

Gets the current position in an open disk file.

```
NOTE #channel, sector, byte
```
(no abbreviation)

Stores the current position of the file open on _channel_ in two integer
variables (or word array elements): _sector_ gets the sector number and
_byte_ the position of the byte in the sector. Use the two values later
with `POINT` to come back to this position. `ERR()` gets the result. `NOTE`
is the CIO command 38 of DOS.

Treat the values as a bookmark: store them and give them back unchanged.
Their meaning depends on the DOS.

### POINT

Moves to a position in an open disk file.

```
POINT #channel, sector, byte
```
Abbreviation: `POI.`

Moves the file open on _channel_ to the position given by _sector_ and
_byte_, two values returned by an earlier `NOTE` on the same file. The
next read or write starts there. `ERR()` gets the result; a position that
is not valid in the file gives an error, such as 166 or 171 with Atari
DOS 2. `POINT` is the CIO command 37 of DOS. See the example in
[NOTE and POINT](#note-and-point).

### STATUS

Gets the status of a channel.

```
STATUS #channel, var
```
Abbreviation: `STAT.`

Calls the CIO status command (13) on _channel_ and stores the result
code in _var_, an integer variable or an element of a word array. The
code is also in `ERR()`. 1 means that the device is ready. For an open
channel the device handler also leaves four bytes of device status in
`DVSTAT`, locations $2EA to $2ED (746 to 749); their meaning depends on
the device (for example, the 850 interface reports the state of the
serial lines there).

On a channel that is not open, `STATUS` returns 130. `STATUS` does not
change the channel's AUX bytes (`$34A` and `$34B` for channel 0), as in
Atari BASIC: a `STATUS #0` leaves the screen editor open for reading and
writing.

```basic
STATUS #0, Status
PRINT "E: status = "; Status
STATUS #3, Status
PRINT "Closed channel = "; Status
```

Output:

```
E: status = 1
Closed channel = 130
```

### ERR

Returns the result of the last I/O operation.

```
ERR()
```
Abbreviation: `E.`

Returns the status code stored by the last statement that used I/O, by
the last conversion or by the last floating point operation:

- 1: the operation was successful.
- 3: a floating point overflow or invalid argument (see
  [Floating Point](09-floating-point.md)).
- 18: `VAL` or `INPUT` could not convert the text to a number (see
  [Strings](08-strings.md)).
- 128 to 255: an I/O error, see [Error codes](#error-codes).

When the program starts, `ERR()` is 1.

These statements set `ERR()`: `OPEN`, `CLOSE`, `PRINT #`, `PUT #`,
`INPUT`, `GET`, `BGET`, `BPUT`, `XIO`, `NOTE`, `POINT`, `STATUS`,
`GRAPHICS`, `PLOT`, `DRAWTO`, `FILLTO`, `LOCATE` and `RUN` (when the file
cannot be loaded). `VAL`, `INT`, `^` and the floating point operations set
it too. The value stays until the next of these operations.

Printing on the screen (`PRINT`, `PUT` and `CLS` on channel 0) does not
change `ERR()`, so `PRINT "Error: "; ERR()` shows the result of the
statement before. `PRINT #` to a file or device sets `ERR()` for each
character written. Store `ERR()` in a variable right after the operation
you want to check, so that later operations don't change it:

```basic
OPEN #1, 4, 0, "D:NOFILE.TXT"
Status = ERR()
PRINT "OPEN error: "; Status
CLOSE #1
```

Output:

```
OPEN error: 170
```

`ERR()` is a function without arguments: the parentheses are required
(or use the abbreviation `E.`).

## Reference tables

### Open modes

The _mode_ of `OPEN` (AUX1) for each device:

| Device | Mode | Meaning |
| ------ | ---- | ------- |
| `D:` | 4 | Read a file. |
| `D:` | 6 | Read the disk directory (the file name can use the wildcards `*` and `?`). |
| `D:` | 8 | Write a new file. An existing file with the same name is replaced. |
| `D:` | 9 | Append: write at the end of an existing file. |
| `D:` | 12 | Update: read and write an existing file. |
| `E:` | 12 | Keyboard input and screen output (normal use). |
| `E:` | 13 | Screen input and output: reads the text on the screen without waiting for the keyboard. |
| `S:` | 8, 12 | Screen output (12: output and input). _aux2_ is the graphics mode. Add 16 for a text window, 32 to keep the screen memory. Normally done by `GRAPHICS`. |
| `K:` | 4 | Read keys. |
| `P:` | 8 | Write to the printer. |
| `C:` | 4, 8 | Read or write a cassette file. _aux2_ = 128 uses short gaps between records. |
| `R:` | 5, 8, 9, 13 | Read (5 = concurrent input), write (8), concurrent output (9), both (13). |
| `N:` | 4, 6, 8, 12, 13 | FujiNet: read, directory, write, read and write, HTTP POST. _aux2_ is the end of line translation. |

### XIO commands

| Command | Device | Operation | Example |
| ------- | ------ | --------- | ------- |
| 3 | all | Open (same as `OPEN`) | `XIO 3, #1, 8, 0, "D:FILE"` |
| 5 | all | Get record (a line) | use `INPUT #` |
| 7 | all | Get characters | use `BGET` or `GET #` |
| 9 | all | Put record | use `PRINT #` |
| 11 | all | Put characters | use `BPUT` or `PUT #` |
| 12 | all | Close (same as `CLOSE`) | `XIO 12, #1, 0, 0, "D:"` |
| 13 | all | Status (same as `STATUS`) | |
| 17 | `S:` | Draw line (same as `DRAWTO`) | |
| 18 | `S:` | Fill area (same as `FILLTO`) | |
| 32 | `D:` | Rename a file | `XIO 32, #1, 0, 0, "D:OLD,NEW"` |
| 33 | `D:` | Delete a file | `XIO 33, #1, 0, 0, "D:FILE.TXT"` |
| 35 | `D:` | Lock a file (read only) | `XIO 35, #1, 0, 0, "D:FILE.TXT"` |
| 36 | `D:` | Unlock a file | `XIO 36, #1, 0, 0, "D:FILE.TXT"` |
| 37 | `D:` | POINT: move to a position in an open file | use [POINT](#point) |
| 38 | `D:` | NOTE: get the position in an open file | use [NOTE](#note) |
| 254 | `D:` | Format a disk (erases it) | `XIO 254, #1, 0, 0, "D2:"` |
| 32 | `R:` | Force output of the buffer | |
| 34 | `R:` | Set the control lines (DTR, RTS, XMT) | |
| 36 | `R:` | Set baud rate, word size and stop bits | |
| 38 | `R:` | Set translation and parity | |
| 40 | `R:` | Start concurrent mode | |

The disk commands (32 to 36, 254) are those of Atari DOS 2 and
compatible DOS versions. The file name can use wildcards with commands
33, 35 and 36 in most DOS versions, so `"D:*.BAK"` deletes all the
`.BAK` files.

### Error codes

The codes returned by `ERR()` (and by `STATUS`). The I/O codes are the
same as in Atari BASIC.

| Code | Meaning |
| ---- | ------- |
| 1 | Success. |
| 3 | Floating point error (overflow, invalid argument). |
| 18 | Invalid number in `VAL` or `INPUT`. |
| 128 | BREAK key pressed during I/O. |
| 129 | Channel (IOCB) already open. |
| 130 | Nonexistent device (no handler for that device letter, or channel not open). |
| 131 | Channel opened for write only, cannot read. |
| 132 | Invalid command for this device. |
| 133 | Device or file not open. |
| 134 | Bad channel number. |
| 135 | Channel opened for read only, cannot write. |
| 136 | End of file. |
| 137 | Truncated record: the line was longer than the buffer. |
| 138 | Device timeout: the device does not answer (not connected or turned off). |
| 139 | Device NAK: the device refused the command (bad sector number, bad parameter). |
| 140 | Serial bus framing error. |
| 141 | Cursor out of range for the screen mode. |
| 142 | Serial bus data frame overrun. |
| 143 | Serial bus checksum error. |
| 144 | Device done error: the device could not complete the command (write protected disk, no disk in the drive). |
| 145 | Read after write compare error (disk), or bad screen mode. |
| 146 | Function not implemented by the device. |
| 147 | Not enough memory for the screen mode. |
| 160 | Drive number error (drive not configured in DOS). |
| 161 | Too many open files. |
| 162 | Disk full. |
| 163 | Unrecoverable system data I/O error (bad disk). |
| 164 | File number mismatch (damaged file or disk). |
| 165 | File name error. |
| 166 | POINT data length error. |
| 167 | File locked. |
| 168 | Invalid command for DOS. |
| 169 | Directory full. |
| 170 | File not found. |
| 171 | Invalid POINT. |

Codes 160 and above come from DOS and can differ slightly between DOS
versions.

### IOCB layout

Each channel has a 16 byte I/O control block at $340 + 16 × channel.
Most programs never need it, but some values are useful after an
operation:

| Offset | Name | Contents |
| ------ | ---- | -------- |
| 0 | `ICHID` | Handler index, 255 if the channel is closed |
| 1 | `ICDNO` | Drive or unit number |
| 2 | `ICCOM` | Last command |
| 3 | `ICSTA` | Last status |
| 4, 5 | `ICBAL`, `ICBAH` | Buffer address |
| 6, 7 | `ICPTL`, `ICPTH` | Address of the handler's put byte routine, minus 1 |
| 8, 9 | `ICBLL`, `ICBLH` | Buffer length; after a read or write, the number of bytes transferred |
| 10 | `ICAX1` | Auxiliary byte 1 (open mode) |
| 11 | `ICAX2` | Auxiliary byte 2 |
| 12 to 15 | `ICAX3` to `ICAX6` | More auxiliary bytes (used by NOTE and POINT) |

`PEEK($340 + 16 * Ch) <> 255` tells if channel _Ch_ is open.

## Examples

### Writing and reading a text file

```basic
' Write three lines to a file, then read them back
OPEN #1, 8, 0, "D:NAMES.TXT"     ' 8 = write
Status = ERR()
IF Status <> 1
  PRINT "Cannot create file, error "; Status
  END
ENDIF
PRINT #1, "ADA"
PRINT #1, "GRACE"
PRINT #1, "KATHERINE"
CLOSE #1

OPEN #1, 4, 0, "D:NAMES.TXT"     ' 4 = read
DO
  INPUT #1, Name$
  IF ERR() <> 1
    EXIT
  ENDIF
  PRINT "Read: "; Name$
LOOP
CLOSE #1
```

Output:

```
Read: ADA
Read: GRACE
Read: KATHERINE
```

### Appending to a file

```basic
' Add a line at the end of an existing file
OPEN #1, 9, 0, "D:NAMES.TXT"     ' 9 = append
PRINT #1, "MARGARET"
CLOSE #1

' Count the lines of the file
OPEN #1, 4, 0, "D:NAMES.TXT"     ' 4 = read
LineCount = 0
DO
  INPUT #1, Line$
  IF ERR() <> 1
    EXIT
  ENDIF
  INC LineCount
LOOP
CLOSE #1
PRINT LineCount; " lines"
```

Output:

```
4 lines
```

### Copying a file

`BGET` and `BPUT` copy any file, text or binary, quickly. The last
block is usually shorter; the IOCB tells how many bytes were read.

```basic
' Copy any file, 1024 bytes at a time
EndOfFile = 136
DIM Buffer(1023) BYTE
OPEN #1, 4, 0, "D:TABLE.DAT"     ' 4 = read
OPEN #2, 8, 0, "D:TABLE.BAK"     ' 8 = write
Total = 0
DO
  BGET #1, ADR(Buffer), 1024
  Status = ERR()
  ' Bytes really read: ICBLL/ICBLH in the IOCB of channel #1
  BytesRead = DPEEK($348 + 1 * 16)
  IF BytesRead > 0
    BPUT #2, ADR(Buffer), BytesRead
  ENDIF
  Total = Total + BytesRead
LOOP WHILE Status = 1
CLOSE #1
CLOSE #2

IF Status = EndOfFile
  PRINT "Copied "; Total; " bytes"
ELSE
  PRINT "Error "; Status
ENDIF
```

Output:

```
Copied 200 bytes
```

### Disk directory

Opening `D:` with mode 6 reads the directory as text lines, the same
lines that DOS shows: one line per file, then a last line with the
free sectors.

```basic
' List the files on drive 1
OPEN #1, 6, 0, "D1:*.*"          ' 6 = read directory
Status = ERR()
IF Status <> 1
  PRINT "Error "; Status
  END
ENDIF
DO
  INPUT #1, Line$
  IF ERR() <> 1
    EXIT
  ENDIF
  PRINT Line$
LOOP
CLOSE #1
```

With Atari DOS 2 the lines look like this (the files depend on the
disk):

```
  GAME    XEX 060
* SCORES  DAT 001
  NOTES   TXT 004
637 FREE SECTORS
```

An `*` marks a locked file, and the number is the size in sectors.

### NOTE and POINT

[NOTE](#note) gets the current position in an open disk file, and
[POINT](#point) moves to a position returned by an earlier `NOTE`.

```basic
' Remember a position with NOTE and go back to it with POINT
OPEN #1, 8, 0, "D:NOTE.TXT"      ' 8 = write
PRINT #1, "FIRST LINE"
PRINT #1, "SECOND LINE"
CLOSE #1

OPEN #1, 4, 0, "D:NOTE.TXT"      ' 4 = read
INPUT #1, Line$
NOTE #1, Sector, Byte            ' the position of the second line
INPUT #1, Line$
PRINT "READ: "; Line$

POINT #1, Sector, Byte           ' go back to the second line
INPUT #1, Line$
PRINT "AGAIN: "; Line$
CLOSE #1
```

Output:

```
READ: SECOND LINE
AGAIN: SECOND LINE
```

The position is passed in the IOCB bytes `ICAX3` to `ICAX5`: with
Atari DOS 2, `ICAX3`/`ICAX4` hold the sector number and `ICAX5` the
byte within the sector.

### Printer

```basic
' Print a short report on the printer
OPEN #3, 8, 0, "P:"              ' 8 = write
Status = ERR()
IF Status <> 1
  PRINT "Printer not ready, error "; Status
  END
ENDIF
PRINT #3, "REPORT"
FOR Line = 1 TO 5
  PRINT #3, "LINE "; Line
NEXT Line
CLOSE #3
```

Most Atari printers and printer interfaces print a line when they
receive the EOL at the end of each `PRINT #` line. If the printer does
not answer, the statements end with error 138 (timeout) after a few
seconds.
