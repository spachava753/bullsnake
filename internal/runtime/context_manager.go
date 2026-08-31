package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

func executeLoadSpecial(
	frame *frame,
	instruction int,
	name string,
) (instructionOutcome, error) {
	owner, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	instance, ok := owner.(*instanceValue)
	if !ok {
		return instructionOutcome{
			kind:      raised,
			exception: missingSpecialMethod(owner, name),
		}, nil
	}
	value, found := instance.class.lookup(name)
	if !found {
		return instructionOutcome{
			kind:      raised,
			exception: missingSpecialMethod(owner, name),
		}, nil
	}
	if function, bind := value.(*functionValue); bind {
		value = &boundMethodValue{function: function, self: instance}
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
