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

; String functions: LEFT$, RIGHT$, MID$, UCASE$, LCASE$, LTRIM$, RTRIM$,
; INSTR and the MID$ statement
; ----------------------------------------------------------------------
;
; One instruction, STRF, followed by a byte that selects the function.  The
; compiler first copies the string argument to a hidden string variable (as
; with "+"), so the functions change that copy in place and return it.
;
;  0 LEFT$   stack: string           AX = count       returns the string
;  1 RIGHT$  stack: string           AX = count
;  2 MID$    stack: string, start    AX = count
;  3 UCASE$                          AX = string
;  4 LCASE$                          AX = string
;  5 LTRIM$                          AX = string
;  6 RTRIM$                          AX = string
;  7 INSTR   stack: string, search   AX = start       returns the position
;  8 MID$ statement:  stack: string, start, count     AX = new text
;
; Positions start at 1, counts below 0 are 0 and above 255 are 255.

        .importzp       cptr, sptr, tmp1, tmp2, tmp3, next_instruction
        .import         stack_l, stack_h

        .segment        "RUNTIME"

str     = tmp1          ; pointer to the string
src     = tmp2          ; second string, or temporary
len     = tmp3          ; length of the string
cnt     = tmp3+1        ; count

.proc   EXE_STRF
        pha                     ; Save A
        ldy     #0              ; Read the function number
        lda     (cptr), y
        inc     cptr
        bne     :+
        inc     cptr+1
:       asl
        tay
        lda     table+1, y
        sta     src+1
        lda     table, y
        sta     src
        pla
        jmp     (src)
table:  .word   left, right, mid, ucase, lcase, ltrim, rtrim, instr, midset
.endproc

        ; Converts AX to a count from 0 to 255 in A
.proc   clamp_count
        cpx     #0
        beq     ok
        bmi     zero
        lda     #255
        rts
zero:   lda     #0
ok:     rts
.endproc

        ; Converts AX to a position from 1 to 255 in A, 0 if bigger
.proc   clamp_pos
        cpx     #0
        bmi     one
        bne     big
        cmp     #0
        bne     ok
one:    lda     #1
ok:     rts
big:    lda     #0
        rts
.endproc

        ; Pops a value from the stack into AX
.proc   pop_ax
        ldy     sptr
        lda     stack_l, y
        ldx     stack_h, y
        inc     sptr
        rts
.endproc

        ; Pops a string pointer from the stack into str, and its length
.proc   pop_str
        jsr     pop_ax
.endproc        ; Fall through
.proc   set_str         ; AX = string
        sta     str
        stx     str+1
        ldy     #0
        lda     (str), y
        sta     len
        rts
.endproc

        ; Returns str in AX
.proc   ret_str
        lda     str
        ldx     str+1
        jmp     next_instruction
.endproc

        ; Sets the length of the string to A and returns it
.proc   set_len
        ldy     #0
        sta     (str), y
        beq     ret_str
        bne     ret_str
.endproc

        ; Moves cnt characters from position Y to position 1, sets the length
        ; to cnt and returns the string
.proc   move_down
        sty     src             ; Source position
        ldy     #1              ; Destination position
loop:   dey
        cpy     cnt
        iny
        bcs     done
        sty     src+1
        ldy     src
        lda     (str), y
        inc     src
        ldy     src+1
        sta     (str), y
        iny
        bne     loop
done:   lda     cnt
        jmp     set_len
.endproc

.proc   left
        jsr     clamp_count
        pha
        jsr     pop_str
        pla
        cmp     len
        bcs     ret_str         ; Count >= length: the whole string
        jmp     set_len
.endproc

.proc   right
        jsr     clamp_count
        pha
        jsr     pop_str
        pla
        cmp     len
        bcs     ret_str
        sta     cnt
        lda     len             ; First character: len - cnt + 1
        sec
        sbc     cnt
        tay
        iny
        jmp     move_down
.endproc

.proc   mid
        jsr     clamp_count
        sta     cnt
        jsr     pop_ax          ; Start position
        jsr     clamp_pos
        pha
        jsr     pop_str
        pla
        beq     empty           ; Start > 255
        cmp     len
        beq     :+
        bcs     empty           ; Start after the end
