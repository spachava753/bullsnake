// Package bytecode defines Bullsnake's private instruction and code-object
// representation.
package bytecode

import "fmt"

// Version identifies the current private Bullsnake bytecode format.
const Version = 1

// CONVERT_VALUE operands select the Python conversion applied before format.
const (
	ConversionString uint32 = iota + 1
	ConversionRepr
	ConversionASCII
)

// Opcode identifies one virtual-machine instruction.
type Opcode uint8

const (
	Nop Opcode = iota
	LoadConst
	LoadName
	StoreName
	Copy
	PopTop
	ReturnValue
	ConvertValue
	FormatSimple
	FormatWithSpec
	BuildString
)

var opcodeNames = [...]string{
	"NOP",
	"LOAD_CONST",
	"LOAD_NAME",
	"STORE_NAME",
	"COPY",
	"POP_TOP",
	"RETURN_VALUE",
	"CONVERT_VALUE",
	"FORMAT_SIMPLE",
	"FORMAT_WITH_SPEC",
	"BUILD_STRING",
}

// String returns the disassembly spelling of an opcode.
func (opcode Opcode) String() string {
	if int(opcode) < len(opcodeNames) {
		return opcodeNames[opcode]
	}
	return fmt.Sprintf("Opcode(%d)", opcode)
}

// HasOperand reports whether the instruction encodes an operand.
func (opcode Opcode) HasOperand() bool {
	switch opcode {
	case LoadConst, LoadName, StoreName, Copy, ConvertValue, BuildString:
		return true
	default:
		return false
	}
}

// StackEffect returns the instruction's change to operand-stack depth.
func (opcode Opcode) StackEffect(operand uint32) int {
	switch opcode {
	case LoadConst, LoadName, Copy:
		return 1
	case StoreName, PopTop, ReturnValue, FormatWithSpec:
		return -1
	case BuildString:
		return 1 - int(operand)
	default:
		return 0
	}
}

// Instruction is one decoded bytecode operation.
type Instruction struct {
	Opcode  Opcode
	Operand uint32
}

// String returns the stable disassembly form of an instruction.
func (instruction Instruction) String() string {
	if instruction.Opcode.HasOperand() {
		return fmt.Sprintf("%s %d", instruction.Opcode, instruction.Operand)
	}
	return instruction.Opcode.String()
}
