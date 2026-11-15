' Deep recursion stops with an error, not a crash
FUNC Deep(N)
  RETURN Deep(N + 1) + 1
ENDFUNC
PRINT "Start"
PRINT Deep(0)
PRINT "Never"
