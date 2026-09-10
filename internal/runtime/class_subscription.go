package runtime

// executeClassSubscription gives the metaclass item slot first refusal, then
// resolves __class_getitem__ through ordinary class attribute binding.
func executeClassSubscription(caller *frame, instruction int, class *typeValue, key Value) (instructionOutcome, error) {
	if class.metaclass != nil {
		if method, found := class.metaclass.lookup("__getitem__"); found {
			return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
				if descriptor, ok := method.(*instanceValue); ok && descriptorHasSpecial(descriptor, "__get__") {
					return executeDescriptorCall(caller, instruction, attributeGet, descriptor, []Value{class, class.metaclass})
				}
				if bound, ok := bindMethodDescriptor(method, class.metaclass); ok {
					method = bound
				} else {
					method = bindInstanceFunction(method, class)
				}
				return pushOutcome(caller, instruction, method)
			}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return executeFunctionCall(current, instruction, len(current.stack), method, []Value{key}, nil)
			})
		}
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, class, "__class_getitem__")
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil && !isAttributeError(exception) {
			return raiseOutcome(exception), nil
		}
		if exception != nil || method == None {
			return raiseOutcome(newException("TypeError", "type '"+class.name+"' is not subscriptable")), nil
		}
		return executeFunctionCall(current, instruction, len(current.stack), method, []Value{key}, nil)
	})
}
