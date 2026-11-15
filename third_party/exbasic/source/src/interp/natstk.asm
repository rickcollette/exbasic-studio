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

; Addition and subtraction with the interpreter stack, called from native code
; ----------------------------------------------------------------------------
;
; Native code keeps most values in zero page; values in the interpreter stack
; only follow slower operations (multiplication, division or bytecode), so
; a subroutine is used to keep the code small.

        .export         nat_add_s, nat_sub_s
        .import         stack_l, stack_h
        .importzp       sptr, tmp1

        .segment        "RUNTIME"

.proc   nat_sub_s       ; AX = (SP+) - AX
        clc
        eor     #$FF
        adc     #1
        sta     tmp1
        txa
        eor     #$FF
        adc     #0
        tax
        lda     tmp1
.endproc        ; Fall through, AX = (SP+) + (-AX)

.proc   nat_add_s       ; AX = (SP+) + AX
        ldy     sptr
        clc
        adc     stack_l, y
        sta     tmp1
        txa
        adc     stack_h, y
        tax
        lda     tmp1
        inc     sptr
        rts
.endproc

; vi:syntax=asm_ca65
