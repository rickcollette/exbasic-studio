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

; Print 16bit number
; ------------------
;
; Converts the integer in AX to a decimal string with the OS floating point
; package (IFP and FASC, in ROM), as Atari BASIC: the string is in the OS
; line buffer.  Only programs without floating point use it: the others
; convert with INT_FP and FP_STR (see compile.cc), so FR0 is free here.

        .import         neg_AX
        .importzp       next_instruction

        .segment        "RUNTIME"

FR0     = $D4
INBUFF  = $F3
IFP     = $D9AA
FASC    = $D8E6

EXE_INT_STR:            ; AX = STRING( AX )
        cpx     #$80
        bcc     EXE_UINT_STR    ; C = 0: positive
        jsr     neg_AX
        sec                     ; C = 1: negative
        .byte   $90             ; BCC, not taken: skips the CLC
EXE_UINT_STR:           ; AX = STRING( AX ), AX unsigned (0 to 65535)
        clc
        php                     ; The sign
        sta     FR0
        stx     FR0+1
        jsr     IFP
        asl     FR0             ; Sign into the exponent byte
        plp
        ror     FR0
        jsr     FASC
        ; One byte before the text for the length; the last character has
        ; the high bit set
        dec     INBUFF
        ldy     #0
:       iny
        lda     (INBUFF), y
        bpl     :-
        and     #$7F
        sta     (INBUFF), y
        tya
        ldy     #0
        sta     (INBUFF), y
        lda     INBUFF
        ldx     INBUFF+1
        jmp     next_instruction

        .include "deftok.inc"
        deftoken "INT_STR"
        deftoken "UINT_STR"

; vi:syntax=asm_ca65
