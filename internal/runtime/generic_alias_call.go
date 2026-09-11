package runtime

// executeGenericAliasCall forwards to the origin and attaches __orig_class__
// when supported. Only attribute/type failures from metadata storage are ignored.
func executeGenericAliasCall(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if self.alias == nil {
		return raiseOutcome(newException("TypeError", "uninitialized GenericAlias")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), self.alias.origin, arguments, keywords)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeBuiltinSetattr(current, instruction, len(current.stack), []Value{result, &stringValue{value: "__orig_class__"}, self}, nil)
		}, func(ready *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil && !isAttributeError(exception) && !exception.class.isSubclassOf(typeErrorType) {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(ready, instruction, result)
		})
	})
}
