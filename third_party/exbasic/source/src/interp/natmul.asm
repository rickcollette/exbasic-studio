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

; 16 bit multiplication, used by the MUL instruction and by native code
; ---------------------------------------------------------------------

        .export         nat_mul, mul_loop_start
        .import         stack_l, stack_h
        .importzp       sptr, tmp1, tmp2, tmp3

        .segment        "RUNTIME"

.proc   nat_mul  ; AX = (SP+) * AX, see EXE_MUL
        ; Shift and add, with the right operand (often a small constant) as
        ; the multiplier: the loop ends when no bits of it are left.
        sta     tmp2
        stx     tmp2+1
        ldy     sptr
        lda     stack_l, y
        sta     tmp1
        lda     stack_h, y
        sta     tmp1+1
        inc     sptr
        ; tmp1 * tmp2: also the entry of nat_mulb
::mul_loop_start:
        ; Both operands < 256: 8x8 bit multiplication, 8 iterations of 8 bit
        lda     tmp1+1
        ora     tmp2+1
        bne     mul16
        ldy     #8
        lsr     tmp2
m8:     bcc     :+
        clc
        adc     tmp1
:       ror
        ror     tmp2
        dey
        bne     m8
        tax
        lda     tmp2
        rts
mul16:  lda     #0
        sta     tmp3+1
loop:   lsr     tmp2+1
        ror     tmp2
        bcc     noadd
        clc
        adc     tmp1
        tax
        lda     tmp3+1
        adc     tmp1+1
        sta     tmp3+1
        txa
noadd:  asl     tmp1
        rol     tmp1+1
        ldx     tmp2
        bne     loop
        ldx     tmp2+1
        bne     loop
        ldx     tmp3+1
        rts
.endproc

; vi:syntax=asm_ca65
