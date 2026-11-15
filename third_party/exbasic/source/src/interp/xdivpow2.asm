;
; exBASIC - BASIC compiler for the Atari 8-bit computers, based on FastBasic
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
;

; DIV_POW2: AX = AX / 2^n, n (1 to 14) from the instruction stream
; ----------------------------------------------------------------
; Same as "PUSH_NUM 2^n" + "DIV": the result is rounded toward zero, so
; negative numbers are negated before and after the shift.

        ; The shift is in natdivp2.asm, shared with native code
        .import         nat_divpow2, inc_cptr_1
        .importzp       cptr

        .segment        "RUNTIME"

.proc   EXE_DIV_POW2
        pha
        ldy     #0
        lda     (cptr), y
        tay
        pla
        jsr     nat_divpow2
        jmp     inc_cptr_1
.endproc

        .include "deftok.inc"
        deftoken "DIV_POW2"

; vi:syntax=asm_ca65
