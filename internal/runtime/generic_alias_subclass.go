package runtime

// executeGenericAliasSubclass lets Python __new__ initialize native alias state
// through the base constructor, then initializes only a compatible result.
func executeGenericAliasSubclass(caller *frame, instruction, base int, class *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	constructor, _ := class.lookup("__new__")
	if wrapper, ok := constructor.(*staticMethodValue); ok {
		constructor = wrapper.callable
	}
	newArguments := append([]Value{class}, arguments...)
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), constructor, newArguments, keywords)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		instance, ok := result.(*instanceValue)
		if !ok || !instance.class.isSubclassOf(class) {
			return pushOutcome(current, instruction, result)
		}
		initializer, hasInitializer := lookupInstanceSpecial(instance, "__init__")
		if !hasInitializer {
			return pushOutcome(current, instruction, result)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, instruction, len(current.stack), initializer, arguments, keywords)
		}, func(ready *frame, initialized Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if initialized != None {
				return raiseOutcome(newException("TypeError", "__init__() should return None, not '"+initialized.TypeName()+"'")), nil
			}
			return pushOutcome(ready, instruction, instance)
		})
	})
}
