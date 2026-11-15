# 18. SIO and FujiNet

This chapter covers the low level access to the Atari serial bus: the
`SIO` statement, which sends any command to any device on the bus, and
the `SERR()` function, which returns its result. It also covers the
FujiNet statements `NOPEN`, `NCLOSE`, `NSTATUS`, `NGET` and `NPUT`,
which use the network devices of a FujiNet adapter without loading any
`N:` handler.

## The serial bus

The Atari talks to its peripherals (disk drives, printers, the 850
interface, the FujiNet adapter) through the **serial input/output bus**,
SIO. Every device on the bus has an ID number. The computer sends a
command frame (device, command and two auxiliary bytes) and then, if
the command needs it, a block of data in one direction.

Normally programs use the bus through CIO and the device handlers (see
[Files and Devices](11-files-and-devices.md)): `OPEN #1, 4, 0, "D:FILE"`
makes DOS send the right SIO commands. The `SIO` statement skips CIO
and DOS and sends one SIO command directly. Use it for things the
handlers do not offer: reading or writing disk sectors, asking a drive
for its status, or sending special commands to a FujiNet.

The OS routine that sends the command is `SIOV` (address $E459). It
reads its parameters from the **device control block** (DCB), 12 bytes
at $300 to $30B.

| Address | Name | Size | Contents |
| ------- | ---- | ---- | -------- |
| $300 | `DDEVIC` | byte | Device ID on the bus |
| $301 | `DUNIT` | byte | Unit number (1 to 8) |
| $302 | `DCOMND` | byte | Command |
| $303 | `DSTATS` | byte | Before: data direction. After: the result code |
| $304, $305 | `DBUF` | word | Address of the data buffer |
| $306 | `DTIMLO` | byte | Timeout, in units of 64 frames (about one second) |
| $307 | | byte | Not used |
| $308, $309 | `DBYT` | word | Number of data bytes to transfer |
| $30A | `DAUX1` | byte | First auxiliary byte |
| $30B | `DAUX2` | byte | Second auxiliary byte |

The OS adds `DUNIT` - 1 to `DDEVIC` to get the ID sent on the bus, so
disk drive 2 is `DDEVIC` = $31 with `DUNIT` = 2.

## SIO

Sends one command to a device on the serial bus.

```
SIO ddevic, dunit, dcomnd, dstats, dbuf, dtimlo, dbyt, daux1, daux2
```
Abbreviation: `SI.`

Stores the nine values in the DCB and calls `SIOV`. The program waits
until the command ends or the timeout expires.

| Parameter | DCB | Range | Meaning |
| --------- | --- | ----- | ------- |
| _ddevic_ | $300 | 0 to 255 | Device ID (see the table below) |
| _dunit_ | $301 | 1 to 8 | Unit number |
| _dcomnd_ | $302 | 0 to 255 | Command; depends on the device |
| _dstats_ | $303 | $40, $80, 0 | $40: the device sends data to the computer (read). $80: the computer sends data to the device (write). 0: no data |
| _dbuf_ | $304 | address | Buffer for the data, usually `ADR()` of a byte array |
| _dtimlo_ | $306 | 1 to 255 | Timeout in units of 64 frames (1.07 s NTSC, 1.28 s PAL) |
| _dbyt_ | $308 | 0 to 65535 | Number of data bytes |
| _daux1_ | $30A | 0 to 255 | First auxiliary byte; depends on the command |
| _daux2_ | $30B | 0 to 255 | Second auxiliary byte; depends on the command |

