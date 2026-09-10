package runtime

import (
	"strconv"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type truthMethod uint8

const (
	truthBoolMethod truthMethod = iota
	truthLengthMethod
)

type truthCall struct {
	instruction   int
	operation     bytecode.Instruction
	original      Value
	method        truthMethod
	returnBoolean bool
	aggregate     *truthAggregateCall
	filtering     *filterCall
	listRemoval   *listRemoveCall
	sorting       *sortCall
	sortReverse   *sortCall
	splitlines    *stringSplitlinesCall
	abstractStore *abstractMethodsStore
}

// executeTruthOperation resolves one value for a bytecode truth operation.
func executeTruthOperation(
	frame *frame,
	instruction int,
	operation bytecode.Instruction,
	value Value,
) (instructionOutcome, error) {
	return executeTruthValue(frame, instruction, operation, value, false)
}

// executeTruthValue either completes immediately or leaves a user special
// method frame carrying the operation that resumes after its return.
func executeTruthValue(
	frame *frame,
	instruction int,
	operation bytecode.Instruction,
	value Value,
	returnBoolean bool,
) (instructionOutcome, error) {
	call := &truthCall{
		instruction:   instruction,
		operation:     operation,
		original:      value,
		returnBoolean: returnBoolean,
	}
	return executeTruthWithCall(frame, value, call)
}

// executeTruthWithCall resolves one value and preserves the caller-specific
// completion state across a user __bool__ or __len__ frame.
func executeTruthWithCall(
	frame *frame,
	value Value,
	call *truthCall,
) (instructionOutcome, error) {
	if value == notImplementedSingleton {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"NotImplemented should not be used in a boolean context",
			),
		}, nil
	}
	if truth, immediate := immediateTruth(value); immediate {
		return completeTruthCall(frame, call, truth)
	}

	instance := value.(*instanceValue)
	method, found := lookupInstanceSpecial(instance, "__bool__")
	call.method = truthBoolMethod
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
		call.method = truthLengthMethod
	}
	if !found {
		return completeTruthCall(frame, call, true)
	}

	outcome, err := executeFunctionCall(
		frame,
		call.instruction,
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
			call.instruction,
			"truth special method returned without a value",
		)
	}
	return finishTruthCall(frame, call, result)
}

// executeBuiltinBool applies ordinary truth testing to zero or one value.
func executeBuiltinBool(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"bool() takes no keyword arguments",
		)), nil
	}
	if len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"bool expected at most 1 argument, got "+strconv.Itoa(len(arguments)),
		)), nil
	}
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return pushOutcome(caller, instruction, falseSingleton)
	}
	value := arguments[0]
	discardCallSegment(caller, base)
	return executeTruthValue(
		caller,
		instruction,
		bytecode.Instruction{},
		value,
		true,
	)
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
	case *bytearrayValue:
		return len(value.buffer.data) != 0, true
	case *bytesValue:
		return len(value.value) != 0, true
	case *tupleValue:
		return len(value.elements) != 0, true
	case *listValue:
		return len(value.elements) != 0, true
	case *dictValue:
		return len(value.entries) != 0, true
	case *mappingProxyValue:
		return len(value.dictionary.entries) != 0, true
	case *dictionaryKeysView:
		return len(value.dictionary.entries) != 0, true
	case *dictionaryItemsView:
		return len(value.dictionary.entries) != 0, true
	case *dictionaryValuesView:
		return len(value.dictionary.entries) != 0, true
	case *setValue:
		return len(value.entries) != 0, true
	case *frozenSetValue:
		return len(value.entries) != 0, true
	case *rangeValue:
		return value.length.Sign() != 0, true
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
	return completeTruthCall(frame, call, truth)
}

// completeTruthCall routes the resolved truth value back to the native
// operation or bytecode instruction that requested it.
func completeTruthCall(
	frame *frame,
	call *truthCall,
	truth bool,
) (instructionOutcome, error) {
	if call.abstractStore != nil {
		return finishAbstractMethodsStore(frame, call.abstractStore, truth)
	}
	if call.filtering != nil {
		return finishFilterTruth(frame, call.filtering, truth)
	}
	if call.aggregate != nil {
		return finishTruthAggregateTruth(frame, call.aggregate, truth)
	}
	if call.listRemoval != nil {
		return finishListRemoveTruth(frame, call.listRemoval, truth)
	}
	if call.sortReverse != nil {
		return finishSortReverse(frame, call.sortReverse, truth)
	}
	if call.sorting != nil {
		finishSortInsertionStep(call.sorting, truth)
		return continueSortInsertion(frame, call.sorting)
	}
	if call.splitlines != nil {
		return finishStringSplitlines(frame, call.splitlines, truth)
	}
	if call.returnBoolean {
		result := falseSingleton
		if truth {
			result = trueSingleton
		}
		return pushOutcome(frame, call.instruction, result)
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

	length, exception := checkedLengthResult(result)
	if exception != nil {
		return false, exception
	}
	return length.value.Sign() != 0, nil
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
	case bytecode.CompareOp:
		if operation.Operand != bytecode.CompareIn &&
			operation.Operand != bytecode.CompareNotIn &&
			operation.Operand != bytecode.CompareNotEqual {
			return instructionOutcome{}, frame.failure(
				instruction,
				"unsupported comparison truth operation",
			)
		}
		if operation.Operand == bytecode.CompareNotIn ||
			operation.Operand == bytecode.CompareNotEqual {
			truth = !truth
		}
		result := falseSingleton
		if truth {
			result = trueSingleton
		}
		return pushOutcome(frame, instruction, result)
	default:
		return instructionOutcome{}, frame.failure(
			instruction,
			"unsupported truth operation "+operation.Opcode.String(),
		)
	}
}
