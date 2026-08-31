package runtime

import (
	"math"
	"math/big"
	"strconv"
)

// executeBuiltinAbs returns native numeric magnitudes immediately and dispatches
// user __abs__ methods through the existing unary continuation.
func executeBuiltinAbs(
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
			"abs() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"abs() takes exactly one argument ("+count+" given)",
		)), nil
	}

	value := arguments[0]
	discardCallSegment(caller, base)
	if result, supported := immediateAbsolute(value); supported {
		return pushOutcome(caller, instruction, result)
	}
	if instance, ok := value.(*instanceValue); ok {
		if method, found := lookupInstanceSpecial(instance, "__abs__"); found {
			return executeUserUnary(caller, instruction, method)
		}
	}
	return raiseOutcome(newException(
		"TypeError",
		"bad operand type for abs(): '"+value.TypeName()+"'",
	)), nil
}

// immediateAbsolute returns an independent magnitude value for each built-in
// numeric representation and leaves every other value for protocol dispatch.
func immediateAbsolute(value Value) (Value, bool) {
	switch value := value.(type) {
	case *intValue:
		var result big.Int
		result.Abs(&value.value)
		return &intValue{value: result}, true
	case *boolValue:
		var result big.Int
		if value.value {
			result.SetInt64(1)
		}
		return &intValue{value: result}, true
	case *floatValue:
		return &floatValue{value: math.Abs(value.value)}, true
	case *complexValue:
		return &floatValue{value: math.Hypot(value.real, value.imaginary)}, true
	default:
		return nil, false
	}
}
