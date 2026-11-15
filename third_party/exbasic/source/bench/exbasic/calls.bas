T0 = TIME
C = 0
FOR I = 1 TO 3000
  AddOne()
NEXT
T1 = TIME
F = FRE()
OPEN #1, 8, 0, "H:R.TXT"
PRINT #1, C; " "; T1 - T0; " "; F; " "; DPEEK(741)
CLOSE #1

FUNC AddOne()
  INC C
ENDFUNC
