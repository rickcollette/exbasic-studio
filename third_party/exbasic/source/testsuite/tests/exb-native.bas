' Loops compiled to machine code: the results must be the same as with the
' bytecode interpreter (compile with -no-native to compare).
DIM W(20), B(20) BYTE

' FOR with constant, negative and variable STEP, nested
S = 0
FOR I = 0 TO 20 : W(I) = I * I - 50 : B(I) = I * 7 : NEXT
FOR I = 20 TO 0 STEP -3 : S = S + W(I) : NEXT
? "step -3: "; S; " "; I
ST = -2
FOR I = 10 TO -10 STEP ST : S = S - I : NEXT
? "var step: "; S; " "; I
FOR I = 1 TO 0 : ? "never" : NEXT
? "empty: "; I
N = 0
FOR I = 1 TO 5
  FOR J = I TO 5
    N = N + I * J
  NEXT
NEXT
? "nested: "; N

' Comparisons, signed, with constants, variables and expressions
C = 0
FOR I = -40 TO 40 STEP 7
  IF I < 0 THEN C = C + 1
  IF I > 10 THEN C = C + 10
  IF I <= -33 THEN C = C + 100
  IF I >= 33 THEN C = C + 1000
  IF I = 2 THEN C = C + 10000
  IF I <> 2 AND I > 30 THEN C = C - 1
  IF NOT (I < W(3)) THEN C = C + 3
  IF B(I AND 15) THEN C = C + 1
NEXT
? "compare: "; C
X = 32767 : Y = -32768
? "limits: "; X > Y; " "; Y < X; " "; X + 1 < X; " "; Y - 1 > Y

' Multiplication, division and modulo
M = 0
FOR I = -9 TO 9
  M = M + I * 3 + I * 10 + I * 100 + I * 256 - I * I
  IF I <> 0 THEN M = M + 1000 / I + 1000 MOD I
  M = M + I / 4 + I MOD 4
NEXT
? "math: "; M

' WHILE, REPEAT, DO with EXIT, inside a FOR
T = 0
FOR I = 1 TO 4
  K = 0
  WHILE K < I : K = K + 1 : T = T + K : WEND
  REPEAT : K = K - 2 : UNTIL K < 0
  DO
    INC K
    IF K > 3 THEN EXIT
    T = T + 100
  LOOP
NEXT
? "loops: "; T; " "; K

' PEEK, POKE, DPEEK, DPOKE, INC and DEC on arrays
FOR I = 0 TO 9
  POKE ADR(B) + I, PEEK(ADR(B) + I) + 1
  DPOKE ADR(W) + 2 * I, DPEEK(ADR(W) + 2 * I) * 2
  INC W(I) : DEC W(I + 10)
NEXT
P = 0
FOR I = 0 TO 9 : P = P + W(I) + B(I) : NEXT
? "memory: "; P

' FUNC calls and PRINT inside a loop return to the interpreter
FOR I = 1 TO 3
  ? Sq(I); ",";
NEXT
?
' Strings in a loop
A$ = ""
FOR I = 1 TO 5 : A$ = A$ + STR$(I) : NEXT
? A$

' Small FUNCs called from a loop, and recursion, are machine code too
C = 0
FOR I = 1 TO 200 : C = C + Clamp(I * 3 - 300, -50, 50) : NEXT
? "clamp: "; C; " fib: "; Fib(15)

FUNC Clamp(V, L, H)
  IF V < L THEN RETURN L
  IF V > H THEN RETURN H
  RETURN V
ENDFUNC

FUNC Fib(N)
  IF N < 2 THEN RETURN N
  RETURN Fib(N - 1) + Fib(N - 2)
ENDFUNC

FUNC Sq(V)
  R = 0
  FOR Q = 1 TO V : R = R + V : NEXT
  RETURN R
ENDFUNC
