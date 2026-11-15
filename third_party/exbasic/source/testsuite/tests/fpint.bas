
a% = 0.1        : pconv()
a% = 0.5        : pconv()
a% = 123.499    : pconv()
a% = 123.500    : pconv()
a% = 32767.4999 : pconv()
a% = 32767.5000 : pconv()

FUNC pconv()
  i1 = int(a%)  : e1 = err()
  i2 = int(-a%) : e2 = err()
  ? a%, i1, e1, i2, e2
ENDFUNC
