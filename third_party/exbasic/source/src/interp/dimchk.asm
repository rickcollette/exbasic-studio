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


; Checks the size of an array in DIM
; ----------------------------------
;
; DIM_CHK is followed by a byte with the biggest high byte allowed.  The
; size in AX (the last index) must be from 0 to that limit; then 1 is added
; to get the number of elements.  Other sizes stop the program with
; "Bad DIM size".  The limits keep the array smaller than 32KB, so the size
; in bytes does not overflow.

        .import         err_dim
        .importzp       next_instruction, cptr, tmp1

        .segment        "RUNTIME"

.proc   EXE_DIM_CHK
        sta     tmp1
        ldy     #0              ; Read the limit
        txa
        cmp     (cptr), y
        beq     ok
        bcs     error           ; High byte above the limit, or negative
ok:     inc     cptr
        bne     :+
        inc     cptr+1
:       lda     tmp1            ; Number of elements: size + 1
        clc
        adc     #1
        bcc     :+
        inx
:       jmp     next_instruction

error:  jmp     err_dim
.endproc

        .include "deftok.inc"
        deftoken "DIM_CHK"

; vi:syntax=asm_ca65
