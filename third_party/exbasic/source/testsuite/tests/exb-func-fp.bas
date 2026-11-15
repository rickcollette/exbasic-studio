' Recursion: locals used after the recursive call must be preserved
? Fib(15)
? Fact(7)
FOR I = 0 TO 6 : ? IsEven(I); : NEXT : ?
Q, R = SumDiv(10)
? Q; " "; R
? Area%(2.5, 4)
? Hyp%(3, 4)
X% = 1.5 + Half%(5) * 2
? X%
? Depth(40)

FUNC Fib(N)
  IF N < 2 THEN RETURN N
  LOCAL A
  A = Fib(N - 1)
  RETURN A + Fib(N - 2)
ENDFUNC

FUNC Fact(N)
  IF N <= 1 THEN RETURN 1
  RETURN Fact(N - 1) * N
ENDFUNC

FUNC IsEven(N)
  IF N = 0 THEN RETURN 1
  RETURN IsOdd(N - 1)
ENDFUNC

FUNC IsOdd(N)
  IF N = 0 THEN RETURN 0
  RETURN IsEven(N - 1)
ENDFUNC

' Multiple results through recursion: sum and count of N..1
FUNC SumDiv(N) S, C
  IF N = 0 THEN RETURN
  S, C = SumDiv(N - 1)
  S = S + N
  C = C + 1
ENDFUNC

FUNC Area%(W%, H%)
  RETURN W% * H%
ENDFUNC

FUNC Hyp%(A%, B%)
  RETURN SQR(A% * A% + B% * B%)
ENDFUNC

FUNC Half%(V%)
  RETURN V% / 2
ENDFUNC

FUNC Depth(N)
  IF N = 0 THEN RETURN 0
  RETURN Depth(N - 1) + 1
ENDFUNC
