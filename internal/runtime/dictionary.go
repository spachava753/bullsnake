package runtime

import "strings"

type dictEntry struct {
	key   Value
	value Value
}

type dictValue struct {
	namespace *Namespace
	entries   []dictEntry
	version   uint64
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

// set preserves existing key positions and synchronizes string names when the
// dictionary is a published module namespace.
func (dictionary *dictValue) set(key, value Value) *Exception {
	if exception := validateDictKey(key); exception != nil {
		return exception
	}
	if dictionary.namespace != nil {
		if name, ok := key.(*stringValue); ok {
			dictionary.namespace.values[name.value] = value
		}
	}
	for index := range dictionary.entries {
		entry := &dictionary.entries[index]
		if entry.key == key || valuesEqual(entry.key, key) {
			entry.value = value
			return nil
		}
	}
	dictionary.entries = append(dictionary.entries, dictEntry{key: key, value: value})
	dictionary.version++
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

// delete removes one equal key from ordered storage and any linked namespace,
// recording a key-set mutation only after a matching entry is found.
func (dictionary *dictValue) delete(key Value) (bool, *Exception) {
	if exception := validateDictKey(key); exception != nil {
		return false, exception
	}
	for index, entry := range dictionary.entries {
		if entry.key != key && !valuesEqual(entry.key, key) {
			continue
		}
		if dictionary.namespace != nil {
			if name, ok := entry.key.(*stringValue); ok {
				delete(dictionary.namespace.values, name.value)
			}
		}
		copy(dictionary.entries[index:], dictionary.entries[index+1:])
		last := len(dictionary.entries) - 1
		dictionary.entries[last] = dictEntry{}
		dictionary.entries = dictionary.entries[:last]
		dictionary.version++
		return true, nil
	}
	return false, nil
}

// executeMatchMapping retains one candidate and reports whether it is a
// concrete dictionary accepted by the current mapping-pattern subset.
func executeMatchMapping(frame *frame, instruction int) (instructionOutcome, error) {
	if len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	_, matched := frame.stack[len(frame.stack)-1].(*dictValue)
	result := falseSingleton
	if matched {
		result = trueSingleton
	}
	return pushOutcome(frame, instruction, result)
}

// executeMatchMappingKey turns a missing key into a false result instead of
// KeyError, while retaining unhashable-key failures as Python exceptions.
func executeMatchMappingKey(frame *frame, instruction int) (instructionOutcome, error) {
	key, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	candidate, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := candidate.(*dictValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"MATCH_MAPPING_KEY candidate is not a dictionary",
		)
	}
	value, found, exception := dictionary.get(key)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if !found {
		value = None
	}
	if !frame.push(value) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack overflow")
	}
	result := falseSingleton
	if found {
		result = trueSingleton
	}
	return pushOutcome(frame, instruction, result)
}

// executeCopyMapping replaces one dictionary with a shallow, independently
// mutable copy used while constructing a mapping pattern's rest capture.
func executeCopyMapping(frame *frame, instruction int) (instructionOutcome, error) {
	candidate, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := candidate.(*dictValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"COPY_MAPPING candidate is not a dictionary",
		)
	}
	entries := make([]dictEntry, len(dictionary.entries))
	copy(entries, dictionary.entries)
	return pushOutcome(frame, instruction, &dictValue{entries: entries})
}

// executeCheckMappingKey rejects keys that compare equal after their source
// expressions have each been evaluated exactly once.
func executeCheckMappingKey(frame *frame, instruction int) (instructionOutcome, error) {
	current, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	previous, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	if valuesEqual(previous, current) {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"ValueError",
				"mapping pattern checks duplicate key ("+current.Repr()+")",
			),
		}, nil
	}
	return instructionOutcome{kind: advance}, nil
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

// unhashableComponent accepts fixed hashable values and recursively finds the
// first invalid value inside tuple and frozenset keys.
func unhashableComponent(value Value) (string, bool) {
	switch value := value.(type) {
	case *classWeakReference:
		return "", false
	case *noneValue, *boolValue, *intValue, *floatValue, *complexValue,
		*stringValue, *bytesValue, *ellipsisValue, *Exception, valueIterator:
		return "", false
	case *tupleValue:
		for _, element := range value.elements {
			if typeName, found := unhashableComponent(element); found {
				return typeName, true
			}
		}
		return "", false
	case *frozenSetValue:
		for _, element := range value.entries {
			if typeName, found := unhashableComponent(element); found {
				return typeName, true
			}
		}
		return "", false
	default:
		return value.TypeName(), true
	}
}

