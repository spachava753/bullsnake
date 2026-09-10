package runtime

// executeRetryIOCall retries structured EINTR failures without growing the Go
// stack for native callbacks, while ordinary Python failures propagate intact.
func executeRetryIOCall(caller *frame, instruction int, receiver Value, name string, arguments []Value) (instructionOutcome, error) {
	for {
		retry, suspended := false, false
		outcome, err := continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			outcome, err := executeMethodCall(caller, instruction, receiver, name, arguments)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if isIOInterrupted(exception) {
				if suspended {
					return executeRetryIOCall(current, instruction, receiver, name, arguments)
				}
				retry = true
				return pushOutcome(current, instruction, None)
			}
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(current, instruction, result)
		})
		if err != nil || outcome.kind != advance || !retry {
			return outcome, err
		}
		caller.pop()
	}
}
