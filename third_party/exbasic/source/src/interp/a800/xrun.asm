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

; COMMON and RUN: keep variables when loading the next program
; -------------------------------------------------------------
;
; RUN "D:FILE" opens the file in channel #7 (in the BASIC code), and then
; calls DO_RUN: if the file was opened, it copies the COMMON variables to a
; buffer just below MEMTOP and loads the program with a small XEX loader in
; page 6, so the load can't overwrite it.
;
; The COMMON statement calls DO_COMMON_LOAD, that copies the saved values to
; the variables if the buffer is valid and the list of variables (types and
; count) is the same.
;
; The compiler generates the table of COMMON variables:
;       .word   signature       ; from the variable count and types
;       .byte   count
;       .byte   type            ; 0 = integer, 1 = floating point, 2 = string
;       .word   address         ; of the variable
;       ...
;
; Each value uses 2, 6 or 256 bytes in the buffer.  The header is at $06F0,
; outside the area used by the string functions ($0600-$067F):
;       .byte   $FB, $C0       ; magic
;       .word   signature
;       .word   buffer address

        .export         DO_RUN, DO_COMMON_LOAD
        .import         get_pointers
        .importzp       saved_cpu_stack, saddr, tmp1, tmp2, tmp3, IOERROR
        .importzp       move_source, move_dest

        .include        "target.inc"

NMIEN_REG = $D40E

HEADER  = $06F0
H_MAGIC = HEADER
H_SIG   = HEADER + 2
H_BUF   = HEADER + 4
MAGIC0  = $FB
MAGIC1  = $C0

table   = tmp1          ; pointer to the variables table
bufp    = tmp2          ; pointer into the buffer
count   = tmp3          ; variables left
vtype   = tmp3+1        ; current variable type

        .segment        "RUNTIME"

        ; Reads the next table entry: type into vtype, variable address
        ; into saddr.  Returns Y=0.
.proc   next_var
        ldy     #0
        lda     (table), y
        sta     vtype
        iny
        lda     (table), y
        sta     saddr
        iny
        lda     (table), y
        sta     saddr+1
        lda     table
        clc
        adc     #3
        sta     table
        bcc     :+
        inc     table+1
:       ldy     #0
        rts
.endproc

        ; Returns in A the size in the buffer of type vtype minus one, 0 for
        ; strings (256 bytes)
.proc   type_size
        ldx     vtype
        lda     sizes, x
        rts
sizes:  .byte   1, 5, 0
.endproc

        ; bufp += size of vtype
.proc   next_buf
        lda     vtype
        cmp     #2
        bne     :+
        inc     bufp+1
        rts
:       jsr     type_size
        sec
        adc     bufp
        sta     bufp
        bcc     :+
        inc     bufp+1
:       rts
.endproc

        ; Starts reading the table at AX: reads signature into A/X and count
.proc   table_start
        sta     table
        stx     table+1
        ldy     #2
        lda     (table), y
        sta     count
        ldy     #1
        lda     (table), y
        tax
        dey
        lda     (table), y
        pha
        lda     table
        clc
        adc     #3
        sta     table
        bcc     :+
        inc     table+1
:       pla
        rts
.endproc

; COMMON: restore the variables saved by the previous program.
; AX = address of the variables table.
.proc   DO_COMMON_LOAD
        jsr     table_start
        ; Check header
        cmp     H_SIG
        bne     invalid
        cpx     H_SIG+1
        bne     invalid
        lda     H_MAGIC
        cmp     #MAGIC0
        bne     invalid
        lda     H_MAGIC+1
        cmp     #MAGIC1
        bne     invalid
        lda     count
        beq     invalid
        lda     H_BUF
        sta     bufp
        lda     H_BUF+1
        sta     bufp+1
loop:   jsr     next_var
        lda     vtype
        cmp     #2
        beq     string
        ; Copy number from buffer to the variable
        jsr     type_size
        tay
