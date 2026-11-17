package debugger

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Memory reads emulated memory in bounded blocks.
type Memory interface {
	ReadMemory(address, length int) ([]byte, error)
}

// ---- 6502 disassembly --------------------------------------------------------

type addressing int

const (
	implied addressing = iota
	accumulator
	immediate
	zeroPage
	zeroPageX
	zeroPageY
	absolute
	absoluteX
	absoluteY
	indirect
	indirectX
	indirectY
	relative
)

var operandBytes = map[addressing]int{
	implied: 0, accumulator: 0, immediate: 1, zeroPage: 1, zeroPageX: 1, zeroPageY: 1,
	absolute: 2, absoluteX: 2, absoluteY: 2, indirect: 2, indirectX: 1, indirectY: 1, relative: 1,
}

type opcode struct {
	name string
	mode addressing
}

// opcodes holds the documented NMOS 6502 instruction set; anything else is
// shown as a data byte.
var opcodes = map[byte]opcode{
	0x00: {"BRK", implied}, 0x01: {"ORA", indirectX}, 0x05: {"ORA", zeroPage}, 0x06: {"ASL", zeroPage}, 0x08: {"PHP", implied},
	0x09: {"ORA", immediate}, 0x0A: {"ASL", accumulator}, 0x0D: {"ORA", absolute}, 0x0E: {"ASL", absolute},
	0x10: {"BPL", relative}, 0x11: {"ORA", indirectY}, 0x15: {"ORA", zeroPageX}, 0x16: {"ASL", zeroPageX}, 0x18: {"CLC", implied},
	0x19: {"ORA", absoluteY}, 0x1D: {"ORA", absoluteX}, 0x1E: {"ASL", absoluteX},
	0x20: {"JSR", absolute}, 0x21: {"AND", indirectX}, 0x24: {"BIT", zeroPage}, 0x25: {"AND", zeroPage}, 0x26: {"ROL", zeroPage},
	0x28: {"PLP", implied}, 0x29: {"AND", immediate}, 0x2A: {"ROL", accumulator}, 0x2C: {"BIT", absolute}, 0x2D: {"AND", absolute}, 0x2E: {"ROL", absolute},
	0x30: {"BMI", relative}, 0x31: {"AND", indirectY}, 0x35: {"AND", zeroPageX}, 0x36: {"ROL", zeroPageX}, 0x38: {"SEC", implied},
	0x39: {"AND", absoluteY}, 0x3D: {"AND", absoluteX}, 0x3E: {"ROL", absoluteX},
	0x40: {"RTI", implied}, 0x41: {"EOR", indirectX}, 0x45: {"EOR", zeroPage}, 0x46: {"LSR", zeroPage}, 0x48: {"PHA", implied},
	0x49: {"EOR", immediate}, 0x4A: {"LSR", accumulator}, 0x4C: {"JMP", absolute}, 0x4D: {"EOR", absolute}, 0x4E: {"LSR", absolute},
	0x50: {"BVC", relative}, 0x51: {"EOR", indirectY}, 0x55: {"EOR", zeroPageX}, 0x56: {"LSR", zeroPageX}, 0x58: {"CLI", implied},
	0x59: {"EOR", absoluteY}, 0x5D: {"EOR", absoluteX}, 0x5E: {"LSR", absoluteX},
	0x60: {"RTS", implied}, 0x61: {"ADC", indirectX}, 0x65: {"ADC", zeroPage}, 0x66: {"ROR", zeroPage}, 0x68: {"PLA", implied},
	0x69: {"ADC", immediate}, 0x6A: {"ROR", accumulator}, 0x6C: {"JMP", indirect}, 0x6D: {"ADC", absolute}, 0x6E: {"ROR", absolute},
	0x70: {"BVS", relative}, 0x71: {"ADC", indirectY}, 0x75: {"ADC", zeroPageX}, 0x76: {"ROR", zeroPageX}, 0x78: {"SEI", implied},
	0x79: {"ADC", absoluteY}, 0x7D: {"ADC", absoluteX}, 0x7E: {"ROR", absoluteX},
	0x81: {"STA", indirectX}, 0x84: {"STY", zeroPage}, 0x85: {"STA", zeroPage}, 0x86: {"STX", zeroPage}, 0x88: {"DEY", implied},
	0x8A: {"TXA", implied}, 0x8C: {"STY", absolute}, 0x8D: {"STA", absolute}, 0x8E: {"STX", absolute},
	0x90: {"BCC", relative}, 0x91: {"STA", indirectY}, 0x94: {"STY", zeroPageX}, 0x95: {"STA", zeroPageX}, 0x96: {"STX", zeroPageY},
	0x98: {"TYA", implied}, 0x99: {"STA", absoluteY}, 0x9A: {"TXS", implied}, 0x9D: {"STA", absoluteX},
	0xA0: {"LDY", immediate}, 0xA1: {"LDA", indirectX}, 0xA2: {"LDX", immediate}, 0xA4: {"LDY", zeroPage}, 0xA5: {"LDA", zeroPage},
	0xA6: {"LDX", zeroPage}, 0xA8: {"TAY", implied}, 0xA9: {"LDA", immediate}, 0xAA: {"TAX", implied}, 0xAC: {"LDY", absolute},
	0xAD: {"LDA", absolute}, 0xAE: {"LDX", absolute},
	0xB0: {"BCS", relative}, 0xB1: {"LDA", indirectY}, 0xB4: {"LDY", zeroPageX}, 0xB5: {"LDA", zeroPageX}, 0xB6: {"LDX", zeroPageY},
	0xB8: {"CLV", implied}, 0xB9: {"LDA", absoluteY}, 0xBA: {"TSX", implied}, 0xBC: {"LDY", absoluteX}, 0xBD: {"LDA", absoluteX}, 0xBE: {"LDX", absoluteY},
	0xC0: {"CPY", immediate}, 0xC1: {"CMP", indirectX}, 0xC4: {"CPY", zeroPage}, 0xC5: {"CMP", zeroPage}, 0xC6: {"DEC", zeroPage},
	0xC8: {"INY", implied}, 0xC9: {"CMP", immediate}, 0xCA: {"DEX", implied}, 0xCC: {"CPY", absolute}, 0xCD: {"CMP", absolute}, 0xCE: {"DEC", absolute},
	0xD0: {"BNE", relative}, 0xD1: {"CMP", indirectY}, 0xD5: {"CMP", zeroPageX}, 0xD6: {"DEC", zeroPageX}, 0xD8: {"CLD", implied},
	0xD9: {"CMP", absoluteY}, 0xDD: {"CMP", absoluteX}, 0xDE: {"DEC", absoluteX},
	0xE0: {"CPX", immediate}, 0xE1: {"SBC", indirectX}, 0xE4: {"CPX", zeroPage}, 0xE5: {"SBC", zeroPage}, 0xE6: {"INC", zeroPage},
	0xE8: {"INX", implied}, 0xE9: {"SBC", immediate}, 0xEA: {"NOP", implied}, 0xEC: {"CPX", absolute}, 0xED: {"SBC", absolute}, 0xEE: {"INC", absolute},
	0xF0: {"BEQ", relative}, 0xF1: {"SBC", indirectY}, 0xF5: {"SBC", zeroPageX}, 0xF6: {"INC", zeroPageX}, 0xF8: {"SED", implied},
	0xF9: {"SBC", absoluteY}, 0xFD: {"SBC", absoluteX}, 0xFE: {"INC", absoluteX},
}

