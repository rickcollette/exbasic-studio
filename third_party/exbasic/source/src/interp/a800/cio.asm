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


; Calls to CIO, shared by the I/O statements
; ------------------------------------------

        .export         CIOV_CMD_A, CIOV_CMD_L, CIOV_CMD, CIOV_IOERR
        .importzp       IOERROR, next_instruction

        .include "atari.inc"

        .segment        "RUNTIME"

        ; Calls CIO with the command in Y, the address in A (low) and on the
        ; CPU stack (high), and the length on the CPU stack
CIOV_CMD_A:
        sta     ICBAL, x        ; Address
        pla
        sta     ICBAH, x
        pla                     ; Length
CIOV_CMD_L:
        sta     ICBLH, x
        pla
        sta     ICBLL, x
        tya                     ; Command
        ; Calls CIO with given command, stores I/O error and pops stack
CIOV_CMD:
        sta     ICCOM, x
        ; Calls CIOV, stores I/O error and pops stack
        jsr     CIOV
        ldx     #0      ; Needed for TOK_GET
CIOV_IOERR:
        sty     IOERROR
        jmp     next_instruction

; vi:syntax=asm_ca65
