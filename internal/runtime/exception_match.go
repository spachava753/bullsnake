package runtime

const cannotCatchMessage = "catching classes that do not inherit from BaseException is not allowed"

// matchException validates every candidate before testing class ancestry so a
// later invalid tuple member fails even when an earlier member would match.
func matchException(exception *Exception, handlerType Value) (bool, *Exception) {
	if !validExceptionHandlerType(handlerType) {
		return false, newException("TypeError", cannotCatchMessage)
	}
	if tuple, ok := handlerType.(*tupleValue); ok {
		for _, item := range tuple.elements {
			if exceptionMatchesClass(exception, item) {
				return true, nil
			}
		}
		return false, nil
	}
	return exceptionMatchesClass(exception, handlerType), nil
}

func exceptionMatchesClass(exception *Exception, handlerType Value) bool {
	switch handlerType := handlerType.(type) {
	case *exceptionTypeValue:
		if exception.userClass != nil {
			return exception.userClass.builtinExceptionBase().isSubclassOf(handlerType)
		}
		return exception.class.isSubclassOf(handlerType)
	case *typeValue:
		return exception.userClass != nil && exception.userClass.isSubclassOf(handlerType)
	default:
		return false
	}
}

func validExceptionClass(value Value) bool {
	switch value := value.(type) {
	case *exceptionTypeValue:
		return true
	case *typeValue:
		return value.isExceptionClass()
	default:
		return false
	}
}

func validExceptionHandlerType(handlerType Value) bool {
	if validExceptionClass(handlerType) {
		return true
	}
	tuple, ok := handlerType.(*tupleValue)
	if !ok {
		return false
	}
	for _, item := range tuple.elements {
		if !validExceptionClass(item) {
			return false
		}
	}
	return true
}
