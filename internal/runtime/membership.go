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
	case *dictValue:
		_, found, exception := container.get(needle)
		return found, exception
	case *setValue:
		return container.contains(needle)
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
