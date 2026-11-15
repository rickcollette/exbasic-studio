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


; Call and Jump to address
; ------------------------

        .export         interpreter_jump_ax, CALL_STACK_MIN
        .import         err_stack, err_break
        .importzp       next_instruction, cptr

        .include        "target.inc"

        ; Lowest CPU stack pointer allowed when calling a function: leaves
        ; room for the OS, interrupts and the runtime subroutines.
CALL_STACK_MIN = $40

        .segment        "RUNTIME"

.proc   EXE_CALL
        ; Check that the CPU stack has room for the return address, and for
        ; the OS and interrupt routines called from the function.
        tsx
        cpx     #CALL_STACK_MIN
        bcc     call_overflow
        lda     cptr
        clc
        adc     #2
        pha
        lda     cptr+1
        adc     #0
        pha
.endproc        ; Fall through

.proc   EXE_JUMP
        ; Stops the program if the BREAK key was pressed.  All loops end in
        ; a jump, so this makes all programs stop on BREAK.
        ldy     BRKKEY
        beq     break
        ldy     #1              ; 2     2
        lda     (cptr), y       ; 5     2
        tax                     ; 2     1
        dey                     ; 2     1
        lda     (cptr), y       ; 5     2
::interpreter_jump_ax:
        stx     cptr+1          ; 3     2
sto:    sta     cptr            ; 3     2
        jmp     next_instruction
break:  jmp     err_break
.endproc

call_overflow:
        jmp     err_stack

        .export EXE_END
.proc   EXE_END ; EXIT from interpreter
        .importzp saved_cpu_stack
        .import SOUND_OFF
        ; Close the I/O channels 1 to 7 and turn the sound off, as the
        ; program can't do it any more.
        ldx     #$70
:       lda     #CLOSE
        sta     ICCOM, x
        jsr     CIOV
        txa
        sec
        sbc     #$10
        tax
        bne     :-
        jsr     SOUND_OFF
        ldx     saved_cpu_stack
        txs
        rts
.endproc


EXE_CNJUMP:
        eor     #1

.proc   EXE_CJUMP
        lsr
        bcc     EXE_JUMP

skip:   lda     cptr
;       sec             ; C is always set from above comparison
        adc     #1
        bcc     EXE_JUMP::sto
        inc     cptr+1
        bcs     EXE_JUMP::sto
.endproc

        .include "deftok.inc"
        deftoken "CALL"
        deftoken "JUMP"
        deftoken "CJUMP"
        deftoken "CNJUMP"

; vi:syntax=asm_ca65
