package runtime

// lookupIterationSpecial selects an instance class or a class's metaclass slot,
// ignoring same-named attributes on the iterated object. Binding waits for the VM.
func lookupIterationSpecial(owner Value, name string) (Value, bool) {
	switch owner.(type) {
	case *instanceValue, *typeValue:
	default:
		return nil, false
	}
	actual, _ := typeOf(owner)
	class, ok := actual.(*typeValue)
	if !ok {
		return nil, false
	}
	method, found := class.lookup(name)
	if !found || method == None {
		return method, found
	}
	return &builtinFunctionValue{name: name, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeClassSlotBinding(caller, instruction, owner, method)
		}, func(current *frame, bound Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeFunctionCall(current, instruction, len(current.stack), bound, arguments, keywords)
		})
	}}, true
}
