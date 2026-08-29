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
	if unhashable, found := unhashableComponent(key); found {
		return newException(
			"TypeError",
			"cannot use '"+key.TypeName()+"' as a dict key (unhashable type: '"+
				unhashable+"')",
		)
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
