package runtime

import "strings"

type setValue struct {
	entries []Value
}

func (*setValue) TypeName() string { return "set" }
func (set *setValue) Repr() string {
	if len(set.entries) == 0 {
		return "set()"
	}
	var builder strings.Builder
	builder.WriteByte('{')
	for index, entry := range set.entries {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(entry.Repr())
	}
	builder.WriteByte('}')
	return builder.String()
}
func (*setValue) isValue() {}

func (set *setValue) add(value Value) *Exception {
	if exception := validateSetElement(value); exception != nil {
		return exception
	}
	for _, entry := range set.entries {
		if entry == value || valuesEqual(entry, value) {
			return nil
		}
	}
	set.entries = append(set.entries, value)
	return nil
}

func (set *setValue) contains(value Value) (bool, *Exception) {
	if exception := validateSetElement(value); exception != nil {
		return false, exception
	}
	for _, entry := range set.entries {
		if entry == value || valuesEqual(entry, value) {
			return true, nil
		}
	}
	return false, nil
}

func validateSetElement(value Value) *Exception {
	if unhashable, found := unhashableComponent(value); found {
		return newException(
			"TypeError",
			"cannot use '"+value.TypeName()+"' as a set element (unhashable type: '"+
				unhashable+"')",
		)
	}
	return nil
}

// executeBuildSet consumes display elements in source order and retains the
// first object from each identity-or-equality group.
func executeBuildSet(
	frame *frame,
	instruction int,
	count int,
) (instructionOutcome, error) {
	if count > len(frame.stack) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	start := len(frame.stack) - count
	set := &setValue{entries: make([]Value, 0, count)}
	for element := start; element < len(frame.stack); element++ {
		if exception := set.add(frame.stack[element]); exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
	}
	for element := start; element < len(frame.stack); element++ {
		frame.stack[element] = nil
	}
	frame.stack = frame.stack[:start]
	return pushOutcome(frame, instruction, set)
}

func executeSetAdd(frame *frame, instruction int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok || len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	set, ok := frame.stack[len(frame.stack)-1].(*setValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"SET_ADD accumulator is not a set",
		)
	}
	if exception := set.add(value); exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return instructionOutcome{kind: advance}, nil
}

// executeSetUpdate keeps the set accumulator on the stack while adding each
// element from a current tuple, list, or set iterable.
func executeSetUpdate(frame *frame, instruction int) (instructionOutcome, error) {
	iterable, ok := frame.pop()
	if !ok || len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	set, ok := frame.stack[len(frame.stack)-1].(*setValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"SET_UPDATE accumulator is not a set",
		)
	}
	var elements []Value
	switch iterable := iterable.(type) {
	case *tupleValue:
		elements = iterable.elements
	case *listValue:
		elements = iterable.elements
	case *setValue:
		elements = iterable.entries
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+iterable.TypeName()+"' object is not iterable",
			),
		}, nil
	}
	for _, element := range elements {
		if exception := set.add(element); exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
	}
	return instructionOutcome{kind: advance}, nil
}
