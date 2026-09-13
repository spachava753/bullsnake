package runtime

type dictionaryFromkeysCall struct {
	instruction int
	iterator    Value
	value       Value
	sentinel    Value
	result      *dictValue
	done        bool
}

func executeDictionaryFromkeys(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("fromkeys", arguments, keywords, 1, 2); exception != nil {
		return raiseOutcome(exception), nil
	}
	if self != dictNativeType {
		return raiseOutcome(newException("NotImplementedError", "dict subclass construction is not supported")), nil
	}
	value := Value(None)
	if len(arguments) == 2 {
		value = arguments[1]
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIteratorLookup(caller, instruction, arguments[0])
	}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call := &dictionaryFromkeysCall{instruction: instruction, iterator: iterator, value: value, sentinel: &dictValue{}, result: &dictValue{}}
		return call.advance(current)
	})
}

// advance inserts each key before requesting the next, preserving duplicate
// order and value identity while stopping iteration on the first key error.
func (call *dictionaryFromkeysCall) advance(caller *frame) (instructionOutcome, error) {
	for !call.done {
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := executeBuiltinNext(caller, call.instruction, len(caller.stack), []Value{call.iterator, call.sentinel}, nil)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, key Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if key == call.sentinel {
				call.done = true
			} else if exception := call.result.set(key, call.value); exception != nil {
				return raiseOutcome(exception), nil
			}
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	return pushOutcome(caller, call.instruction, call.result)
}