// Instruction is one disassembled 6502 instruction.
type Instruction struct {
	Address int    `json:"address"`
	Bytes   string `json:"bytes"`
	Text    string `json:"text"`
}

// Disassemble decodes up to count instructions from data, which starts at
// address base. Decoding stops when an instruction would run past the data.
func Disassemble(data []byte, base, count int) []Instruction {
	out := make([]Instruction, 0, count)
	for offset := 0; offset < len(data) && len(out) < count; {
		address := (base + offset) & 0xFFFF
		op, known := opcodes[data[offset]]
		size := 1
		if known {
			size += operandBytes[op.mode]
		}
		if offset+size > len(data) {
			break
		}
		raw := data[offset : offset+size]
		hexBytes := make([]string, len(raw))
		for i, b := range raw {
			hexBytes[i] = fmt.Sprintf("%02X", b)
		}
		text := fmt.Sprintf(".BYTE $%02X", raw[0])
		if known {
			operand := 0
			if size == 2 {
				operand = int(raw[1])
			} else if size == 3 {
				operand = int(raw[1]) | int(raw[2])<<8
			}
			switch op.mode {
			case implied:
				text = op.name
			case accumulator:
				text = op.name + " A"
			case immediate:
				text = fmt.Sprintf("%s #$%02X", op.name, operand)
			case zeroPage:
				text = fmt.Sprintf("%s $%02X", op.name, operand)
			case zeroPageX:
				text = fmt.Sprintf("%s $%02X,X", op.name, operand)
			case zeroPageY:
				text = fmt.Sprintf("%s $%02X,Y", op.name, operand)
			case absolute:
				text = fmt.Sprintf("%s $%04X", op.name, operand)
			case absoluteX:
				text = fmt.Sprintf("%s $%04X,X", op.name, operand)
			case absoluteY:
				text = fmt.Sprintf("%s $%04X,Y", op.name, operand)
			case indirect:
				text = fmt.Sprintf("%s ($%04X)", op.name, operand)
			case indirectX:
				text = fmt.Sprintf("%s ($%02X,X)", op.name, operand)
			case indirectY:
				text = fmt.Sprintf("%s ($%02X),Y", op.name, operand)
			case relative:
				target := (address + 2 + int(int8(raw[1]))) & 0xFFFF
				text = fmt.Sprintf("%s $%04X", op.name, target)
			}
		}
		out = append(out, Instruction{Address: address, Bytes: strings.Join(hexBytes, " "), Text: text})
		offset += size
	}
	return out
}

