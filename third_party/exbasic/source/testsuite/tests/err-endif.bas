
A=1
IF A=1
 ? "OK"
' This should be parsed as a variable, not as ENDIF A=1:
ENDIFA=1

? A,ENDIFA