`SIO` does not change `ERR()`. Read the result with [SERR()](#serr).
The DCB keeps its values after the call, so you can also read them with
`PEEK`.

Common device IDs:

| ID | Device |
| -- | ------ |
| $31 | Disk drives (units 1 to 8) |
| $40 | Printer |
| $50 | 850 interface serial ports (units 1 to 4) |
| $70 | FujiNet adapter control device |
| $71 | FujiNet network devices `N1:` to `N8:` (units 1 to 8) |

Disk drive commands (the sector number goes in _daux1_ and _daux2_,
low byte first):

| Command | Name | _dstats_ | Bytes | Operation |
| ------- | ---- | -------- | ----- | --------- |
| $52 (`R`) | Read sector | $40 | 128 or 256 | Read one sector into the buffer |
| $57 (`W`) | Write sector | $80 | 128 or 256 | Write one sector and verify it |
| $50 (`P`) | Put sector | $80 | 128 or 256 | Write one sector without verify (faster) |
| $53 (`S`) | Status | $40 | 4 | Get 4 status bytes from the drive |
| $21 (`!`) | Format | $40 | 128 | Format the whole disk (erases it) |

Sectors have 128 bytes on single and enhanced density disks, and 256
bytes on double density disks (except the first three sectors, which
always have 128 bytes). A single density disk has sectors 1 to 720.

**Be careful with disk writes.** Writing a sector changes the disk
below DOS: a wrong sector number can destroy files or the directory.
Test with a disk you do not need.

```basic
' Ask drive 1 for its status (4 bytes into DVSTAT, $2EA)
DiskDrive = $31
StatusCommand = $53
ReadData = $40
DvStat = $2EA

SIO DiskDrive, 1, StatusCommand, ReadData, DvStat, 7, 4, 0, 0
IF SERR() = 1
  PRINT "Drive 1 status: "; PEEK(DvStat); " "; PEEK(DvStat + 1);
  PRINT " "; PEEK(DvStat + 2); " "; PEEK(DvStat + 3)
ELSE
  PRINT "Drive 1 error "; SERR()
ENDIF
```

Output (the values depend on the drive):

```
Drive 1 status: 16 255 224 0
```

```basic
' Write a sector, then read it back
DiskDrive = $31
WriteCommand = $57
ReadCommand = $52
WriteData = $80
ReadData = $40
Sector = 700

DIM SectorData(127) BYTE
FOR I = 0 TO 127
  SectorData(I) = I
NEXT I

Low = Sector & 255         ' sector number, low byte first
High = Sector / 256
Buffer = ADR(SectorData)

SIO DiskDrive, 1, WriteCommand, WriteData, Buffer, 7, 128, Low, High
PRINT "Write: "; SERR()

' Clear the buffer, then read the sector again
MSET Buffer, 128, 0
SIO DiskDrive, 1, ReadCommand, ReadData, Buffer, 7, 128, Low, High
PRINT "Read: "; SERR(); ", byte 100 = "; SectorData(100)
```

Output:

```
Write: 1
Read: 1, byte 100 = 100
```

## SERR

Returns the result of the last SIO operation.

```
SERR()
```
Abbreviation: `SE.`

Returns the value of `DSTATS` ($303), where `SIOV` stores its result
code. It is the same as `PEEK($303)`. The `SIO` statement and all the
FujiNet statements set it; so does any other SIO operation, including
the disk accesses that DOS makes for `OPEN`, `INPUT #` and the other
file statements.

| Code | Meaning |
| ---- | ------- |
| 1 | Success |
| 138 | Timeout: no device with that ID answered |
| 139 | NAK: the device refused the command (bad command or parameter) |
| 140 | Serial bus framing error |
| 142 | Serial bus data overrun |
| 143 | Checksum error |
| 144 | Device error: the device accepted the command but could not do it (for example a write protected disk; for FujiNet, see `NSTATUS`) |

These are the same codes as the I/O errors of
[ERR()](11-files-and-devices.md#error-codes).

## FujiNet

The FujiNet is a network adapter for the Atari serial bus. It provides
disk drives, a printer and up to eight **network devices**, `N1:` to
`N8:`, that can open connections with protocols like TCP, UDP, HTTP,
HTTPS, FTP and TNFS. A connection is opened with a URL, for example
`N:HTTP://www.example.com/file.txt` or `N:TCP://192.168.1.10:6502/`.

The FujiNet network devices are usually used through the `N:` CIO
handler (`NDEV`), loaded from disk; then `OPEN #1, 4, 0, "N:..."`,
`INPUT #1` and the other file statements work with them. The exBASIC
FujiNet statements do not need the handler: they send the SIO commands
of the network device directly, with device ID $71.

Each statement takes a _unit_ number, 1 to 8, that selects the network
device `N1:` to `N8:`; several connections can be open at the same
time on different units. The usual sequence is:

1. `NOPEN` the connection and check `SERR()`.
2. In a loop: `NSTATUS` to see how many bytes are waiting, `NGET` to
   read them, `NPUT` to send data.
3. `NCLOSE` when done.

All the FujiNet statements set [SERR()](#serr). If no FujiNet is
connected, `SERR()` is 138 (timeout). Error 144 means that the network
operation failed; `NSTATUS` then gives the network error code.

The FujiNet statements fill the DCB as follows (DTIMLO is always $1F,
31 units, about 33 seconds):

| Statement | `DCOMND` | `DSTATS` | `DBUF` | `DBYT` | `DAUX1`, `DAUX2` |
| --------- | -------- | -------- | ------ | ------ | ---------------- |
| `NOPEN` | $4F (`O`) | $80 | URL | 256 | _mode_, _trans_ |
| `NCLOSE` | $43 (`C`) | 0 | unchanged | 0 | unchanged |
| `NSTATUS` | $53 (`S`) | $40 | $2EA | 4 | unchanged |
| `NGET` | $52 (`R`) | $40 | _address_ | _length_ | _length_ |
| `NPUT` | $57 (`W`) | $80 | _address_ | _length_ | _length_ |

### NOPEN

Opens a network connection.

```
NOPEN unit, mode, trans, url$
```
Abbreviation: `NO.`

- _unit_: the network device, 1 to 8.
- _mode_: what the connection is used for (see the table).
- _trans_: how line endings are translated (see the table).
- _url$_: the address to open, starting with `N:`.

| _mode_ | Meaning |
| ------ | ------- |
| 4 | Read (HTTP GET) |
| 6 | Directory (HTTP PROPFIND, FTP and TNFS listings) |
| 8 | Write (HTTP PUT) |
| 12 | Read and write (TCP, UDP, HTTP GET with headers) |
| 13 | HTTP POST |

| _trans_ | Translation of received and sent line endings |
| ------- | --------------------------------------------- |
| 0 | None |
| 1 | CR (13) is changed to ATASCII EOL (155) |
| 2 | LF (10) is changed to EOL |
| 3 | CR and LF are changed to EOL |

The _url$_ can be given in two ways:

- **A string variable** (or string array element). The URL is used
  where it is stored: `NOPEN` writes a zero byte after the last
  character to end it. The URL can have up to 254 characters; a 255
  character string loses its last character.
- **Any other string expression**, such as a constant or `"N:TCP://" +
  Host$`. The URL is copied to the OS line buffer (at $580) with an EOL
  at the end. It can have up to 127 characters; longer URLs are cut.

```basic
Url$ = "N:HTTP://www.example.com/"
NOPEN 1, 4, 2, Url$
IF SERR() <> 1
  PRINT "Open error "; SERR()
ENDIF
```

### NCLOSE

Closes a network connection.

```
NCLOSE unit
```
Abbreviation: `NC.`

Closes the connection open on _unit_ (1 to 8). Close every connection
you open: a connection that is not closed can stay open in the
FujiNet after the program ends.

### NSTATUS

Gets the state of a network connection.

```
NSTATUS unit
```
Abbreviation: `NS.`

Asks the FujiNet for the state of _unit_ (1 to 8). The answer is stored
in the four bytes of `DVSTAT`, at $2EA to $2ED:

| Address | Contents |
| ------- | -------- |
| $2EA, $2EB | Number of bytes waiting to be read: `DPEEK($2EA)` |
| $2EC | 1 if the connection is open, 0 if it was closed by the other side |
| $2ED | Error code of the last network operation (1 = no error, 136 = end of data) |

```basic
NSTATUS 1
BytesWaiting = DPEEK($2EA)
```

### NGET

Reads bytes from a network connection.

```
NGET unit, address, length
```
Abbreviation: `NG.`

Reads _length_ bytes from the connection on _unit_ (1 to 8) and stores
them in memory at _address_. _length_ must not be larger than the
number of bytes waiting, as returned by `NSTATUS`, or the operation
fails with an SIO error. It must not be larger than the buffer either.

### NPUT

Sends bytes to a network connection.

```
NPUT unit, address, length
```
Abbreviation: `NP.`

Sends _length_ bytes from memory at _address_ to the connection on
_unit_ (1 to 8). To send the characters of a string, skip its length
byte: `NPUT 1, ADR(Text$) + 1, LEN(Text$)`.

### Examples

This program reads a text file from a web server and prints it. The
translation mode 2 changes the line feeds of the file to Atari EOL
characters.

```basic
' Read a text file over HTTP with FujiNet and print it
DvStat = $2EA              ' NSTATUS results
EndOfData = 136
DIM Buffer(255) BYTE

Url$ = "N:HTTP://www.textfiles.com/art/alpine.txt"
NOPEN 1, 4, 2, Url$        ' 4 = read, 2 = LF to EOL
IF SERR() <> 1
  PRINT "Open error "; SERR()
  END
ENDIF

DO
  NSTATUS 1
  BytesWaiting = DPEEK(DvStat)
  IF BytesWaiting = 0
    ' Nothing waiting: stop at the end or when disconnected
    IF PEEK(DvStat + 3) = EndOfData OR PEEK(DvStat + 2) = 0
      EXIT
    ENDIF
    PAUSE 1
  ELSE
    IF BytesWaiting > 256
      BytesWaiting = 256
    ENDIF
    NGET 1, ADR(Buffer), BytesWaiting
    IF SERR() <> 1
      EXIT
    ENDIF
    BPUT #0, ADR(Buffer), BytesWaiting    ' print the bytes
  ENDIF
LOOP

NCLOSE 1
PRINT "Done"
```

This program sends a line to a TCP server and prints the answer:

```basic
' Send a line to a TCP server and print the reply
DvStat = $2EA              ' NSTATUS results
DIM Buffer(255) BYTE

NOPEN 1, 12, 0, "N:TCP://192.168.1.10:6502/"   ' 12 = read/write
IF SERR() <> 1
  NSTATUS 1
  PRINT "Error "; PEEK(DvStat + 3)
  END
ENDIF

Message$ = "HELLO" + CHR$(13) + CHR$(10)
NPUT 1, ADR(Message$) + 1, LEN(Message$)

' Wait for the answer
REPEAT
  NSTATUS 1
  BytesWaiting = DPEEK(DvStat)
UNTIL BytesWaiting > 0

IF BytesWaiting > 256
  BytesWaiting = 256
ENDIF
NGET 1, ADR(Buffer), BytesWaiting
PRINT "Got "; BytesWaiting; " bytes:"
BPUT #0, ADR(Buffer), BytesWaiting
NCLOSE 1
```

The FujiNet supports many more SIO commands (for example to set the
login and password of a connection, or to send HTTP headers). Send them
with the `SIO` statement and device ID $71; they are listed in the
FujiNet documentation:
[SIO Commands for FujiNet Devices](https://github.com/FujiNetWIFI/fujinet-platformio/wiki/SIO-Commands-for-Device-IDs-%2471-to-%2478).
