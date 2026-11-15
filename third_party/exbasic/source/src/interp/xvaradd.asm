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

; VAR_ADD_VAR: AX = VAR1 + VAR2, same as "VAR_LOAD v1" + "ADD_VAR v2"
; --------------------------------------------------------------------

        .import         var_page, inc_cptr_2
        .importzp       cptr, tmp1, tmp2, tmp3

        .segment        "RUNTIME"

.proc   EXE_VAR_ADD_VAR
        ; Get both variable addresses, as "get_op_var":
        ;   var_address = var_num * 2 + var_page * 256
        ldy     #0
        lda     (cptr), y
        asl
        sta     tmp1
        lda     var_page
        adc     #0
        sta     tmp1+1
        iny
        lda     (cptr), y
        asl
        sta     tmp2
        lda     var_page
        adc     #0
        sta     tmp2+1

        ; AX = VAR1 + VAR2
        dey
        clc
        lda     (tmp1), y
        adc     (tmp2), y
        sta     tmp3
        iny
        lda     (tmp1), y
        adc     (tmp2), y
        tax
        lda     tmp3
        jmp     inc_cptr_2      ; Skip the two variable numbers
.endproc

        .include "deftok.inc"
        deftoken "VAR_ADD_VAR"

; vi:syntax=asm_ca65
