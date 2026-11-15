' NOTE and POINT: remember a file position and go back to it
OPEN #1, 8, 0, "D:NOTETEST"
PRINT #1; "first line"
NOTE #1, Sector, Byte
PRINT #1; "second line"
CLOSE #1
OPEN #1, 4, 0, "D:NOTETEST"
POINT #1, Sector, Byte
INPUT #1, Text$
CLOSE #1
PRINT Text$
PRINT ERR()
