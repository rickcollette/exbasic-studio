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

; Division and modulus with a cache, used by the DIVC and MODC instructions
; and by native code
; -------------------------------------------------------------------------

        .export         nat_divc, nat_modc
        .import         divmod_sign_adjust, neg_AX, stack_l, stack_h
        .importzp       sptr, tmp1, tmp2, tmp3, divmod_sign

        .segment        "RUNTIME"

; AX = (SP+) % AX and AX = (SP+) / AX, see EXE_DIVC.  The last operands and
; results are kept, so "A / B" followed by "A MOD B" divides only once.
.proc   nat_modc
        sec
        .byte   $24             ; BIT zp, skips the CLC
.endproc        ; Fall through

.proc   nat_divc
        clc
        php
        ldy     sptr
        sta     tmp1
        cmp     c_divisor
        bne     miss
        cpx     c_divisor+1
        bne     miss
        lda     stack_l, y
        cmp     c_dividend
        bne     miss
        lda     stack_h, y
        cmp     c_dividend+1
        bne     miss
hit:    inc     sptr
        plp
        bcs     rem
        lda     c_quotient
        ldx     c_quotient+1
        rts
rem:    lda     c_remainder
        ldx     c_remainder+1
        rts

miss:   stx     c_divisor+1
        lda     stack_l, y
        sta     c_dividend
        lda     stack_h, y
        sta     c_dividend+1
        lda     tmp1
        sta     c_divisor
        jsr     divmod_sign_adjust
        ldx     tmp2+1
        bit     divmod_sign
        bvc     rem_pos
        jsr     neg_AX
rem_pos:
        sta     c_remainder
        stx     c_remainder+1
        lda     tmp3
        ldx     tmp3+1
        bit     divmod_sign
        bpl     quo_pos
        jsr     neg_AX
quo_pos:
        sta     c_quotient
        stx     c_quotient+1
        jmp     hit
.endproc

        .data
c_dividend:     .word   0
c_divisor:      .word   1
c_quotient:     .word   0
c_remainder:    .word   0

; vi:syntax=asm_ca65
