package runtime

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
	default:
		return false, newException(
			"TypeError",
			"argument of type '"+container.TypeName()+"' is not a container or iterable",
		)
	}
}

func containsElement(elements []Value, needle Value) bool {
	for _, element := range elements {
		if element == needle || valuesEqual(element, needle) {
			return true
		}
	}
	return false
}
