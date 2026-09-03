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

func (value *listValue) attribute(name string) (Value, bool) {
	return listMethod(value, name)
}

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
		iterator, exception, err := newIteratorForFrame(frame, sequence)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if iterator == nil {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "cannot unpack non-iterable "+sequence.TypeName()+" object",
			)}, nil
		}
		for {
			value, available, exception, err := nextNativeIterator(frame.runtime, iterator)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			if !available {
				break
			}
			elements = append(elements, value)
		}
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

func executeListAppend(frame *frame, instruction int, depth int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok || depth < 0 || depth >= len(frame.stack) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	list, ok := frame.stack[len(frame.stack)-1-depth].(*listValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"LIST_APPEND accumulator is not a list",
		)
	}
	list.elements = append(list.elements, value)
	return instructionOutcome{kind: advance}, nil
}

// executeListExtend keeps the accumulator on the stack and drains the iterable
// above it in source order, including VM-backed generators.
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
	iterator, exception, err := newIteratorForFrame(frame, iterable)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if iterator == nil {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"Value after * must be an iterable, not "+iterable.TypeName(),
			),
		}, nil
	}
	for {
		value, available, exception, err := nextNativeIterator(frame.runtime, iterator)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !available {
			break
		}
		list.elements = append(list.elements, value)
	}
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
	if class, ok := container.(*builtinTypeValue); ok {
		return pushOutcome(frame, instruction, newGenericAlias(class, indexValue))
	}
	if class, ok := container.(*typeValue); ok {
		if method, found := class.lookup("__class_getitem__"); found {
			callable := bindCallable(method, class)
			if _, descriptor := method.(*descriptorValue); descriptor {
				callable = bindClassAttribute(method, class)
			}
			value, exception, err := callValueSynchronously(frame, callable, []Value{indexValue})
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return pushOutcome(frame, instruction, value)
		}
		return pushOutcome(frame, instruction, newGenericAlias(class, indexValue))
	}
	if namespace, ok := container.(*namespaceValue); ok {
		name, stringKey := indexValue.(*stringValue)
		if !stringKey {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "namespace keys must be strings",
			)}, nil
		}
		value, found := namespace.namespace.get(name.value)
		if !found {
			return instructionOutcome{kind: raised, exception: newException("KeyError", name.Repr())}, nil
		}
		return pushOutcome(frame, instruction, value)
	}
	var mappingOwner *instanceValue
	if instance, ok := container.(*instanceValue); ok {
		if method, found, exception, err := lookupBoundSpecialMethod(frame, instance, "__getitem__"); found {
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			value, exception, err := callValueSynchronously(
				frame, method, []Value{indexValue},
			)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return pushOutcome(frame, instruction, value)
		}
		switch {
		case instance.mapping != nil:
			mappingOwner = instance
			container = instance.mapping
		case instance.sequence != nil:
			container = instance.sequence
		case instance.tuple != nil:
			container = instance.tuple
		}
	}

	if dictionary, ok := container.(*dictValue); ok {
		value, found, exception := dictionary.get(indexValue)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !found {
			if mappingOwner != nil {
				if missing, exists := mappingOwner.class.lookup("__missing__"); exists {
					value, callException, err := callValueSynchronously(
						frame, bindCallable(missing, mappingOwner), []Value{indexValue},
					)
					if err != nil {
						return instructionOutcome{}, err
					}
					if callException != nil {
						return instructionOutcome{kind: raised, exception: callException}, nil
					}
					return pushOutcome(frame, instruction, value)
				}
			}
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
		iterator, exception, err := newIteratorForFrame(frame, sequence)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if iterator == nil {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "cannot unpack non-iterable "+sequence.TypeName()+" object",
			)}, nil
		}
		for {
			value, available, exception, err := nextNativeIterator(frame.runtime, iterator)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			if !available {
				break
			}
			elements = append(elements, value)
		}
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
