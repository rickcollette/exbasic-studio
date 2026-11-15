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

; Division and modulo for the machine code: (SP+) / AX and (SP+) MOD AX,
; and the same by a byte constant in Y.  For one division alone; a division
; and a modulo of the same operands use nat_divc / nat_modc, which keep the
; last result.
; --------------------------------------------------------------------------

        .export         nat_div, nat_mod
        .import         divmod_sign_adjust, neg_AX
        .importzp       sptr, tmp2, tmp3, divmod_sign

        .segment        "RUNTIME"

.proc   nat_div
        jsr     divmod_sign_adjust
        inc     sptr
        lda     tmp3
        ldx     tmp3+1
        bit     divmod_sign
        bpl     :+
        jmp     neg_AX
:       rts
.endproc

.proc   nat_mod
        jsr     divmod_sign_adjust
        inc     sptr
        ldx     tmp2+1
        bit     divmod_sign
        bvc     :+
        jmp     neg_AX
:       rts
.endproc

; vi:syntax=asm_ca65
