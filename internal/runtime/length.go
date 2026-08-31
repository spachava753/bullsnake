package runtime

import (
	"math/big"
	"strconv"
)

type lengthCall struct {
	instruction int
}

// executeBuiltinLen returns fixed built-in lengths immediately and suspends the
// caller while a user-defined __len__ method runs in the VM loop.
func executeBuiltinLen(
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
			"len() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"len() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}

	value := arguments[0]
	if rangeObject, ok := value.(*rangeValue); ok {
		var integer big.Int
		integer.Set(&rangeObject.length)
		length, exception := checkedLengthResult(&intValue{value: integer})
		discardCallSegment(caller, base)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, length)
	}
	if length, found := immediateLength(value); found {
		discardCallSegment(caller, base)
		var integer big.Int
		integer.SetUint64(uint64(length))
		return pushOutcome(caller, instruction, &intValue{value: integer})
	}
	instance, ok := value.(*instanceValue)
	if !ok {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"object of type '"+value.TypeName()+"' has no len()",
		)), nil
	}
	method, found := lookupInstanceSpecial(instance, "__len__")
	if !found {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"object of type '"+value.TypeName()+"' has no len()",
		)), nil
	}

	discardCallSegment(caller, base)
	call := &lengthCall{instruction: instruction}
	outcome, err := executeFunctionCall(
		caller,
		instruction,
		len(caller.stack),
		method,
		nil,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.length = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := caller.pop()
	if !ok {
		return instructionOutcome{}, caller.failure(
			instruction,
			"length special method returned without a value",
		)
	}
	return finishLengthCall(caller, call, result)
}

func finishLengthCall(
	caller *frame,
	call *lengthCall,
	result Value,
) (instructionOutcome, error) {
	length, exception := checkedLengthResult(result)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, call.instruction, length)
}

// checkedLengthResult applies Python's index, sign, and host-size checks and
// converts bool results to ordinary integer length values.
func checkedLengthResult(result Value) (*intValue, *Exception) {
	integer, ok := integerOperand(result)
	if !ok {
		return nil, newException(
			"TypeError",
			"'"+result.TypeName()+"' object cannot be interpreted as an integer",
		)
	}
	if integer.Sign() < 0 {
		return nil, newException("ValueError", "__len__() should return >= 0")
	}
	maximum := new(big.Int).SetUint64(uint64(^uint(0) >> 1))
	if integer.Cmp(maximum) > 0 {
		return nil, newException(
			"OverflowError",
			"cannot fit 'int' into an index-sized integer",
		)
	}
	return &intValue{value: integer}, nil
}

// immediateLength returns the element count for built-in values whose length
// cannot invoke Python code.
func immediateLength(value Value) (int, bool) {
	switch value := value.(type) {
	case *stringValue:
		length := 0
		for offset := 0; offset < len(value.value); length++ {
			_, size, _ := decodeStringRune(value.value[offset:])
			offset += size
		}
		return length, true
	case *bytesValue:
		return len(value.value), true
	case *tupleValue:
		return len(value.elements), true
	case *listValue:
		return len(value.elements), true
	case *dictValue:
		return len(value.entries), true
	case *dictionaryItemsView:
		return len(value.dictionary.entries), true
	case *setValue:
		return len(value.entries), true
	case *frozenSetValue:
		return len(value.entries), true
	default:
		return 0, false
	}
}
