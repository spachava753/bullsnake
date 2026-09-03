package runtime

import "strings"

type dictEntry struct {
	key   Value
	value Value
}

type dictValue struct {
	entries []dictEntry
	version uint64
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

// attribute exposes mapping views and mutable dictionary methods.
func (dictionary *dictValue) attribute(name string) (Value, bool) {
	var kind dictViewKind
	switch name {
	case "__contains__":
		return nativeFunctionNamed("dict.__contains__", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				_, found, exception := dictionary.get(arguments[0])
				if exception != nil {
					return nil, exception, nil
				}
				return pythonBool(found), nil, nil
			}), true
	case "__getitem__":
		return nativeFunctionNamed("dict.__getitem__", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				value, found, exception := dictionary.get(arguments[0])
				if exception != nil {
					return nil, exception, nil
				}
				if !found {
					return nil, newException("KeyError", arguments[0].Repr()), nil
				}
				return value, nil, nil
			}), true
	case "keys":
		kind = dictKeysView
	case "values":
		kind = dictValuesView
	case "items":
		kind = dictItemsView
	case "get":
		return nativeFunctionNamed("dict.get", 1, 2, dictionary.getMethod), true
	case "copy":
		return nativeFunctionNamed("dict.copy", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &dictValue{entries: append([]dictEntry(nil), dictionary.entries...)}, nil, nil
			}), true
	case "clear":
		return nativeFunctionNamed("dict.clear", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				dictionary.entries = nil
				dictionary.version++
				return None, nil, nil
			}), true
	case "pop":
		return nativeFunctionNamed("dict.pop", 1, 2, dictionary.popMethod), true
	case "setdefault":
		return nativeFunctionNamed("dict.setdefault", 1, 2, dictionary.setdefaultMethod), true
	case "update":
		return nativeKeywordAwareFunctionNamed("dict.update", 0, 1, dictionary.updateMethod), true
	default:
		return nil, false
	}
	return nativeFunctionNamed("dict."+name, 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &dictViewValue{dictionary: dictionary, kind: kind}, nil, nil
		}), true
}