// ---- Return stack ------------------------------------------------------------

// StackFrame is a subroutine call found on the 6502 stack.
type StackFrame struct {
	// Return is where execution continues after the routine returns.
	Return int `json:"return"`
	// Call is the address of the JSR instruction and Target its operand.
	Call   int `json:"call"`
	Target int `json:"target"`
	// Path and Line are set when the call site belongs to a source line.
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
}

// Frames walks the 6502 stack from the stack pointer upwards and reports
// the entries that are return addresses of JSR instructions. The stack also
// holds pushed data, so each candidate is confirmed by reading the JSR
// opcode that would have pushed it.
func Frames(memory Memory, sp int, sourceMap *SourceMap) ([]StackFrame, error) {
	sp &= 0xFF
	if sp == 0xFF {
		return []StackFrame{}, nil
	}
	stack, err := memory.ReadMemory(0x100+sp+1, 0xFF-sp)
	if err != nil {
		return nil, err
	}
	frames := []StackFrame{}
	for i := 0; i+1 < len(stack) && len(frames) < 32; i++ {
		pushed := int(stack[i]) | int(stack[i+1])<<8 // address of the last byte of the JSR
		call := (pushed - 2) & 0xFFFF
		if call > 0xFFFD {
			continue
		}
		code, readErr := memory.ReadMemory(call, 3)
		if readErr != nil || code[0] != 0x20 {
			continue
		}
		frame := StackFrame{Return: (pushed + 1) & 0xFFFF, Call: call, Target: int(code[1]) | int(code[2])<<8}
		if sourceMap != nil {
			if path, line, ok := sourceMap.Locate(call); ok {
				frame.Path, frame.Line = path, line
			}
		}
		frames = append(frames, frame)
		i++ // the high byte is part of this entry
	}
	return frames, nil
}

// ---- Watches -----------------------------------------------------------------

