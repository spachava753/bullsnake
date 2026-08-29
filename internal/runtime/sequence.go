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
