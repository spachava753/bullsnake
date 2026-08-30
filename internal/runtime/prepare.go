package runtime

import (
	"fmt"
	"math"
	"math/big"
	"slices"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type preparedCode struct {
	code              *bytecode.Code
	instructions      []bytecode.Instruction
	constants         []Value
	names             []string
	locals            []string
	cells             []string
	freeVars          []string
	cellLocals        []int
	childCodes        []*bytecode.Code
	children          []*preparedCode
	exceptionHandlers []bytecode.ExceptionHandler
	stackSize         int
}

// prepareCode copies runtime-facing tables, materializes constants, and rejects
// every invalid or unsupported instruction before execution can begin.
func prepareCode(code *bytecode.Code) (*preparedCode, error) {
	if code == nil {
		return nil, &BytecodeError{Instruction: -1, Message: "nil code object"}
	}
	prepared := &preparedCode{
		code:              code,
		instructions:      code.Instructions(),
		names:             code.Names(),
		locals:            code.Locals(),
		cells:             code.Cells(),
		freeVars:          code.FreeVars(),
		childCodes:        code.Children(),
		exceptionHandlers: code.ExceptionHandlers(),
		stackSize:         code.StackSize(),
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
	if err := prepared.validateExceptionHandlers(); err != nil {
		return nil, err
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
	if flags&bytecode.Generator != 0 &&
		flags&(bytecode.Optimized|bytecode.NewLocals) != bytecode.Optimized|bytecode.NewLocals {
		return code.failure(-1, "generator code requires optimized new locals")
	}
	supportedFlags := bytecode.Optimized | bytecode.NewLocals | bytecode.Nested |
		bytecode.VarArgs | bytecode.VarKeywords | bytecode.Generator
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

// validateExceptionHandlers rejects malformed protected ranges before stack
// analysis or execution can observe them.
func (code *preparedCode) validateExceptionHandlers() error {
	previousEnd := 0
	for index, handler := range code.exceptionHandlers {
		start := int(handler.Start)
		end := int(handler.End)
		target := int(handler.Target)
		if start >= end {
			return code.failure(-1, "exception handler %d has empty or reversed range", index)
		}
		if start < previousEnd {
			return code.failure(-1, "exception handler %d overlaps or is out of order", index)
		}
		if end > len(code.instructions) {
			return code.failure(-1, "exception handler %d range end %d out of range", index, end)
		}
		if target < 0 || target >= len(code.instructions) {
			return code.failure(-1, "exception handler %d target %d out of range", index, target)
		}
		if handler.StackDepth < 0 || handler.StackDepth >= code.stackSize {
			return code.failure(
				-1,
				"exception handler %d stack depth %d cannot receive an exception with stack size %d",
				index,
				handler.StackDepth,
				code.stackSize,
			)
		}
		previousEnd = end
	}
	return nil
}

func (code *preparedCode) exceptionHandler(instruction int) (bytecode.ExceptionHandler, bool) {
	for _, handler := range code.exceptionHandlers {
		if instruction < int(handler.Start) {
			break
		}
		if instruction < int(handler.End) {
			return handler, true
		}
	}
	return bytecode.ExceptionHandler{}, false
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

type exceptionScopeState struct {
	start int
	end   int
}

// validateInstructions validates every operand before propagating stack depths
// and handled-exception scopes through reachable edges with a worklist.
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
	scopes := make([][]exceptionScopeState, len(code.instructions))
	for index := range depths {
		depths[index] = -1
	}
	depths[0] = 0
	worklist := []int{0}
	reachableReturn := false
	for len(worklist) != 0 {
		index := worklist[0]
		worklist = worklist[1:]
		instruction := code.instructions[index]
		nextScopes, err := code.instructionExceptionScopes(index, scopes[index], instruction)
		if err != nil {
			return err
		}
		edges, returns, err := code.instructionEdges(index, depths[index], instruction)
		if err != nil {
			return err
		}
		if handler, ok := code.exceptionHandler(index); ok {
			if depths[index] < handler.StackDepth {
				return code.failure(
					index,
					"exception handler stack depth %d exceeds instruction depth %d",
					handler.StackDepth,
					depths[index],
				)
			}
			edges = append(edges, stackEdge{
				target: int(handler.Target),
				depth:  handler.StackDepth + 1,
			})
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
			edgeScopes := pruneExceptionScopes(nextScopes, edge.target)
			if depths[edge.target] < 0 {
				depths[edge.target] = edge.depth
				scopes[edge.target] = slices.Clone(edgeScopes)
				worklist = append(worklist, edge.target)
				continue
			}
			if depths[edge.target] != edge.depth {
				return code.failure(
					index,
					"stack depth mismatch at instruction %d: %d and %d",
					edge.target,
					depths[edge.target],
					edge.depth,
				)
			}
			if !slices.Equal(scopes[edge.target], edgeScopes) {
				return code.failure(
					index,
					"exception scope mismatch at instruction %d",
					edge.target,
				)
			}
		}
	}
	if !reachableReturn {
		return code.failure(len(code.instructions), "code has no reachable RETURN_VALUE")
	}
	return nil
}

// instructionExceptionScopes tracks lexical handled-exception entries across
// control flow and rejects operations that require a scope when none is active.
func (code *preparedCode) instructionExceptionScopes(
	index int,
	scopes []exceptionScopeState,
	instruction bytecode.Instruction,
) ([]exceptionScopeState, error) {
	switch instruction.Opcode {
	case bytecode.EnterExcept:
		next := slices.Clone(scopes)
		return append(next, exceptionScopeState{
			start: index + 1,
			end:   int(instruction.Operand),
		}), nil
	case bytecode.LoadHandledExceptionType:
		if len(scopes) == 0 {
			return nil, code.failure(
				index,
				"LOAD_HANDLED_EXCEPTION_TYPE has no active exception",
			)
		}
		return scopes, nil
	case bytecode.LeaveExcept:
		if len(scopes) == 0 {
			return nil, code.failure(index, "LEAVE_EXCEPT has no active handler")
		}
		return scopes[:len(scopes)-1], nil
	default:
		return scopes, nil
	}
}

func pruneExceptionScopes(scopes []exceptionScopeState, instruction int) []exceptionScopeState {
	for len(scopes) != 0 {
		last := scopes[len(scopes)-1]
		if instruction >= last.start && instruction < last.end {
			break
		}
		scopes = scopes[:len(scopes)-1]
	}
	return scopes
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
	case bytecode.Reraise:
		if err := require(1); err != nil {
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
	case bytecode.Send:
		if err := require(2); err != nil {
			return nil, false, err
		}
		return []stackEdge{
			{target: next, depth: depth},
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
	case bytecode.YieldValue:
		if code.code.Flags()&bytecode.Generator == 0 {
			return code.failure(index, "YIELD_VALUE requires generator code")
		}
		return nil
	case bytecode.Nop, bytecode.PopTop, bytecode.ReturnValue, bytecode.GetIter,
		bytecode.BinarySubscript, bytecode.StoreSubscript, bytecode.DeleteSubscript,
		bytecode.FormatSimple, bytecode.FormatWithSpec, bytecode.BuildString,
		bytecode.ListAppend, bytecode.ListExtend, bytecode.ListToTuple,
		bytecode.SetAdd, bytecode.SetUpdate, bytecode.MapSet, bytecode.MapUpdate,
		bytecode.MapMerge, bytecode.LoadNotImplementedError,
		bytecode.LoadAssertionError, bytecode.LoadBuildClass, bytecode.ImportStar,
		bytecode.CheckExceptionMatch, bytecode.CheckExceptionGroupMatch,
		bytecode.PrepareReraiseStar, bytecode.Reraise, bytecode.LeaveExcept,
		bytecode.LoadHandledExceptionType, bytecode.MatchSequence, bytecode.GetLen,
		bytecode.MatchMapping, bytecode.MatchMappingKey, bytecode.CopyMapping,
		bytecode.CheckMappingKey:
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
	case bytecode.Send:
		if code.code.Flags()&bytecode.Generator == 0 {
			return code.failure(index, "SEND requires generator code")
		}
		if uint64(instruction.Operand) >= uint64(len(code.instructions)) {
			return code.failure(index, "jump target %d out of range", instruction.Operand)
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
	case bytecode.EnterExcept:
		if uint64(instruction.Operand) <= uint64(index) ||
			uint64(instruction.Operand) > uint64(len(code.instructions)) {
			return code.failure(
				index,
				"exception scope end %d must follow its entry and stay within code",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.ConvertValue:
		switch instruction.Operand {
		case bytecode.ConversionString, bytecode.ConversionRepr, bytecode.ConversionASCII:
			return nil
		default:
			return code.failure(
				index,
				"unsupported CONVERT_VALUE operand %d",
				instruction.Operand,
			)
		}
	case bytecode.LoadConst:
		if uint64(instruction.Operand) >= uint64(len(code.constants)) {
			return code.failure(index, "constant index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.LoadName, bytecode.StoreName, bytecode.DeleteName,
		bytecode.LoadGlobal, bytecode.StoreGlobal, bytecode.DeleteGlobal,
		bytecode.LoadAttr, bytecode.LoadSpecial, bytecode.StoreAttr, bytecode.DeleteAttr,
		bytecode.ImportName, bytecode.ImportFrom:
		if uint64(instruction.Operand) >= uint64(len(code.names)) {
			return code.failure(index, "name index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.LoadFast, bytecode.StoreFast, bytecode.DeleteFast:
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
		if instruction.Operand > 2 {
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
	case bytecode.MatchClass:
		if uint64(instruction.Operand) > uint64(code.stackSize) {
			return code.failure(
				index,
				"class positional pattern count %d exceeds stack size",
				instruction.Operand,
			)
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
	case bytecode.BinaryOp, bytecode.InplaceOp:
		switch instruction.Operand {
		case bytecode.BinaryAdd,
			bytecode.BinarySubtract,
			bytecode.BinaryMultiply,
			bytecode.BinaryDivide,
			bytecode.BinaryFloorDivide,
			bytecode.BinaryModulo,
			bytecode.BinaryPower,
			bytecode.BinaryLeftShift,
			bytecode.BinaryRightShift,
			bytecode.BinaryOr,
			bytecode.BinaryXor,
			bytecode.BinaryAnd:
			return nil
		default:
			return code.failure(
				index,
				"unsupported %s operand %d",
				instruction.Opcode,
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
		bytecode.LoadNotImplementedError, bytecode.LoadAssertionError,
		bytecode.LoadBuildClass, bytecode.MakeFunction, bytecode.ImportFrom,
		bytecode.LoadHandledExceptionType:
		return 0, 1
	case bytecode.StoreName, bytecode.StoreFast, bytecode.StoreGlobal,
		bytecode.StoreDeref, bytecode.PopTop, bytecode.ReturnValue,
		bytecode.ImportStar:
		return 1, 0
	case bytecode.YieldValue:
		return 1, 1
	case bytecode.DeleteName, bytecode.DeleteFast, bytecode.DeleteGlobal,
		bytecode.DeleteDeref:
		return 0, 0
	case bytecode.StoreAttr:
		return 2, 0
	case bytecode.DeleteAttr:
		return 1, 0
	case bytecode.RaiseVarargs:
		return int(instruction.Operand), 0
	case bytecode.Reraise, bytecode.EnterExcept:
		return 1, 0
	case bytecode.DeleteSubscript, bytecode.CheckMappingKey:
		return 2, 0
	case bytecode.StoreSubscript:
		return 3, 0
	case bytecode.BinaryOp, bytecode.InplaceOp, bytecode.CompareOp,
		bytecode.FormatWithSpec, bytecode.BinarySubscript, bytecode.ImportName,
		bytecode.ListAppend, bytecode.ListExtend, bytecode.SetAdd, bytecode.SetUpdate,
		bytecode.MapUpdate, bytecode.MapMerge:
		return 2, 1
	case bytecode.CheckExceptionMatch, bytecode.CheckExceptionGroupMatch:
		return 2, 2
	case bytecode.PrepareReraiseStar:
		return 2, 1
	case bytecode.MapSet:
		return 3, 1
	case bytecode.SetFunctionAttribute:
		return 2, 1
	case bytecode.Call:
		return int(instruction.Operand) + 1, 1
	case bytecode.CallEx:
		return 2 + int(instruction.Operand), 1
	case bytecode.UnaryOp, bytecode.ConvertValue, bytecode.FormatSimple,
		bytecode.GetIter, bytecode.ListToTuple, bytecode.LoadAttr,
		bytecode.LoadSpecial:
		return 1, 1
	case bytecode.MatchSequence, bytecode.GetLen, bytecode.MatchMapping:
		return 1, 2
	case bytecode.CopyMapping:
		return 1, 1
	case bytecode.MatchMappingKey:
		return 2, 2
	case bytecode.MatchClass:
		return 3, 2
	case bytecode.BuildString, bytecode.BuildTuple, bytecode.BuildList,
		bytecode.BuildSet, bytecode.BuildSlice:
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
