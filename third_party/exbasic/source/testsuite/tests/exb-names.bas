' Names that start with keywords, and the same name with different types
COLORS = 3
PRINTER = 4
TOTAL = COLORS + PRINTER
PRINT TOTAL
A = 1
A$ = "one"
A% = 1.5
PRINT A; " "; A$; " "; A%
IF A = 1 THEN PRINT "then" ELSE PRINT "else"
IF A = 2 THEN PRINT "then" ELSE PRINT "else"
IF A = 1 THEN PRINT "a"; : PRINT "b"
IF A = 2 THEN PRINT "c"; : PRINT "d"
PRINT "e"
' Mixed integer and floating point
PRINT 1 + 0.5; " "; 3 / 2; " "; (7 MOD 3) + 1.5
IF 1.5 > 1 THEN PRINT "fp compare"
PRINT 1E3; " "; 2.5E-1
