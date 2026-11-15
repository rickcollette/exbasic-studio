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


; Convert FP to integer
; ---------------------

        .import         neg_AX, pop_fr0
        .importzp       IOERROR, next_instruction

        .include "atari.inc"

        .segment        "RUNTIME"

.proc   EXE_FP_INT      ; Convert FP to INT, with rounding
        asl     FR0
        php             ; Store sign
        lsr     FR0
        jsr     FPI     ; Absolute value to 0-65535, C=1 if bigger
        bcs     sat
        ldx     FR0+1
        bpl     ok1     ; 0 to 32767
        ; 32768 to 65535 is only valid as -32768
        plp
        php
        bcc     sat
        cpx     #$80
        bne     sat
        lda     FR0
        beq     ok1
        ; Out of range: saturate to 32767 or -32768 and store error #3
sat:    lda     #3
        sta     IOERROR
        lda     #$FF
        ldx     #$7F
        plp
        php
        bcc     set
        lda     #0      ; 32768, negated to -32768 below
        ldx     #$80
set:    sta     FR0
        stx     FR0+1
        jmp     ok
ok1:    lda     #1      ; ERR() = 1, the conversion is valid
        sta     IOERROR
ok:     lda     FR0
        ldx     FR0+1
        ; Save A, pop FP stack (keeps X) and restore
        pha
        jsr     pop_fr0
        pla

        ; Negate result if original number was negative
        plp
        bcc     pos

        jsr     neg_AX
pos:
        jmp     next_instruction
.endproc

        .include "deftok.inc"
        deftoken "FP_INT"

; vi:syntax=asm_ca65
