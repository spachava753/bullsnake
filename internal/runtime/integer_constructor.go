package runtime

// executeIntegerAllocation validates native layout before converting the value.
// A subtype retains typed immutable integer storage, separate from attributes.
func executeIntegerAllocation(caller *frame, instruction int, target Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if target == boolNativeType {
		return raiseOutcome(newException("TypeError", "int.__new__(bool) is not safe, use bool.__new__()")), nil
	}
	class, user := target.(*typeValue)
	if target != intNativeType && (!user || !class.isSubclassOfNative(intNativeType)) {
		name := allocationClassName(target)
		return raiseOutcome(newException("TypeError", "int.__new__("+name+"): "+name+" is not a subtype of int")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIntegerConversion(caller, instruction, arguments, keywords)
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if user {
			value = &instanceValue{class: class, integer: value.(*intValue), attributes: newNamespace()}
		}
		return pushOutcome(current, instruction, value)
	})
}

// executeIntegerConversion calls class int/index hooks before converting native
// numeric values. Conversion returns an exact integer without copying subtype attributes.
func executeIntegerConversion(caller *frame, instruction int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if len(arguments) == 1 && (keywords == nil || len(keywords.entries) == 0) {
		if instance, ok := arguments[0].(*instanceValue); ok {
			name := "__int__"
			method, found := instance.class.lookup(name)
			if !found {
				name = "__index__"
				method, found = instance.class.lookup(name)
			}
			if found {
				return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
					return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
						return executeClassSlotBinding(caller, instruction, instance, method)
					}, func(current *frame, callable Value, exception *Exception) (instructionOutcome, error) {
						if exception != nil {
							return raiseOutcome(exception), nil
						}
						return executeFunctionCall(current, instruction, len(current.stack), callable, nil, nil)
					})
				}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
					if exception != nil {
						return raiseOutcome(exception), nil
					}
					if _, exact := result.(*intValue); exact {
						return pushOutcome(current, instruction, result)
					}
					if _, integer := integerOperand(result); integer {
						return raiseOutcome(newException("NotImplementedError", "non-exact integer conversion results require deprecation warning support")), nil
					}
					return raiseOutcome(newException("TypeError", name+" returned non-int (type "+result.TypeName()+")")), nil
				})
			}
		}
	}
	result, exception := builtinInt(arguments, keywords)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, result)
}
