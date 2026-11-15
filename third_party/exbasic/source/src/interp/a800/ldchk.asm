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

; Load-time memory check
; ----------------------
;
; The first segment of a compiled program (memory area LDCHK in
; exbasic.cfg, run through INITAD before the rest is loaded).  The program
; is loaded from __STARTADDRESS__ up to EXB_FREE, the start of the free
; memory: if the DOS (with its drivers) ends above the start (MEMLO), or the
; program does not fit below MEMTOP (for example with the BASIC cartridge
; on), loading it would overwrite the DOS or end in the cartridge.  Then it
; prints a message and goes back to DOS before anything else is loaded.
;
; It is loaded in page zero, at $80, where the interpreter loop is loaded
; later: no DOS uses that memory while loading, and RUN's loader keeps its
; own variables in page 6.

        .export         ld_check
        .import         __STARTADDRESS__, EXB_FREE

        .include        "atari.inc"

        .segment        "LDCHK"

.proc   ld_check
        lda     #<__STARTADDRESS__      ; start < MEMLO: over the DOS
        cmp     MEMLO
        lda     #>__STARTADDRESS__
        sbc     MEMLO+1
        bcc     fail
        lda     MEMTOP                  ; MEMTOP < last byte: does not fit
        cmp     #<(EXB_FREE - 1)
        lda     MEMTOP+1
        sbc     #>(EXB_FREE - 1)
        bcs     ok
fail:   ldx     #0                      ; the message to E: (channel 0)
        lda     #PUTREC
        sta     ICCOM
        lda     #<msg
        sta     ICBAL
        stx     ICBAH                   ; (msg is in page zero)
        lda     #msg_len
        sta     ICBLL
        stx     ICBLH
        jsr     CIOV
        jmp     (DOSVEC)
ok:     rts
msg:    .byte   "ERROR- 19", $9B  ; (Atari BASIC's "LOAD program too long")
msg_len = * - msg
.endproc

; vi:syntax=asm_ca65
