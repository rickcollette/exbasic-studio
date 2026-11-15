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


; FOR_NEXT_JUMP: same as "FOR_NEXT" followed by "CJUMP"
; -----------------------------------------------------
; Adds STEP to the FOR variable, compares with the limit (signed, the
; direction depends on the sign of STEP) and jumps to the loop body if the
; loop continues.  As FOR_NEXT, the stack is preserved for FOR_EXIT.

        .import         stack_l, stack_h, EXE_JUMP, inc_cptr_2
        .importzp       tmp2, tmp3

        .segment        "RUNTIME"

.proc   EXE_FOR_NEXT_JUMP
        ; Y = stack pointer: Y = step, Y+1 = limit, Y+2 = variable address
        lda     stack_l+2, y
        sta     tmp3
        lda     stack_h+2, y
        sta     tmp3+1
        tya
        tax

        ; VAR = VAR + STEP
        ldy     #0
        clc
        lda     (tmp3), y
        adc     stack_l, x
        sta     (tmp3), y
        sta     tmp2
        iny
        lda     (tmp3), y
        adc     stack_h, x
        sta     (tmp3), y
        sta     tmp2+1
        bvs     exit            ; Overflow: the loop ends

        ldy     stack_h, x
        bmi     negative

        ; Positive STEP: continue while VAR <= LIMIT
        lda     stack_l+1, x
        cmp     tmp2
        lda     stack_h+1, x
        sbc     tmp2+1
        bvc     test
        eor     #$80
test:   bmi     exit
        jmp     EXE_JUMP
exit:   jmp     inc_cptr_2

        ; Negative STEP: continue while VAR >= LIMIT
negative:
        lda     tmp2
        cmp     stack_l+1, x
        lda     tmp2+1
        sbc     stack_h+1, x
        bvc     test
        eor     #$80
        jmp     test
.endproc

        .include "deftok.inc"
        deftoken "FOR_NEXT_JUMP"

; vi:syntax=asm_ca65
