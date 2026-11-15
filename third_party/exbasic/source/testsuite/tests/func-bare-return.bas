' Bare RETURN exits the current FUNC and unwinds nested FOR loops.
FUNC Early()
  ? "early"
  RETURN
  ? "after"
ENDFUNC

FUNC Nested()
  FOR I = 1 TO 2
    FOR J = 1 TO 2
      ? "loop"
      RETURN
    NEXT J
  NEXT I
  ? "after"
ENDFUNC

Early()
Nested()
? "done"
