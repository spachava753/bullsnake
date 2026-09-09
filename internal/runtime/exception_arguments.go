package runtime

import "slices"

// arguments retains the original objects, including non-string values. Older
// internal exception producers receive a tuple lazily from their structured data.
func (exception *Exception) arguments() *tupleValue {
	if exception.args == nil {
		switch {
		case exception.group != nil:
			exception.args = &tupleValue{elements: []Value{&stringValue{value: exception.message}, exception.group}}
		case exception.stopIterationValue != nil && exception.stopIterationValue != None:
			exception.args = &tupleValue{elements: []Value{exception.stopIterationValue}}
		case exception.message != "":
			exception.args = &tupleValue{elements: []Value{&stringValue{value: exception.message}}}
		default:
			exception.args = &tupleValue{}
		}
	}
	return exception.args
}

// setArguments stores constructor arguments and initializes specialized fields.
func (exception *Exception) setArguments(arguments []Value) {
	exception.args = &tupleValue{elements: slices.Clone(arguments)}
	exception.message = exceptionMessage(arguments)
	if exception.class.isSubclassOf(systemExitType) {
		code := Value(None)
		if len(arguments) == 1 {
			code = arguments[0]
		} else if len(arguments) > 1 {
			code = exception.args
		}
		exception.fields = newNamespace()
		exception.fields.values["code"] = code
	}
	if exception.class.isSubclassOf(osErrorType) {
		exception.initializeOSError(arguments)
	}
}
