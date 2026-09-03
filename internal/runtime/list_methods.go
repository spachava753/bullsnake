package runtime

import (
	"fmt"
	"slices"
)

// listMethod selects a bound mutable-sequence operation by attribute name.
func listMethod(list *listValue, name string) (Value, bool) {
	switch name {
	case "__iter__":
		return nativeFunctionNamed("list.__iter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &sequenceIterator{sequence: list}, nil, nil
			}), true
	case "__setitem__":
		return nativeFunctionNamed("list.__setitem__", 2, 2,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				if descriptor, ok := arguments[0].(*sliceValue); ok {
					start, stop, step, exception := normalizeSlice(descriptor, len(list.elements))
					if exception != nil {
						return nil, exception, nil
					}
					if !step.IsInt64() || step.Int64() != 1 {
						return nil, newException("NotImplementedError", "extended slice assignment is not supported"), nil
					}
					replacement, exception := iterableElements(arguments[1], "slice assignment")
					if exception != nil {
						return nil, exception, nil
					}
					updated := make([]Value, 0, len(list.elements)-(stop-start)+len(replacement))
					updated = append(updated, list.elements[:start]...)
					updated = append(updated, replacement...)
					updated = append(updated, list.elements[stop:]...)
					list.elements = updated
					return None, nil, nil
				}
				index, ok := integerOperand(arguments[0])
				if !ok || !index.IsInt64() {
					return nil, newException("TypeError", "list indices must be integers or slices"), nil
				}
				normalized := int(index.Int64())
				if normalized < 0 {
					normalized += len(list.elements)
				}
				if normalized < 0 || normalized >= len(list.elements) {
					return nil, newException("IndexError", "list assignment index out of range"), nil
				}
				list.elements[normalized] = arguments[1]
				return None, nil, nil
			}), true
	case "append":
		return nativeFunctionNamed("list.append", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				list.elements = append(list.elements, arguments[0])
				return None, nil, nil
			}), true
	case "extend":
		return nativeFunctionNamed("list.extend", 1, 1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				iterator, exception, err := newIteratorForFrame(caller, arguments[0])
				if err != nil || exception != nil {
					return nil, exception, err
				}
				if iterator == nil {
					return nil, newException("TypeError", "list.extend argument is not iterable"), nil
				}
				for {
					value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
					if err != nil || exception != nil {
						return nil, exception, err
					}
					if !available {
						return None, nil, nil
					}
					list.elements = append(list.elements, value)
				}
			}), true
	case "insert":
		return nativeFunctionNamed("list.insert", 2, 2, list.insert), true
	case "pop":
		return nativeFunctionNamed("list.pop", 0, 1, list.pop), true
	case "remove":
		return nativeFunctionNamed("list.remove", 1, 1, list.remove), true
	case "clear":
		return nativeFunctionNamed("list.clear", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				clear(list.elements)
				list.elements = nil
				return None, nil, nil
			}), true
	case "copy":
		return nativeFunctionNamed("list.copy", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &listValue{elements: append([]Value(nil), list.elements...)}, nil, nil
			}), true
	case "reverse":
		return nativeFunctionNamed("list.reverse", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				slices.Reverse(list.elements)
				return None, nil, nil
			}), true
	case "count":
		return nativeFunctionNamed("list.count", 1, 1, list.count), true
	case "index":
		return nativeFunctionNamed("list.index", 1, 3, list.index), true
	case "sort":
		return nativeKeywordAwareFunctionNamed("list.sort", 0, 0, list.sort), true
	default:
		return nil, false
	}
}

// sort parses Python's key and reverse options before stably sorting in place.
func (list *listValue) sort(
	caller *frame,
	_ []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	key := Value(None)
	reverse := false
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "list.sort() keywords must be strings"), nil
			}
			switch name.value {
			case "key":
				key = entry.value
			case "reverse":
				truth, exception, err := truthValueForFrame(caller, entry.value)
				if err != nil || exception != nil {
					return nil, exception, err
				}
				reverse = truth
			default:
				return nil, newException(
					"TypeError", "sort() got an unexpected keyword argument '"+name.value+"'",
				), nil
			}
		}
	}
	if exception, err := sortValues(caller, list.elements, key, reverse); err != nil || exception != nil {
		return nil, exception, err
	}
	return None, nil, nil
}

