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

; Signed division by a power of two, used by the DIV_POW2 instruction and by
; native code
; --------------------------------------------------------------------------

        .export         nat_divpow2
        .import         neg_AX
        .importzp       tmp1

        .segment        "RUNTIME"

; AX = AX / 2^Y, signed, see EXE_DIV_POW2
.proc   nat_divpow2
        cpx     #$80
        php
        bcc     pos
        jsr     neg_AX
pos:    sta     tmp1
        stx     tmp1+1
loop:   lsr     tmp1+1
        ror     tmp1
        dey
        bne     loop
        lda     tmp1
        ldx     tmp1+1
        plp
        bcc     done
        jmp     neg_AX
done:   rts
.endproc

; vi:syntax=asm_ca65
