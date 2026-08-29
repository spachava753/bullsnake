package runtime

import "strings"

type dictEntry struct {
	key   Value
	value Value
}

type dictValue struct {
	entries []dictEntry
}

func (*dictValue) TypeName() string { return "dict" }
func (dictionary *dictValue) Repr() string {
	var builder strings.Builder
	builder.WriteByte('{')
	for index, entry := range dictionary.entries {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(entry.key.Repr())
		builder.WriteString(": ")
		builder.WriteString(entry.value.Repr())
	}
	builder.WriteByte('}')
	return builder.String()
}
func (*dictValue) isValue() {}

func (dictionary *dictValue) set(key, value Value) *Exception {
	if exception := validateDictKey(key); exception != nil {
		return exception
	}
	for index := range dictionary.entries {
		entry := &dictionary.entries[index]
		if entry.key == key || valuesEqual(entry.key, key) {
			entry.value = value
			return nil
		}
	}
	dictionary.entries = append(dictionary.entries, dictEntry{key: key, value: value})
	return nil
}

func (dictionary *dictValue) get(key Value) (Value, bool, *Exception) {
	if exception := validateDictKey(key); exception != nil {
		return nil, false, exception
	}
	for _, entry := range dictionary.entries {
		if entry.key == key || valuesEqual(entry.key, key) {
			return entry.value, true, nil
		}
	}
	return nil, false, nil
}

func (dictionary *dictValue) delete(key Value) (bool, *Exception) {
	if exception := validateDictKey(key); exception != nil {
		return false, exception
	}
	for index, entry := range dictionary.entries {
		if entry.key != key && !valuesEqual(entry.key, key) {
			continue
		}
		copy(dictionary.entries[index:], dictionary.entries[index+1:])
		last := len(dictionary.entries) - 1
		dictionary.entries[last] = dictEntry{}
		dictionary.entries = dictionary.entries[:last]
		return true, nil
	}
	return false, nil
}

func validateDictKey(key Value) *Exception {
	if unhashable, found := unhashableComponent(key); found {
		return newException(
			"TypeError",
			"cannot use '"+key.TypeName()+"' as a dict key (unhashable type: '"+
				unhashable+"')",
		)
	}
	return nil
}

func unhashableComponent(value Value) (string, bool) {
	switch value := value.(type) {
	case *noneValue, *boolValue, *intValue, *floatValue, *complexValue,
		*stringValue, *bytesValue, *ellipsisValue, *Exception, *sequenceIterator:
		return "", false
	case *tupleValue:
		for _, element := range value.elements {
			if typeName, found := unhashableComponent(element); found {
				return typeName, true
			}
		}
		return "", false
	default:
		return value.TypeName(), true
	}
}

// executeStoreSubscript consumes value, container, and key in compiler stack
// order, then applies mapping key validation and insertion semantics.
func executeStoreSubscript(frame *frame, instruction int) (instructionOutcome, error) {
	key, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	container, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := container.(*dictValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+container.TypeName()+"' object does not support item assignment",
			),
		}, nil
	}
	if exception := dictionary.set(key, value); exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return instructionOutcome{kind: advance}, nil
}

// executeDeleteSubscript consumes a mapping and key, validates hashability, and
// raises KeyError without changing entry order when no key matches.
func executeDeleteSubscript(frame *frame, instruction int) (instructionOutcome, error) {
	key, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	container, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := container.(*dictValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+container.TypeName()+"' object does not support item deletion",
			),
		}, nil
	}
	deleted, exception := dictionary.delete(key)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if !deleted {
		return instructionOutcome{
			kind:      raised,
			exception: newException("KeyError", key.Repr()),
		}, nil
	}
	return instructionOutcome{kind: advance}, nil
}

// executeBuildMap consumes key/value pairs in source order so replacement
// retains the first equal key's insertion position and identity.
func executeBuildMap(
	frame *frame,
	instruction int,
	count int,
) (instructionOutcome, error) {
	required := 2 * count
	if required > len(frame.stack) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	start := len(frame.stack) - required
	dictionary := &dictValue{entries: make([]dictEntry, 0, count)}
	for pair := 0; pair < count; pair++ {
		key := frame.stack[start+2*pair]
		value := frame.stack[start+2*pair+1]
		if exception := dictionary.set(key, value); exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
	}
	for value := start; value < len(frame.stack); value++ {
		frame.stack[value] = nil
	}
	frame.stack = frame.stack[:start]
	return pushOutcome(frame, instruction, dictionary)
}
