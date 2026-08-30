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
			if thread.current == nil {
				return outcome.value, nil, nil
			}
			if !thread.current.push(outcome.value) {
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
		defaults, ok := payload.(*tupleValue)
		if !ok {
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
