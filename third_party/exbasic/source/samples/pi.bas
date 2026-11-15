' Calculate PI to 254 digits
' ==========================

' Uses Machin formula:
'  Pi/4 = 4 * ATAN(1/5) - ATAN(1/239)
'
' The calculation is done in base 100,
' to simplify the code and printing.

' The arrays store two digits on each
' location (from 00 to 99).
DIM P(130), T(130) BYTE, SV(130) BYTE

' Arguments to procedures bellow
ZERO=0

' Precision in digit pairs
Q=127

' Start measuring time
STIME=TIME

' Cleanup numbers
FOR I=0 TO Q
 P(I) = 0
 T(I) = 0
NEXT

' Calculate ATAN(1/5), positive:
PRINT "A(1/5):";
ARCTAN_SMALL(1, 5)
PRINT

' Multiply by 4
MUL4()

' Calculate ATAN(1/239), negative:
PRINT "A(1/239):";
ARCTAN(0, 239)
PRINT

' Multiply all by 4
MUL4()

' Get calculation end time
ETIME=TIME

' Show our value of PI
SHOW()

ETIME=ETIME-STIME
PRINT "Elapsed time: ";ETIME/60;" s"

END

'-------------------------------------
' Prints number on P()
'
FUNC SHOW()
 PRINT "PI=";P(0);".";
 FOR I=1 TO Q-1
  S=P(I)
  IF S<10
   PRINT "0";S;
  ELSE
   PRINT S;
  ENDIF
 NEXT
 PRINT
ENDFUNC

'-------------------------------------
' Calculate ATAN(1/AT), adding the
' result to P()
'
FUNC ARCTAN(SIGN, AT)
 AS=SIGN ' ADDSUB uses the global AS
 T(0)=1
 ZERO=0
 DIV(AT)
 SAVE_T()
 N=1
 REPEAT
  ADDSUB()
  RESTORE_T()
  DIV(AT)
  DIV(AT)
  SAVE_T()
  ZERO=CHKZERO()
  N=N+2
  DIV(N)
  PRINT ".";
 UNTIL ZERO<0
ENDFUNC

'-------------------------------------
' Calculate ATAN(1/AT), with AT a small
' number (so that AT*AT*100 < 32768),
' adding the result to P().
'
FUNC ARCTAN_SMALL(SIGN, AT)
 AS=SIGN ' ADDSUB uses the global AS
 T(0)=1
 ZERO=0
 DIV(AT)
 SAVE_T()
 N=1
 AT=AT*AT
 REPEAT
  ADDSUB()
  RESTORE_T()
  N=N+2
  DIV(AT)
  ZERO=CHKZERO()
  SAVE_T()
  DIV(N)
  PRINT ".";
 UNTIL ZERO<0
ENDFUNC

'-------------------------------------
' ADDs or SUBs T() to P(), depending
' on AS.
'
FUNC ADDSUB()
 IF AS
  AS=0
  ADD()
 ELSE
  AS=1
  SUB()
 ENDIF
ENDFUNC

'-------------------------------------
' Returns the index of the first non-zero
' digit, or -1 if T() is all zero.
'
FUNC CHKZERO()
 FOR I=0 TO Q
  IF T(I) THEN RETURN I
 NEXT
 RETURN -1
ENDFUNC

'-------------------------------------
' Adds T() to P(), so P()=P()+T()
'
FUNC ADD()
 FOR J=Q TO ZERO STEP -1
  S=P(J)+T(J)
  IF S>99
   INC P(J-1)
   S=S-100
  ENDIF
  P(J)=S
 NEXT
 FOR J=ZERO-1 TO 0 STEP -1
  S=P(J)
  IF S>99
   INC P(J-1)
   S=S-100
   P(J)=S
  ELSE
   EXIT
  ENDIF
 NEXT
ENDFUNC

'-------------------------------------
' Subtract T() from P(), so P()=P()-T()
'
FUNC SUB()
 FOR J=Q TO ZERO STEP -1
  S=P(J)-T(J)
  IF S<0
   DEC P(J-1)
   S=S+100
  ENDIF
  P(J)=S
 NEXT
 FOR J=ZERO-1 TO 0 STEP -1
  S=P(J)
  IF S<0
   DEC P(J-1)
   S=S+100
   P(J)=S
  ELSE
   EXIT
  ENDIF
 NEXT
ENDFUNC

'-------------------------------------
' Multiplies T() by the small number
' MULTI, only works if MULTI*100<32768
'
FUNC MUL(MULTI)
 C=0
 FOR I=Q TO 0 STEP -1
  B = T(I) * MULTI + C
  T(I) = B MOD 100
  C = B / 100
 NEXT
ENDFUNC

'-------------------------------------
' Divides T() by the small number DIVI,
' only works if DIVI*100<32768
'
FUNC DIV(DIVI)
 C=0
 FOR I=ZERO TO Q
  B = 100 * C + T(I)
  T(I) = B / DIVI
  C = B MOD DIVI
 NEXT
ENDFUNC

'-------------------------------------
' Divides P() by 4. UNUSED!
'
FUNC DIV4()
 C=0
 FOR I=0 TO Q
  B = 100 * C + P(I)
  D = B MOD 4
  P(I) = B / 4
 NEXT
ENDFUNC

'-------------------------------------
' Multiplies P() by 4.
'
FUNC MUL4()
 C=0
 FOR I=Q TO 0 STEP -1
  B = P(I) * 4 + C
  C = B / 100
  P(I) = B MOD 100
 NEXT
ENDFUNC

'-------------------------------------
' Saves the value of T
'
FUNC SAVE_T()
 MOVE ADR(T), ADR(SV), Q+1
ENDFUNC

'-------------------------------------
' Restores the value of T
'
FUNC RESTORE_T()
 MOVE ADR(SV), ADR(T), Q+1
ENDFUNC


