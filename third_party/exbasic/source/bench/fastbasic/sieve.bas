DIM FLAG(8190) BYTE
T0 = TIME
C = 0
FOR I = 0 TO 8190
  IF NOT FLAG(I)
    P = I + I + 3
    FOR K = I + P TO 8190 STEP P
      FLAG(K) = 1
    NEXT
    INC C
  ENDIF
NEXT
T1 = TIME
F = FRE()
OPEN #1, 8, 0, "H:R.TXT"
PRINT #1, C; " "; T1 - T0; " "; F; " "; DPEEK(741)
CLOSE #1
