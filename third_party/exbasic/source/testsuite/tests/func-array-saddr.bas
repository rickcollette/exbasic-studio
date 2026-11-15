DIM B(3) BYTE
DIM W(3)

FUNC Plus(X)
  RETURN X + 10
ENDFUNC

FUNC Twice(X)
  RETURN X + X
ENDFUNC

B(1) = Plus(5)
W(2) = Plus(7)
B(2) = Twice(Plus(1))

? B(1)
? B(2)
? W(2)
