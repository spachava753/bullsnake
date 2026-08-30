// Package bytecode defines Bullsnake's private instruction and code-object
// representation.
package bytecode

import "fmt"

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

const unpackExBeforeBits = 8

// PackUnpackEx encodes counts around one starred assignment target.
func PackUnpackEx(before, after uint32) (uint32, bool) {
	if before >= 1<<unpackExBeforeBits || after > ^uint32(0)>>unpackExBeforeBits {
		return 0, false
	}
	return before | after<<unpackExBeforeBits, true
}

// UnpackExCounts decodes counts around one starred assignment target.
func UnpackExCounts(operand uint32) (before, after uint32) {
	return operand & (1<<unpackExBeforeBits - 1), operand >> unpackExBeforeBits
}

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

// FunctionAttribute identifies one payload attached while creating a function.
type FunctionAttribute uint32

const (
	FunctionDefaults        FunctionAttribute = 0x01
	FunctionKeywordDefaults FunctionAttribute = 0x02
	FunctionClosure         FunctionAttribute = 0x08
	FunctionAnnotate        FunctionAttribute = 0x10
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
	GetIter
	ForIter
	StoreAttr
	StoreSubscript
	UnpackSequence
	UnpackEx
	InplaceOp
	DeleteName
	DeleteAttr
	DeleteSubscript
	PopJumpIfTrue
	LoadAssertionError
	LoadNotImplementedError
	RaiseVarargs
	LoadFast
	StoreFast
	DeleteFast
	LoadGlobal
	StoreGlobal
	DeleteGlobal
	MakeFunction
	SetFunctionAttribute
	LoadDeref
	StoreDeref
	DeleteDeref
	LoadClosure
	ImportName
	ImportFrom
	ImportStar
	LoadBuildClass
	LoadLocals
	LoadFromDictOrGlobals
	LoadFromDictOrDeref
	CheckExceptionMatch
	CheckExceptionGroupMatch
	PrepareReraiseStar
	Reraise
	EnterExcept
	LeaveExcept
	LoadSpecial
	LoadHandledExceptionType
	YieldValue
	Send
	MatchSequence
	GetLen
	MatchMapping
	MatchMappingKey
	CopyMapping
	CheckMappingKey
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
	"GET_ITER",
	"FOR_ITER",
	"STORE_ATTR",
	"STORE_SUBSCR",
	"UNPACK_SEQUENCE",
	"UNPACK_EX",
	"INPLACE_OP",
	"DELETE_NAME",
	"DELETE_ATTR",
	"DELETE_SUBSCR",
	"POP_JUMP_IF_TRUE",
	"LOAD_ASSERTION_ERROR",
	"LOAD_NOT_IMPLEMENTED_ERROR",
	"RAISE_VARARGS",
	"LOAD_FAST",
	"STORE_FAST",
	"DELETE_FAST",
	"LOAD_GLOBAL",
	"STORE_GLOBAL",
	"DELETE_GLOBAL",
	"MAKE_FUNCTION",
	"SET_FUNCTION_ATTRIBUTE",
	"LOAD_DEREF",
	"STORE_DEREF",
	"DELETE_DEREF",
	"LOAD_CLOSURE",
	"IMPORT_NAME",
	"IMPORT_FROM",
	"IMPORT_STAR",
	"LOAD_BUILD_CLASS",
	"LOAD_LOCALS",
	"LOAD_FROM_DICT_OR_GLOBALS",
	"LOAD_FROM_DICT_OR_DEREF",
	"CHECK_EXC_MATCH",
	"CHECK_EG_MATCH",
	"PREP_RERAISE_STAR",
	"RERAISE",
	"ENTER_EXCEPT",
	"LEAVE_EXCEPT",
	"LOAD_SPECIAL",
	"LOAD_HANDLED_EXCEPTION_TYPE",
	"YIELD_VALUE",
	"SEND",
	"MATCH_SEQUENCE",
	"GET_LEN",
	"MATCH_MAPPING",
	"MATCH_MAPPING_KEY",
	"COPY_MAPPING",
	"CHECK_MAPPING_KEY",
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
		CompareOp, Jump, PopJumpIfFalse, PopJumpIfTrue, JumpIfFalseOrPop,
		JumpIfTrueOrPop, Send, LoadAttr, BuildSlice, Call, CallEx, ForIter, StoreAttr,
		UnpackSequence, UnpackEx, InplaceOp, DeleteName, DeleteAttr,
		RaiseVarargs, LoadFast, StoreFast, DeleteFast, LoadGlobal, StoreGlobal,
		DeleteGlobal, MakeFunction, SetFunctionAttribute, LoadDeref, StoreDeref,
		DeleteDeref, LoadClosure, ImportName, ImportFrom, LoadFromDictOrGlobals,
		LoadFromDictOrDeref, EnterExcept, LoadSpecial:
		return true
	default:
		return false
	}
}

// StackEffect returns the instruction's fallthrough operand-stack change.
// Jump edges with different effects are tracked by the compiler's labels.
func (opcode Opcode) StackEffect(operand uint32) int {
	switch opcode {
	case LoadConst, LoadName, Copy, ForIter, LoadAssertionError,
		LoadNotImplementedError, LoadFast, LoadGlobal, MakeFunction, LoadDeref,
		LoadClosure, ImportFrom, LoadBuildClass, LoadLocals,
		LoadHandledExceptionType, MatchSequence, MatchMapping, GetLen:
		return 1
	case StoreName, StoreFast, StoreGlobal, PopTop, ReturnValue, FormatWithSpec,
		BinaryOp, InplaceOp, CompareOp, PopJumpIfFalse, PopJumpIfTrue,
		JumpIfFalseOrPop, JumpIfTrueOrPop, BinarySubscript, DeleteAttr,
		ListAppend, ListExtend, SetAdd, SetUpdate, MapUpdate, MapMerge,
		SetFunctionAttribute, StoreDeref, ImportName, ImportStar, PrepareReraiseStar,
		Reraise, EnterExcept:
		return -1
	case MapSet, StoreAttr, DeleteSubscript, CheckMappingKey:
		return -2
	case StoreSubscript:
		return -3
	case UnpackSequence:
		return int(operand) - 1
	case UnpackEx:
		before, after := UnpackExCounts(operand)
		return int(before + after)
	case RaiseVarargs:
		return -int(operand)
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
	if instruction.Opcode == UnpackEx {
		before, after := UnpackExCounts(instruction.Operand)
		return fmt.Sprintf("%s %d %d", instruction.Opcode, before, after)
	}
	if instruction.Opcode.HasOperand() {
		return fmt.Sprintf("%s %d", instruction.Opcode, instruction.Operand)
	}
	return instruction.Opcode.String()
}
