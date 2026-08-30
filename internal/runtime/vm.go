package runtime

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type outcomeKind uint8

const (
	advance outcomeKind = iota
	called
	returned
	raised
)

type instructionOutcome struct {
	kind      outcomeKind
	value     Value
	exception *Exception
	frame     *frame
}

type raisedOutcome struct {
	exception   *Exception
	frame       *frame
	instruction int
}

// execute advances the active heap frame until it returns, raises a Python
// exception, or encounters a validated-bytecode invariant failure.
func execute(thread *threadState) (Value, *raisedOutcome, error) {
	for thread.current != nil {
		active := thread.current
		index := active.instruction
		if index < 0 || index >= len(active.code.instructions) {
			return nil, nil, active.failure(index, "instruction index out of range")
		}
		instruction := active.code.instructions[index]
		active.instruction++
		outcome, err := executeInstruction(active, index, instruction)
		if err != nil {
			return nil, nil, err
		}
		switch outcome.kind {
		case advance:
			continue
		case called:
			if outcome.frame == nil || outcome.frame.previous != active {
				return nil, nil, active.failure(index, "invalid call frame transition")
			}
			thread.current = outcome.frame
		case returned:
			thread.current = active.previous
			result := outcome.value
			if active.classBuild != nil {
				result = active.classBuild.finish(result)
			}
			if thread.current == nil {
				return result, nil, nil
			}
			if !thread.current.push(result) {
				return nil, nil, thread.current.failure(
					thread.current.instruction,
					"operand stack overflow while returning to caller",
				)
			}
		case raised:
			return nil, &raisedOutcome{
				exception:   outcome.exception,
				frame:       active,
				instruction: index,
			}, nil
		default:
			return nil, nil, active.failure(index, "unknown execution outcome")
		}
	}
	return nil, nil, &BytecodeError{Instruction: -1, Message: "execution has no frame"}
}

