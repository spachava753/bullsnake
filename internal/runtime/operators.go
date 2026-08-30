package runtime

import (
	"fmt"
	"math"
	"math/big"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// executeUnary applies numeric unary slots or scalar truth testing and returns
// a Python TypeError when the operand type has no selected numeric behavior.
func executeUnary(frame *frame, index int, operand uint32) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	if operand == bytecode.UnaryNot {
		result := falseSingleton
		if !truthValue(value) {
			result = trueSingleton
		}
		return pushOutcome(frame, index, result)
	}

	var result Value
	switch value := value.(type) {
	case *intValue:
		result = integerUnary(&value.value, operand)
	case *boolValue:
		var integer big.Int
		if value.value {
			integer.SetInt64(1)
		}
		result = integerUnary(&integer, operand)
	case *floatValue:
		switch operand {
		case bytecode.UnaryPositive:
			result = &floatValue{value: value.value}
		case bytecode.UnaryNegative:
			result = &floatValue{value: -value.value}
		}
	case *complexValue:
		switch operand {
		case bytecode.UnaryPositive:
			result = &complexValue{real: value.real, imaginary: value.imaginary}
		case bytecode.UnaryNegative:
			result = &complexValue{real: -value.real, imaginary: -value.imaginary}
		}
	}
	if result != nil {
		return pushOutcome(frame, index, result)
	}

	operator := "+"
	if operand == bytecode.UnaryNegative {
		operator = "-"
	} else if operand == bytecode.UnaryInvert {
		operator = "~"
	}
	return instructionOutcome{
		kind: raised,
		exception: newException(
			"TypeError",
			fmt.Sprintf("bad operand type for unary %s: '%s'", operator, value.TypeName()),
		),
	}, nil
}

func integerUnary(value *big.Int, operand uint32) *intValue {
	var result big.Int
	switch operand {
	case bytecode.UnaryPositive:
		result.Set(value)
	case bytecode.UnaryNegative:
		result.Neg(value)
	case bytecode.UnaryInvert:
		result.Not(value)
	}
	return &intValue{value: result}
}

// truthValue implements the fixed truth behavior of every scalar value in the
// current runtime. User-defined truth protocols enter in a later object slice.
func truthValue(value Value) bool {
	switch value := value.(type) {
	case *noneValue:
		return false
	case *boolValue:
		return value.value
	case *intValue:
		return value.value.Sign() != 0
	case *floatValue:
		return value.value != 0
	case *complexValue:
		return value.real != 0 || value.imaginary != 0
	case *stringValue:
		return len(value.value) != 0
	case *bytesValue:
		return len(value.value) != 0
	case *tupleValue:
		return len(value.elements) != 0
	case *listValue:
		return len(value.elements) != 0
	case *dictValue:
		return len(value.entries) != 0
	case *setValue:
		return len(value.entries) != 0
	default:
		return true
	}
}

