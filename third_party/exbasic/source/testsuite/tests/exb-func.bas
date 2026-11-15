' Calls before definition, multiple results, locals, strings
Q, R = DivMod(17, 5)
? Q; " "; R
? Greet$("BOB"); "!"
? Square(7) + Square(2)
Show("X=", 42)
X = 100
? Twice(X); " "; X
A$ = "Z"
N, M$ = Pair(3)
? N; " "; M$
_, R = DivMod(22, 7)
? R
I = 0
DO WHILE I < 3
  ? "W"; I;
  INC I
LOOP
?
DO
  ? "U"; I;
  DEC I
LOOP UNTIL I = 0
?
I = 5
DO UNTIL I = 0
  DEC I
  IF I = 2 THEN EXIT
LOOP
? "I="; I
REPEAT : I = I + 1 : UNTIL I > 4
? I

FUNC DivMod(A, B) Q, R
  Q = A / B
  R = A MOD B
ENDFUNC

FUNC Greet$(Name$)
  LOCAL T$
  T$ = "HI " : T$ =+ Name$
  RETURN T$
ENDFUNC

FUNC Square(X)
  RETURN X * X
ENDFUNC

FUNC Show(T$, V)
  ? T$; V
ENDFUNC

FUNC Twice(X)
  X = X * 2
  RETURN X
ENDFUNC

FUNC Pair(K) N, S$
  N = K * 10
  S$ = STR$(K) : S$ =+ "!"
ENDFUNC
