' Function calls inside a DIM expression use both the variable stack and
' the function-label stack.  They must share the same stack correctly.
FUNC Three()
  RETURN 3
ENDFUNC

DIM A(Three()) BYTE
A(2) = 7
? A(2)

FUNC Show(X)
  ? X
ENDFUNC

Show(Three())
