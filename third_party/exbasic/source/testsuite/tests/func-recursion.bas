FUNC Fact(N)
  IF N <= 1 THEN RETURN 1
  RETURN N * Fact(N-1)
ENDFUNC

? Fact(5)
