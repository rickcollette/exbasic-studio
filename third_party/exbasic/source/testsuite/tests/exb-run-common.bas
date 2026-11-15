' RUN and COMMON: pass variables to the next program.
' The programs loaded with RUN are in the "run" folder.
COMMON N, X%, NAME$, EMPTY$, B
N = 1234
X% = 3.25
NAME$ = "HELLO FROM FIRST"
B = -7
? "FIRST: chaining"
RUN "D:MISSING.XEX"
E = ERR() : ? "missing file, ERR()="; E
RUN "D:RUNCMN2.XEX"
? "SHOULD NOT GET HERE"
