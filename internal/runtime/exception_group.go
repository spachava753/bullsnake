package runtime

import (
	"fmt"
	"slices"
)

func isExceptionGroupType(exceptionType *exceptionTypeValue) bool {
	return exceptionType == baseExceptionGroupType || exceptionType == exceptionGroupType
}

func raiseOutcome(exception *Exception) instructionOutcome {
	return instructionOutcome{kind: raised, exception: exception}
}

func exceptionGroupArityError(count int) *Exception {
	message := fmt.Sprintf(
		"BaseExceptionGroup.__new__() takes exactly 2 arguments (%d given)",
		count,
	)
	return newException("TypeError", message)
}

// executeExceptionGroupTypeCall validates CPython's two constructor arguments,
// selects BaseExceptionGroup or ExceptionGroup from the children, consumes the
// call segment, and pushes an immutable group value.
func executeExceptionGroupTypeCall(
	caller *frame,
	instruction int,
	base int,
	groupType *exceptionTypeValue,
	userClass *typeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		name := groupType.name
		if userClass != nil {
			name = userClass.name
		}
		return raiseOutcome(newException("TypeError", name+"() takes no keyword arguments")), nil
	}
	if len(arguments) != 2 {
		return raiseOutcome(exceptionGroupArityError(len(arguments))), nil
	}
	message, ok := arguments[0].(*stringValue)
	if !ok {
		detail := "BaseExceptionGroup.__new__() argument 1 must be str, not " +
			arguments[0].TypeName()
		return raiseOutcome(newException("TypeError", detail)), nil
	}
	children, ok := exceptionGroupChildren(arguments[1])
	if !ok {
		detail := "second argument (exceptions) must be a sequence"
		return raiseOutcome(newException("TypeError", detail)), nil
	}
	if len(children) == 0 {
		detail := "second argument (exceptions) must be a non-empty sequence"
		return raiseOutcome(newException("ValueError", detail)), nil
	}

	containsBaseException := false
	for index, child := range children {
		exception, isException := child.(*Exception)
		if !isException {
			detail := fmt.Sprintf(
				"Item %d of second argument (exceptions) is not an exception",
				index,
			)
			return raiseOutcome(newException("ValueError", detail)), nil
		}
		if !exception.class.isSubclassOf(exceptionType) {
			containsBaseException = true
		}
	}

	if containsBaseException && groupType == exceptionGroupType {
		detail := "Cannot nest BaseExceptions in an ExceptionGroup"
		if userClass != nil {
			detail = "Cannot nest BaseExceptions in '" + userClass.name + "'"
		}
		return raiseOutcome(newException("TypeError", detail)), nil
	}
	if userClass == nil && groupType == baseExceptionGroupType && !containsBaseException {
		groupType = exceptionGroupType
	}

	group := &Exception{
		class:     groupType,
		userClass: userClass,
		message:   message.value,
		group:     &tupleValue{elements: children},
	}
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	return pushOutcome(caller, instruction, group)
}

func exceptionGroupChildren(value Value) ([]Value, bool) {
	switch value := value.(type) {
	case *tupleValue:
		return slices.Clone(value.elements), true
	case *listValue:
		return slices.Clone(value.elements), true
	default:
		return nil, false
	}
}
