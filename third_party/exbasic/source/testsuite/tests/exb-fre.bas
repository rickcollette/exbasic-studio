' FRE is never negative: printed, compared or converted to floating point
F = FRE(0)
IF FRE() > 0 THEN PRINT "compare ok"
IF FRE > 32000 AND FRE() >= 1000 THEN PRINT "big ok"
PRINT FRE(0) = FRE()
X% = FRE
IF X% = FRE() AND X% > 1000 THEN PRINT "float ok"
PRINT LEN(STR$(FRE)) = 5
' Differences work in integer math
DIM A(10)
PRINT F - FRE()
IF 0 < FRE() THEN PRINT "right side ok"
IF F - FRE() > 10 THEN PRINT "difference ok"
