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
	if operand == bytecode.CompareEqual || operand == bytecode.CompareNotEqual {
		var result bool
		var exception *Exception
		var err error
		if operand == bytecode.CompareNotEqual {
			result, exception, err = valuesNotEqualForFrame(frame, left, right)
		} else {
			result, exception, err = valuesEqualForFrame(frame, left, right)
		}
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return pushOutcome(frame, index, pythonBool(result))
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
	case bytecode.CompareIn, bytecode.CompareNotIn:
		contained, exception, err := containsValueForFrame(frame, right, left)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		result = contained
		if operand == bytecode.CompareNotIn {
			result = !result
		}
	default:
		if result, supported := compareSetValues(operand, left, right); supported {
			return pushOutcome(frame, index, pythonBool(result))
		}
		comparison, ordered, supported := orderedValues(left, right)
		if !supported {
			result, handled, exception, err := comparisonSpecialMethod(
				frame, operand, left, right,
			)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			if handled {
				return pushOutcome(frame, index, pythonBool(result))
			}
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

// valuesNotEqualForFrame tries reflected Python __ne__ methods before falling
// back to the inverse of rich equality.
func valuesNotEqualForFrame(
	frame *frame,
	left Value,
	right Value,
) (bool, *Exception, error) {
	for _, candidate := range []struct {
		owner    Value
		argument Value
	}{{left, right}, {right, left}} {
		instance, ok := candidate.owner.(*instanceValue)
		if !ok {
			continue
		}
		method, found, exception, err := lookupBoundSpecialMethod(frame, instance, "__ne__")
		if err != nil || exception != nil {
			return false, exception, err
		}
		if !found {
			continue
		}
		if native, objectMethod := method.(*boundNativeMethodValue); objectMethod && native.function.name == "object.__ne__" {
			continue
		}
		value, exception, err := callValueSynchronously(frame, method, []Value{candidate.argument})
		if err != nil || exception != nil {
			return false, exception, err
		}
		if value != notImplementedSingleton {
			return truthValueForFrame(frame, value)
		}
	}
	equal, exception, err := valuesEqualForFrame(frame, left, right)
	return !equal, exception, err
}

// compareSetValues implements proper and non-strict subset ordering for sets.
func compareSetValues(operand uint32, left, right Value) (bool, bool) {
	setElements := func(value Value) ([]Value, bool) {
		switch value := value.(type) {
		case *setValue:
			return value.entries, true
		case *frozenSetValue:
			return value.entries, true
		default:
			return nil, false
		}
	}
	leftElements, leftOK := setElements(left)
	rightElements, rightOK := setElements(right)
	if !leftOK || !rightOK {
		return false, false
	}
	subset := func(candidate, container []Value) bool {
		for _, value := range candidate {
			if !containsElement(container, value) {
				return false
			}
		}
		return true
	}
	switch operand {
	case bytecode.CompareLess:
		return len(leftElements) < len(rightElements) && subset(leftElements, rightElements), true
	case bytecode.CompareLessEqual:
		return subset(leftElements, rightElements), true
	case bytecode.CompareGreater:
		return len(leftElements) > len(rightElements) && subset(rightElements, leftElements), true
	case bytecode.CompareGreaterEqual:
		return subset(rightElements, leftElements), true
	default:
		return false, false
	}
}

// comparisonSpecialMethod invokes direct or reflected user rich-comparison slots.
func comparisonSpecialMethod(
	frame *frame,
	operand uint32,
	left Value,
	right Value,
) (bool, bool, *Exception, error) {
	direct, reflected := "", ""
	switch operand {
	case bytecode.CompareLess:
		direct, reflected = "__lt__", "__gt__"
	case bytecode.CompareLessEqual:
		direct, reflected = "__le__", "__ge__"
	case bytecode.CompareGreater:
		direct, reflected = "__gt__", "__lt__"
	case bytecode.CompareGreaterEqual:
		direct, reflected = "__ge__", "__le__"
	default:
		return false, false, nil, nil
	}
	for _, candidate := range []struct {
		owner    Value
		argument Value
		name     string
	}{{left, right, direct}, {right, left, reflected}} {
		instance, ok := candidate.owner.(*instanceValue)
		if !ok {
			continue
		}
		method, found := instance.class.lookup(candidate.name)
		if !found {
			continue
		}
		value, exception, err := callValueSynchronously(
			frame, bindCallable(method, instance), []Value{candidate.argument},
		)
		if err != nil || exception != nil {
			return false, true, exception, err
		}
		truth, exception, err := truthValueForFrame(frame, value)
		return truth, true, exception, err
	}
	return false, false, nil, nil
}

// valuesEqualForFrame recursively compares sequences while allowing contained
// user instances to dispatch their Python __eq__ implementation.
func valuesEqualForFrame(
	frame *frame,
	left Value,
	right Value,
) (bool, *Exception, error) {
	if left == right {
		return true, nil, nil
	}
	// Python gives the reflected operation of a strict subtype priority. A
	// list subclass therefore compares its own elements first even when it is
	// the right operand; unittest.mock relies on this for ANY comparisons.
	if leftList, ok := left.(*listValue); ok {
		if rightInstance, subtype := right.(*instanceValue); subtype && rightInstance.sequence != nil {
			return equalElementsForFrame(frame, rightInstance.sequence.elements, leftList.elements)
		}
	}
	if instance, ok := left.(*instanceValue); ok {
		if method, found := instance.class.lookup("__eq__"); found {
			if !isDefaultContainerEquality(method) {
				if resolved, descriptor, exception, err := resolvePythonDescriptor(
					frame, method, instance, instance.class,
				); descriptor {
					if err != nil || exception != nil {
						return false, exception, err
					}
					method = resolved
				}
				value, exception, err := callValueSynchronously(
					frame, bindCallable(method, instance), []Value{right},
				)
				if err != nil || exception != nil {
					return false, exception, err
				}
				if value != notImplementedSingleton {
					return truthValueForFrame(frame, value)
				}
			}
		}
	}
	if instance, ok := right.(*instanceValue); ok {
		if method, found := instance.class.lookup("__eq__"); found {
			if !isDefaultContainerEquality(method) {
				if resolved, descriptor, exception, err := resolvePythonDescriptor(
					frame, method, instance, instance.class,
				); descriptor {
					if err != nil || exception != nil {
						return false, exception, err
					}
					method = resolved
				}
				value, exception, err := callValueSynchronously(
					frame, bindCallable(method, instance), []Value{left},
				)
				if err != nil || exception != nil {
					return false, exception, err
				}
				if value != notImplementedSingleton {
					return truthValueForFrame(frame, value)
				}
			}
		}
	}
	if instance, ok := left.(*instanceValue); ok {
		switch {
		case instance.sequence != nil:
			left = instance.sequence
		case instance.tuple != nil:
			left = instance.tuple
		case instance.mapping != nil:
			left = instance.mapping
		}
	}
	if instance, ok := right.(*instanceValue); ok {
		switch {
		case instance.sequence != nil:
			right = instance.sequence
		case instance.tuple != nil:
			right = instance.tuple
		case instance.mapping != nil:
			right = instance.mapping
		}
	}
	var leftElements, rightElements []Value
	switch left := left.(type) {
	case *listValue:
		leftElements = left.elements
		if rightList, ok := right.(*listValue); ok {
			rightElements = rightList.elements
		} else {
			return false, nil, nil
		}
	case *tupleValue:
		leftElements = left.elements
		if rightTuple, ok := right.(*tupleValue); ok {
			rightElements = rightTuple.elements
		} else {
			return false, nil, nil
		}
	case *dictValue:
		rightDictionary, ok := right.(*dictValue)
		if !ok || len(left.entries) != len(rightDictionary.entries) {
			return false, nil, nil
		}
		for _, entry := range left.entries {
			value, found, exception := rightDictionary.get(entry.key)
			if exception != nil || !found {
				return false, exception, nil
			}
			equal, exception, err := valuesEqualForFrame(frame, entry.value, value)
			if err != nil || exception != nil || !equal {
				return equal, exception, err
			}
		}
		return true, nil, nil
	default:
		return valuesEqual(left, right), nil, nil
	}
	return equalElementsForFrame(frame, leftElements, rightElements)
}

// equalElementsForFrame compares ordered elements through Python's rich
// equality protocol and stops at the first difference or exception.
func equalElementsForFrame(frame *frame, left, right []Value) (bool, *Exception, error) {
	if len(left) != len(right) {
		return false, nil, nil
	}
	for index := range left {
		equal, exception, err := valuesEqualForFrame(frame, left[index], right[index])
		if err != nil || exception != nil || !equal {
			return equal, exception, err
		}
	}
	return true, nil, nil
}

// isDefaultContainerEquality identifies inherited native equality slots that
// should run after collection-backed instances have been normalized. This
// preserves left-to-right element comparison for list and tuple subclasses.
func isDefaultContainerEquality(method Value) bool {
	native, ok := method.(*nativeFunctionValue)
	if !ok {
		return false
	}
	switch native.name {
	case "object.__eq__", "list.__eq__", "tuple.__eq__", "dict.__eq__":
		return true
	default:
		return false
	}
}

// valuesEqual implements equality for current scalar and tuple values. Numeric
// equality includes booleans and exact integer-to-binary64 comparison.
func valuesEqual(left, right Value) bool {
	if left == right {
		return true
	}
	if instance, ok := left.(*instanceValue); ok && instance.tuple != nil {
		left = instance.tuple
	}
	if instance, ok := right.(*instanceValue); ok && instance.tuple != nil {
		right = instance.tuple
	}
	if instance, ok := left.(*instanceValue); ok {
		if instance.sequence != nil {
			left = instance.sequence
		} else if instance.mapping != nil {
			left = instance.mapping
		}
	}
	if instance, ok := right.(*instanceValue); ok {
		if instance.sequence != nil {
			right = instance.sequence
		} else if instance.mapping != nil {
			right = instance.mapping
		}
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
		return ok && equalElements(left.elements, right.elements)
	case *dequeValue:
		right, ok := right.(*dequeValue)
		return ok && equalElements(left.elements, right.elements)
	case *dictValue:
		right, ok := right.(*dictValue)
		if !ok || len(left.entries) != len(right.entries) {
			return false
		}
		for _, entry := range left.entries {
			value, found, exception := right.get(entry.key)
			if exception != nil || !found || !valuesEqual(entry.value, value) {
				return false
			}
		}
		return true
	case *setValue:
		right, ok := right.(*setValue)
		return ok && equalUnorderedElements(left.entries, right.entries)
	case *frozenSetValue:
		right, ok := right.(*frozenSetValue)
		return ok && equalUnorderedElements(left.entries, right.entries)
	case *noneValue:
		_, ok := right.(*noneValue)
		return ok
	case *ellipsisValue:
		_, ok := right.(*ellipsisValue)
		return ok
	case *Exception:
		right, ok := right.(*Exception)
		return ok && left == right
	case *weakReferenceValue:
		right, ok := right.(*weakReferenceValue)
		return ok && valuesEqual(left.referent, right.referent)
	}
	return left == right
}

func equalElements(left, right []Value) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !valuesEqual(left[index], right[index]) {
			return false
		}
	}
	return true
}

