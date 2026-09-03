package runtime

import (
	"fmt"
	"math"
	"math/big"
	"strings"

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
		truth, exception, err := truthValueForFrame(frame, value)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		result := falseSingleton
		if !truth {
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
	if instance, ok := value.(*instanceValue); ok {
		methodName := "__pos__"
		switch operand {
		case bytecode.UnaryNegative:
			methodName = "__neg__"
		case bytecode.UnaryInvert:
			methodName = "__invert__"
		}
		if method, found := instance.class.lookup(methodName); found {
			result, exception, err := callValueSynchronously(
				frame, bindCallable(method, instance), nil,
			)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return pushOutcome(frame, index, result)
		}
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

// truthValueForFrame adds Python __bool__ and __len__ dispatch to the fixed
// truth rules used for built-in values.
func truthValueForFrame(caller *frame, value Value) (bool, *Exception, error) {
	instance, ok := value.(*instanceValue)
	if !ok {
		return truthValue(value), nil, nil
	}
	if method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__bool__"); err != nil || exception != nil {
		return false, exception, err
	} else if found {
		result, exception, err := callValueSynchronously(
			caller, method, nil,
		)
		if err != nil || exception != nil {
			return false, exception, err
		}
		boolean, ok := result.(*boolValue)
		if !ok {
			return false, newException("TypeError", "__bool__ should return bool"), nil
		}
		return boolean.value, nil, nil
	}
	if method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__len__"); err != nil || exception != nil {
		return false, exception, err
	} else if found {
		result, exception, err := callValueSynchronously(
			caller, method, nil,
		)
		if err != nil || exception != nil {
			return false, exception, err
		}
		integer, ok := integerOperand(result)
		if !ok {
			return false, newException("TypeError", "__len__ should return an integer"), nil
		}
		if integer.Sign() < 0 {
			return false, newException("ValueError", "__len__() should return >= 0"), nil
		}
		return integer.Sign() != 0, nil, nil
	}
	return truthValue(value), nil, nil
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
	case *dequeValue:
		return len(value.elements) != 0
	case *instanceValue:
		if value.sequence != nil {
			return len(value.sequence.elements) != 0
		}
		if value.tuple != nil {
			return len(value.tuple.elements) != 0
		}
		if value.mapping != nil {
			return len(value.mapping.entries) != 0
		}
		return true
	default:
		return true
	}
}

