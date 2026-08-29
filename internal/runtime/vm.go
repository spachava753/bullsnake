package runtime

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type outcomeKind uint8

const (
	advance outcomeKind = iota
	returned
	raised
)

type instructionOutcome struct {
	kind      outcomeKind
	value     Value
	exception *Exception
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
	case bytecode.StoreName:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.locals.values[frame.code.names[instruction.Operand]] = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.PopTop:
		if _, ok := frame.pop(); !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.UnaryOp:
		return executeUnary(frame, index, instruction.Operand)
	case bytecode.BinaryOp:
		return executeBinaryAdd(frame, index)
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
