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

; Integer to floating point and floating point to string conversions
; ------------------------------------------------------------------
;
; Used by the floating point runtime, with the OS math package.

        .export         int_to_fp, uint_to_fp, fp_to_str
        .import         neg_AX

        .segment        "RUNTIME"

FR0     = $D4
uint_to_fp:             ; AX unsigned, 0 to 65535
        clc
        php
        bcc     positive
int_to_fp:
IFP     = $D9AA
        cpx     #$80
        php                     ; Store sign in C
        bcc     positive
        jsr     neg_AX
positive:
        sta     FR0
        stx     FR0+1
        jsr     IFP

        asl     FR0
        plp                     ; Get sign and store in FR0
        ror     FR0

        rts

fp_to_str:
FASC    = $D8E6
INBUFF  = $F3
        jsr     FASC
        ; Decrease INBUFF, adds one byte for the string length
        dec     INBUFF
        ldy     #$0
        ; Calculate length
ploop:  iny
        lda     (INBUFF), y
        bpl     ploop
        ; Ok, last character, set high bit to 0
        and     #$7F
        sta     (INBUFF), y
        ; And store length into string start
        tya
        ldy     #0
        sta     (INBUFF), y
        ; Returns buffer in AX
        lda     INBUFF
        ldx     INBUFF+1
        rts

; vi:syntax=asm_ca65