// executeBinary applies the selected arbitrary-precision integer operations.
// Booleans enter this path as the integer values zero and one. In-place calls
// share immutable results but retain their diagnostic operator spelling.
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
	if format, ok := left.(*stringValue); ok && operand == bytecode.BinaryModulo {
		result, exception := percentFormat(format.value, right)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return pushOutcome(frame, index, &stringValue{value: result})
	}
	if value, supported := binarySetOperation(operand, left, right); supported {
		return pushOutcome(frame, index, value)
	}
	if operand == bytecode.BinaryOr {
		if union, ok := unionOperands(left, right); ok {
			return pushOutcome(frame, index, union)
		}
	}
	if leftText, ok := left.(*stringValue); ok {
		if rightText, rightOK := right.(*stringValue); rightOK && operand == bytecode.BinaryAdd {
			return pushOutcome(frame, index, &stringValue{value: leftText.value + rightText.value})
		}
		if count, countOK := integerOperand(right); countOK && operand == bytecode.BinaryMultiply {
			return repeatText(frame, index, leftText.value, &count)
		}
	}
	if leftBytes, ok := left.(*bytesValue); ok {
		if rightBytes, rightOK := right.(*bytesValue); rightOK && operand == bytecode.BinaryAdd {
			return pushOutcome(frame, index, &bytesValue{value: leftBytes.value + rightBytes.value})
		}
	}
	if count, ok := integerOperand(left); ok && operand == bytecode.BinaryMultiply {
		if rightText, rightOK := right.(*stringValue); rightOK {
			return repeatText(frame, index, rightText.value, &count)
		}
	}
	if operand == bytecode.BinaryMultiply {
		if count, ok := integerOperand(right); ok {
			if result, supported, exception := repeatSequence(left, &count); supported {
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				return pushOutcome(frame, index, result)
			}
		}
		if count, ok := integerOperand(left); ok {
			if result, supported, exception := repeatSequence(right, &count); supported {
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				return pushOutcome(frame, index, result)
			}
		}
	}
	if operand == bytecode.BinaryAdd {
		if leftList, ok := left.(*listValue); ok {
			if rightList, rightOK := right.(*listValue); rightOK {
				elements := append([]Value(nil), leftList.elements...)
				elements = append(elements, rightList.elements...)
				return pushOutcome(frame, index, &listValue{elements: elements})
			}
		}
		if leftTuple, ok := left.(*tupleValue); ok {
			if rightTuple, rightOK := right.(*tupleValue); rightOK {
				elements := append([]Value(nil), leftTuple.elements...)
				elements = append(elements, rightTuple.elements...)
				return pushOutcome(frame, index, &tupleValue{elements: elements})
			}
		}
	}
	if _, leftIsComplex := left.(*complexValue); leftIsComplex {
		if outcome, supported, err := executeComplexBinary(frame, index, operand, left, right); supported {
			return outcome, err
		}
	}
	if _, rightIsComplex := right.(*complexValue); rightIsComplex {
		if outcome, supported, err := executeComplexBinary(frame, index, operand, left, right); supported {
			return outcome, err
		}
	}
	if operand == bytecode.BinaryDivide {
		return executeTrueDivision(frame, index, left, right, inPlace)
	}
	if _, leftIsFloat := left.(*floatValue); leftIsFloat {
		if outcome, supported := executeFloatBinary(frame, index, operand, left, right); supported {
			return outcome, nil
		}
	}
	if _, rightIsFloat := right.(*floatValue); rightIsFloat {
		if outcome, supported := executeFloatBinary(frame, index, operand, left, right); supported {
			return outcome, nil
		}
	}
	if outcome, supported, err := executeBinarySpecialMethod(
		frame, index, operand, inPlace, left, right,
	); supported {
		return outcome, err
	}
	leftInteger, leftOK := integerOperand(left)
	rightInteger, rightOK := integerOperand(right)
	if !leftOK || !rightOK {
		operator := "+"
		switch operand {
		case bytecode.BinarySubtract:
			operator = "-"
		case bytecode.BinaryMultiply:
			operator = "*"
		case bytecode.BinaryMatrixMultiply:
			operator = "@"
		case bytecode.BinaryFloorDivide:
			operator = "//"
		case bytecode.BinaryModulo:
			operator = "%"
		case bytecode.BinaryPower:
			operator = "**"
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
	if operand == bytecode.BinaryPower {
		if rightInteger.Sign() < 0 {
			if leftInteger.Sign() == 0 {
				return instructionOutcome{
					kind:      raised,
					exception: newException("ZeroDivisionError", "0.0 cannot be raised to a negative power"),
				}, nil
			}
			leftFloat, _ := new(big.Float).SetInt(&leftInteger).Float64()
			rightFloat, _ := new(big.Float).SetInt(&rightInteger).Float64()
			return pushOutcome(frame, index, &floatValue{value: math.Pow(leftFloat, rightFloat)})
		}
		var result big.Int
		result.Exp(&leftInteger, &rightInteger, nil)
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

// binarySetOperation applies union, difference, intersection, and symmetric difference.
func binarySetOperation(operand uint32, left, right Value) (Value, bool) {
	leftElements, leftOK := setOperandElements(left)
	rightElements, rightOK := setOperandElements(right)
	if !leftOK || !rightOK {
		return nil, false
	}
	var result []Value
	switch operand {
	case bytecode.BinaryOr:
		result = append(result, leftElements...)
		for _, value := range rightElements {
			if !containsElement(result, value) {
				result = append(result, value)
			}
		}
	case bytecode.BinarySubtract:
		for _, value := range leftElements {
			if !containsElement(rightElements, value) {
				result = append(result, value)
			}
		}
	case bytecode.BinaryAnd:
		for _, value := range leftElements {
			if containsElement(rightElements, value) {
				result = append(result, value)
			}
		}
	case bytecode.BinaryXor:
		for _, value := range append(append([]Value(nil), leftElements...), rightElements...) {
			if containsElement(leftElements, value) == containsElement(rightElements, value) {
				continue
			}
			if !containsElement(result, value) {
				result = append(result, value)
			}
		}
	default:
		return nil, false
	}
	if _, frozen := left.(*frozenSetValue); frozen {
		return &frozenSetValue{entries: result}, true
	}
	return &setValue{entries: result}, true
}

func setOperandElements(value Value) ([]Value, bool) {
	switch value := value.(type) {
	case *setValue:
		return value.entries, true
	case *frozenSetValue:
		return value.entries, true
	default:
		return nil, false
	}
}

// executeBinarySpecialMethod dispatches direct and reflected Python arithmetic slots.
func executeBinarySpecialMethod(
	frame *frame,
	index int,
	operand uint32,
	inPlace bool,
	left Value,
	right Value,
) (instructionOutcome, bool, error) {
	direct, reflected, inPlaceName := "", "", ""
	switch operand {
	case bytecode.BinaryAdd:
		direct, reflected, inPlaceName = "__add__", "__radd__", "__iadd__"
	case bytecode.BinarySubtract:
		direct, reflected, inPlaceName = "__sub__", "__rsub__", "__isub__"
	case bytecode.BinaryMultiply:
		direct, reflected, inPlaceName = "__mul__", "__rmul__", "__imul__"
	case bytecode.BinaryMatrixMultiply:
		direct, reflected, inPlaceName = "__matmul__", "__rmatmul__", "__imatmul__"
	case bytecode.BinaryDivide:
		direct, reflected, inPlaceName = "__truediv__", "__rtruediv__", "__itruediv__"
	case bytecode.BinaryFloorDivide:
		direct, reflected, inPlaceName = "__floordiv__", "__rfloordiv__", "__ifloordiv__"
	case bytecode.BinaryModulo:
		direct, reflected, inPlaceName = "__mod__", "__rmod__", "__imod__"
	case bytecode.BinaryPower:
		direct, reflected, inPlaceName = "__pow__", "__rpow__", "__ipow__"
	case bytecode.BinaryLeftShift:
		direct, reflected, inPlaceName = "__lshift__", "__rlshift__", "__ilshift__"
	case bytecode.BinaryRightShift:
		direct, reflected, inPlaceName = "__rshift__", "__rrshift__", "__irshift__"
	case bytecode.BinaryOr:
		direct, reflected, inPlaceName = "__or__", "__ror__", "__ior__"
	case bytecode.BinaryXor:
		direct, reflected, inPlaceName = "__xor__", "__rxor__", "__ixor__"
	case bytecode.BinaryAnd:
		direct, reflected, inPlaceName = "__and__", "__rand__", "__iand__"
	default:
		return instructionOutcome{}, false, nil
	}
	type methodCandidate struct {
		owner    Value
		argument Value
		name     string
	}
	candidates := make([]methodCandidate, 0, 3)
	if inPlace {
		candidates = append(candidates, methodCandidate{left, right, inPlaceName})
	}
	candidates = append(candidates,
		methodCandidate{left, right, direct},
		methodCandidate{right, left, reflected},
	)
	for _, candidate := range candidates {
		instance, ok := candidate.owner.(*instanceValue)
		if !ok {
			continue
		}
		method, found, exception, err := lookupBoundSpecialMethod(frame, instance, candidate.name)
		if err != nil {
			return instructionOutcome{}, true, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, true, nil
		}
		if !found {
			continue
		}
		value, exception, err := callValueSynchronously(
			frame, method, []Value{candidate.argument},
		)
		if err != nil {
			return instructionOutcome{}, true, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, true, nil
		}
		outcome, err := pushOutcome(frame, index, value)
		return outcome, true, err
	}
	return instructionOutcome{}, false, nil
}

// executeComplexBinary promotes real operands and applies supported complex arithmetic.
func executeComplexBinary(
	frame *frame,
	index int,
	operand uint32,
	left Value,
	right Value,
) (instructionOutcome, bool, error) {
	leftValue, leftOK := numericComplex(left)
	rightValue, rightOK := numericComplex(right)
	if !leftOK || !rightOK {
		return instructionOutcome{}, false, nil
	}
	var result complex128
	switch operand {
	case bytecode.BinaryAdd:
		result = leftValue + rightValue
	case bytecode.BinarySubtract:
		result = leftValue - rightValue
	case bytecode.BinaryMultiply:
		result = leftValue * rightValue
	case bytecode.BinaryDivide:
		if rightValue == 0 {
			return instructionOutcome{kind: raised, exception: newException(
				"ZeroDivisionError", "complex division by zero",
			)}, true, nil
		}
		result = leftValue / rightValue
	default:
		return instructionOutcome{}, false, nil
	}
	outcome, err := pushOutcome(frame, index, &complexValue{real: real(result), imaginary: imag(result)})
	return outcome, true, err
}

func numericComplex(value Value) (complex128, bool) {
	if value, ok := value.(*complexValue); ok {
		return complex(value.real, value.imaginary), true
	}
	if value, ok := numericFloat(value); ok {
		return complex(value, 0), true
	}
	return 0, false
}

// repeatSequence repeats list or tuple contents after validating the multiplier size.
func repeatSequence(value Value, count *big.Int) (Value, bool, *Exception) {
	var elements []Value
	list := false
	switch sequence := value.(type) {
	case *listValue:
		elements = sequence.elements
		list = true
	case *tupleValue:
		elements = sequence.elements
	default:
		return nil, false, nil
	}
	if count.Sign() <= 0 {
		elements = nil
	} else {
		if !count.IsInt64() || count.Int64() > int64(^uint(0)>>1) {
			return nil, true, newException("OverflowError", "repeated sequence is too long")
		}
		original := append([]Value(nil), elements...)
		elements = make([]Value, 0, len(original)*int(count.Int64()))
		for range count.Int64() {
			elements = append(elements, original...)
		}
	}
	if list {
		return &listValue{elements: elements}, true, nil
	}
	return &tupleValue{elements: elements}, true, nil
}

func repeatText(frame *frame, index int, text string, count *big.Int) (instructionOutcome, error) {
	if count.Sign() <= 0 {
		return pushOutcome(frame, index, &stringValue{})
	}
	if !count.IsInt64() || count.Int64() > int64(^uint(0)>>1) {
		return instructionOutcome{
			kind:      raised,
			exception: newException("OverflowError", "repeated string is too long"),
		}, nil
	}
	return pushOutcome(frame, index, &stringValue{value: strings.Repeat(text, int(count.Int64()))})
}

// executeFloatBinary applies arithmetic where at least one numeric operand is
// a float while leaving bitwise and shifting operations to integer dispatch.
func executeFloatBinary(
	frame *frame,
	index int,
	operand uint32,
	left Value,
	right Value,
) (instructionOutcome, bool) {
	leftFloat, leftOK := numericFloat(left)
	rightFloat, rightOK := numericFloat(right)
	if !leftOK || !rightOK {
		return instructionOutcome{}, false
	}
	var result float64
	switch operand {
	case bytecode.BinaryAdd:
		result = leftFloat + rightFloat
	case bytecode.BinarySubtract:
		result = leftFloat - rightFloat
	case bytecode.BinaryMultiply:
		result = leftFloat * rightFloat
	case bytecode.BinaryFloorDivide:
		if rightFloat == 0 {
			return instructionOutcome{kind: raised, exception: newException(
				"ZeroDivisionError", "float floor division by zero",
			)}, true
		}
		result = math.Floor(leftFloat / rightFloat)
	case bytecode.BinaryModulo:
		if rightFloat == 0 {
			return instructionOutcome{kind: raised, exception: newException(
				"ZeroDivisionError", "float modulo",
			)}, true
		}
		result = math.Mod(leftFloat, rightFloat)
		if result != 0 && math.Signbit(result) != math.Signbit(rightFloat) {
			result += rightFloat
		}
	case bytecode.BinaryPower:
		if leftFloat == 0 && rightFloat < 0 {
			return instructionOutcome{kind: raised, exception: newException(
				"ZeroDivisionError", "0.0 cannot be raised to a negative power",
			)}, true
		}
		result = math.Pow(leftFloat, rightFloat)
	default:
		return instructionOutcome{}, false
	}
	outcome, err := pushOutcome(frame, index, &floatValue{value: result})
	if err != nil {
		return instructionOutcome{}, false
	}
	return outcome, true
}

// executeTrueDivision implements the numeric int/bool/float division path and
// always returns a float, matching Python's true-division result type.
func executeTrueDivision(
	frame *frame,
	index int,
	left Value,
	right Value,
	inPlace bool,
) (instructionOutcome, error) {
	leftInteger, leftIsInteger := integerOperand(left)
	rightInteger, rightIsInteger := integerOperand(right)
	if leftIsInteger && rightIsInteger {
		if rightInteger.Sign() == 0 {
			return instructionOutcome{
				kind:      raised,
				exception: newException("ZeroDivisionError", "division by zero"),
			}, nil
		}
		var ratio big.Rat
		ratio.SetFrac(&leftInteger, &rightInteger)
		result, _ := ratio.Float64()
		return pushOutcome(frame, index, &floatValue{value: result})
	}

	leftFloat, leftOK := numericFloat(left)
	rightFloat, rightOK := numericFloat(right)
	if !leftOK || !rightOK {
		if outcome, supported, err := executeBinarySpecialMethod(
			frame, index, bytecode.BinaryDivide, inPlace, left, right,
		); supported {
			return outcome, err
		}
		operator := "/"
		if inPlace {
			operator = "/="
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
	if rightFloat == 0 {
		return instructionOutcome{
			kind:      raised,
			exception: newException("ZeroDivisionError", "float division by zero"),
		}, nil
	}
	return pushOutcome(frame, index, &floatValue{value: leftFloat / rightFloat})
}

// integerOperand extracts arbitrary-precision values from integers and the
// supported integer-backed enum and mock representations.
func integerOperand(value Value) (big.Int, bool) {
	var integer big.Int
	switch value := value.(type) {
	case *intValue:
		integer.Set(&value.value)
		return integer, true
	case *instanceValue:
		if value.class.enumKind != "int" {
			return integer, false
		}
		underlying, found := value.attributes.get("_value_")
		if !found || underlying == value {
			return integer, false
		}
		return integerOperand(underlying)
	case *boolValue:
		if value.value {
			integer.SetInt64(1)
		}
		return integer, true
	default:
		return integer, false
	}
}