func equalUnorderedElements(left, right []Value) bool {
	if len(left) != len(right) {
		return false
	}
	for _, value := range left {
		if !containsElement(right, value) {
			return false
		}
	}
	return true
}

func integerFloatEqual(integer *big.Int, value float64) bool {
	comparison, ordered := compareIntegerFloat(integer, value)
	return ordered && comparison == 0
}

// orderedValues compares supported numeric, string, and bytes pairs. Its
// ordered result is false for NaN without turning that case into a TypeError.
func orderedValues(left, right Value) (comparison int, ordered, supported bool) {
	if instance, ok := left.(*instanceValue); ok && instance.tuple != nil {
		left = instance.tuple
	}
	if instance, ok := right.(*instanceValue); ok && instance.tuple != nil {
		right = instance.tuple
	}
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
	var leftElements, rightElements []Value
	switch left := left.(type) {
	case *tupleValue:
		right, ok := right.(*tupleValue)
		if !ok {
			return 0, false, false
		}
		leftElements, rightElements = left.elements, right.elements
	case *listValue:
		right, ok := right.(*listValue)
		if !ok {
			return 0, false, false
		}
		leftElements, rightElements = left.elements, right.elements
	}
	if leftElements != nil {
		for index := 0; index < min(len(leftElements), len(rightElements)); index++ {
			if valuesEqual(leftElements[index], rightElements[index]) {
				continue
			}
			comparison, ordered, supported := orderedValues(leftElements[index], rightElements[index])
			if !supported || !ordered {
				return 0, ordered, supported
			}
			return comparison, true, true
		}
		switch {
		case len(leftElements) < len(rightElements):
			return -1, true, true
		case len(leftElements) > len(rightElements):
			return 1, true, true
		default:
			return 0, true, true
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
