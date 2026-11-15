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

; FRAME_SAVE / FRAME_LOAD: save and restore the variables of a recursive FUNC
; ---------------------------------------------------------------------------
;
; Both are followed by a count of variables, and for each variable the
; variable number and the size in words (1, or 3 for floating point).
; FRAME_SAVE copies the variables to the frame stack, FRAME_LOAD copies them
; back, the compiler lists the variables in reverse order for FRAME_LOAD.
;
; The frame stack is in the BSS segment, so it does not use space in the
; program file.  On overflow, the program stops with a memory error.

        .import         get_op_var, err_stack
        .importzp       cptr, tmp1, tmp2, tmp3, next_instruction

FRAME_SIZE = 512

        .bss
frame_stack:    .res    FRAME_SIZE

        .data
frame_ptr:      .word   frame_stack + FRAME_SIZE
        ; Strings not used yet are saved as this empty string
empty_str:      .byte   0

        .segment        "RUNTIME"

        ; Reads one byte from the instruction stream
.proc   read_byte
        ldy     #0
        lda     (cptr), y
        inc     cptr
        bne     :+
        inc     cptr+1
:       rts
.endproc

        ; Reads the variable count and the frame pointer
.proc   frame_start
        jsr     read_byte
        sta     tmp3
        lda     frame_ptr
        sta     tmp2
        lda     frame_ptr+1
        sta     tmp2+1
        rts
.endproc

        ; Reads a variable from the instruction stream: its address and a
        ; size code, 1 for integers, 3 for floating point and $80 for
        ; strings.  Returns in tmp1 the address of the data and in tmp3+1
        ; the number of bytes to copy (0 means 256).  For strings the data
        ; is the string itself, its length plus one bytes: on save (C=1) the
        ; length is read from the string, on load (C=0) from the frame.
.proc   frame_var
        php
        jsr     get_op_var
        sta     tmp1
        stx     tmp1+1
        jsr     read_byte
        cmp     #$80            ; read_byte leaves the flags of the pointer
        bcs     string
        plp
        asl
        sta     tmp3+1
        rts
string: ldy     #1              ; The string address, 0 if not used yet
        lda     (tmp1), y
        tax
        dey
        lda     (tmp1), y
        cpx     #0              ; Not used yet: save it as an empty string
        bne     :+
        lda     #<empty_str
        ldx     #>empty_str
:       sta     tmp1
        stx     tmp1+1
        plp
        bcc     load
        lda     (tmp1), y       ; Save: length of the string
        bcs     size
load:   lda     (tmp2), y       ; Load: length in the frame
size:   clc
        adc     #1
        sta     tmp3+1
        rts
.endproc

        ; Copies tmp3+1 bytes (0 = 256)
.proc   copy_to_frame
        ldy     tmp3+1
:       dey
        lda     (tmp1), y
        sta     (tmp2), y
        tya
        bne     :-
        rts
.endproc

.proc   copy_from_frame
        ldy     tmp3+1
:       dey
        lda     (tmp2), y
        sta     (tmp1), y
        tya
        bne     :-
        rts
.endproc

        ; Stores the frame pointer and continues
.proc   frame_end
        lda     tmp2
        sta     frame_ptr
        lda     tmp2+1
        sta     frame_ptr+1
        jmp     next_instruction
.endproc

.proc   EXE_FRAME_SAVE
        jsr     frame_start
loop:   sec
        jsr     frame_var
        ; frame_ptr -= size (0 = 256)
        lda     tmp2
        sec
        sbc     tmp3+1
        sta     tmp2
        bcs     :+
        dec     tmp2+1
:       ldx     tmp3+1
        bne     :+
        dec     tmp2+1
:       ; Check for overflow
        cmp     #<frame_stack
        lda     tmp2+1
        sbc     #>frame_stack
        bcc     overflow
        jsr     copy_to_frame
        dec     tmp3
        bne     loop
        jmp     frame_end
overflow:
        jmp     err_stack
.endproc

.proc   EXE_FRAME_LOAD
        jsr     frame_start
loop:   clc
        jsr     frame_var
        jsr     copy_from_frame
        ; frame_ptr += size (0 = 256)
        lda     tmp2
        clc
        adc     tmp3+1
        sta     tmp2
        bcc     :+
        inc     tmp2+1
:       ldx     tmp3+1
        bne     :+
        inc     tmp2+1
:       dec     tmp3
        bne     loop
        jmp     frame_end
.endproc

        .include "deftok.inc"
        deftoken "FRAME_SAVE"
        deftoken "FRAME_LOAD"

; vi:syntax=asm_ca65
