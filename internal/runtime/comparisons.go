package runtime

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// executeComparison evaluates current equality, ordering, identity, and
// membership operations while retaining Python exceptions as VM outcomes.
func executeComparison(
	frame *frame,
	index int,
	operand uint32,
) (instructionOutcome, error) {
	right, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	left, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}

	if operand == bytecode.CompareIn || operand == bytecode.CompareNotIn {
		return executeMembership(frame, index, operand, right, left)
	}
	if operand == bytecode.CompareEqual || operand == bytecode.CompareNotEqual {
		_, leftUser := left.(*instanceValue)
		_, rightUser := right.(*instanceValue)
		if leftUser || rightUser {
			return executeUserEquality(frame, index, operand, left, right)
		}
	}
	if operand >= bytecode.CompareLess && operand <= bytecode.CompareGreaterEqual {
		_, leftUser := left.(*instanceValue)
		_, rightUser := right.(*instanceValue)
		if leftUser || rightUser {
			return executeUserOrdering(frame, index, operand, left, right)
		}
	}

	var result bool
	switch operand {
	case bytecode.CompareEqual:
		result = valuesEqual(left, right)
	case bytecode.CompareNotEqual:
		result = !valuesEqual(left, right)
	case bytecode.CompareIs:
		result = left == right
	case bytecode.CompareIsNot:
		result = left != right
	default:
		comparison, ordered, supported := orderedValues(left, right)
		if !supported {
			operator := "<"
			switch operand {
			case bytecode.CompareLessEqual:
				operator = "<="
			case bytecode.CompareGreater:
				operator = ">"
			case bytecode.CompareGreaterEqual:
				operator = ">="
			}
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					fmt.Sprintf(
						"'%s' not supported between instances of '%s' and '%s'",
						operator,
						left.TypeName(),
						right.TypeName(),
					),
				),
			}, nil
		}
		if ordered {
			switch operand {
			case bytecode.CompareLess:
				result = comparison < 0
			case bytecode.CompareLessEqual:
				result = comparison <= 0
			case bytecode.CompareGreater:
				result = comparison > 0
			case bytecode.CompareGreaterEqual:
				result = comparison >= 0
			}
		}
	}
	value := falseSingleton
	if result {
		value = trueSingleton
	}
	return pushOutcome(frame, index, value)
}

// valuesEqual implements equality for current scalar, tuple, and list values.
// Numeric equality includes booleans and exact integer-to-binary64 comparison.
func valuesEqual(left, right Value) bool {
	if left == right {
		return true
	}
	if leftInteger, ok := integerOperand(left); ok {
		if rightInteger, ok := integerOperand(right); ok {
			return leftInteger.Cmp(&rightInteger) == 0
		}
		switch right := right.(type) {
		case *floatValue:
			return integerFloatEqual(&leftInteger, right.value)
		case *complexValue:
			return right.imaginary == 0 &&
				integerFloatEqual(&leftInteger, right.real)
		}
	}

	switch left := left.(type) {
	case *floatValue:
		if rightInteger, ok := integerOperand(right); ok {
			return integerFloatEqual(&rightInteger, left.value)
		}
		switch right := right.(type) {
		case *floatValue:
			return left.value == right.value
		case *complexValue:
			return right.imaginary == 0 && left.value == right.real
		}
	case *complexValue:
		if rightInteger, ok := integerOperand(right); ok {
			return left.imaginary == 0 &&
				integerFloatEqual(&rightInteger, left.real)
		}
		switch right := right.(type) {
		case *floatValue:
			return left.imaginary == 0 && left.real == right.value
		case *complexValue:
			return left.real == right.real && left.imaginary == right.imaginary
		}
	case *stringValue:
		right, ok := right.(*stringValue)
		return ok && left.value == right.value
	case *bytesValue:
		right, ok := right.(*bytesValue)
		return ok && left.value == right.value
	case *tupleValue:
		right, ok := right.(*tupleValue)
		if !ok || len(left.elements) != len(right.elements) {
			return false
		}
		for index := range left.elements {
			if !valuesEqual(left.elements[index], right.elements[index]) {
				return false
			}
		}
		return true
	case *listValue:
		right, ok := right.(*listValue)
		if !ok || len(left.elements) != len(right.elements) {
			return false
		}
		for index := range left.elements {
			if !valuesEqual(left.elements[index], right.elements[index]) {
				return false
			}
		}
		return true
	case *noneValue:
		_, ok := right.(*noneValue)
		return ok
	case *ellipsisValue:
		_, ok := right.(*ellipsisValue)
		return ok
	case *Exception:
		right, ok := right.(*Exception)
		return ok && left == right
	}
	return false
}

func integerFloatEqual(integer *big.Int, value float64) bool {
	comparison, ordered := compareIntegerFloat(integer, value)
	return ordered && comparison == 0
}

// orderedValues compares supported numeric, string, and bytes pairs. Its
// ordered result is false for NaN without turning that case into a TypeError.
func orderedValues(left, right Value) (comparison int, ordered, supported bool) {
	if leftInteger, ok := integerOperand(left); ok {
		if rightInteger, ok := integerOperand(right); ok {
			return leftInteger.Cmp(&rightInteger), true, true
		}
		if right, ok := right.(*floatValue); ok {
			comparison, ordered := compareIntegerFloat(&leftInteger, right.value)
			return comparison, ordered, true
		}
	}
	if left, ok := left.(*floatValue); ok {
		if rightInteger, ok := integerOperand(right); ok {
			comparison, ordered := compareIntegerFloat(&rightInteger, left.value)
			return -comparison, ordered, true
		}
		if right, ok := right.(*floatValue); ok {
			if math.IsNaN(left.value) || math.IsNaN(right.value) {
				return 0, false, true
			}
			switch {
			case left.value < right.value:
				return -1, true, true
			case left.value > right.value:
				return 1, true, true
			default:
				return 0, true, true
			}
		}
	}
	if left, ok := left.(*stringValue); ok {
		if right, ok := right.(*stringValue); ok {
			return strings.Compare(left.value, right.value), true, true
		}
	}
	if left, ok := left.(*bytesValue); ok {
		if right, ok := right.(*bytesValue); ok {
			return strings.Compare(left.value, right.value), true, true
		}
	}
	return 0, false, false
}

func compareIntegerFloat(integer *big.Int, value float64) (int, bool) {
	switch {
	case math.IsNaN(value):
		return 0, false
	case math.IsInf(value, 1):
		return -1, true
	case math.IsInf(value, -1):
		return 1, true
	}
	var integerRatio big.Rat
	integerRatio.SetInt(integer)
	var floatRatio big.Rat
	floatRatio.SetFloat64(value)
	return integerRatio.Cmp(&floatRatio), true
}
