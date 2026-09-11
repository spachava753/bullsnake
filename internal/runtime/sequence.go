package runtime

import (
	"fmt"
	"strings"
)

type tupleValue struct {
	elements []Value
}

func (*tupleValue) TypeName() string { return "tuple" }
func (value *tupleValue) Repr() string {
	if len(value.elements) == 0 {
		return "()"
	}
	var builder strings.Builder
	builder.WriteByte('(')
	for index, element := range value.elements {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(element.Repr())
	}
	if len(value.elements) == 1 {
		builder.WriteByte(',')
	}
	builder.WriteByte(')')
	return builder.String()
}
func (*tupleValue) isValue() {}

type listValue struct {
	elements []Value
}

func (*listValue) TypeName() string { return "list" }
func (value *listValue) Repr() string {
	var builder strings.Builder
	builder.WriteByte('[')
	for index, element := range value.elements {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(element.Repr())
	}
	builder.WriteByte(']')
	return builder.String()
}
func (*listValue) isValue() {}

func executeBuildSequence(
	frame *frame,
	index int,
	count int,
	tuple bool,
) (instructionOutcome, error) {
	if count < 0 || count > len(frame.stack) {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	start := len(frame.stack) - count
	elements := make([]Value, count)
	copy(elements, frame.stack[start:])
	for element := start; element < len(frame.stack); element++ {
		frame.stack[element] = nil
	}
	frame.stack = frame.stack[:start]
	if tuple {
		return pushOutcome(frame, index, &tupleValue{elements: elements})
	}
	return pushOutcome(frame, index, &listValue{elements: elements})
}

// executeMatchSequence retains one candidate and reports whether it has the
// tuple or list representation currently accepted by sequence patterns.
func executeMatchSequence(frame *frame, index int) (instructionOutcome, error) {
	if len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	candidate := frame.stack[len(frame.stack)-1]
	_, tuple := candidate.(*tupleValue)
	_, list := candidate.(*listValue)
	result := falseSingleton
	if tuple || list {
		result = trueSingleton
	}
	return pushOutcome(frame, index, result)
}

// executeGetLen retains one matched sequence and pushes its concrete length.
func executeGetLen(frame *frame, index int) (instructionOutcome, error) {
	if len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	candidate := frame.stack[len(frame.stack)-1]
	var length int
	switch candidate := candidate.(type) {
	case *tupleValue:
		length = len(candidate.elements)
	case *listValue:
		length = len(candidate.elements)
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"object of type '"+candidate.TypeName()+"' has no len()",
			),
		}, nil
	}
	return pushOutcome(frame, index, integerFromInt64(int64(length)))
}

// executeUnpackSequence accepts the fixed sequence types, checks exact arity,
// and pushes elements in reverse so target stores consume them left to right.
func executeUnpackSequence(
	frame *frame,
	index int,
	count int,
) (instructionOutcome, error) {
	sequence, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	var elements []Value
	switch sequence := sequence.(type) {
	case *tupleValue:
		elements = sequence.elements
	case *listValue:
		elements = sequence.elements
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"cannot unpack non-iterable "+sequence.TypeName()+" object",
			),
		}, nil
	}
	if len(elements) < count {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"ValueError",
				fmt.Sprintf(
					"not enough values to unpack (expected %d, got %d)",
					count,
					len(elements),
				),
			),
		}, nil
	}
	if len(elements) > count {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"ValueError",
				fmt.Sprintf("too many values to unpack (expected %d)", count),
			),
		}, nil
	}
	for element := len(elements) - 1; element >= 0; element-- {
		if !frame.push(elements[element]) {
			return instructionOutcome{}, frame.failure(index, "operand stack overflow")
		}
	}
	return instructionOutcome{kind: advance}, nil
}

func executeListAppend(frame *frame, instruction int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok || len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	list, ok := frame.stack[len(frame.stack)-1].(*listValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"LIST_APPEND accumulator is not a list",
		)
	}
	list.elements = append(list.elements, value)
	return instructionOutcome{kind: advance}, nil
}

// executeListExtend keeps the accumulator on the stack and appends elements
// from the tuple/list iterable above it in source order.
func executeListExtend(frame *frame, instruction int) (instructionOutcome, error) {
	iterable, ok := frame.pop()
	if !ok || len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	list, ok := frame.stack[len(frame.stack)-1].(*listValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"LIST_EXTEND accumulator is not a list",
		)
	}
	var elements []Value
	switch iterable := iterable.(type) {
	case *tupleValue:
		elements = iterable.elements
	case *listValue:
		elements = iterable.elements
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"Value after * must be an iterable, not "+iterable.TypeName(),
			),
		}, nil
	}
	list.elements = append(list.elements, elements...)
	return instructionOutcome{kind: advance}, nil
}

func executeListToTuple(frame *frame, instruction int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	list, ok := value.(*listValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"LIST_TO_TUPLE value is not a list",
		)
	}
	elements := make([]Value, len(list.elements))
	copy(elements, list.elements)
	return pushOutcome(frame, instruction, &tupleValue{elements: elements})
}

