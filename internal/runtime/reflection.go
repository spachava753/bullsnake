package runtime

import "strconv"

// executeBuiltinGetattr performs dynamic attribute lookup and records a default
// on any child frame whose escaping AttributeError should be suppressed.
func executeBuiltinGetattr(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"getattr() takes no keyword arguments",
		)), nil
	}
	if len(arguments) < 2 || len(arguments) > 3 {
		message := "getattr expected at least 2 arguments, got " +
			strconv.Itoa(len(arguments))
		if len(arguments) > 3 {
			message = "getattr expected at most 3 arguments, got " +
				strconv.Itoa(len(arguments))
		}
		discardCallSegment(caller, base)
		return raiseOutcome(newException("TypeError", message)), nil
	}
	name, ok := arguments[1].(*stringValue)
	if !ok {
		invalidType := arguments[1].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"attribute name must be string, not '"+invalidType+"'",
		)), nil
	}

	owner := arguments[0]
	hasDefault := len(arguments) == 3
	defaultValue := None
	if hasDefault {
		defaultValue = arguments[2]
	}
	discardCallSegment(caller, base)
	if !hasDefault {
		return executeDynamicAttributeLoad(caller, instruction, owner, name.value)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, owner, name.value)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if isAttributeError(exception) {
				return pushOutcome(current, instruction, defaultValue)
			}
			return raiseOutcome(exception), nil
		}
		return pushOutcome(current, instruction, result)
	})
}

// executeBuiltinHasattr returns one boolean after the same dynamic lookup used
// by getattr, suppressing only AttributeError from immediate or suspended code.
func executeBuiltinHasattr(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"hasattr() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 2 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"hasattr expected 2 arguments, got "+strconv.Itoa(len(arguments)),
		)), nil
	}
	name, ok := arguments[1].(*stringValue)
	if !ok {
		invalidType := arguments[1].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"attribute name must be string, not '"+invalidType+"'",
		)), nil
	}

	owner := arguments[0]
	discardCallSegment(caller, base)
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, owner, name.value)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if isAttributeError(exception) {
				return pushOutcome(current, instruction, falseSingleton)
			}
			return raiseOutcome(exception), nil
		}
		return pushOutcome(current, instruction, trueSingleton)
	})
}

func isAttributeError(exception *Exception) bool {
	return exception != nil && exception.class != nil &&
		exception.class.isSubclassOf(attributeErrorType)
}
