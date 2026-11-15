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

; Runtime errors
; --------------
;
; Each error prints "ERROR-" and its number on the screen and ends the
; program, as Atari BASIC does, with Atari BASIC's numbers (Appendix B of
; the manual):
;
;   2   out of memory
;   9   bad DIM size
;   10  stack overflow
;   11  division by zero
;   128 the BREAK key was pressed (as the I/O error of BREAK)
;
; The entry points load the offset of the number in "codes" and skip the
; next ones with a BIT abs.

        .export         err_break, err_nomem, err_div0, err_stack, err_dim
        .import         putc, EXE_END
        .importzp       IOCHN

        .include        "atari.inc"

        .segment        "RUNTIME"

err_break:
        ldy     #$80            ; Clear the BREAK key flag
        sty     BRKKEY
        ldx     #c_break - codes
        .byte   $2C             ; BIT abs: skips the next LDX
err_nomem:
        ldx     #c_nomem - codes
        .byte   $2C
err_div0:
        ldx     #c_div0 - codes
        .byte   $2C
err_stack:
        ldx     #c_stack - codes
        .byte   $2C
err_dim:
        ldx     #c_dim - codes
        txa                     ; (putc changes X)
        pha
        ldy     #0              ; To the screen
        sty     IOCHN
:       lda     msg, y
        jsr     putc            ; (keeps Y)
        iny
        cpy     #msg_len
        bne     :-
        pla                     ; The number, up to the EOL
        tay
:       lda     codes, y
        pha
        jsr     putc
        pla
        iny
        cmp     #$9B
        bne     :-
        jmp     EXE_END

msg:    .byte   $9B, "ERROR- "
msg_len = * - msg
codes:
c_break: .byte  "128", $9B
c_nomem: .byte  "2", $9B
c_div0:  .byte  "11", $9B
c_stack: .byte  "10", $9B
c_dim:   .byte  "9", $9B

; vi:syntax=asm_ca65
