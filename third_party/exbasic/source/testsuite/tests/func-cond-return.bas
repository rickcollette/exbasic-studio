' A function only needs at least one RETURN; full path coverage is not checked.
FUNC Maybe(X)
  IF X THEN RETURN 1
ENDFUNC

? Maybe(1)
