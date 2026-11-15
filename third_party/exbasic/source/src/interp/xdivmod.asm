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

; DIVC / MODC: Division and modulus with a one entry result cache
; ---------------------------------------------------------------
; Same as "DIV" and "MOD", but remembers the operands and both results of the
; last division.  The compiler uses these when a program computes both A/B
; and A MOD B, so the second operation does not need to divide again.  The
; cache is checked against the actual operand values, so the result is always
; correct even if the operands changed in between.

        ; The division is in natdiv.asm, shared with native code
        .import         nat_divc, nat_modc
        .importzp       next_instruction

        .segment        "RUNTIME"

.proc   EXE_MODC  ; AX = (SP+) % AX
        jsr     nat_modc
        jmp     next_instruction
.endproc

.proc   EXE_DIVC  ; AX = (SP+) / AX
        jsr     nat_divc
        jmp     next_instruction
.endproc

        .include "deftok.inc"
        deftoken "DIVC"
        deftoken "MODC"

; vi:syntax=asm_ca65
