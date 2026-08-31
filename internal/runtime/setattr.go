package runtime

import "strconv"

// executeBuiltinSetattr validates the dynamic name and returns None after an
// immediate store or a suspended data-descriptor call.
func executeBuiltinSetattr(
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
			"setattr() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 3 {
		count := len(arguments)
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"setattr expected 3 arguments, got "+strconv.Itoa(count),
		)), nil
	}
	name, ok := arguments[1].(*stringValue)
	if !ok {
		typeName := arguments[1].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"attribute name must be string, not '"+typeName+"'",
		)), nil
	}
	owner := arguments[0]
	value := arguments[2]
	discardCallSegment(caller, base)
	outcome, err := executeDynamicAttributeStore(
		caller,
		instruction,
		owner,
		name.value,
		value,
	)
	if err != nil || outcome.kind != advance {
		if outcome.kind == called {
			if outcome.frame == nil || outcome.frame.attribute == nil {
				return instructionOutcome{}, caller.failure(
					instruction,
					"setattr descriptor call has no continuation",
				)
			}
			outcome.frame.attribute.returnNone = true
		}
		return outcome, err
	}
	return pushOutcome(caller, instruction, None)
}
