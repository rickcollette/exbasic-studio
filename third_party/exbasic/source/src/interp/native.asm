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

; Native code: the bytecode continues with 6502 machine code
; ----------------------------------------------------------
;
; The compiler places machine code after this instruction, at the address in
; the code pointer.  The machine code returns to the interpreter by setting
; the code pointer and jumping to "next_instruction".

        .importzp       cptr
        .exportzp       nat_stk

        ; Zero page slots for the values pushed by the machine code
        .zeropage
nat_stk:        .res    8

        .segment        "RUNTIME"

        ; JMP (cptr) needs the pointer inside a page
        .assert <cptr <> $FF, error, "cptr can't be at the end of a page"

.proc   EXE_NATIVE
        jmp     (cptr)
.endproc

        .include "deftok.inc"
        deftoken "NATIVE"

; vi:syntax=asm_ca65
