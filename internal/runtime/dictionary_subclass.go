package runtime

// dictionaryStorage unwraps real dictionary state for native slots that must
// bypass Python overrides. Ordinary operations still resolve subclass methods.
func dictionaryStorage(value Value) (*dictValue, bool) {
	if dictionary, ok := value.(*dictValue); ok {
		return dictionary, true
	}
	if instance, ok := value.(*instanceValue); ok && instance.dictionary != nil {
		return instance.dictionary, true
	}
	return nil, false
}

// executeDictionaryGetitem invokes __missing__ only after a genuine absent key
// on a subclass; get, membership, and other methods bypass that hook.
func executeDictionaryGetitem(caller *frame, instruction int, self Value, storage *dictValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__getitem__", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	value, found, exception := storage.get(arguments[0])
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if found {
		return pushOutcome(caller, instruction, value)
	}
	if instance, ok := self.(*instanceValue); ok {
		if method, found := lookupInstanceSpecial(instance, "__missing__"); found {
			return executeFunctionCall(caller, instruction, len(caller.stack), method, arguments, nil)
		}
	}
	return raiseOutcome(newException("KeyError", arguments[0].Repr())), nil
}
