' String functions
Text$ = "Hello, World"
PRINT LEFT$(Text$, 5); "|"; RIGHT$(Text$, 5); "|"; MID$(Text$, 8, 3); "|"
PRINT LEFT$(Text$, 0); "|"; LEFT$(Text$, 99); "|"; RIGHT$(Text$, -1); "|"
PRINT MID$(Text$, 8); "|"; MID$(Text$, 20, 2); "|"; MID$(Text$, 12, 5); "|"
PRINT UCASE$(Text$); "|"; LCASE$(Text$); "|"
PRINT LTRIM$("   abc  "); "|"; RTRIM$("   abc  "); "|"; LTRIM$("    "); "|"
PRINT INSTR(Text$, "o"); " "; INSTR(Text$, "o", 6); " "; INSTR(Text$, "xyz"); " "; INSTR(Text$, "")
PRINT INSTR("abc", "abcd"); " "; INSTR(Text$, "World"); " "; INSTR(Text$, "d", 12)
MID$(Text$, 8, 5) = "Atari!!"
PRINT Text$; "|"; LEN(Text$)
MID$(Text$, 1) = "J"
PRINT Text$
' Text$ is not changed by the functions
Name$ = "abc"
Up$ = UCASE$(Name$)
PRINT Name$; " "; Up$
PRINT ASC(""); " "; ASC("A")