:       lda     (bufp), y
        sta     (saddr), y
        dey
        bpl     :-
        bmi     next
string: ; Copy string from buffer, allocating the variable.  The allocation
        ; uses tmp1, so save the table pointer.
        lda     table
        pha
        lda     table+1
        pha
        lda     bufp
        ldx     bufp+1
        jsr     get_pointers    ; Returns Y=0
        pla
        sta     table+1
        pla
        sta     table
        lda     (move_source), y
        sta     (move_dest), y
        tax
        beq     next
:       iny
        lda     (move_source), y
        sta     (move_dest), y
        dex
        bne     :-
next:   jsr     next_buf
        dec     count
        bne     loop
invalid:
        ; Invalidate the header, so the values are only used once
        lda     #0
        sta     H_MAGIC
        rts
.endproc

; RUN: if the file was opened, save COMMON variables and load the program.
; AX = address of the variables table.
.proc   DO_RUN
        ldy     IOERROR
        cpy     #1
        beq     :+
        rts                     ; Error opening, return to BASIC with ERR()
:       jsr     table_start
        stx     H_SIG+1
        sta     H_SIG
        lda     #0
        sta     H_MAGIC         ; Invalid until the values are saved
        sta     no_common
        lda     count
        bne     :+
        inc     no_common
        jmp     load            ; No COMMON variables, just load
:
        ; Calculate buffer size and save table position
        lda     table
        pha
        lda     table+1
        pha
        lda     count
        pha
        lda     MEMTOP
        sta     bufp
        lda     MEMTOP+1
        sta     bufp+1
size:   jsr     next_var
        ; bufp -= size of type
        lda     vtype
        cmp     #2
        bne     :+
        dec     bufp+1
        bne     snext
:       jsr     type_size
        eor     #$FF            ; ~(size-1) = -size
        clc
        adc     bufp
        sta     bufp
        bcs     snext
        dec     bufp+1
snext:  dec     count
        bne     size
        lda     bufp
        sta     H_BUF
        lda     bufp+1
        sta     H_BUF+1
        pla
        sta     count
        pla
        sta     table+1
        pla
        sta     table

        ; Copy all values to the buffer
copy:   jsr     next_var
        lda     vtype
        cmp     #2
        beq     cstr
        jsr     type_size
        tay
:       lda     (saddr), y
        sta     (bufp), y
        dey
        bpl     :-
        bmi     cnext
cstr:   ; String: the variable holds the address of the string, 0 if empty
        lda     (saddr), y
        sta     move_source
        iny
        lda     (saddr), y
        sta     move_source+1
        ldy     #0
        tya
        ldx     move_source+1
        beq     slen            ; Never assigned, store an empty string
        lda     (move_source), y
slen:   sta     (bufp), y
        tax
        beq     cnext
:       iny
        lda     (move_source), y
        sta     (bufp), y
        dex
        bne     :-
cnext:  jsr     next_buf
        dec     count
        bne     copy

        lda     #MAGIC0
        sta     H_MAGIC
        lda     #MAGIC1
        sta     H_MAGIC+1

load:   ; Disable display list interrupts, the handlers will be overwritten
        lda     #$40
        sta     NMIEN_REG
        ; Copy loader to page 6 and run it
        ldx     #0
:       lda     loader_image, x
        sta     LOADER, x
        inx
        cpx     #LOADER_SIZE
        bne     :-
        lda     saved_cpu_stack
        sta     ld_stack
        lda     no_common
        sta     ld_no_buf
        jmp     LOADER

no_common:
        .byte   0
.endproc

; The XEX loader, runs in page 6.  Reads from channel #7, already open.
; Its variables are in the interpreter zero page temporaries, not part of the
; program file, so they can't be overwritten by the loaded program.
        ; The loader's variables are in page zero above $CB: the program's
        ; first segment (the load-time check, ldchk.asm, memory area LDCHK)
        ; is loaded at $80-$CB.  They are in the runtime's page zero area,
        ; free as the old program is gone.
