' Test abbreviations of ENDIF inside functions
FUNC P1(X)
  IF X > 1
    ? "BIG ";
  EN.
  ? X
ENDFUNC
FUNC P2(X)
  IF X = 1
    ? "ONE"
  ELIF X = 2
    ? "TWO"
  ELSE
    ? "MANY"
  E.
ENDFUNC
FUNC P3(X)
  DO
    IF X > 2 THEN EXIT
    INC X
  LOOP
  IF X = 3
    ? "THREE"
  ENDI.
ENDFUNC
FUNC P4()
  ? "P4"
ENDFUNC
? "Start"
P1(1):P1(5)
P2(1):P2(2):P2(3)
P3(0)
P4()
IF 1
  ? "IF"
EN.
