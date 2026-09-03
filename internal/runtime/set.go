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

// attribute exposes mutable set operations used by standard-library initialization.
func (set *setValue) attribute(name string) (Value, bool) {
	switch name {
	case "__contains__":
		return nativeFunctionNamed("set.__contains__", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				contains, exception := set.contains(arguments[0])
				if exception != nil {
					return nil, exception, nil
				}
				return pythonBool(contains), nil, nil
			}), true
	case "add":
		return nativeFunctionNamed("set.add", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				if exception := set.add(arguments[0]); exception != nil {
					return nil, exception, nil
				}
				return None, nil, nil
			}), true
	case "discard":
		return nativeFunctionNamed("set.discard", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				for index, entry := range set.entries {
					if entry == arguments[0] || valuesEqual(entry, arguments[0]) {
						set.entries = append(set.entries[:index], set.entries[index+1:]...)
						break
					}
				}
				return None, nil, nil
			}), true
	case "difference":
		return nativeFunctionNamed("set.difference", 0, -1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				result := &setValue{entries: append([]Value(nil), set.entries...)}
				for _, argument := range arguments {
					iterator, exception, err := newIteratorForFrame(caller, argument)
					if err != nil || exception != nil {
						return nil, exception, err
					}
					if iterator == nil {
						return nil, newException("TypeError", "set difference argument is not iterable"), nil
					}
					for {
						value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
						if err != nil || exception != nil {
							return nil, exception, err
						}
						if !available {
							break
						}
						for index, entry := range result.entries {
							if valuesEqual(entry, value) {
								result.entries = append(result.entries[:index], result.entries[index+1:]...)
								break
							}
						}
					}
				}
				return result, nil, nil
			}), true
	case "intersection":
		return nativeFunctionNamed("set.intersection", 0, -1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				result := &setValue{entries: append([]Value(nil), set.entries...)}
				for _, argument := range arguments {
					iterator, exception, err := newIteratorForFrame(caller, argument)
					if err != nil || exception != nil {
						return nil, exception, err
					}
					if iterator == nil {
						return nil, newException("TypeError", "set intersection argument is not iterable"), nil
					}
					var candidates []Value
					for {
						value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
						if err != nil || exception != nil {
							return nil, exception, err
						}
						if !available {
							break
						}
						candidates = append(candidates, value)
					}
					filtered := result.entries[:0]
					for _, entry := range result.entries {
						if containsElement(candidates, entry) {
							filtered = append(filtered, entry)
						}
					}
					result.entries = filtered
				}
				return result, nil, nil
			}), true
	case "union":
		return nativeFunctionNamed("set.union", 0, -1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				result := &setValue{entries: append([]Value(nil), set.entries...)}
				for _, argument := range arguments {
					iterator, exception, err := newIteratorForFrame(caller, argument)
					if err != nil || exception != nil {
						return nil, exception, err
					}
					if iterator == nil {
						return nil, newException("TypeError", "set union argument is not iterable"), nil
					}
					for {
						value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
						if err != nil || exception != nil {
							return nil, exception, err
						}
						if !available {
							break
						}
						if exception := result.add(value); exception != nil {
							return nil, exception, nil
						}
					}
				}
				return result, nil, nil
			}), true
	default:
		return nil, false
	}
}

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

// executeSetAdd consumes a value above active iterators and mutates the set
// accumulator at the compiler-recorded stack depth.
func executeSetAdd(frame *frame, instruction int, depth int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok || depth < 0 || depth >= len(frame.stack) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	set, ok := frame.stack[len(frame.stack)-1-depth].(*setValue)
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

// executeSetUpdate keeps the set accumulator on the stack while draining and
// adding each element from an arbitrary iterable.
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
	iterator, iterableOK := newIterator(iterable)
	if !iterableOK {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+iterable.TypeName()+"' object is not iterable",
			),
		}, nil
	}
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
		if exception := set.add(element); exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
	}
	return instructionOutcome{kind: advance}, nil
}
