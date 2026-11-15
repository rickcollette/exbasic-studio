' FOR loops with constant start, limit and step (machine code)
S = 0
FOR I = 1 TO 10 : S = S + I : NEXT
PRINT S; " "; I
S = 0
FOR I = 10 TO 1 STEP -1 : S = S + I : NEXT
PRINT S; " "; I
S = 0
FOR I = -5 TO 5 STEP 3 : S = S + I : NEXT
PRINT S; " "; I
S = 0
FOR I = 5 TO -5 STEP -3 : S = S + I : NEXT
PRINT S; " "; I
C = 0
FOR I = 5 TO 1 : INC C : NEXT
FOR I = 1 TO 5 STEP -1 : INC C : NEXT
PRINT C; " "; I
C = 0
FOR I = 0 TO 0 : INC C : NEXT
PRINT C; " "; I
C = 0
FOR I = 32760 TO 32766 : INC C : NEXT
PRINT C; " "; I
C = 0
FOR I = 32760 TO 32767 : INC C : NEXT
PRINT C; " "; I
C = 0
FOR I = -32760 TO -32767 STEP -1 : INC C : NEXT
PRINT C; " "; I
C = 0
FOR I = -32760 TO -32768 STEP -1 : INC C : NEXT
PRINT C; " "; I
C = 0
FOR I = 1 TO 100 : IF I = 10 THEN I = 95
  INC C
NEXT
PRINT C; " "; I
C = 0
FOR I = 1 TO 20 : Bump() : INC C : NEXT
PRINT C; " "; I
C = 0
FOR I = -100 TO 32700 STEP 1000 : INC C : NEXT
PRINT C; " "; I
FUNC Bump()
  INC I
ENDFUNC