// executeBinarySubscript dispatches dictionary lookup or fixed-sequence integer
// and slice subscription while preserving each container's error order.
func executeBinarySubscript(frame *frame, instruction int) (instructionOutcome, error) {
	indexValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	container, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}

	if class, ok := container.(*nativeTypeValue); ok && (class == typeNativeType || hasNativeClassGetitem(class)) {
		return pushOutcome(frame, instruction, newGenericAlias(frame.runtime.genericAliasClass, class, indexValue))
	}
	if class, ok := container.(*typeValue); ok {
		return executeClassSubscription(frame, instruction, class, indexValue)
	}
	if buffer, ok := container.(*bytearrayValue); ok {
		value, exception := buffer.subscript(indexValue)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(frame, instruction, value)
	}
	if proxy, ok := container.(*frameLocalsProxy); ok {
		value, found, exception := proxy.get(indexValue)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if !found {
			return raiseOutcome(newException("KeyError", indexValue.Repr())), nil
		}
		return pushOutcome(frame, instruction, value)
	}
	if proxy, ok := container.(*mappingProxyValue); ok {
		container = proxy.dictionary
	}
	if dictionary, ok := container.(*dictValue); ok {
		value, found, exception := dictionary.get(indexValue)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !found {
			return instructionOutcome{
				kind:      raised,
				exception: newException("KeyError", indexValue.Repr()),
			}, nil
		}
		return pushOutcome(frame, instruction, value)
	}
	if _, ok := container.(*stringValue); ok {
		value, exception := executeTextSubscript(container, indexValue)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return pushOutcome(frame, instruction, value)
	}
	if _, ok := container.(*bytesValue); ok {
		value, exception := executeTextSubscript(container, indexValue)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return pushOutcome(frame, instruction, value)
	}
	if instance, ok := container.(*instanceValue); ok {
		return executeUserSubscription(
			frame,
			instruction,
			subscriptionGet,
			instance,
			[]Value{indexValue},
		)
	}

	var elements []Value
	var sequenceName string
	switch container := container.(type) {
	case *tupleValue:
		elements = container.elements
		sequenceName = "tuple"
	case *listValue:
		elements = container.elements
		sequenceName = "list"
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+container.TypeName()+"' object is not subscriptable",
			),
		}, nil
	}

	if descriptor, ok := indexValue.(*sliceValue); ok {
		value, exception := sliceSequence(container, elements, descriptor)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return pushOutcome(frame, instruction, value)
	}

	index, ok := integerOperand(indexValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				fmt.Sprintf(
					"%s indices must be integers or slices, not %s",
					sequenceName,
					indexValue.TypeName(),
				),
			),
		}, nil
	}
	if !index.IsInt64() {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"IndexError",
				"cannot fit '"+indexValue.TypeName()+"' into an index-sized integer",
			),
		}, nil
	}
	rawIndex := index.Int64()
	normalized := int(rawIndex)
	if int64(normalized) != rawIndex {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"IndexError",
				"cannot fit '"+indexValue.TypeName()+"' into an index-sized integer",
			),
		}, nil
	}
	if normalized < 0 {
		normalized += len(elements)
	}
	if normalized < 0 || normalized >= len(elements) {
		return instructionOutcome{
			kind:      raised,
			exception: newException("IndexError", sequenceName+" index out of range"),
		}, nil
	}
	return pushOutcome(frame, instruction, elements[normalized])
}

// executeUnpackEx splits a tuple/list around one starred target and pushes
// trailing, middle, and leading values so stores consume targets left to right.
func executeUnpackEx(
	frame *frame,
	index int,
	before int,
	after int,
) (instructionOutcome, error) {
	sequence, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	var elements []Value
	switch sequence := sequence.(type) {
	case *tupleValue:
		elements = sequence.elements
	case *listValue:
		elements = sequence.elements
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"cannot unpack non-iterable "+sequence.TypeName()+" object",
			),
		}, nil
	}
	minimum := before + after
	if len(elements) < minimum {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"ValueError",
				fmt.Sprintf(
					"not enough values to unpack (expected at least %d, got %d)",
					minimum,
					len(elements),
				),
			),
		}, nil
	}

	middleEnd := len(elements) - after
	middle := make([]Value, middleEnd-before)
	copy(middle, elements[before:middleEnd])
	for element := len(elements) - 1; element >= middleEnd; element-- {
		if !frame.push(elements[element]) {
			return instructionOutcome{}, frame.failure(index, "operand stack overflow")
		}
	}
	if !frame.push(&listValue{elements: middle}) {
		return instructionOutcome{}, frame.failure(index, "operand stack overflow")
	}
	for element := before - 1; element >= 0; element-- {
		if !frame.push(elements[element]) {
			return instructionOutcome{}, frame.failure(index, "operand stack overflow")
		}
	}
	return instructionOutcome{kind: advance}, nil
}