// WatchValue is the read-only result of evaluating a watch expression.
type WatchValue struct {
	Expression string `json:"expression"`
	Address    int    `json:"address"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	Error      string `json:"error,omitempty"`
}

// arrayPreviewBytes bounds the byte view shown for arrays and strings.
const arrayPreviewBytes = 32

// ParseWatch resolves "symbol", an address ("1536", "$0600", "0x600") or
// "symbol ± constant" to an address and the type to read there.
func ParseWatch(expression string, sourceMap *SourceMap) (address int, typ string, err error) {
	text := strings.TrimSpace(expression)
	if text == "" {
		return 0, "", errors.New("empty expression")
	}
	if len(text) > 80 {
		return 0, "", errors.New("expression is too long")
	}
	base, offset := text, 0
	// A sign after the first character separates the symbol and constant.
	if cut := strings.LastIndexAny(text, "+-"); cut > 0 {
		value, numErr := parseNumber(strings.TrimSpace(text[cut+1:]))
		if numErr != nil {
			return 0, "", fmt.Errorf("expected a constant after %q", text[cut:cut+1])
		}
		if text[cut] == '-' {
			value = -value
		}
		base, offset = strings.TrimSpace(text[:cut]), value
	}
	if value, numErr := parseNumber(base); numErr == nil {
		address, typ = value+offset, "byte"
	} else {
		if sourceMap == nil {
			return 0, "", errors.New("symbols are available only while debugging a build with debug information")
		}
		symbol, ok := sourceMap.Symbol(base)
		if !ok {
			return 0, "", fmt.Errorf("unknown symbol %q", base)
		}
		address, typ = symbol.Address+offset, symbol.Type
		if offset != 0 {
			typ = "byte" // an offset addresses raw memory
		}
	}
	if address < 0 || address > 0xFFFF {
		return 0, "", fmt.Errorf("address %d is outside memory", address)
	}
	return address, typ, nil
}

func parseNumber(text string) (int, error) {
	switch {
	case text == "":
		return 0, errors.New("empty")
	case strings.HasPrefix(text, "$"):
		value, err := strconv.ParseInt(text[1:], 16, 32)
		return int(value), err
	case strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X"):
		value, err := strconv.ParseInt(text[2:], 16, 32)
		return int(value), err
	default:
		value, err := strconv.ParseInt(text, 10, 32)
		return int(value), err
	}
}

func read(memory Memory, address, length int) ([]byte, error) {
	if address+length > 0x10000 {
		length = 0x10000 - address
	}
	if length <= 0 {
		return nil, errors.New("address is outside memory")
	}
	return memory.ReadMemory(address, length)
}

func hexView(data []byte) string {
	parts := make([]string, len(data))
	for i, b := range data {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, " ")
}

// DecodeFloat converts the 6-byte Atari BCD floating point format: an
// excess-64 base-100 exponent with the sign in bit 7, then five BCD bytes.
func DecodeFloat(data []byte) (float64, error) {
	if len(data) < 6 {
		return 0, errors.New("a floating point value needs 6 bytes")
	}
	if data[0] == 0 {
		return 0, nil
	}
	mantissa := 0.0
	for _, b := range data[1:6] {
		high, low := b>>4, b&0x0F
		if high > 9 || low > 9 {
			return 0, errors.New("not a valid BCD number")
		}
		mantissa = mantissa*100 + float64(high*10+low)
	}
	exponent := int(data[0]&0x7F) - 64
	value := mantissa * math.Pow(100, float64(exponent-4))
	if data[0]&0x80 != 0 {
		value = -value
	}
	return value, nil
}

// atascii renders Atari text as printable ASCII for display.
func atascii(data []byte) string {
	var out strings.Builder
	for _, b := range data {
		c := b & 0x7F // bit 7 is inverse video
		if c >= 0x20 && c < 0x7F {
			out.WriteByte(c)
		} else {
			out.WriteByte('.')
		}
	}
	return out.String()
}

// Evaluate reads one watch expression. Scalars are decoded with the type
// from the compiler metadata; arrays and strings are shown as bounded views.
func Evaluate(expression string, sourceMap *SourceMap, memory Memory) WatchValue {
	result := WatchValue{Expression: expression}
	address, typ, err := ParseWatch(expression, sourceMap)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Address, result.Type = address, typ
	fail := func(err error) WatchValue { result.Error = err.Error(); return result }
	switch typ {
	case "word":
		data, readErr := read(memory, address, 2)
		if readErr != nil || len(data) < 2 {
			return fail(errors.New("cannot read the value"))
		}
		value := int(data[0]) | int(data[1])<<8
		result.Value = fmt.Sprintf("%d ($%04X)", int(int16(value)), value)
	case "float":
		data, readErr := read(memory, address, 6)
		if readErr != nil {
			return fail(readErr)
		}
		value, decodeErr := DecodeFloat(data)
		if decodeErr != nil {
			result.Value = hexView(data)
			result.Error = decodeErr.Error()
			return result
		}
		result.Value = strconv.FormatFloat(value, 'G', 10, 64)
	case "string":
		// The variable holds the address of a length-prefixed string.
		pointer, readErr := read(memory, address, 2)
		if readErr != nil || len(pointer) < 2 {
			return fail(errors.New("cannot read the string address"))
		}
		at := int(pointer[0]) | int(pointer[1])<<8
		if at == 0 {
			result.Value = `"" (empty)`
			return result
		}
		header, readErr := read(memory, at, 1)
		if readErr != nil {
			return fail(readErr)
		}
		length := int(header[0])
		if length == 0 {
			result.Value = fmt.Sprintf(`"" at $%04X`, at)
			return result
		}
		text, readErr := read(memory, (at+1)&0xFFFF, length)
		if readErr != nil {
			return fail(readErr)
		}
		result.Value = fmt.Sprintf("%q (%d characters at $%04X)", atascii(text), length, at)
	case "word-array", "byte-array", "string-array", "float-array":
		pointer, readErr := read(memory, address, 2)
		if readErr != nil || len(pointer) < 2 {
			return fail(errors.New("cannot read the array address"))
		}
		at := int(pointer[0]) | int(pointer[1])<<8
		if at == 0 {
			result.Value = "not dimensioned"
			return result
		}
		data, readErr := read(memory, at, arrayPreviewBytes)
		if readErr != nil {
			return fail(readErr)
		}
		result.Value = fmt.Sprintf("$%04X: %s …", at, hexView(data))
	case "function", "data":
		result.Value = fmt.Sprintf("$%04X", address)
		if typ == "data" {
			if data, readErr := read(memory, address, arrayPreviewBytes); readErr == nil {
				result.Value = fmt.Sprintf("$%04X: %s …", address, hexView(data))
			}
		}
	default:
		data, readErr := read(memory, address, 2)
		if readErr != nil {
			return fail(readErr)
		}
		result.Type = "byte"
		if len(data) >= 2 {
			result.Value = fmt.Sprintf("%d ($%02X), word %d ($%04X)", data[0], data[0], int(data[0])|int(data[1])<<8, int(data[0])|int(data[1])<<8)
		} else {
			result.Value = fmt.Sprintf("%d ($%02X)", data[0], data[0])
		}
	}
	return result
}
