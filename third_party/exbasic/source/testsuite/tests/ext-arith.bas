' Multiplication, division by constants and DIV/MOD pairs
DATA V() = 0,1,2,7,100,255,256,1000,32767,-32768,-1,-7,-100,-256,-1000
S=0
FOR I=0 TO 14
  FOR J=0 TO 14
    A=V(I):B=V(J)
    S=S*3+A*B
    IF B
      S=S*5+A/B+A MOD B
    ENDIF
  NEXT
NEXT
? S
FOR I=0 TO 14
  A=V(I)
  ? A/2;" ";A/4;" ";A/8;" ";A/256;" ";A/16384;" ";A*3;" ";A*100;" ";100*A
NEXT
' Pairs of DIV and MOD, also with changing operands
S=0
FOR A=-500 TO 500 STEP 17
  B=A MOD 23+30
  S=S*7+A/B+A MOD B
  S=S*3+A MOD B+A/B
  S=S+(A+1)/B+(A+1) MOD B
NEXT
? S