ld_hdr          = $CC
ld_start        = $CE
ld_stack        = $D0
ld_no_buf       = $D1

loader_image:
        .org    $0600
LOADER:
        ldx     ld_stack        ; Restore the stack of the program start,
        txs                     ; so the new program returns to DOS on END
        lda     #<ld_fail       ; Return to DOS if there is no run address
        sta     RUNAD
        lda     #>ld_fail
        sta     RUNAD+1
        jsr     ld_read2        ; Binary header, must be $FFFF
        bmi     ld_fail_j
        lda     ld_hdr
        and     ld_hdr+1
        cmp     #$FF
        beq     ld_seg
ld_fail_j:
        jmp     ld_fail
ld_seg: jsr     ld_read2        ; Segment start
        bpl     :+
        cpy     #136            ; End of file?
        bne     ld_fail_j
        jmp     ld_run
:       lda     ld_hdr
        and     ld_hdr+1
        cmp     #$FF
        bne     :+
        jsr     ld_read2        ; Skip $FFFF marks
        bmi     ld_fail_j
:       lda     ld_hdr
        sta     ld_start
        lda     ld_hdr+1
        sta     ld_start+1
        jsr     ld_read2        ; Segment end
        bmi     ld_fail
        ; Can't load into page 6
        lda     ld_start+1
        cmp     #7
        bcs     :+              ; start >= $0700, ok
        lda     ld_hdr+1
        cmp     #6
        bcs     ld_fail         ; end >= $0600, overlaps
:       ; Can't load over the COMMON buffer, from H_BUF to MEMTOP
        lda     ld_no_buf
        bne     :+
        lda     ld_hdr          ; end < H_BUF ?
        cmp     H_BUF
        lda     ld_hdr+1
        sbc     H_BUF+1
        bcc     :+
        lda     ld_start        ; start >= MEMTOP ?
        cmp     MEMTOP
        lda     ld_start+1
        sbc     MEMTOP+1
        bcc     ld_fail
:       ; Length = end - start + 1
        lda     ld_hdr
        sec
        sbc     ld_start
        sta     ICBLL+$70
        lda     ld_hdr+1
        sbc     ld_start+1
        sta     ICBLH+$70
        inc     ICBLL+$70
        bne     :+
        inc     ICBLH+$70
:       lda     #<ld_rts        ; INITAD = RTS
        sta     INITAD
        lda     #>ld_rts
        sta     INITAD+1
        lda     ld_start
        ldy     ld_start+1
        jsr     ld_read         ; Read segment data
        bmi     ld_fail
        lda     INITAD          ; Call INITAD if changed
        cmp     #<ld_rts
        bne     :+
        lda     INITAD+1
        cmp     #>ld_rts
        beq     :++
:       jsr     ld_init
:       jmp     ld_seg

ld_run: jsr     ld_close
        jmp     (RUNAD)

ld_fail:
        jsr     ld_close
        ldx     ld_stack        ; Return to DOS
        txs
ld_rts: rts

ld_init:
        jmp     (INITAD)

        ; Reads two bytes to ld_hdr
ld_read2:
        lda     #2
        sta     ICBLL+$70
        lda     #0
        sta     ICBLH+$70
        lda     #<ld_hdr
        ldy     #>ld_hdr
        ; Reads ICBLL bytes to address A/Y, returns N=1 and Y=status on error
ld_read:
        ldx     #$70
        sta     ICBAL, x
        tya
        sta     ICBAH, x
        lda     #7              ; GET CHARS
        sta     ICCOM, x
        jmp     CIOV

ld_close:
        ldx     #$70
        lda     #12             ; CLOSE
        sta     ICCOM, x
        jmp     CIOV

ld_end:
        .reloc

LOADER_SIZE = ld_end - LOADER
        .assert LOADER_SIZE <= $F0, error, "RUN loader too big for page 6"

; vi:syntax=asm_ca65
