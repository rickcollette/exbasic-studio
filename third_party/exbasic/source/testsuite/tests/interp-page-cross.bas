' Regression test for page-crossing bugs in immediate-operand tokens.
' The companion interp-page-cross.asm builds a chain of token handlers with
' each operand at $xxFF.  Every handler must carry into cptr+1 to reach
' the next page; otherwise execution wraps to the start of the current
' page and loops.
' The floating point variable makes the program initialize floating
' point, needed by the TOK_FLOAT test.
DIM A
F% = 0
A = 0
A = USR(@PAGE_TEST)
