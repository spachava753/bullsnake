package runtime

import "strconv"

// builtinIsInstance validates the fixed call shape and turns the recursive class
// match into the canonical boolean singletons.
func builtinIsInstance(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if keywords != nil && len(keywords.entries) != 0 {
		return nil, newException("TypeError", "isinstance() takes no keyword arguments")
	}
	if len(arguments) != 2 {
		return nil, newException(
			"TypeError",
			"isinstance expected 2 arguments, got "+strconv.Itoa(len(arguments)),
		)
	}
	matched, exception := instanceMatchesClass(arguments[0], arguments[1])
	if exception != nil {
		return nil, exception
	}
	if matched {
		return trueSingleton, nil
	}
	return falseSingleton, nil
}

// instanceMatchesClass applies concrete native, user, and exception ancestry.
// Tuple candidates retain CPython's left-to-right short-circuit behavior.
func instanceMatchesClass(instance Value, candidate Value) (bool, *Exception) {
	if candidates, ok := candidate.(*tupleValue); ok {
		for _, item := range candidates.elements {
			matched, exception := instanceMatchesClass(instance, item)
			if matched || exception != nil {
				return matched, exception
			}
		}
		return false, nil
	}

	switch candidate := candidate.(type) {
	case *nativeTypeValue:
		actual, exception := typeOf(instance)
		if exception != nil {
			return false, exception
		}
		class, native := actual.(*nativeTypeValue)
		return native && class.isSubclassOf(candidate), nil
	case *typeValue:
		switch instance := instance.(type) {
		case *instanceValue:
			return instance.class.isSubclassOf(candidate), nil
		case *Exception:
			return instance.userClass != nil &&
				instance.userClass.isSubclassOf(candidate), nil
		default:
			return false, nil
		}
	case *exceptionTypeValue:
		exception, ok := instance.(*Exception)
		if !ok {
			return false, nil
		}
		if exception.userClass != nil {
			return exception.userClass.isSubclassOfBuiltinException(candidate), nil
		}
		return exception.class.isSubclassOf(candidate), nil
	default:
		return false, newException(
			"TypeError",
			"isinstance() arg 2 must be a type, a tuple of types, or a union",
		)
	}
}

func (class *nativeTypeValue) isSubclassOf(parent *nativeTypeValue) bool {
	for current := class; current != nil; current = current.base {
		if current == parent {
			return true
		}
	}
	return false
}