func (dictionary *dictValue) getMethod(_ *frame, arguments []Value) (Value, *Exception, error) {
	value, found, exception := dictionary.get(arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	if found {
		return value, nil, nil
	}
	if len(arguments) == 2 {
		return arguments[1], nil, nil
	}
	return None, nil, nil
}

func (dictionary *dictValue) popMethod(_ *frame, arguments []Value) (Value, *Exception, error) {
	value, found, exception := dictionary.get(arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	if found {
		dictionary.delete(arguments[0])
		return value, nil, nil
	}
	if len(arguments) == 2 {
		return arguments[1], nil, nil
	}
	return nil, newException("KeyError", arguments[0].Repr()), nil
}

func (dictionary *dictValue) setdefaultMethod(_ *frame, arguments []Value) (Value, *Exception, error) {
	value, found, exception := dictionary.get(arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	if found {
		return value, nil, nil
	}
	value = None
	if len(arguments) == 2 {
		value = arguments[1]
	}
	if exception := dictionary.set(arguments[0], value); exception != nil {
		return nil, exception, nil
	}
	return value, nil, nil
}

// updateMethod merges a mapping or pair iterable followed by keyword entries.
func (dictionary *dictValue) updateMethod(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	if len(arguments) == 1 {
		source, exception, err := constructBuiltinDict(caller.runtime, arguments)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		for _, entry := range source.(*dictValue).entries {
			if exception := dictionary.set(entry.key, entry.value); exception != nil {
				return nil, exception, nil
			}
		}
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			if exception := dictionary.set(entry.key, entry.value); exception != nil {
				return nil, exception, nil
			}
		}
	}
	return None, nil, nil
}

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
		dictionary.version++
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
		*stringValue, *bytesValue, *ellipsisValue, *Exception, valueIterator,
		*builtinTypeValue, *typeValue, *exceptionTypeValue, *functionValue,
		*nativeFunctionValue, *boundMethodValue, *boundNativeMethodValue,
		*objectValue, *frozenSetValue, *weakReferenceValue, *instanceValue,
		*partialValue:
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

// executeMapSet consumes a key and value above the dictionary accumulator,
// leaving that accumulator in place for later display entries.
func executeMapSet(frame *frame, instruction int, depth int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	key, ok := frame.pop()
	if !ok || depth < 0 || depth >= len(frame.stack) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	dictionary, ok := frame.stack[len(frame.stack)-1-depth].(*dictValue)
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
	source, ok := dictionaryOperand(update)
	if namespace, namespaceOK := update.(*namespaceValue); namespaceOK {
		source, ok = namespace.dictionary(), true
	}
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
	source, ok := dictionaryOperand(update)
	if namespace, namespaceOK := update.(*namespaceValue); namespaceOK {
		source, ok = namespace.dictionary(), true
	}
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
	if namespace, ok := container.(*namespaceValue); ok {
		name, stringKey := key.(*stringValue)
		if !stringKey {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "namespace keys must be strings",
			)}, nil
		}
		namespace.namespace.values[name.value] = value
		return instructionOutcome{kind: advance}, nil
	}
	if list, ok := container.(*listValue); ok {
		if descriptor, isSlice := key.(*sliceValue); isSlice {
			start, stop, step, exception := normalizeSlice(descriptor, len(list.elements))
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			iterator, exception, err := newIteratorForFrame(frame, value)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			if iterator == nil {
				return instructionOutcome{kind: raised, exception: newException(
					"TypeError", "can only assign an iterable",
				)}, nil
			}
			replacement := make([]Value, 0)
			for {
				element, available, exception, err := nextNativeIterator(frame.runtime, iterator)
				if err != nil {
					return instructionOutcome{}, err
				}
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				if !available {
					break
				}
				replacement = append(replacement, element)
			}
			if step.IsInt64() && step.Int64() == 1 {
				updated := make([]Value, 0, len(list.elements)-(stop-start)+len(replacement))
				updated = append(updated, list.elements[:start]...)
				updated = append(updated, replacement...)
				updated = append(updated, list.elements[stop:]...)
				list.elements = updated
				return instructionOutcome{kind: advance}, nil
			}
			positions := make([]int, 0)
			for current := start; (step.Sign() > 0 && current < stop) || (step.Sign() < 0 && current > stop); current += int(step.Int64()) {
				positions = append(positions, current)
			}
			if len(positions) != len(replacement) {
				return instructionOutcome{kind: raised, exception: newException(
					"ValueError", "attempt to assign sequence of size to extended slice",
				)}, nil
			}
			for index, position := range positions {
				list.elements[position] = replacement[index]
			}
			return instructionOutcome{kind: advance}, nil
		}
		index, integer := integerOperand(key)
		if !integer || !index.IsInt64() {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "list indices must be integers",
			)}, nil
		}
		position := index.Int64()
		if position < 0 {
			position += int64(len(list.elements))
		}
		if position < 0 || position >= int64(len(list.elements)) {
			return instructionOutcome{kind: raised, exception: newException(
				"IndexError", "list assignment index out of range",
			)}, nil
		}
		list.elements[position] = value
		return instructionOutcome{kind: advance}, nil
	}
	if instance, ok := container.(*instanceValue); ok {
		if method, found, exception, err := lookupBoundSpecialMethod(frame, instance, "__setitem__"); found {
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			_, exception, err := callValueSynchronously(
				frame, method, []Value{key, value},
			)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return instructionOutcome{kind: advance}, nil
		}
		if instance.mapping != nil {
			if exception := instance.mapping.set(key, value); exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return instructionOutcome{kind: advance}, nil
		}
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
	if namespace, ok := container.(*namespaceValue); ok {
		name, stringKey := key.(*stringValue)
		if !stringKey {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "namespace keys must be strings",
			)}, nil
		}
		if _, found := namespace.namespace.values[name.value]; !found {
			return instructionOutcome{kind: raised, exception: newException("KeyError", name.Repr())}, nil
		}
		delete(namespace.namespace.values, name.value)
		return instructionOutcome{kind: advance}, nil
	}
	if instance, ok := container.(*instanceValue); ok {
		if method, found, exception, err := lookupBoundSpecialMethod(frame, instance, "__delitem__"); found {
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			_, exception, err := callValueSynchronously(
				frame, method, []Value{key},
			)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return instructionOutcome{kind: advance}, nil
		}
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
