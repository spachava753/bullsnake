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

// CALL_EX operands report whether a keyword map follows the positional tuple.
const (
	CallExNoKeywords uint32 = iota
	CallExWithKeywords
)

// UNARY_OP operands identify Python unary operations.
const (
	UnaryPositive uint32 = iota
	UnaryNegative
	UnaryInvert
	UnaryNot
)

// BINARY_OP operands identify Python binary and future in-place operations.
const (
	BinaryAdd uint32 = iota
	BinarySubtract
	BinaryMultiply
	BinaryMatrixMultiply
	BinaryDivide
	BinaryFloorDivide
	BinaryModulo
	BinaryPower
	BinaryLeftShift
	BinaryRightShift
	BinaryOr
	BinaryXor
	BinaryAnd
)

// COMPARE_OP operands identify Python comparison operations.
const (
	CompareEqual uint32 = iota
	CompareNotEqual
	CompareLess
	CompareLessEqual
	CompareGreater
	CompareGreaterEqual
	CompareIn
	CompareNotIn
	CompareIs
	CompareIsNot
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
	BuildTuple
	BuildList
	BuildSet
	BuildMap
	ListAppend
	ListExtend
	ListToTuple
	SetAdd
	SetUpdate
	MapSet
	MapUpdate
	UnaryOp
	BinaryOp
	Swap
	CompareOp
	Jump
	PopJumpIfFalse
	JumpIfFalseOrPop
	JumpIfTrueOrPop
	LoadAttr
	BinarySubscript
	BuildSlice
	MapMerge
	Call
	CallEx
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
	"BUILD_TUPLE",
	"BUILD_LIST",
	"BUILD_SET",
	"BUILD_MAP",
	"LIST_APPEND",
	"LIST_EXTEND",
	"LIST_TO_TUPLE",
	"SET_ADD",
	"SET_UPDATE",
	"MAP_SET",
	"MAP_UPDATE",
	"UNARY_OP",
	"BINARY_OP",
	"SWAP",
	"COMPARE_OP",
	"JUMP",
	"POP_JUMP_IF_FALSE",
	"JUMP_IF_FALSE_OR_POP",
	"JUMP_IF_TRUE_OR_POP",
	"LOAD_ATTR",
	"BINARY_SUBSCR",
	"BUILD_SLICE",
	"MAP_MERGE",
	"CALL",
	"CALL_EX",
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
	case LoadConst, LoadName, StoreName, Copy, ConvertValue, BuildString,
		BuildTuple, BuildList, BuildSet, BuildMap, UnaryOp, BinaryOp, Swap,
		CompareOp, Jump, PopJumpIfFalse, JumpIfFalseOrPop, JumpIfTrueOrPop,
		LoadAttr, BuildSlice, Call, CallEx:
		return true
	default:
		return false
	}
}

// StackEffect returns the instruction's fallthrough operand-stack change.
// Jump edges with different effects are tracked by the compiler's labels.
func (opcode Opcode) StackEffect(operand uint32) int {
	switch opcode {
	case LoadConst, LoadName, Copy:
		return 1
	case StoreName, PopTop, ReturnValue, FormatWithSpec, BinaryOp, CompareOp,
		PopJumpIfFalse, JumpIfFalseOrPop, JumpIfTrueOrPop, BinarySubscript,
		ListAppend, ListExtend, SetAdd, SetUpdate, MapUpdate, MapMerge:
		return -1
	case MapSet:
		return -2
	case Call:
		return -int(operand)
	case CallEx:
		return -1 - int(operand)
	case BuildString, BuildTuple, BuildList, BuildSet, BuildSlice:
		return 1 - int(operand)
	case BuildMap:
		return 1 - 2*int(operand)
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
