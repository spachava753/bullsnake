package runtime

import "strconv"

type attributeBuiltinCall struct {
	instruction    int
	attributeError Value
	presence       bool
}

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
	outcome, err := executeDynamicAttributeLoad(caller, instruction, owner, name.value)
	if err != nil {
		return instructionOutcome{}, err
	}
	if !hasDefault {
		return outcome, nil
	}
	if outcome.kind == raised && isAttributeError(outcome.exception) {
		return pushOutcome(caller, instruction, defaultValue)
	}
	if outcome.kind == called {
		if outcome.frame == nil {
			return instructionOutcome{}, caller.failure(
				instruction,
				"getattr attribute call has no frame",
			)
		}
		outcome.frame.attributeBuiltin = &attributeBuiltinCall{
			instruction:    instruction,
			attributeError: defaultValue,
		}
	}
	return outcome, nil
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
	outcome, err := executeDynamicAttributeLoad(caller, instruction, owner, name.value)
	if err != nil {
		return instructionOutcome{}, err
	}
	switch outcome.kind {
	case advance:
		if _, ok := caller.pop(); !ok {
			return instructionOutcome{}, caller.failure(
				instruction,
				"hasattr lookup returned without a value",
			)
		}
		return pushOutcome(caller, instruction, trueSingleton)
	case called:
		if outcome.frame == nil {
			return instructionOutcome{}, caller.failure(
				instruction,
				"hasattr attribute call has no frame",
			)
		}
		outcome.frame.attributeBuiltin = &attributeBuiltinCall{
			instruction:    instruction,
			attributeError: falseSingleton,
			presence:       true,
		}
		return outcome, nil
	case raised:
		if isAttributeError(outcome.exception) {
			return pushOutcome(caller, instruction, falseSingleton)
		}
		return outcome, nil
	default:
		return instructionOutcome{}, caller.failure(
			instruction,
			"invalid hasattr attribute outcome",
		)
	}
}

func isAttributeError(exception *Exception) bool {
	return exception != nil && exception.class != nil &&
		exception.class.isSubclassOf(attributeErrorType)
}
