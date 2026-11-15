' Test for using a DATA array in FUNC argument
DATA Arr() = 1234,5678
Test(Arr(0))
Test(Arr(1))
? "Ok"

FUNC Test(X)
  ? X
ENDFUNC

