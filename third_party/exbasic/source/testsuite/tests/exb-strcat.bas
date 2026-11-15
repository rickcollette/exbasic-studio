' String concatenation with +
A$ = "HELLO"
B$ = A$ + ", " + "WORLD" + "!"
? B$
? A$ + "-" + STR$(42) + "-" + CHR$(65)
? STR$(1) + STR$(2)
IF A$ + "X" = "HELLOX" THEN ? "cmp ok"
IF "A" + "B" = "A" + "B" THEN ? "both sides ok"
? LEN(A$ + A$)
C$ = "<" + (A$ + A$)[3,4] + ">"
? C$
C$ = C$ + C$ : ? C$
N = LEN(A$) + 1 : ? N
? Wrap$("x") + Wrap$(A$ + "1")
? Two$("a" + "b", "c" + "d")
' Append in place
D$ = "" : FOR I = 1 TO 3 : D$ = D$ + STR$(I) + "," : NEXT : ? D$
E$ = "" : FOR I = 1 TO 60 : E$ = E$ + "ABCDE" : NEXT : ? LEN(E$)
' Cases where append in place is not possible
Y$ = "AB" : Y$ = Y$ + Y$ : ? Y$
Y$ = Y$ + "-" + Y$ : ? Y$
Y$ = Y$ + Up$("q") : ? Y$
DIM S$(2) : S$(1) = "Z" : S$(1) = S$(1) + "Y" : ? S$(1)

FUNC Wrap$(S$) R$
  RETURN "[" + S$ + "]"
ENDFUNC

' Changes Y$ while Y$ + Up$() is evaluated
FUNC Up$(S$) R$
  Y$ = "changed"
  RETURN S$ + "!"
ENDFUNC

FUNC Two$(A$, B$) R$
  RETURN A$ + "/" + B$
ENDFUNC