// sortValues performs a stable insertion sort so Python comparisons can raise
// exceptions without crossing a callback boundary in Go's sorting package.
func sortValues(
	caller *frame,
	elements []Value,
	key Value,
	reverse bool,
) (*Exception, error) {
	keys := slices.Clone(elements)
	if key != None {
		for index, element := range elements {
			value, exception, err := callValueSynchronously(caller, key, []Value{element})
			if err != nil || exception != nil {
				return exception, err
			}
			keys[index] = value
		}
	}
	for index := 1; index < len(elements); index++ {
		element, elementKey := elements[index], keys[index]
		position := index
		for position > 0 {
			left, right := elementKey, keys[position-1]
			if reverse {
				left, right = right, left
			}
			less, exception, err := lessThanValue(caller, left, right)
			if err != nil || exception != nil {
				return exception, err
			}
			if !less {
				break
			}
			elements[position] = elements[position-1]
			keys[position] = keys[position-1]
			position--
		}
		elements[position] = element
		keys[position] = elementKey
	}
	return nil, nil
}

// lessThanValue compares built-ins directly and dispatches user __lt__ methods.
func lessThanValue(caller *frame, left, right Value) (bool, *Exception, error) {
	if comparison, ordered, supported := orderedValues(left, right); supported {
		return ordered && comparison < 0, nil, nil
	}
	if instance, ok := left.(*instanceValue); ok {
		if method, found := instance.class.lookup("__lt__"); found {
			value, exception, err := callValueSynchronously(
				caller, bindCallable(method, instance), []Value{right},
			)
			if err != nil || exception != nil {
				return false, exception, err
			}
			return truthValueForFrame(caller, value)
		}
	}
	return false, newException(
		"TypeError",
		fmt.Sprintf("'<' not supported between instances of '%s' and '%s'", left.TypeName(), right.TypeName()),
	), nil
}

func (list *listValue) insert(_ *frame, arguments []Value) (Value, *Exception, error) {
	integer, ok := integerOperand(arguments[0])
	if !ok || !integer.IsInt64() {
		return nil, newException("TypeError", "list index must be integer"), nil
	}
	index := int(integer.Int64())
	if index < 0 {
		index = max(0, len(list.elements)+index)
	}
	index = min(index, len(list.elements))
	list.elements = append(list.elements, nil)
	copy(list.elements[index+1:], list.elements[index:])
	list.elements[index] = arguments[1]
	return None, nil, nil
}

// pop removes and returns a normalized list index, defaulting to the final item.
func (list *listValue) pop(_ *frame, arguments []Value) (Value, *Exception, error) {
	index := len(list.elements) - 1
	if len(arguments) == 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "list index must be integer"), nil
		}
		index = int(integer.Int64())
		if index < 0 {
			index += len(list.elements)
		}
	}
	if index < 0 || index >= len(list.elements) {
		return nil, newException("IndexError", "pop index out of range"), nil
	}
	value := list.elements[index]
	copy(list.elements[index:], list.elements[index+1:])
	list.elements[len(list.elements)-1] = nil
	list.elements = list.elements[:len(list.elements)-1]
	return value, nil, nil
}

func (list *listValue) remove(caller *frame, arguments []Value) (Value, *Exception, error) {
	for index, value := range list.elements {
		equal, exception, err := valuesEqualForFrame(caller, value, arguments[0])
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if equal {
			_, _, _ = list.pop(nil, []Value{newInt64(int64(index))})
			return None, nil, nil
		}
	}
	return nil, newException("ValueError", "list.remove(x): x not in list"), nil
}

func (list *listValue) count(caller *frame, arguments []Value) (Value, *Exception, error) {
	count := int64(0)
	for _, value := range list.elements {
		equal, exception, err := valuesEqualForFrame(caller, value, arguments[0])
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if equal {
			count++
		}
	}
	return newInt64(count), nil, nil
}

// index searches within normalized optional bounds and reports a missing value.
func (list *listValue) index(caller *frame, arguments []Value) (Value, *Exception, error) {
	start, end := 0, len(list.elements)
	if len(arguments) >= 2 {
		integer, ok := integerOperand(arguments[1])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "list index must be integer"), nil
		}
		start = max(0, int(integer.Int64()))
	}
	if len(arguments) == 3 {
		integer, ok := integerOperand(arguments[2])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "list index must be integer"), nil
		}
		end = min(end, int(integer.Int64()))
	}
	for index := start; index < end; index++ {
		equal, exception, err := valuesEqualForFrame(caller, list.elements[index], arguments[0])
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if equal {
			return newInt64(int64(index)), nil, nil
		}
	}
	return nil, newException("ValueError", "value is not in list"), nil
}
