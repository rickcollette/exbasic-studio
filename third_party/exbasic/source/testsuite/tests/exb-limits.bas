' FOR at the ends of the integer range, INT saturation and VAL
Count = 0
FOR I = 32765 TO 32767
  Count = Count + 1
NEXT I
PRINT Count
Count = 0
FOR I = -32766 TO -32768 STEP -1
  Count = Count + 1
NEXT I
PRINT Count
PRINT INT(1E10); " "; INT(-1E10)
PRINT VAL("12"); " "; ERR(); " "; VAL("xx"); " "; ERR()
PRINT "screen"; ERR()
