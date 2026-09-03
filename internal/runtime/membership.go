package runtime

import "strings"

// containsValue implements membership for current collections without exposing
// their temporary linear storage to comparison dispatch.
func containsValue(container, needle Value) (bool, *Exception) {
	switch container := container.(type) {
	case *tupleValue:
		return containsElement(container.elements, needle), nil
	case *listValue:
		return containsElement(container.elements, needle), nil
	case *dequeValue:
		return containsElement(container.elements, needle), nil
	case *dictValue:
		_, found, exception := container.get(needle)
		return found, exception
	case *namespaceValue:
		name, ok := needle.(*stringValue)
		if !ok {
			return false, nil
		}
		_, found := container.namespace.get(name.value)
		return found, nil
	case *setValue:
		return container.contains(needle)
	case *frozenSetValue:
		return (&setValue{entries: container.entries}).contains(needle)
	case *rangeValue:
		iterator := newRangeIterator(container)
		for {
			value, available, exception := iterator.next()
			if exception != nil || !available {
				return false, exception
			}
			if value == needle || valuesEqual(value, needle) {
				return true, nil
			}
		}
	case *instanceValue:
		switch {
		case container.mapping != nil:
			return containsValue(container.mapping, needle)
		case container.sequence != nil:
			return containsValue(container.sequence, needle)
		case container.tuple != nil:
			return containsValue(container.tuple, needle)
		}
		return false, newException(
			"TypeError", "argument of type '"+container.TypeName()+"' is not a container or iterable",
		)
	case *stringValue:
		text, ok := needle.(*stringValue)
		if !ok {
			return false, newException(
				"TypeError",
				"'in <string>' requires string as left operand, not "+needle.TypeName(),
			)
		}
		return strings.Contains(container.value, text.value), nil
	case *bytesValue:
		return bytesContains(container, needle)
	default:
		return false, newException(
			"TypeError",
			"argument of type '"+container.TypeName()+"' is not a container or iterable",
		)
	}
}

// containsValueForFrame honors the Python membership protocol before falling
// back to the runtime's concrete collection implementations. This is required
// for standard-library mappings such as weakref.WeakKeyDictionary.
func containsValueForFrame(
	caller *frame,
	container Value,
	needle Value,
) (bool, *Exception, error) {
	if instance, ok := container.(*instanceValue); ok {
		if method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__contains__"); err != nil || exception != nil {
			return false, exception, err
		} else if found {
			value, exception, err := callValueSynchronously(
				caller, method, []Value{needle},
			)
			if err != nil || exception != nil {
				return false, exception, err
			}
			return truthValueForFrame(caller, value)
		}
		if iterator, exception, err := newIteratorForFrame(caller, instance); err != nil || exception != nil {
			return false, exception, err
		} else if iterator != nil {
			for {
				value, available, nextException, nextErr := nextNativeIterator(caller.runtime, iterator)
				if nextErr != nil || nextException != nil || !available {
					return false, nextException, nextErr
				}
				equal, equalException, equalErr := valuesEqualForFrame(caller, value, needle)
				if equalErr != nil || equalException != nil || equal {
					return equal, equalException, equalErr
				}
			}
		}
		if getter, found := instance.class.lookup("__getitem__"); found {
			for index := int64(0); ; index++ {
				value, exception, err := callValueSynchronously(
					caller, bindCallable(getter, instance), []Value{newInt64(index)},
				)
				if err != nil {
					return false, nil, err
				}
				if exception != nil {
					if exception.class == indexErrorType {
						return false, nil, nil
					}
					return false, exception, nil
				}
				equal, equalException, equalErr := valuesEqualForFrame(caller, value, needle)
				if equalErr != nil || equalException != nil || equal {
					return equal, equalException, equalErr
				}
			}
		}
	}
	contained, exception := containsValue(container, needle)
	return contained, exception, nil
}

// bytesContains checks bytes subsequences before integer conversion so invalid or
// oversized integers follow CPython's bytes-like error path in the same order.
func bytesContains(container *bytesValue, needle Value) (bool, *Exception) {
	if data, ok := needle.(*bytesValue); ok {
		return strings.Contains(container.value, data.value), nil
	}
	integer, ok := integerOperand(needle)
	if !ok || !integer.IsInt64() {
		return false, bytesNeedleTypeError(needle)
	}
	raw := integer.Int64()
	if int64(int(raw)) != raw {
		return false, bytesNeedleTypeError(needle)
	}
	if raw < 0 || raw >= 256 {
		return false, newException("ValueError", "byte must be in range(0, 256)")
	}
	return strings.IndexByte(container.value, byte(raw)) >= 0, nil
}

func bytesNeedleTypeError(needle Value) *Exception {
	return newException(
		"TypeError",
		"a bytes-like object is required, not '"+needle.TypeName()+"'",
	)
}

func containsElement(elements []Value, needle Value) bool {
	for _, element := range elements {
		if element == needle || valuesEqual(element, needle) {
			return true
		}
	}
	return false
}
