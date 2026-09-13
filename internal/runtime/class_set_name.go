package runtime

type classNamesCall struct {
	class   *typeValue
	entries []dictEntry
	next    int
}

// advance walks a snapshot of the completed namespace. Native completions loop
// locally; Python hooks resume the remaining names on the existing VM frame.
func (call *classNamesCall) advance(caller *frame, instruction int) (instructionOutcome, error) {
	for call.next < len(call.entries) {
		entry := call.entries[call.next]
		call.next++
		class, _ := typeOf(entry.value)
		var method Value
		var found bool
		switch class := class.(type) {
		case *typeValue:
			method, found = class.lookup("__set_name__")
		case *nativeTypeValue:
			method, found = caller.runtime.nativeClassAttribute(class, "__set_name__")
		}
		if !found {
			continue
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			outcome, err := call.invoke(caller, instruction, entry, method)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if suspended {
				return call.advance(current, instruction)
			}
			return pushOutcome(current, instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	return pushOutcome(caller, instruction, call.class)
}

// invoke binds the class-special hook, ignores its return value, and annotates
// callback failures without changing their exception identity or traceback.
func (call *classNamesCall) invoke(caller *frame, instruction int, entry dictEntry, method Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeClassSlotBinding(caller, instruction, entry.value, method)
	}, func(current *frame, bound Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, instruction, len(current.stack), bound, []Value{call.class, entry.key}, nil)
		}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				if exception.fields == nil {
					exception.fields = newNamespace()
				}
				value, _ := exception.fields.get("__notes__")
				notes, ok := value.(*listValue)
				if !ok {
					notes = &listValue{}
					exception.fields.store("__notes__", notes)
				}
				notes.elements = append(notes.elements, &stringValue{value: "Error calling __set_name__ on '" + entry.value.TypeName() + "' instance " + entry.key.Repr() + " in '" + call.class.name + "'"})
				return raiseOutcome(exception), nil
			}
			return pushOutcome(current, instruction, None)
		})
	})
}
