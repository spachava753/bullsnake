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
			exception: missingContextMethod(owner, name),
		}, nil
	}
	value, found := instance.class.lookup(name)
	if !found {
		return instructionOutcome{
			kind:      raised,
			exception: missingContextMethod(owner, name),
		}, nil
	}
	if function, bind := value.(*functionValue); bind {
		value = &boundMethodValue{function: function, self: instance}
	}
	return pushOutcome(frame, instruction, value)
}

func missingContextMethod(owner Value, name string) *Exception {
	return newException(
		"TypeError",
		"'"+owner.TypeName()+"' object does not support the context manager protocol "+
			"(missed "+name+" method)",
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
