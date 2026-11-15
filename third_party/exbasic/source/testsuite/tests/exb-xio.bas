' XIO in the Atari BASIC order: command, #channel, aux1, aux2, "device"
' XIO 3 is OPEN and XIO 12 is CLOSE
XIO 3, #1, 8, 0, "D:XIOTEST"
PRINT #1; "written after XIO 3"
XIO 12, #1, 0, 0, "D:"
OPEN #2, 4, 0, "D:XIOTEST"
INPUT #2, Text$
CLOSE #2
PRINT Text$
' The abbreviation
X. 3, #1, 4, 0, "D:XIOTEST"
INPUT #1, Text$
X. 12, #1, 0, 0, "D:"
PRINT Text$; " "; ERR()
