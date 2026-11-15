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


; Clear Memory
; ------------

        .export         clear_data, alloc_array, mem_set, mem_set_0
        .import         err_nomem
        .export         CLEAR_DATA
        .exportzp       saved_cpu_stack, heap_top

        .import         putc, EXB_VARS, EXB_VARS_END, EXB_FREE
        .importzp       tmp1, array_ptr, move_dest

        .include        "target.inc"


        ; Allocation size
alloc_size=     tmp1

        ; Uppercase to export as exBASIC symbol
CLEAR_DATA = clear_data

        ; ZP location to save the old CPU stack
        .zeropage
saved_cpu_stack:
        .res    1
        ; Page where the memory for arrays ends, if lower than MEMTOP, or 0:
        ; set by PMGRAPHICS to protect the P/M graphics area.
heap_top:
        .res    1

;----------------------------------------------------------
; Following routines are part of the runtime
        .segment        "RUNTIME"

        ; Clears data pointers before starting the interpreter
.proc   clear_data
        ; Clear the variables, "allocating" them
        lda     #<EXB_VARS
        sta     array_ptr
        lda     #>EXB_VARS
        sta     array_ptr+1
        lda     #<(EXB_VARS_END - EXB_VARS)
        ldx     #>(EXB_VARS_END - EXB_VARS)
        jsr     alloc_array
        ; Arrays and strings are allocated from the end of the program
        lda     #<EXB_FREE
        sta     array_ptr
        lda     #>EXB_FREE
        sta     array_ptr+1
        rts
.endproc

        ; Allocate space for a new array AX = SIZE
        ; Returns: pointer to allocated memory in MOVE_DEST
        ;          size of allocated memory in ALLOC_SIZE
        ;          X=0 and Y=0
.proc alloc_array

        sta     alloc_size
        stx     alloc_size + 1

        lda     array_ptr
        sta     move_dest
        clc
        adc     alloc_size
        sta     array_ptr
        tay

        lda     array_ptr+1
        sta     move_dest+1
        adc     alloc_size+1
        sta     array_ptr+1
        bcs     nomem

        cpy     MEMTOP
        sbc     MEMTOP+1
        bcs     nomem

        ; Must also end before the P/M graphics area (page in heap_top)
        lda     heap_top
        beq     ok
        cmp     array_ptr+1
        bcc     nomem
        bne     ok
        lda     array_ptr
        bne     nomem
ok:
        ; Tell the OS where the program memory ends, so opening a graphics
        ; mode can't put the screen over the arrays (it gives error 147).
        lda     array_ptr
        sta     APPMHI
        lda     array_ptr+1
        sta     APPMHI+1

        ; Clears memory from (move_dest) of (alloc_size) size
::mem_set_0:
        ldy     #0      ; Value to set

::mem_set:
        txa     ; X = (alloc_size+1)
        clc
        adc     move_dest+1
        sta     move_dest+1
        tya
        inx
        ldy     alloc_size
        jmp     nxt     ; Start loop

pgloop: dec     move_dest+1
loop:   dey
        sta     (move_dest), y
nxt:    bne     loop

        dex
        bne     pgloop

        rts
.endproc

nomem:  jmp     err_nomem

; vi:syntax=asm_ca65
