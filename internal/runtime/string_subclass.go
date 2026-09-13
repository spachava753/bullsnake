package runtime

func stringStorage(value Value) (*stringValue, bool) {
	if text, ok := value.(*stringValue); ok {
		return text, true
	}
	if instance, ok := value.(*instanceValue); ok && instance.text != nil {
		return instance.text, true
	}
	return nil, false
}

// executeStringAllocation validates the requested layout before converting its
// source, then retains immutable native text separately from subtype attributes.
func executeStringAllocation(caller *frame, instruction int, target Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	class, user := target.(*typeValue)
	if target != stringNativeType && (!user || !class.isSubclassOfNative(stringNativeType)) {
		name := allocationClassName(target)
		return raiseOutcome(newException("TypeError", "str.__new__("+name+"): "+name+" is not a subtype of str")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeBuiltinStr(caller, instruction, len(caller.stack), arguments, keywords)
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if user {
			value = &instanceValue{class: class, text: value.(*stringValue), attributes: newNamespace()}
		}
		return pushOutcome(current, instruction, value)
	})
}
