package runtime

// executeIOIndex accepts native integers or calls the class's __index__ method
// through the VM. Conversion completes before the stream inspects closed state.
func executeIOIndex(caller *frame, instruction int, value Value) (instructionOutcome, error) {
	if number, ok := integerOperand(value); ok {
		if !number.IsInt64() || int64(int(number.Int64())) != number.Int64() {
			return raiseOutcome(newException("OverflowError", "Python int too large to convert to C ssize_t")), nil
		}
		return pushOutcome(caller, instruction, integerFromInt64(number.Int64()))
	}
	instance, ok := value.(*instanceValue)
	if !ok {
		return raiseOutcome(newException("TypeError", "'"+value.TypeName()+"' object cannot be interpreted as an integer")), nil
	}
	method, found := lookupInstanceSpecial(instance, "__index__")
	if !found {
		return raiseOutcome(newException("TypeError", "'"+value.TypeName()+"' object cannot be interpreted as an integer")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), method, nil, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if _, ok := integerOperand(result); !ok {
			return raiseOutcome(newException("TypeError", "__index__ returned non-int (type "+result.TypeName()+")")), nil
		}
		return executeIOIndex(current, instruction, result)
	})
}
