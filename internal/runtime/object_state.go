package runtime

// executeObjectGetState captures the live attribute dictionary before unchanged
// copyreg discovers slot names. Native storage is not part of this default state.
func executeObjectGetState(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__getstate__", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	state, exception := defaultObjectDictionary(self)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	class, _ := typeOf(self)
	var slots Value
	var cached bool
	switch class := class.(type) {
	case *typeValue:
		slots, cached = class.namespace.get("__slotnames__")
	case *nativeTypeValue:
		slots, cached, _ = caller.runtime.nativeNamespace(class).get(&stringValue{value: "__slotnames__"})
	}
	finish := func(current *frame, slots Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if slots != None {
			names, ok := slots.(*listValue)
			if !ok {
				message := "copyreg._slotnames didn't return a list or None"
				if cached {
					message = allocationClassName(class) + ".__slotnames__ should be a list or None, not " + slots.TypeName()
				}
				return raiseOutcome(newException("TypeError", message)), nil
			}
			if len(names.elements) != 0 {
				return raiseOutcome(newException("TypeError", "object state for nonempty slots is not implemented")), nil
			}
		}
		return pushOutcome(current, instruction, state)
	}
	if cached {
		return finish(caller, slots, nil)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeNativeImport(caller, instruction, newImportRequest("copyreg", "copyreg", &tupleValue{}))
		}, func(current *frame, module Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeMethodCall(current, instruction, module, "_slotnames", []Value{class})
		})
	}, finish)
}

// defaultObjectDictionary returns the actual attribute state for supported
// layouts. Unknown native layouts are rejected rather than losing private state.
func defaultObjectDictionary(self Value) (Value, *Exception) {
	var namespace *Namespace
	switch self := self.(type) {
	case *instanceValue:
		namespace = self.attributes
	case *functionValue:
		namespace = self.attributes
	case *Module:
		namespace = self.globals
	default:
		class, _ := typeOf(self)
		switch class {
		case objectNativeType, noneNativeType, intNativeType, boolNativeType, floatNativeType, complexNativeType, stringNativeType, bytesNativeType, listNativeType, tupleNativeType, dictNativeType, setNativeType, frozenSetNativeType, rangeNativeType:
			return None, nil
		default:
			return nil, newException("TypeError", "object state for '"+self.TypeName()+"' is not implemented")
		}
	}
	if namespace == nil || len(namespace.asDictionary().entries) == 0 {
		return None, nil
	}
	return namespace.asDictionary(), nil
}
