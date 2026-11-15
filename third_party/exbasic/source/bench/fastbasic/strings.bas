T0 = TIME
C = 0
FOR N = 1 TO 20
  S$ = ""
  FOR I = 1 TO 100
    S$ =+ CHR$(65 + I MOD 26)
  NEXT
  FOR I = 1 TO LEN(S$)
    IF S$[I, 1] = "Q" THEN INC C
  NEXT
NEXT
T1 = TIME
F = FRE()
OPEN #1, 8, 0, "H:R.TXT"
PRINT #1, C; " "; T1 - T0; " "; F; " "; DPEEK(741)
CLOSE #1
