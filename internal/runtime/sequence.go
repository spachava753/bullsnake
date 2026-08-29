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

// executeBinarySubscript implements integer indexing for the fixed sequence
// types while retaining Python's conversion, normalization, and error order.
func executeBinarySubscript(frame *frame, instruction int) (instructionOutcome, error) {
	indexValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	container, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
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

type sequenceIterator struct {
	sequence Value
	index    int
}

func (iterator *sequenceIterator) TypeName() string {
	switch iterator.sequence.(type) {
	case *tupleValue:
		return "tuple_iterator"
	case *listValue:
		return "list_iterator"
	default:
		return "iterator"
	}
}
func (iterator *sequenceIterator) Repr() string {
	return "<" + iterator.TypeName() + " object>"
}
func (*sequenceIterator) isValue() {}

func newSequenceIterator(value Value) (*sequenceIterator, bool) {
	switch value := value.(type) {
	case *tupleValue, *listValue:
		return &sequenceIterator{sequence: value}, true
	case *sequenceIterator:
		return value, true
	default:
		return nil, false
	}
}

func (iterator *sequenceIterator) next() (Value, bool) {
	var elements []Value
	switch sequence := iterator.sequence.(type) {
	case *tupleValue:
		elements = sequence.elements
	case *listValue:
		elements = sequence.elements
	default:
		return nil, false
	}
	if iterator.index >= len(elements) {
		return nil, false
	}
	value := elements[iterator.index]
	iterator.index++
	return value, true
}

func executeGetIter(frame *frame, index int) (instructionOutcome, error) {
	iterable, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	iterator, ok := newSequenceIterator(iterable)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+iterable.TypeName()+"' object is not iterable",
			),
		}, nil
	}
	return pushOutcome(frame, index, iterator)
}

// executeForIter retains the iterator while yielding and removes it before the
// exhaustion jump, matching the compiler's two stack-depth edges.
func executeForIter(
	frame *frame,
	index int,
	target int,
) (instructionOutcome, error) {
	if len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	value := frame.stack[len(frame.stack)-1]
	iterator, ok := value.(*sequenceIterator)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+value.TypeName()+"' object is not an iterator",
			),
		}, nil
	}
	next, ok := iterator.next()
	if !ok {
		frame.pop()
		frame.instruction = target
		return instructionOutcome{kind: advance}, nil
	}
	return pushOutcome(frame, index, next)
}
