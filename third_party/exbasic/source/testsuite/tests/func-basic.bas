FUNC Add(X)
  RETURN X + 1
ENDFUNC

FUNC Mul(X, Y)
  RETURN X * Y
ENDFUNC

FUNC Forty()
  RETURN 42
ENDFUNC

? Add(4)
? Add(5)
? Mul(3,5)
? Add(Mul(2,3))
? Add(1) + Add(2)
? Forty()
