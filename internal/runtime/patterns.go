package runtime

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// executeMatchSequence rejects non-sequences and length mismatches, otherwise
// returning fixed fields with an optional starred remainder list.
func executeMatchSequence(
	frame *frame,
	instruction int,
	operand uint32,
) (instructionOutcome, error) {
	subject, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	var elements []Value
	switch subject := subject.(type) {
	case *tupleValue:
		elements = subject.elements
	case *listValue:
		elements = subject.elements
	default:
		return pushOutcome(frame, instruction, None)
	}
	count, starIndex := bytecode.MatchSequenceCounts(operand)
	if starIndex < 0 {
		if len(elements) != count {
			return pushOutcome(frame, instruction, None)
		}
		return pushOutcome(frame, instruction, &tupleValue{elements: append([]Value(nil), elements...)})
	}
	minimum := count - 1
	if len(elements) < minimum {
		return pushOutcome(frame, instruction, None)
	}
	result := make([]Value, count)
	copy(result[:starIndex], elements[:starIndex])
	after := count - starIndex - 1
	middleEnd := len(elements) - after
	result[starIndex] = &listValue{elements: append([]Value(nil), elements[starIndex:middleEnd]...)}
	copy(result[starIndex+1:], elements[middleEnd:])
	return pushOutcome(frame, instruction, &tupleValue{elements: result})
}

// executeMatchMapping finds each requested dictionary key, detects duplicates,
// and optionally returns a dictionary containing unmatched entries.
func executeMatchMapping(
	frame *frame,
	instruction int,
	includeRest bool,
) (instructionOutcome, error) {
	keysValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	subjectValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	keys, keysOK := keysValue.(*tupleValue)
	subject, subjectOK := subjectValue.(*dictValue)
	if !keysOK {
		return instructionOutcome{}, frame.failure(instruction, "MATCH_MAPPING keys are not a tuple")
	}
	if !subjectOK {
		return pushOutcome(frame, instruction, None)
	}
	result := make([]Value, 0, len(keys.elements)+1)
	for keyIndex, key := range keys.elements {
		for earlier := 0; earlier < keyIndex; earlier++ {
			if valuesEqual(keys.elements[earlier], key) {
				return instructionOutcome{
					kind:      raised,
					exception: newException("ValueError", "mapping pattern checks duplicate key "+key.Repr()),
				}, nil
			}
		}
		value, found, exception := subject.get(key)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !found {
			return pushOutcome(frame, instruction, None)
		}
		result = append(result, value)
	}
	if includeRest {
		rest := &dictValue{}
		for _, entry := range subject.entries {
			matched := false
			for _, key := range keys.elements {
				if entry.key == key || valuesEqual(entry.key, key) {
					matched = true
					break
				}
			}
			if !matched {
				rest.entries = append(rest.entries, entry)
			}
		}
		result = append(result, rest)
	}
	return pushOutcome(frame, instruction, &tupleValue{elements: result})
}

// executeMatchClass validates the class, expands positional and keyword
// attributes, and uses None as the internal no-match result.
func executeMatchClass(
	frame *frame,
	instruction int,
	positionalCount int,
) (instructionOutcome, error) {
	keywordsValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	class, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	subject, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	keywords, ok := keywordsValue.(*tupleValue)
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "MATCH_CLASS keywords are not a tuple")
	}
	matches, exception := valueIsInstance(subject, class)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if !matches {
		return pushOutcome(frame, instruction, None)
	}
	positionalNames, exception := matchClassPositionalNames(class, positionalCount)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	names := append(positionalNames, make([]string, len(keywords.elements))...)
	for index, keyword := range keywords.elements {
		name, ok := keyword.(*stringValue)
		if !ok {
			return instructionOutcome{}, frame.failure(instruction, "MATCH_CLASS keyword is not a string")
		}
		names[len(positionalNames)+index] = name.value
	}
	result := make([]Value, len(names))
	for index, name := range names {
		for earlier := 0; earlier < index; earlier++ {
			if names[earlier] == name {
				return instructionOutcome{
					kind:      raised,
					exception: newException("TypeError", "multiple sub-patterns for attribute '"+name+"'"),
				}, nil
			}
		}
		value, found := matchAttribute(subject, name)
		if !found {
			return pushOutcome(frame, instruction, None)
		}
		result[index] = value
	}
	return pushOutcome(frame, instruction, &tupleValue{elements: result})
}

// matchClassPositionalNames implements built-in self matching and validates a
// user class's __match_args__ tuple for positional subpatterns.
func matchClassPositionalNames(class Value, count int) ([]string, *Exception) {
	if count == 0 {
		return nil, nil
	}
	if builtin, ok := class.(*builtinTypeValue); ok {
		if count == 1 && builtinMatchesSelf(builtin.name) {
			return []string{"__match_self__"}, nil
		}
		return nil, newException("TypeError", fmt.Sprintf("%s() accepts 0 positional sub-patterns (%d given)", builtin.name, count))
	}
	userClass, ok := class.(*typeValue)
	if !ok {
		return nil, newException("TypeError", "called match pattern must be a type")
	}
	matchArgsValue, found := userClass.lookup("__match_args__")
	matchArgs, tupleOK := matchArgsValue.(*tupleValue)
	if !found || !tupleOK {
		return nil, newException("TypeError", fmt.Sprintf("%s() accepts 0 positional sub-patterns (%d given)", userClass.name, count))
	}
	if count > len(matchArgs.elements) {
		return nil, newException("TypeError", fmt.Sprintf("%s() accepts %d positional sub-patterns (%d given)", userClass.name, len(matchArgs.elements), count))
	}
	names := make([]string, count)
	for index := range count {
		name, ok := matchArgs.elements[index].(*stringValue)
		if !ok {
			return nil, newException("TypeError", "__match_args__ elements must be strings")
		}
		names[index] = name.value
	}
	return names, nil
}

func builtinMatchesSelf(name string) bool {
	switch name {
	case "bool", "bytes", "dict", "float", "int", "list", "set", "str", "tuple":
		return true
	default:
		return false
	}
}

// matchAttribute performs non-raising pattern lookup while preserving ordinary
// method binding for instance attributes.
func matchAttribute(subject Value, name string) (Value, bool) {
	if name == "__match_self__" {
		return subject, true
	}
	switch subject := subject.(type) {
	case attributeValue:
		return subject.attribute(name)
	case *Module:
		return subject.globals.get(name)
	case *typeValue:
		return subject.lookup(name)
	case *instanceValue:
		if value, found := subject.attributes.get(name); found {
			return value, true
		}
		value, found := subject.class.lookup(name)
		if !found {
			return nil, false
		}
		switch value := value.(type) {
		case *functionValue:
			return &boundMethodValue{function: value, self: subject}, true
		case *nativeFunctionValue:
			return &boundNativeMethodValue{function: value, self: subject}, true
		default:
			return value, true
		}
	default:
		return nil, false
	}
}
