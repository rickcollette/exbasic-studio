' USR calls inside the arguments of another USR call
' The routine returns the bit inverse of its argument:
'  PLA / EOR $FF / TAX / PLA / EOR $FF / RTS
DATA Invert() BYTE = $68,$49,$FF,$AA,$68,$49,$FF,$60
' And this one adds its two arguments
'  PLA / STA $CB / PLA / STA $CC / PLA / TAX / PLA / CLC / ADC $CC / PHA / TXA / ADC $CB / TAX / PLA / RTS
DATA AddTwo() BYTE = $68,$85,$CB,$68,$85,$CC,$68,$AA,$68,$18,$65,$CC,$48,$8A,$65,$CB,$AA,$68,$60
PRINT USR(ADR(Invert), USR(ADR(Invert), 1234))
PRINT USR(ADR(AddTwo), USR(ADR(Invert), -1), USR(ADR(AddTwo), 20, 30))
PRINT USR(ADR(AddTwo), 1000, USR(ADR(Invert), USR(ADR(Invert), 7)))
