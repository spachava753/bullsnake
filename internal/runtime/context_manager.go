package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

// executeLoadSpecial resolves built-in async-generator methods directly and
// user-defined protocol methods on an instance's class, never its attributes.
func executeLoadSpecial(
	frame *frame,
	instruction int,
	name string,
) (instructionOutcome, error) {
	owner, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	if stream, ok := owner.(*stringIOValue); ok {
		if name == "__enter__" || name == "__exit__" {
			return executeStringIOAttributeLoad(frame, instruction, stream, name)
		}
		return raiseOutcome(missingSpecialMethod(owner, name)), nil
	}
	if stream, ok := owner.(*hostTextStream); ok {
		if name == "__enter__" || name == "__exit__" {
			return executeHostStreamAttributeLoad(frame, instruction, stream, name)
		}
		return raiseOutcome(missingSpecialMethod(owner, name)), nil
	}
	if generator, asyncGenerator := owner.(*generatorValue); asyncGenerator &&
		generator.kind == asyncGeneratorObject {
		var value Value
		switch name {
		case "__aiter__":
			value = &asyncGeneratorAIterMethod{generator: generator}
		case "__anext__":
			value = &asyncGeneratorANextMethod{generator: generator}
		default:
			return instructionOutcome{
				kind:      raised,
				exception: missingSpecialMethod(owner, name),
			}, nil
		}
		return pushOutcome(frame, instruction, value)
	}
	instance, ok := owner.(*instanceValue)
	if !ok {
		return instructionOutcome{
			kind:      raised,
			exception: missingSpecialMethod(owner, name),
		}, nil
	}
	value, found := lookupInstanceSpecial(instance, name)
	if !found {
		return instructionOutcome{
			kind:      raised,
			exception: missingSpecialMethod(owner, name),
		}, nil
	}
	return pushOutcome(frame, instruction, value)
}

func missingSpecialMethod(owner Value, name string) *Exception {
	switch name {
	case "__aiter__":
		return newException(
			"TypeError",
			"'async for' requires an object with __aiter__ method, got "+owner.TypeName(),
		)
	case "__anext__":
		return newException(
			"TypeError",
			"'async for' requires an iterator with __anext__ method, got "+owner.TypeName(),
		)
	}
	protocol := "context manager protocol"
	if name == "__aenter__" || name == "__aexit__" {
		protocol = "asynchronous context manager protocol"
	}
	return newException(
		"TypeError",
		"'"+owner.TypeName()+"' object does not support the "+protocol+
			" (missed "+name+" method)",
	)
}

func executeLoadHandledExceptionType(
	frame *frame,
	instruction int,
) (instructionOutcome, error) {
	exception := activeHandledException(frame, instruction)
	if exception == nil {
		return instructionOutcome{}, frame.failure(
			instruction,
			bytecode.LoadHandledExceptionType.String()+" has no active exception",
		)
	}
	var exceptionType Value = exception.class
	if exception.userClass != nil {
		exceptionType = exception.userClass
	}
	return pushOutcome(frame, instruction, exceptionType)
}
