package runtime

const cannotCatchMessage = "catching classes that do not inherit from BaseException is not allowed"

// matchException validates every candidate before testing class ancestry so a
// later invalid tuple member fails even when an earlier member would match.
func matchException(exception *Exception, handlerType Value) (bool, *Exception) {
	if !validExceptionHandlerType(handlerType) {
		return false, newException("TypeError", cannotCatchMessage)
	}
	switch handlerType := handlerType.(type) {
	case *exceptionTypeValue:
		return exception.class.isSubclassOf(handlerType), nil
	case *tupleValue:
		for _, item := range handlerType.elements {
			if exception.class.isSubclassOf(item.(*exceptionTypeValue)) {
				return true, nil
			}
		}
	}
	return false, nil
}

func validExceptionHandlerType(handlerType Value) bool {
	switch handlerType := handlerType.(type) {
	case *exceptionTypeValue:
		return true
	case *tupleValue:
		for _, item := range handlerType.elements {
			if _, ok := item.(*exceptionTypeValue); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}