:       sta     src
        lda     len             ; Available: len - start + 1
        sec
        sbc     src
        tax
        inx
        cpx     cnt
        bcs     :+
        stx     cnt
:       ldy     src
        jmp     move_down
empty:  lda     #0
        jmp     set_len
.endproc

.proc   ucase
        jsr     set_str
        lda     #'a'
        ldy     #'z'
        bne     change_case
.endproc

.proc   lcase
        jsr     set_str
        lda     #'A'
        ldy     #'Z'
.endproc        ; Fall through

        ; Flips bit 5 of the characters from A to Y
.proc   change_case
        sta     src             ; First letter
        sty     src+1           ; Last letter
        ldy     len
        beq     done
loop:   lda     (str), y
        cmp     src
        bcc     next
        cmp     src+1
        beq     :+
        bcs     next
:       eor     #$20
        sta     (str), y
next:   dey
        bne     loop
done:   jmp     ret_str
.endproc

.proc   ltrim
        jsr     set_str
        beq     done            ; Empty string
        ldy     #0
:       iny
        lda     (str), y
        cmp     #' '
        bne     found
        cpy     len
        bne     :-
        lda     #0              ; All spaces
        jmp     set_len
done:   jmp     ret_str
found:  sty     src             ; Keep len - Y + 1 characters from Y
        lda     len
        sec
        sbc     src
        sta     cnt
        inc     cnt
        jmp     move_down
.endproc

.proc   rtrim
        jsr     set_str
        tay
        beq     done
:       lda     (str), y
        cmp     #' '
        bne     done
        dey
        bne     :-
done:   tya
        jmp     set_len
.endproc

        ; INSTR(string, search, start): position of "search" in "string",
        ; from position "start", or 0 if not found.
.proc   instr
        jsr     clamp_pos
        sta     cnt             ; Position to test (0 if > 255)
        jsr     pop_ax          ; Search string
        sta     src
        stx     src+1
        jsr     pop_str
        lda     cnt
        beq     not_found
        ; Last position: len - search length + 1, in len
        ldy     #0
        lda     len
        sec
        sbc     (src), y
        bcc     not_found       ; The search is longer than the string
        sta     len
        inc     len
        ; str = string + position - 1: (str),y is the character at
        ; position + y - 1
        lda     cnt
        sec
        sbc     #1
        clc
        adc     str
        sta     str
        bcc     pos
        inc     str+1
pos:    lda     len             ; Position after the last?
        cmp     cnt
        bcc     not_found
        ldy     #0
        lda     (src), y
        beq     found           ; An empty search is found at the start
        tax
        ldy     #1
cmp_ch: lda     (src), y
        cmp     (str), y
        bne     next
        iny
        dex
        bne     cmp_ch
found:  lda     cnt
        ldx     #0
        jmp     next_instruction
next:   inc     str
        bne     :+
        inc     str+1
:       inc     cnt
        bne     pos
not_found:
        lda     #0
        tax
        jmp     next_instruction
.endproc

        ; MID$(string, start, count) = text: replaces characters of the
        ; string with the text, without changing its length
.proc   midset
        sta     src             ; New text
        stx     src+1
        jsr     pop_ax          ; Count
        jsr     clamp_count
        sta     cnt
        jsr     pop_ax          ; Start
        jsr     clamp_pos
        pha
        jsr     pop_str
        pla
        beq     done            ; Start > 255
        cmp     len
        beq     :+
        bcs     done            ; Start after the end
:       ; Available: len - start + 1
        tax
        eor     #$FF
        sec
        adc     len
        clc
        adc     #1
        cmp     cnt
        bcs     :+
        sta     cnt
:       ldy     #0              ; Length of the text
        lda     (src), y
        cmp     cnt
        bcs     :+
        sta     cnt
:       ; str = string + start - 1, so the copy is (src),y to (str),y
        dex
        txa
        clc
        adc     str
        sta     str
        bcc     :+
        inc     str+1
:       ldy     cnt
        beq     done
copy:   lda     (src), y
        sta     (str), y
        dey
        bne     copy
done:   jmp     next_instruction
.endproc

        .include "deftok.inc"
        deftoken "STRF"

; vi:syntax=asm_ca65
