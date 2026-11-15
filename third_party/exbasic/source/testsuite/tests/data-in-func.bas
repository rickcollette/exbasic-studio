' Test for DATA inside FUNC
? "Start"
X()
' Define a DATA inside the FUNC
FUNC X()
  data y() = 1,2
  ? y(0)
ENDFUNC
' A FUNC that calls another FUNC twice
FUNC Z()
  A()
  A()
ENDFUNC
FUNC A()
  ? y(1)
ENDFUNC
Z()
