T0 = TIME
C = 0
FOR I = 1 TO 40
  FOR J = 1 TO 40
    X = I : Y = J
    WHILE Y
      Z = X MOD Y : X = Y : Y = Z
    WEND
    C = C + X
  NEXT
NEXT
T1 = TIME
F = FRE()
OPEN #1, 8, 0, "H:R.TXT"
PRINT #1, C; " "; T1 - T0; " "; F; " "; DPEEK(741)
CLOSE #1
