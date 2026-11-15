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

; String concatenation with "+" in expressions
; --------------------------------------------
;
; The compiler uses a hidden string variable as the result of a "+" chain:
;       A$ + B$ + C$   ->   A$ TSTR_SET t  B$ TSTR_ADD t  C$ TSTR_ADD t
; TSTR_SET copies the string in AX to the variable "t", TSTR_ADD appends it,
; both return the address of the variable string in AX.  The SADDR pointer
; is preserved, so the result can be assigned with COPY_STR.

        .import         get_pointers, get_op_var
        .importzp       tmp1, saddr, next_instruction
        .importzp       move_source, move_dest, move_loop

        .segment        "RUNTIME"

.proc   EXE_TSTR_ADD    ; AX: string to append, opcode: variable
        ldy     #1
        .byte   $2C     ; BIT abs, skips the LDY
::EXE_TSTR_SET:         ; AX: string to copy, opcode: variable
        ldy     #0
        sty     tmp1+1
        sta     move_source
        stx     move_source+1
        lda     saddr
        pha
        lda     saddr+1
        pha
        jsr     get_op_var
        sta     saddr
        stx     saddr+1
        lda     move_source
        ldx     move_source+1
        jsr     get_pointers    ; Allocates the string if needed, Y = 0
        pla
        sta     saddr+1
        pla
        sta     saddr
        ; Result address
        lda     move_dest+1
        pha
        lda     move_dest
        pha

        lda     tmp1+1
        bne     append

        ; Copy length and data
        lda     (move_source), y
        sta     (move_dest), y
        tay
        bne     copy
        beq     xit

append: lda     (move_dest), y  ; Destination length
        sta     tmp1
        clc
        adc     (move_source), y; Source length
        bcc     :+
        lda     #255            ; Fix at maximum length
:       sta     (move_dest), y
        sec
        sbc     tmp1
        beq     xit             ; No bytes to copy
        tay
        lda     tmp1
        clc
        adc     move_dest
        sta     move_dest
        bcc     copy
        inc     move_dest+1
copy:   jsr     move_loop
xit:    pla
        tay
        pla
        tax
        tya
        jmp     next_instruction
.endproc

        .include "deftok.inc"
        deftoken "TSTR_SET"
        deftoken "TSTR_ADD"

; vi:syntax=asm_ca65