// executeMapSet consumes a key and value above the dictionary accumulator,
// leaving that accumulator in place for later display entries.
func executeMapSet(frame *frame, instruction int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	key, ok := frame.pop()
	if !ok || len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := frame.stack[len(frame.stack)-1].(*dictValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"MAP_SET accumulator is not a dictionary",
		)
	}
	if exception := dictionary.set(key, value); exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return instructionOutcome{kind: advance}, nil
}

// executeMapUpdate keeps the target dictionary on the stack while replaying an
// unpacked dictionary's entries in insertion order.
func executeMapUpdate(frame *frame, instruction int) (instructionOutcome, error) {
	update, ok := frame.pop()
	if !ok || len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := frame.stack[len(frame.stack)-1].(*dictValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"MAP_UPDATE accumulator is not a dictionary",
		)
	}
	source, ok := update.(*dictValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+update.TypeName()+"' object is not a mapping",
			),
		}, nil
	}
	for _, entry := range source.entries {
		if exception := dictionary.set(entry.key, entry.value); exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
	}
	return instructionOutcome{kind: advance}, nil
}

// executeMapMerge combines one call keyword mapping into its accumulator while
// rejecting duplicate keys before CALL_EX receives the finished dictionary.
func executeMapMerge(frame *frame, instruction int) (instructionOutcome, error) {
	update, ok := frame.pop()
	if !ok || len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := frame.stack[len(frame.stack)-1].(*dictValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"MAP_MERGE accumulator is not a dictionary",
		)
	}
	name := callTargetName(frame)
	source, ok := update.(*dictValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				name+"() argument after ** must be a mapping, not "+update.TypeName(),
			),
		}, nil
	}
	for _, entry := range source.entries {
		_, found, exception := dictionary.get(entry.key)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if found {
			keyword := "'" + entry.key.Repr() + "'"
			if text, ok := entry.key.(*stringValue); ok {
				keyword = quoteString(text.value)
			}
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					name+"() got multiple values for keyword argument "+keyword,
				),
			}, nil
		}
		if exception := dictionary.set(entry.key, entry.value); exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
	}
	return instructionOutcome{kind: advance}, nil
}

func callTargetName(frame *frame) string {
	index := len(frame.stack) - 3
	if index >= 0 {
		if function, ok := frame.stack[index].(*functionValue); ok {
			return function.code.code.QualifiedName()
		}
	}
	return "function"
}

// executeStoreSubscript consumes value, container, and key in compiler stack
// order, then applies native list replacement or mapping insertion semantics.
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
	if buffer, ok := container.(*bytearrayValue); ok {
		if exception := buffer.assign(key, value, false); exception != nil {
			return raiseOutcome(exception), nil
		}
		return instructionOutcome{kind: advance}, nil
	}
	if list, ok := container.(*listValue); ok {
		if exception := list.assignIndex(key, value); exception != nil {
			return raiseOutcome(exception), nil
		}
		return instructionOutcome{kind: advance}, nil
	}
	dictionary, ok := container.(*dictValue)
	if !ok {
		if instance, userObject := container.(*instanceValue); userObject {
			return executeUserSubscription(
				frame,
				instruction,
				subscriptionSet,
				instance,
				[]Value{key, value},
			)
		}
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
	if buffer, ok := container.(*bytearrayValue); ok {
		if exception := buffer.assign(key, nil, true); exception != nil {
			return raiseOutcome(exception), nil
		}
		return instructionOutcome{kind: advance}, nil
	}
	dictionary, ok := container.(*dictValue)
	if !ok {
		if instance, userObject := container.(*instanceValue); userObject {
			return executeUserSubscription(
				frame,
				instruction,
				subscriptionDelete,
				instance,
				[]Value{key},
			)
		}
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
