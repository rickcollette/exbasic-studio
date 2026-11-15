T0 = TIME
S% = 0 : R% = 0
FOR N = 1 TO 100
  A% = N
  FOR I = 1 TO 10
    A% = SQR(A%) : R% = R% + RND()
  NEXT
  FOR I = 1 TO 10
    A% = A% ^ 2 : R% = R% + RND()
  NEXT
  S% = S% + A%
NEXT
C = INT(ABS(1010 - S% / 5) * 100000)
T1 = TIME
F = FRE()
OPEN #1, 8, 0, "H:R.TXT"
PRINT #1, C; " "; T1 - T0; " "; F; " "; DPEEK(741)
CLOSE #1
