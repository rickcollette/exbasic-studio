' AND and OR only evaluate the right side when needed
Calls = 0
FUNC Check(V)
  Calls = Calls + 1
  RETURN V
ENDFUNC
FUNC Show(A, B)
  ? A AND B; " "; A OR B; " "; NOT A AND B; " "; A OR B AND 0
ENDFUNC
IF 0 AND Check(1) THEN ? "wrong"
? "and false: "; Calls
IF 1 AND Check(1) THEN ? "and true: "; Calls
IF 1 OR Check(1) THEN ? "or true: "; Calls
IF 0 OR Check(0) THEN ? "wrong"
? "or false: "; Calls
' Avoid an error: the index is checked first
DIM Values(4)
I = 9
IF I <= 4 AND Values(I) = 0 THEN ? "wrong" ELSE ? "safe"
' Values as integers
FOR A = 0 TO 1
  FOR B = 0 TO 1
    Show(A, B)
  NEXT B
NEXT A
X = 3
R = X > 2 AND X < 5 OR X = 10
? R; " "; (X = 1 OR X = 3) AND X <> 0
' Loops
N = 0
WHILE N < 10 AND Check(N) < 5
  N = N + 1
WEND
? N
REPEAT
  N = N - 1
UNTIL N = 0 OR Check(N) = 3
? N
? 1.5 > 1 AND 2.5 < 3; " "; 1.5 > 2 OR 0.5 < 1
