package runtime

import (
	"math/big"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type truthMethod uint8

const (
	truthBoolMethod truthMethod = iota
	truthLengthMethod
)

type truthCall struct {
	instruction int
	operation   bytecode.Instruction
	original    Value
	method      truthMethod
}

// executeTruthOperation either completes truth testing immediately or calls a
// user special method and leaves the original instruction suspended.
func executeTruthOperation(
	frame *frame,
	instruction int,
	operation bytecode.Instruction,
	value Value,
) (instructionOutcome, error) {
	if truth, immediate := immediateTruth(value); immediate {
		return completeTruthOperation(frame, instruction, operation, value, truth)
	}

	instance := value.(*instanceValue)
	method, found := lookupInstanceSpecial(instance, "__bool__")
	methodKind := truthBoolMethod
	if found && method == None {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+instance.TypeName()+"' cannot be interpreted as a boolean",
			),
		}, nil
	}
	if !found {
		method, found = lookupInstanceSpecial(instance, "__len__")
		methodKind = truthLengthMethod
	}
	if !found {
		return completeTruthOperation(frame, instruction, operation, value, true)
	}

	call := &truthCall{
		instruction: instruction,
		operation:   operation,
		original:    value,
		method:      methodKind,
	}
	outcome, err := executeFunctionCall(
		frame,
		instruction,
		len(frame.stack),
		method,
		nil,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.truth = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"truth special method returned without a value",
		)
	}
	return finishTruthCall(frame, call, result)
}

// immediateTruth returns the fixed truth value for built-in runtime objects and
// leaves user instances unresolved so their class protocols can run.
func immediateTruth(value Value) (bool, bool) {
	switch value := value.(type) {
	case *noneValue:
		return false, true
	case *boolValue:
		return value.value, true
	case *intValue:
		return value.value.Sign() != 0, true
	case *floatValue:
		return value.value != 0, true
	case *complexValue:
		return value.real != 0 || value.imaginary != 0, true
	case *stringValue:
		return len(value.value) != 0, true
	case *bytesValue:
		return len(value.value) != 0, true
	case *tupleValue:
		return len(value.elements) != 0, true
	case *listValue:
		return len(value.elements) != 0, true
	case *dictValue:
		return len(value.entries) != 0, true
	case *setValue:
		return len(value.entries) != 0, true
	case *instanceValue:
		return false, false
	default:
		return true, true
	}
}

func finishTruthCall(
	frame *frame,
	call *truthCall,
	result Value,
) (instructionOutcome, error) {
	truth, exception := truthMethodResult(call.method, result)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return completeTruthOperation(
		frame,
		call.instruction,
		call.operation,
		call.original,
		truth,
	)
}

// truthMethodResult enforces the distinct return contracts for __bool__ and
// __len__, including Python's negative and host-index overflow checks.
func truthMethodResult(method truthMethod, result Value) (bool, *Exception) {
	if method == truthBoolMethod {
		boolean, ok := result.(*boolValue)
		if !ok {
			return false, newException(
				"TypeError",
				"__bool__ should return bool, returned "+result.TypeName(),
			)
		}
		return boolean.value, nil
	}

	integer, ok := integerOperand(result)
	if !ok {
		return false, newException(
			"TypeError",
			"'"+result.TypeName()+"' object cannot be interpreted as an integer",
		)
	}
	if integer.Sign() < 0 {
		return false, newException("ValueError", "__len__() should return >= 0")
	}
	maximum := new(big.Int).SetUint64(uint64(^uint(0) >> 1))
	if integer.Cmp(maximum) > 0 {
		return false, newException(
			"OverflowError",
			"cannot fit 'int' into an index-sized integer",
		)
	}
	return integer.Sign() != 0, nil
}

// completeTruthOperation applies one resolved truth value to the original
// opcode, restoring a retained short-circuit operand only on its jump path.
func completeTruthOperation(
	frame *frame,
	instruction int,
	operation bytecode.Instruction,
	original Value,
	truth bool,
) (instructionOutcome, error) {
	switch operation.Opcode {
	case bytecode.UnaryOp:
		result := falseSingleton
		if !truth {
			result = trueSingleton
		}
		return pushOutcome(frame, instruction, result)
	case bytecode.PopJumpIfFalse, bytecode.PopJumpIfTrue:
		takeJump := truth
		if operation.Opcode == bytecode.PopJumpIfFalse {
			takeJump = !takeJump
		}
		if takeJump {
			frame.instruction = int(operation.Operand)
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.JumpIfFalseOrPop, bytecode.JumpIfTrueOrPop:
		takeJump := truth
		if operation.Opcode == bytecode.JumpIfFalseOrPop {
			takeJump = !takeJump
		}
		if !takeJump {
			return instructionOutcome{kind: advance}, nil
		}
		frame.instruction = int(operation.Operand)
		return pushOutcome(frame, instruction, original)
	default:
		return instructionOutcome{}, frame.failure(
			instruction,
			"unsupported truth operation "+operation.Opcode.String(),
		)
	}
}
