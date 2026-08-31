package runtime

import "strconv"

type representationCall struct {
	instruction int
}

// executeBuiltinRepr validates its call shape before resolving the value's
// native representation or class __repr__ method.
func executeBuiltinRepr(
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
			"repr() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"repr() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	value := arguments[0]
	discardCallSegment(caller, base)
	return executeRepresentation(caller, instruction, value)
}

// executeRepresentation invokes only a class-level user __repr__; all other
// runtime values already provide their fixed representation through Value.
func executeRepresentation(
	frame *frame,
	instruction int,
	value Value,
) (instructionOutcome, error) {
	instance, user := value.(*instanceValue)
	if !user {
		return pushOutcome(frame, instruction, &stringValue{value: value.Repr()})
	}
	method, found := lookupInstanceSpecial(instance, "__repr__")
	if !found {
		return pushOutcome(frame, instruction, &stringValue{value: value.Repr()})
	}
	call := &representationCall{instruction: instruction}
	outcome, err := executeFunctionCall(
		frame,
		instruction,
		len(frame.stack),
		method,
		nil,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.representation = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"representation method returned without a value",
		)
	}
	return finishRepresentationCall(frame, call, result)
}

func finishRepresentationCall(
	frame *frame,
	call *representationCall,
	result Value,
) (instructionOutcome, error) {
	representation, ok := result.(*stringValue)
	if !ok {
		return raiseOutcome(newException(
			"TypeError",
			"__repr__ returned non-string (type "+result.TypeName()+")",
		)), nil
	}
	return pushOutcome(frame, call.instruction, representation)
}
