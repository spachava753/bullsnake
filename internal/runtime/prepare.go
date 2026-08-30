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
	locals       []string
	cells        []string
	freeVars     []string
	cellLocals   []int
	childCodes   []*bytecode.Code
	children     []*preparedCode
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
		locals:       code.Locals(),
		cells:        code.Cells(),
		freeVars:     code.FreeVars(),
		childCodes:   code.Children(),
		stackSize:    code.StackSize(),
	}
	prepared.cellLocals = make([]int, len(prepared.cells))
	for cellIndex, name := range prepared.cells {
		prepared.cellLocals[cellIndex] = -1
		for localIndex, localName := range prepared.locals {
			if localName == name {
				prepared.cellLocals[cellIndex] = localIndex
				break
			}
		}
	}
	if prepared.stackSize < 0 {
		return nil, prepared.failure(-1, "negative operand stack size %d", prepared.stackSize)
	}
	if err := prepared.validateMetadata(); err != nil {
		return nil, err
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

// validateMetadata limits prepared function signatures and closure layout to
// the basic-call slice before any instruction in the code tree can execute.
func (code *preparedCode) validateMetadata() error {
	positionalOnly := code.code.PositionalOnlyCount()
	positional := code.code.PositionalCount()
	keywordOnly := code.code.KeywordOnlyCount()
	if positionalOnly < 0 || positional < 0 || keywordOnly < 0 {
		return code.failure(-1, "negative function parameter count")
	}
	if positionalOnly > positional {
		return code.failure(
			-1,
			"positional-only parameter count %d exceeds positional count %d",
			positionalOnly,
			positional,
		)
	}
	if positional > len(code.locals) {
		return code.failure(
			-1,
			"positional parameter count %d exceeds local table length %d",
			positional,
			len(code.locals),
		)
	}
	flags := code.code.Flags()
	keywordStart := positional
	if flags&bytecode.VarArgs != 0 {
		if positional >= len(code.locals) {
			return code.failure(
				-1,
				"variadic positional parameter index %d out of range",
				positional,
			)
		}
		keywordStart++
	}
	if keywordStart+keywordOnly > len(code.locals) {
		return code.failure(
			-1,
			"keyword-only parameter range exceeds local table length",
		)
	}
	if flags&bytecode.VarKeywords != 0 && keywordStart+keywordOnly >= len(code.locals) {
		return code.failure(
			-1,
			"variadic keyword parameter index %d out of range",
			keywordStart+keywordOnly,
		)
	}
	supportedFlags := bytecode.Optimized | bytecode.NewLocals | bytecode.Nested |
		bytecode.VarArgs | bytecode.VarKeywords
	if unsupported := flags &^ supportedFlags; unsupported != 0 {
		return code.failure(-1, "unsupported code flags %s", unsupported)
	}
	seenDeref := make(map[string]struct{}, len(code.cells)+len(code.freeVars))
	for _, names := range [][]string{code.cells, code.freeVars} {
		for _, name := range names {
			if _, exists := seenDeref[name]; exists {
				return code.failure(-1, "duplicate dereference name %q", name)
			}
			seenDeref[name] = struct{}{}
		}
	}
	return nil
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
	case bytecode.RaiseVarargs:
		if err := require(int(instruction.Operand)); err != nil {
			return nil, false, err
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
		bytecode.ListAppend, bytecode.ListExtend, bytecode.ListToTuple,
		bytecode.SetAdd, bytecode.SetUpdate, bytecode.MapSet, bytecode.MapUpdate,
		bytecode.MapMerge, bytecode.LoadNotImplementedError, bytecode.LoadBuildClass:
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
	case bytecode.LoadName, bytecode.StoreName, bytecode.LoadGlobal, bytecode.StoreGlobal:
		if uint64(instruction.Operand) >= uint64(len(code.names)) {
			return code.failure(index, "name index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.LoadFast, bytecode.StoreFast:
		if uint64(instruction.Operand) >= uint64(len(code.locals)) {
			return code.failure(index, "local index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.LoadDeref, bytecode.StoreDeref, bytecode.DeleteDeref,
		bytecode.LoadClosure:
		if uint64(instruction.Operand) >= uint64(len(code.cells)+len(code.freeVars)) {
			return code.failure(index, "deref index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.MakeFunction:
		if uint64(instruction.Operand) >= uint64(len(code.childCodes)) {
			return code.failure(
				index,
				"child code index %d out of range",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.Call:
		if uint64(instruction.Operand) >= uint64(code.stackSize) {
			return code.failure(index, "operand stack underflow")
		}
		return nil
	case bytecode.CallEx:
		if instruction.Operand > bytecode.CallExWithKeywords {
			return code.failure(
				index,
				"unsupported CALL_EX operand %d",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.RaiseVarargs:
		if instruction.Operand != 1 {
			return code.failure(
				index,
				"unsupported RAISE_VARARGS operand %d",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.SetFunctionAttribute:
		switch bytecode.FunctionAttribute(instruction.Operand) {
		case bytecode.FunctionDefaults, bytecode.FunctionKeywordDefaults,
			bytecode.FunctionClosure, bytecode.FunctionAnnotate:
			return nil
		default:
			return code.failure(
				index,
				"unsupported SET_FUNCTION_ATTRIBUTE operand %d",
				instruction.Operand,
			)
		}
	case bytecode.BuildTuple, bytecode.BuildList:
		if uint64(instruction.Operand) > uint64(code.stackSize) {
			return code.failure(
				index,
				"sequence element count %d exceeds stack size",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.BuildSet:
		if uint64(instruction.Operand) > uint64(code.stackSize) {
			return code.failure(
				index,
				"set element count %d exceeds stack size",
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
			bytecode.CompareIn,
			bytecode.CompareNotIn,
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
	case bytecode.LoadConst, bytecode.LoadName, bytecode.LoadFast,
		bytecode.LoadGlobal, bytecode.LoadDeref, bytecode.LoadClosure,
		bytecode.LoadNotImplementedError, bytecode.LoadBuildClass,
		bytecode.MakeFunction:
		return 0, 1
	case bytecode.StoreName, bytecode.StoreFast, bytecode.StoreGlobal,
		bytecode.StoreDeref, bytecode.PopTop, bytecode.ReturnValue:
		return 1, 0
	case bytecode.DeleteDeref:
		return 0, 0
	case bytecode.RaiseVarargs:
		return int(instruction.Operand), 0
	case bytecode.DeleteSubscript:
		return 2, 0
	case bytecode.StoreSubscript:
		return 3, 0
	case bytecode.BinaryOp, bytecode.CompareOp, bytecode.BinarySubscript,
		bytecode.ListAppend, bytecode.ListExtend, bytecode.SetAdd,
		bytecode.SetUpdate, bytecode.MapUpdate, bytecode.MapMerge:
		return 2, 1
	case bytecode.MapSet:
		return 3, 1
	case bytecode.SetFunctionAttribute:
		return 2, 1
	case bytecode.Call:
		return int(instruction.Operand) + 1, 1
	case bytecode.CallEx:
		return 2 + int(instruction.Operand), 1
	case bytecode.UnaryOp, bytecode.GetIter, bytecode.ListToTuple:
		return 1, 1
	case bytecode.BuildTuple, bytecode.BuildList, bytecode.BuildSet,
		bytecode.BuildSlice:
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
