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


; Short-circuit AND and OR
; ------------------------
;
; In "A AND B", B is not evaluated when A is false, and in "A OR B" it is not
; evaluated when A is true.  SC_AND and SC_OR are followed by the address of
; the end of the expression; they jump there when the result is already
; known, keeping it in A (0 or 1), else they continue to evaluate B.

        .importzp       next_instruction, cptr

        .segment        "RUNTIME"

.proc   EXE_SC_AND
        cmp     #1
        bcc     jump            ; False: the result is false
skip:   pha                     ; Continue after the address
        lda     cptr
        clc
        adc     #2
        sta     cptr
        bcc     :+
        inc     cptr+1
:       pla
        jmp     next_instruction
jump:   pha
        ldy     #1
        lda     (cptr), y
        tax
        dey
        lda     (cptr), y
        sta     cptr
        stx     cptr+1
        ldx     #0
        pla
        jmp     next_instruction
.endproc

.proc   EXE_SC_OR
        cmp     #1
        bcs     EXE_SC_AND::jump        ; True: the result is true
        bcc     EXE_SC_AND::skip
.endproc

        .include "deftok.inc"
        deftoken "SC_AND"
        deftoken "SC_OR"

; vi:syntax=asm_ca65
