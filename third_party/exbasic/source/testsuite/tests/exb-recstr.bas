' Recursive FUNC with string locals does not lose memory
FUNC Rev$(S$)
  LOCAL R$
  IF LEN(S$) <= 1 THEN RETURN S$
  R$ = Rev$(S$[2]) + S$[1, 1]
  RETURN R$
ENDFUNC
' The first call creates the strings; later calls must not use more memory
T$ = Rev$("abcdef")
Free = FRE()
FOR I = 1 TO 50
  T$ = Rev$("abcdef")
NEXT I
PRINT T$
PRINT FRE() = Free
