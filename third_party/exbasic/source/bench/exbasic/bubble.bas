DIM A(199)
X = 1
FOR I = 0 TO 199
  X = (X * 21 + 7) MOD 1000
  A(I) = X
NEXT
T0 = TIME
FOR I = 198 TO 0 STEP -1
  FOR J = 0 TO I
    IF A(J) > A(J + 1)
      T = A(J) : A(J) = A(J + 1) : A(J + 1) = T
    ENDIF
  NEXT
NEXT
C = 0
FOR I = 0 TO 198
  IF A(I) > A(I + 1) THEN INC C
NEXT
C = C * 10000 + A(0) * 1000 + A(199)
T1 = TIME
F = FRE()
OPEN #1, 8, 0, "H:R.TXT"
PRINT #1, C; " "; T1 - T0; " "; F; " "; DPEEK(741)
CLOSE #1
