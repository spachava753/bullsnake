package runtime

// executeRawCount accepts None or an index-protocol count from a raw callback.
// Overflow uses the raw I/O ValueError contract instead of a machine-size error.
func executeRawCount(caller *frame, instruction int, raw Value, name string, arguments []Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeRetryIOCall(caller, instruction, raw, name, arguments)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == None {
			return pushOutcome(current, instruction, None)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) { return executeIOIndex(current, instruction, result) }, func(resumed *frame, count Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				if exception.class.name == "OverflowError" {
					exception = newException("ValueError", "cannot fit raw I/O count into an index-sized integer")
				}
				return raiseOutcome(exception), nil
			}
			return pushOutcome(resumed, instruction, count)
		})
	})
}