// executeInstruction applies one validated operation and reports whether the
// frame advances, returns a value, or raises a Python exception.
func executeInstruction(
	frame *frame,
	index int,
	instruction bytecode.Instruction,
) (instructionOutcome, error) {
	switch instruction.Opcode {
	case bytecode.Nop:
		return instructionOutcome{kind: advance}, nil
	case bytecode.LoadConst:
		value := frame.code.constants[instruction.Operand]
		return pushOutcome(frame, index, value)
	case bytecode.LoadNotImplementedError:
		return pushOutcome(frame, index, newException("NotImplementedError", ""))
	case bytecode.LoadBuildClass:
		return pushOutcome(frame, index, buildClassSingleton)
	case bytecode.LoadName:
		name := frame.code.names[instruction.Operand]
		value, ok := frame.lookupName(name)
		if !ok {
			return instructionOutcome{
				kind:      raised,
				exception: newException("NameError", fmt.Sprintf("name '%s' is not defined", name)),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadFast:
		localIndex := int(instruction.Operand)
		value := frame.fastLocals[localIndex]
		if value == nil {
			name := frame.code.locals[localIndex]
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"UnboundLocalError",
					"cannot access local variable '"+name+
						"' where it is not associated with a value",
				),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadDeref:
		derefIndex := int(instruction.Operand)
		value := frame.deref[derefIndex].value
		if value == nil {
			return instructionOutcome{
				kind:      raised,
				exception: unboundDerefException(frame.code, derefIndex),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadClosure:
		return pushOutcome(frame, index, frame.deref[instruction.Operand])
	case bytecode.LoadGlobal:
		name := frame.code.names[instruction.Operand]
		value, ok := frame.globals.get(name)
		if !ok {
			value, ok = frame.builtins.get(name)
		}
		if !ok {
			return instructionOutcome{
				kind:      raised,
				exception: newException("NameError", fmt.Sprintf("name '%s' is not defined", name)),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.StoreName:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.locals.values[frame.code.names[instruction.Operand]] = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.StoreFast:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.fastLocals[instruction.Operand] = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.StoreDeref:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.deref[instruction.Operand].value = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.DeleteDeref:
		derefIndex := int(instruction.Operand)
		cell := frame.deref[derefIndex]
		if cell.value == nil {
			return instructionOutcome{
				kind:      raised,
				exception: unboundDerefException(frame.code, derefIndex),
			}, nil
		}
		cell.value = nil
		return instructionOutcome{kind: advance}, nil
	case bytecode.StoreGlobal:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.globals.values[frame.code.names[instruction.Operand]] = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.Copy:
		depth := int(instruction.Operand)
		if depth < 1 || depth > len(frame.stack) {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return pushOutcome(frame, index, frame.stack[len(frame.stack)-depth])
	case bytecode.Swap:
		depth := int(instruction.Operand)
		if depth < 2 || depth > len(frame.stack) {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		top := len(frame.stack) - 1
		other := len(frame.stack) - depth
		frame.stack[top], frame.stack[other] = frame.stack[other], frame.stack[top]
		return instructionOutcome{kind: advance}, nil
	case bytecode.PopTop:
		if _, ok := frame.pop(); !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.Jump:
		frame.instruction = int(instruction.Operand)
		return instructionOutcome{kind: advance}, nil
	case bytecode.PopJumpIfFalse, bytecode.PopJumpIfTrue:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		takeJump := truthValue(value)
		if instruction.Opcode == bytecode.PopJumpIfFalse {
			takeJump = !takeJump
		}
		if takeJump {
			frame.instruction = int(instruction.Operand)
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.JumpIfFalseOrPop, bytecode.JumpIfTrueOrPop:
		if len(frame.stack) == 0 {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		value := frame.stack[len(frame.stack)-1]
		takeJump := truthValue(value)
		if instruction.Opcode == bytecode.JumpIfFalseOrPop {
			takeJump = !takeJump
		}
		if takeJump {
			frame.instruction = int(instruction.Operand)
		} else {
			frame.pop()
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.BuildTuple:
		return executeBuildSequence(frame, index, int(instruction.Operand), true)
	case bytecode.BuildList:
		return executeBuildSequence(frame, index, int(instruction.Operand), false)
	case bytecode.BuildSet:
		return executeBuildSet(frame, index, int(instruction.Operand))
	case bytecode.BuildMap:
		return executeBuildMap(frame, index, int(instruction.Operand))
	case bytecode.MapSet:
		return executeMapSet(frame, index)
	case bytecode.MapUpdate:
		return executeMapUpdate(frame, index)
	case bytecode.MapMerge:
		return executeMapMerge(frame, index)
	case bytecode.MakeFunction:
		function := &functionValue{
			code:    frame.code.children[instruction.Operand],
			globals: frame.globals,
		}
		return pushOutcome(frame, index, function)
	case bytecode.SetFunctionAttribute:
		target, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		payload, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		function, ok := target.(*functionValue)
		if !ok {
			return instructionOutcome{}, frame.failure(
				index,
				"SET_FUNCTION_ATTRIBUTE target is not a function",
			)
		}
		switch bytecode.FunctionAttribute(instruction.Operand) {
		case bytecode.FunctionDefaults:
			defaults, defaultsOK := payload.(*tupleValue)
			if !defaultsOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function defaults payload is not a tuple",
				)
			}
			if len(defaults.elements) > function.code.code.PositionalCount() {
				return instructionOutcome{}, frame.failure(
					index,
					"function default count exceeds positional parameter count",
				)
			}
			function.defaults = make([]Value, len(defaults.elements))
			copy(function.defaults, defaults.elements)
		case bytecode.FunctionKeywordDefaults:
			defaults, defaultsOK := payload.(*dictValue)
			if !defaultsOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function keyword defaults payload is not a dictionary",
				)
			}
			function.keywordDefaults = make(map[string]Value, len(defaults.entries))
			start, end := keywordOnlyRange(function.code)
			for _, entry := range defaults.entries {
				name, nameOK := entry.key.(*stringValue)
				if !nameOK {
					return instructionOutcome{}, frame.failure(
						index,
						"function keyword default name is not a string",
					)
				}
				found := false
				for parameter := start; parameter < end; parameter++ {
					if function.code.locals[parameter] == name.value {
						found = true
						break
					}
				}
				if !found {
					return instructionOutcome{}, frame.failure(
						index,
						"function keyword default has no keyword-only parameter",
					)
				}
				function.keywordDefaults[name.value] = entry.value
			}
		case bytecode.FunctionClosure:
			closure, closureOK := payload.(*tupleValue)
			if !closureOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function closure payload is not a tuple",
				)
			}
			if len(closure.elements) != len(function.code.freeVars) {
				return instructionOutcome{}, frame.failure(
					index,
					fmt.Sprintf(
						"function closure has %d cells for %d free variables",
						len(closure.elements),
						len(function.code.freeVars),
					),
				)
			}
			function.closure = make([]*cellValue, len(closure.elements))
			for closureIndex, value := range closure.elements {
				cell, cellOK := value.(*cellValue)
				if !cellOK {
					return instructionOutcome{}, frame.failure(
						index,
						fmt.Sprintf(
							"function closure item %d is not a cell",
							closureIndex,
						),
					)
				}
				function.closure[closureIndex] = cell
			}
		case bytecode.FunctionAnnotate:
			annotation, annotationOK := payload.(*functionValue)
			if !annotationOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function annotate payload is not a function",
				)
			}
			function.annotate = annotation
		}
		return pushOutcome(frame, index, function)
	case bytecode.Call:
		return executeCall(frame, index, int(instruction.Operand))
	case bytecode.CallEx:
		return executeUnpackedCall(
			frame,
			index,
			instruction.Operand == bytecode.CallExWithKeywords,
		)
	case bytecode.SetAdd:
		return executeSetAdd(frame, index)
	case bytecode.SetUpdate:
		return executeSetUpdate(frame, index)
	case bytecode.ListAppend:
		return executeListAppend(frame, index)
	case bytecode.ListExtend:
		return executeListExtend(frame, index)
	case bytecode.ListToTuple:
		return executeListToTuple(frame, index)
	case bytecode.BuildSlice:
		return executeBuildSlice(frame, index, int(instruction.Operand))
	case bytecode.GetIter:
		return executeGetIter(frame, index)
	case bytecode.ForIter:
		return executeForIter(frame, index, int(instruction.Operand))
	case bytecode.BinarySubscript:
		return executeBinarySubscript(frame, index)
	case bytecode.StoreSubscript:
		return executeStoreSubscript(frame, index)
	case bytecode.DeleteSubscript:
		return executeDeleteSubscript(frame, index)
	case bytecode.UnpackSequence:
		return executeUnpackSequence(frame, index, int(instruction.Operand))
	case bytecode.UnpackEx:
		before, after := bytecode.UnpackExCounts(instruction.Operand)
		return executeUnpackEx(frame, index, int(before), int(after))
	case bytecode.UnaryOp:
		return executeUnary(frame, index, instruction.Operand)
	case bytecode.BinaryOp:
		return executeBinary(frame, index, instruction.Operand)
	case bytecode.CompareOp:
		return executeComparison(frame, index, instruction.Operand)
	case bytecode.RaiseVarargs:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		exception, ok := value.(*Exception)
		if !ok {
			exception = newException("TypeError", "exceptions must derive from BaseException")
		}
		return instructionOutcome{kind: raised, exception: exception}, nil
	case bytecode.ReturnValue:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return instructionOutcome{kind: returned, value: value}, nil
	default:
		return instructionOutcome{}, frame.failure(
			index,
			"unsupported opcode reached dispatch: "+instruction.Opcode.String(),
		)
	}
}

func pushOutcome(frame *frame, index int, value Value) (instructionOutcome, error) {
	if !frame.push(value) {
		return instructionOutcome{}, frame.failure(index, "operand stack overflow")
	}
	return instructionOutcome{kind: advance}, nil
}