// executeBinary applies the selected numeric operations. Booleans enter integer
// operations as zero and one. In-place calls share immutable results but retain
// their diagnostic operator spelling.
func executeBinary(
	frame *frame,
	index int,
	operand uint32,
	inPlace bool,
) (instructionOutcome, error) {
	right, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	left, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	if result, exception, handled := floatBinary(left, right, operand); handled {
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return pushOutcome(frame, index, result)
	}
	leftInteger, leftOK := integerOperand(left)
	rightInteger, rightOK := integerOperand(right)
	if !leftOK || !rightOK || operand == bytecode.BinaryDivide {
		operator := "+"
		switch operand {
		case bytecode.BinarySubtract:
			operator = "-"
		case bytecode.BinaryMultiply:
			operator = "*"
		case bytecode.BinaryPower:
			operator = "**"
		case bytecode.BinaryDivide:
			operator = "/"
		case bytecode.BinaryFloorDivide:
			operator = "//"
		case bytecode.BinaryModulo:
			operator = "%"
		case bytecode.BinaryLeftShift:
			operator = "<<"
		case bytecode.BinaryRightShift:
			operator = ">>"
		case bytecode.BinaryOr:
			operator = "|"
		case bytecode.BinaryXor:
			operator = "^"
		case bytecode.BinaryAnd:
			operator = "&"
		}
		if inPlace {
			operator += "="
		}
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				fmt.Sprintf(
					"unsupported operand type(s) for %s: '%s' and '%s'",
					operator,
					left.TypeName(),
					right.TypeName(),
				),
			),
		}, nil
	}

	if operand == bytecode.BinaryPower {
		result, exception := integerPower(&leftInteger, &rightInteger)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return pushOutcome(frame, index, result)
	}

	if operand == bytecode.BinaryLeftShift || operand == bytecode.BinaryRightShift {
		if rightInteger.Sign() < 0 {
			return instructionOutcome{
				kind:      raised,
				exception: newException("ValueError", "negative shift count"),
			}, nil
		}
		if operand == bytecode.BinaryLeftShift && leftInteger.Sign() == 0 {
			return pushOutcome(frame, index, &intValue{})
		}
		countFits := rightInteger.IsUint64()
		var count uint64
		if countFits {
			count = rightInteger.Uint64()
		}
		if operand == bytecode.BinaryRightShift &&
			(!countFits || count >= uint64(leftInteger.BitLen())) {
			var result big.Int
			if leftInteger.Sign() < 0 {
				result.SetInt64(-1)
			}
			return pushOutcome(frame, index, &intValue{value: result})
		}
		maxInt := uint64(^uint(0) >> 1)
		if !countFits || count > maxInt {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"OverflowError",
					"too many digits in integer",
				),
			}, nil
		}
		var result big.Int
		if operand == bytecode.BinaryLeftShift {
			result.Lsh(&leftInteger, uint(count))
		} else {
			result.Rsh(&leftInteger, uint(count))
		}
		return pushOutcome(frame, index, &intValue{value: result})
	}

	if (operand == bytecode.BinaryFloorDivide || operand == bytecode.BinaryModulo) &&
		rightInteger.Sign() == 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"ZeroDivisionError",
				"integer division or modulo by zero",
			),
		}, nil
	}

	var result big.Int
	switch operand {
	case bytecode.BinaryAdd:
		result.Add(&leftInteger, &rightInteger)
	case bytecode.BinarySubtract:
		result.Sub(&leftInteger, &rightInteger)
	case bytecode.BinaryMultiply:
		result.Mul(&leftInteger, &rightInteger)
	case bytecode.BinaryFloorDivide, bytecode.BinaryModulo:
		var remainder big.Int
		result.QuoRem(&leftInteger, &rightInteger, &remainder)
		if remainder.Sign() != 0 && remainder.Sign() != rightInteger.Sign() {
			var one big.Int
			one.SetInt64(1)
			result.Sub(&result, &one)
			remainder.Add(&remainder, &rightInteger)
		}
		if operand == bytecode.BinaryModulo {
			result.Set(&remainder)
		}
	case bytecode.BinaryOr:
		result.Or(&leftInteger, &rightInteger)
	case bytecode.BinaryXor:
		result.Xor(&leftInteger, &rightInteger)
	case bytecode.BinaryAnd:
		result.And(&leftInteger, &rightInteger)
	}
	return pushOutcome(frame, index, &intValue{value: result})
}

// floatBinary applies operations that require binary64 coercion.
func floatBinary(left, right Value, operand uint32) (Value, *Exception, bool) {
	switch operand {
	case bytecode.BinaryAdd, bytecode.BinarySubtract,
		bytecode.BinaryMultiply, bytecode.BinaryDivide, bytecode.BinaryModulo:
	default:
		return nil, nil, false
	}
	_, leftIsFloat := left.(*floatValue)
	_, rightIsFloat := right.(*floatValue)
	if !leftIsFloat && !rightIsFloat && operand != bytecode.BinaryDivide {
		return nil, nil, false
	}
	leftNumber, exception, leftOK := numericFloat(left)
	if exception != nil {
		return nil, exception, true
	}
	rightNumber, exception, rightOK := numericFloat(right)
	if exception != nil {
		return nil, exception, true
	}
	if !leftOK || !rightOK {
		return nil, nil, false
	}

	var result float64
	switch operand {
	case bytecode.BinaryAdd:
		result = leftNumber + rightNumber
	case bytecode.BinarySubtract:
		result = leftNumber - rightNumber
	case bytecode.BinaryMultiply:
		result = leftNumber * rightNumber
	case bytecode.BinaryDivide:
		if rightNumber == 0 {
			return nil, newException("ZeroDivisionError", "division by zero"), true
		}
		result = leftNumber / rightNumber
	case bytecode.BinaryModulo:
		if rightNumber == 0 {
			return nil, newException("ZeroDivisionError", "division by zero"), true
		}
		result = math.Mod(leftNumber, rightNumber)
		if result != 0 {
			if (rightNumber < 0) != (result < 0) {
				result += rightNumber
			}
		} else {
			result = math.Copysign(0, rightNumber)
		}
	}
	return &floatValue{value: result}, nil, true
}

func numericFloat(value Value) (float64, *Exception, bool) {
	if value, ok := value.(*floatValue); ok {
		return value.value, nil, true
	}
	integer, ok := integerOperand(value)
	if !ok {
		return 0, nil, false
	}
	converted, _ := integer.Float64()
	if math.IsInf(converted, 0) {
		return 0, newException("OverflowError", "int too large to convert to float"), true
	}
	return converted, nil, true
}

func integerOperand(value Value) (big.Int, bool) {
	var integer big.Int
	switch value := value.(type) {
	case *intValue:
		integer.Set(&value.value)
		return integer, true
	case *boolValue:
		if value.value {
			integer.SetInt64(1)
		}
		return integer, true
	default:
		return integer, false
	}
}
