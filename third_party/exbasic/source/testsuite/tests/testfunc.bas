' Test FUNC

' Calls a FUNC defined after use
for i=0 to 10
    printLoop()
next i

FUNC printLoop()
    if i = 5
      ? "Skipping 5!"
      exit
    endif
    ? "Loop: "; i
ENDFUNC

' Now, define FUNC first
FUNC before()
    ? "-- test --"
ENDFUNC

before()

param(123, i * i * i)

' FUNC with parameters
FUNC param(x, y)
   ? "X="; x
   ? "Y="; y
ENDFUNC

' FUNC with parameters
FUNC param3(x, y, z)
   ? "P="; x, y, z
ENDFUNC

param3(1, 2, 3)

