' Second program: same COMMON, receives all the values.
COMMON N, X%, NAME$, EMPTY$, B
? "SECOND: N="; N; " X%="; X%; " B="; B
? "NAME$="; NAME$; " LEN(EMPTY$)="; LEN(EMPTY$)
DIM A(10)
FOR I = 0 TO 10 : A(I) = I * I : NEXT
? "array ok "; A(10)
N = N + 1
RUN "D:RUNCMN3.XEX"
