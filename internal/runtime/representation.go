package runtime

import "strconv"

type representationCall struct {
	instruction int
	method      string
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

// executeBuiltinStr handles the object form and rejects the separate bytes
// decoding form until encoding support is implemented.
func executeBuiltinStr(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if len(arguments) > 3 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str() takes at most 3 arguments ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	if (keywords != nil && len(keywords.entries) != 0) || len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"NotImplementedError",
			"str() encoding form is not supported",
		)), nil
	}
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return pushOutcome(caller, instruction, &stringValue{})
	}
	value := arguments[0]
	discardCallSegment(caller, base)
	return executeString(caller, instruction, value)
}

// executeString gives strings and exceptions their direct text form, then uses
// class __str__ or the ordinary representation fallback for user instances.
func executeString(
	frame *frame,
	instruction int,
	value Value,
) (instructionOutcome, error) {
	switch value := value.(type) {
	case *stringValue:
		return pushOutcome(frame, instruction, value)
	case *Exception:
		return pushOutcome(frame, instruction, &stringValue{value: value.Message()})
	case *instanceValue:
		method, found := lookupInstanceSpecial(value, "__str__")
		if found {
			return executeRepresentationMethod(
				frame,
				instruction,
				method,
				"__str__",
			)
		}
		return executeRepresentation(frame, instruction, value)
	default:
		return pushOutcome(frame, instruction, &stringValue{value: value.Repr()})
	}
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
	return executeRepresentationMethod(
		frame,
		instruction,
		method,
		"__repr__",
	)
}

func executeRepresentationMethod(
	frame *frame,
	instruction int,
	method Value,
	name string,
) (instructionOutcome, error) {
	call := &representationCall{instruction: instruction, method: name}
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
			call.method+" returned non-string (type "+result.TypeName()+")",
		)), nil
	}
	return pushOutcome(frame, call.instruction, representation)
}
