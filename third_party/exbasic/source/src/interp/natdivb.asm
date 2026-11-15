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

; Division and modulus by a byte constant, called from native code
; ----------------------------------------------------------------

        .export         nat_divcb, nat_modcb
        .import         nat_divc, nat_modc, pushAX
        .importzp       sptr, tmp3

        .segment        "RUNTIME"

; AX = AX % Y and AX = AX / Y
.proc   nat_modcb
        sec
        .byte   $24             ; BIT zp, skips the CLC
.endproc        ; Fall through
.proc   nat_divcb
        clc
        php
        sty     tmp3            ; Divisor
        ldy     sptr
        jsr     pushAX          ; Dividend
        lda     tmp3
        ldx     #0
        plp
        bcs     :+
        jmp     nat_divc
:       jmp     nat_modc
.endproc

; vi:syntax=asm_ca65
