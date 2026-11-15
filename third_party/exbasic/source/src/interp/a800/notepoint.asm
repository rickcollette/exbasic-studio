;
; FastBasic - Fast basic interpreter for the Atari 8-bit computers
; Copyright (C) 2017-2025 Daniel Serpell
;
; This program is free software; you can redistribute it and/or modify
; it under the terms of the GNU General Public License as published by
; the Free Software Foundation, either version 2 of the License, or
; (at your option) any later version.
;
; This program is distributed in the hope that it will be useful,
; but WITHOUT ANY WARRANTY; without even the implied warranty of
; MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
; GNU General Public License for more details.
;
; You should have received a copy of the GNU General Public License along
; with this program.  If not, see <http://www.gnu.org/licenses/>
;
; In addition to the permissions in the GNU General Public License, the
; authors give you unlimited permission to link the compiled version of
; this file into combinations with other programs, and to distribute those
; combinations without any restriction coming from the use of this file.
; (The General Public License restrictions do apply in other respects; for
; example, they cover modification of the file, and distribution when not
; linked into a combine executable.)

; NOTE and POINT: position in a disk file
; ---------------------------------------
;
; NOTE #chan, sector, byte    reads the position with CIO command 38
; POINT #chan, sector, byte   sets the position with CIO command 37
;
; The position is passed in ICAX3/ICAX4 (sector) and ICAX5 (byte) of the
; channel.  The channel is in IOCHN (set with the IOCHN instruction).

        .export         NOTE_SECTOR, NOTE_BYTE
        .importzp       IOCHN, IOERROR, sptr, next_instruction
        .import         stack_l, stack_h

        .include "atari.inc"

        .segment        "RUNTIME"

.proc   EXE_NOTE
        ldx     IOCHN
        lda     #38
        sta     ICCOM, x
        jsr     CIOV
        sty     IOERROR
        lda     ICAX3, x
        sta     NOTE_SECTOR
        lda     ICAX4, x
        sta     NOTE_SECTOR+1
        lda     ICAX5, x
        sta     NOTE_BYTE
        lda     #0
        sta     NOTE_BYTE+1
        jmp     next_instruction
.endproc

.proc   EXE_POINT       ; Sector in the stack, byte in AX
        ldx     IOCHN
        sta     ICAX5, x
        ldy     sptr
        lda     stack_l, y
        sta     ICAX3, x
        lda     stack_h, y
        sta     ICAX4, x
        inc     sptr
        lda     #37
        sta     ICCOM, x
        jsr     CIOV
        sty     IOERROR
        jmp     next_instruction
.endproc

        .bss
NOTE_SECTOR:    .res    2
NOTE_BYTE:      .res    2

        .include "deftok.inc"
        deftoken "NOTE"
        deftoken "POINT"

; vi:syntax=asm_ca65
