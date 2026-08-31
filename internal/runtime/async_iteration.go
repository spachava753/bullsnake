package runtime

// executeCheckAsyncIterator validates the direct result of __aiter__ while
// retaining it for the compiler's next-item loop.
func executeCheckAsyncIterator(
	frame *frame,
	instruction int,
) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	if generator, asyncGenerator := value.(*generatorValue); asyncGenerator &&
		generator.kind == asyncGeneratorObject {
		return pushOutcome(frame, instruction, value)
	}
	instance, ok := value.(*instanceValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'async for' received an object from __aiter__ that does not "+
					"implement __anext__: "+value.TypeName(),
			),
		}, nil
	}
	if _, found := instance.class.lookup("__anext__"); !found {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'async for' received an object from __aiter__ that does not "+
					"implement __anext__: "+value.TypeName(),
			),
		}, nil
	}
	return pushOutcome(frame, instruction, value)
}
