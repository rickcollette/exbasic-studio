' Test for statement "CLR"
? "Start"

n = 0
f = fre()

' Dim inside a FUNC to call multiple times
FUNC dima()
  ? "---- DIM "; n; " ----"
  ? "Before: "; f-fre()
  dim a(n)
  ? "After: "; f-fre()
ENDFUNC

n=10 : dima()
n=10 : dima()

? "---- CLR ----"
clr

? f, n, adr(a)

f = fre()
n=100 : dima()

