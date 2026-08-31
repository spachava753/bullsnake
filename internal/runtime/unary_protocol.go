package runtime

type unaryCall struct {
	instruction int
}

func executeUserUnary(
	frame *frame,
	instruction int,
	method Value,
) (instructionOutcome, error) {
	call := &unaryCall{instruction: instruction}
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
		outcome.frame.unary = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"unary special method returned without a value",
		)
	}
	return finishUnaryCall(frame, call, result)
}

func finishUnaryCall(
	frame *frame,
	call *unaryCall,
	result Value,
) (instructionOutcome, error) {
	return pushOutcome(frame, call.instruction, result)
}
