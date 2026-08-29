package runtime

import (
	"fmt"
	"math"
	"math/big"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type preparedCode struct {
	code         *bytecode.Code
	instructions []bytecode.Instruction
	constants    []Value
	names        []string
	stackSize    int
}

// prepareCode copies runtime-facing tables, materializes constants, and rejects
// every invalid or unsupported instruction before execution can begin.
func prepareCode(code *bytecode.Code) (*preparedCode, error) {
	if code == nil {
		return nil, &BytecodeError{Instruction: -1, Message: "nil code object"}
	}
	prepared := &preparedCode{
		code:         code,
		instructions: code.Instructions(),
		names:        code.Names(),
		stackSize:    code.StackSize(),
	}
	if prepared.stackSize < 0 {
		return nil, prepared.failure(-1, "negative operand stack size %d", prepared.stackSize)
	}

	constants := code.Constants()
	prepared.constants = make([]Value, len(constants))
	for index, constant := range constants {
		value, err := materializeConstant(constant)
		if err != nil {
			return nil, prepared.failure(-1, "constant %d: %s", index, err)
		}
		prepared.constants[index] = value
	}
	if err := prepared.validateInstructions(); err != nil {
		return nil, err
	}
	return prepared, nil
}

// materializeConstant converts every compiler literal descriptor into its
// runtime object while rejecting malformed strings and integer descriptors.
func materializeConstant(constant bytecode.Constant) (Value, error) {
	switch constant.Kind {
	case bytecode.NoneConstant:
		return None, nil
	case bytecode.BoolConstant:
		if constant.Bool {
			return trueSingleton, nil
		}
		return falseSingleton, nil
	case bytecode.EllipsisConstant:
		return ellipsisSingleton, nil
	case bytecode.IntegerConstant:
		value, ok := new(big.Int).SetString(constant.Text, 10)
		if !ok {
			return nil, fmt.Errorf("invalid integer %q", constant.Text)
		}
		return &intValue{value: *value}, nil
	case bytecode.FloatConstant:
		return &floatValue{value: math.Float64frombits(constant.Bits)}, nil
	case bytecode.ImaginaryConstant:
		return &complexValue{imaginary: math.Float64frombits(constant.Bits)}, nil
	case bytecode.StringConstant:
		if !validStringEncoding(constant.Text) {
			return nil, fmt.Errorf("invalid string constant encoding")
		}
		return &stringValue{value: constant.Text}, nil
	case bytecode.BytesConstant:
		return &bytesValue{value: constant.Text}, nil
	default:
		return nil, fmt.Errorf("unsupported constant %s", constant)
	}
}

type stackEdge struct {
	target int
	depth  int
}

// validateInstructions validates every operand before propagating stack depths
// through reachable fallthrough and jump edges with a worklist.
func (code *preparedCode) validateInstructions() error {
	for index, instruction := range code.instructions {
		if err := code.validateOperand(index, instruction); err != nil {
			return err
		}
	}
	if len(code.instructions) == 0 {
		return code.failure(0, "code has no reachable RETURN_VALUE")
	}

	depths := make([]int, len(code.instructions))
	for index := range depths {
		depths[index] = -1
	}
	depths[0] = 0
	worklist := []int{0}
	reachableReturn := false
	for len(worklist) != 0 {
		index := worklist[0]
		worklist = worklist[1:]
		edges, returns, err := code.instructionEdges(
			index,
			depths[index],
			code.instructions[index],
		)
		if err != nil {
			return err
		}
		if returns {
			reachableReturn = true
		}
		for _, edge := range edges {
			if edge.target == len(code.instructions) {
				return code.failure(index, "code falls through without RETURN_VALUE")
			}
			if edge.depth > code.stackSize {
				return code.failure(
					index,
					"operand stack exceeds declared size %d",
					code.stackSize,
				)
			}
			if depths[edge.target] < 0 {
				depths[edge.target] = edge.depth
				worklist = append(worklist, edge.target)
			} else if depths[edge.target] != edge.depth {
				return code.failure(
					index,
					"stack depth mismatch at instruction %d: %d and %d",
					edge.target,
					depths[edge.target],
					edge.depth,
				)
			}
		}
	}
	if !reachableReturn {
		return code.failure(len(code.instructions), "code has no reachable RETURN_VALUE")
	}
	return nil
}

// instructionEdges applies one instruction's stack contract and returns its
// reachable successor depths without executing the operation.
func (code *preparedCode) instructionEdges(
	index, depth int,
	instruction bytecode.Instruction,
) ([]stackEdge, bool, error) {
	require := func(values int) error {
		if depth < values {
			return code.failure(index, "operand stack underflow")
		}
		return nil
	}
	next := index + 1
	target := int(instruction.Operand)
	switch instruction.Opcode {
	case bytecode.Copy:
		if uint64(instruction.Operand) > uint64(depth) {
			return nil, false, code.failure(index, "operand stack underflow")
		}
		return []stackEdge{{target: next, depth: depth + 1}}, false, nil
	case bytecode.Swap:
		if uint64(instruction.Operand) > uint64(depth) {
			return nil, false, code.failure(index, "operand stack underflow")
		}
		return []stackEdge{{target: next, depth: depth}}, false, nil
	case bytecode.ReturnValue:
		if err := require(1); err != nil {
			return nil, false, err
		}
		if depth != 1 {
			return nil, false, code.failure(
				index,
				"RETURN_VALUE leaves %d values on the operand stack",
				depth-1,
			)
		}
		return nil, true, nil
	case bytecode.Jump:
		return []stackEdge{{target: target, depth: depth}}, false, nil
	case bytecode.ForIter:
		if err := require(1); err != nil {
			return nil, false, err
		}
		return []stackEdge{
			{target: next, depth: depth + 1},
			{target: target, depth: depth - 1},
		}, false, nil
	case bytecode.PopJumpIfFalse, bytecode.PopJumpIfTrue:
		if err := require(1); err != nil {
			return nil, false, err
		}
		return []stackEdge{
			{target: next, depth: depth - 1},
			{target: target, depth: depth - 1},
		}, false, nil
	case bytecode.JumpIfFalseOrPop, bytecode.JumpIfTrueOrPop:
		if err := require(1); err != nil {
			return nil, false, err
		}
		return []stackEdge{
			{target: next, depth: depth - 1},
			{target: target, depth: depth},
		}, false, nil
	default:
		pops, pushes := instructionStackUse(instruction)
		if err := require(pops); err != nil {
			return nil, false, err
		}
		return []stackEdge{{target: next, depth: depth + pushes - pops}}, false, nil
	}
}

// validateOperand checks table bounds and limits opcode variants to the
// behavior implemented by the current runtime slice.
func (code *preparedCode) validateOperand(index int, instruction bytecode.Instruction) error {
	switch instruction.Opcode {
	case bytecode.Nop, bytecode.PopTop, bytecode.ReturnValue, bytecode.GetIter,
		bytecode.BinarySubscript, bytecode.StoreSubscript, bytecode.DeleteSubscript,
		bytecode.ListAppend, bytecode.ListExtend, bytecode.ListToTuple:
		return nil
	case bytecode.Copy:
		if instruction.Operand < 1 {
			return code.failure(index, "COPY depth must be at least 1")
		}
		return nil
	case bytecode.Swap:
		if instruction.Operand < 2 {
			return code.failure(index, "SWAP depth must be at least 2")
		}
		return nil
	case bytecode.Jump,
		bytecode.ForIter,
		bytecode.PopJumpIfFalse,
		bytecode.PopJumpIfTrue,
		bytecode.JumpIfFalseOrPop,
		bytecode.JumpIfTrueOrPop:
		if uint64(instruction.Operand) >= uint64(len(code.instructions)) {
			return code.failure(index, "jump target %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.LoadConst:
		if uint64(instruction.Operand) >= uint64(len(code.constants)) {
			return code.failure(index, "constant index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.LoadName, bytecode.StoreName:
		if uint64(instruction.Operand) >= uint64(len(code.names)) {
			return code.failure(index, "name index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.BuildTuple, bytecode.BuildList:
		if uint64(instruction.Operand) > uint64(code.stackSize) {
			return code.failure(
				index,
				"sequence element count %d exceeds stack size",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.BuildMap:
		if 2*uint64(instruction.Operand) > uint64(code.stackSize) {
			return code.failure(
				index,
				"map item count %d exceeds stack size",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.UnpackSequence:
		if uint64(instruction.Operand) > uint64(code.stackSize) {
			return code.failure(
				index,
				"unpack count %d exceeds stack size",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.UnpackEx:
		before, after := bytecode.UnpackExCounts(instruction.Operand)
		count := uint64(before) + uint64(after) + 1
		if count > uint64(code.stackSize) {
			return code.failure(
				index,
				"unpack count %d exceeds stack size",
				count,
			)
		}
		return nil
	case bytecode.BuildSlice:
		if instruction.Operand != 2 && instruction.Operand != 3 {
			return code.failure(
				index,
				"unsupported BUILD_SLICE operand %d",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.UnaryOp:
		if instruction.Operand > bytecode.UnaryNot {
			return code.failure(
				index,
				"unsupported UNARY_OP operand %d",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.BinaryOp:
		switch instruction.Operand {
		case bytecode.BinaryAdd,
			bytecode.BinarySubtract,
			bytecode.BinaryMultiply,
			bytecode.BinaryFloorDivide,
			bytecode.BinaryModulo,
			bytecode.BinaryLeftShift,
			bytecode.BinaryRightShift,
			bytecode.BinaryOr,
			bytecode.BinaryXor,
			bytecode.BinaryAnd:
			return nil
		default:
			return code.failure(
				index,
				"unsupported BINARY_OP operand %d",
				instruction.Operand,
			)
		}
	case bytecode.CompareOp:
		switch instruction.Operand {
		case bytecode.CompareEqual,
			bytecode.CompareNotEqual,
			bytecode.CompareLess,
			bytecode.CompareLessEqual,
			bytecode.CompareGreater,
			bytecode.CompareGreaterEqual,
			bytecode.CompareIs,
			bytecode.CompareIsNot:
			return nil
		default:
			return code.failure(
				index,
				"unsupported COMPARE_OP operand %d",
				instruction.Operand,
			)
		}
	default:
		return code.failure(index, "unsupported opcode %s", instruction.Opcode)
	}
}

// instructionStackUse returns the ordinary fallthrough consumption and
// production for opcodes whose effects do not split across control-flow edges.
func instructionStackUse(instruction bytecode.Instruction) (pops, pushes int) {
	switch instruction.Opcode {
	case bytecode.LoadConst, bytecode.LoadName:
		return 0, 1
	case bytecode.StoreName, bytecode.PopTop, bytecode.ReturnValue:
		return 1, 0
	case bytecode.DeleteSubscript:
		return 2, 0
	case bytecode.StoreSubscript:
		return 3, 0
	case bytecode.BinaryOp, bytecode.CompareOp, bytecode.BinarySubscript,
		bytecode.ListAppend, bytecode.ListExtend:
		return 2, 1
	case bytecode.UnaryOp, bytecode.GetIter, bytecode.ListToTuple:
		return 1, 1
	case bytecode.BuildTuple, bytecode.BuildList, bytecode.BuildSlice:
		return int(instruction.Operand), 1
	case bytecode.BuildMap:
		return 2 * int(instruction.Operand), 1
	case bytecode.UnpackSequence:
		return 1, int(instruction.Operand)
	case bytecode.UnpackEx:
		before, after := bytecode.UnpackExCounts(instruction.Operand)
		return 1, int(before + after + 1)
	default:
		return 0, 0
	}
}

func (code *preparedCode) failure(index int, format string, arguments ...any) error {
	var span lexer.Span
	if position, ok := code.code.Position(index); ok {
		span = position
	}
	return &BytecodeError{
		Filename:    code.code.Filename(),
		Instruction: index,
		Span:        span,
		Message:     fmt.Sprintf(format, arguments...),
	}
}
