package runtime

// executeAbstractMarker reads the live marker through ordinary attributes and
// resolves its truth value through the VM, suppressing only missing attributes.
func executeAbstractMarker(caller *frame, instruction int, value Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, value, "__isabstractmethod__")
	}, func(current *frame, marker Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if isAttributeError(exception) {
				return pushOutcome(current, instruction, falseSingleton)
			}
			return raiseOutcome(exception), nil
		}
		return executeBuiltinBool(current, instruction, len(current.stack), []Value{marker}, nil)
	})
}

// executePropertyAbstractMarker tests accessors from getter to deleter and
// stops at the first true marker, including when a Python callback is needed.
func executePropertyAbstractMarker(caller *frame, instruction int, accessors []Value) (instructionOutcome, error) {
	for len(accessors) != 0 && accessors[0] == nil {
		accessors = accessors[1:]
	}
	if len(accessors) == 0 {
		return pushOutcome(caller, instruction, falseSingleton)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeAbstractMarker(caller, instruction, accessors[0])
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == trueSingleton {
			return pushOutcome(current, instruction, result)
		}
		return executePropertyAbstractMarker(current, instruction, accessors[1:])
	})
}
