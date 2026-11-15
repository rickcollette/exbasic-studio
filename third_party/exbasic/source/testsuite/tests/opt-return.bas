' Check peephole conditional-return rules.
DIM A
A = 1

FUNC P()
  IF A THEN RETURN
  ? "p"
ENDFUNC

FUNC Q()
  IF NOT A THEN RETURN
  ? "q"
ENDFUNC

P()
Q()

A = 0
P()
Q()
? "done"
