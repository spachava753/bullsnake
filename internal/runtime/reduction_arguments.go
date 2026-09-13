package runtime

import "strconv"

// executeNewObjectArguments searches class slots, ignoring instance replacements.
// Extended arguments take precedence; only an absent slot permits fallback.
func executeNewObjectArguments(caller *frame, instruction int, self Value) (instructionOutcome, error) {
	instance, ok := self.(*instanceValue)
	if !ok {
		return executeNewObjectReduction(caller, instruction, self, &tupleValue{}, None)
	}
	name := "__getnewargs_ex__"
	method, found := instance.class.lookup(name)
	if !found {
		name = "__getnewargs__"
		method, found = instance.class.lookup(name)
	}
	if !found {
		return executeNewObjectReduction(caller, instruction, self, &tupleValue{}, None)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeClassSlotBinding(caller, instruction, instance, method)
		}, func(current *frame, bound Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeFunctionCall(current, instruction, len(current.stack), bound, nil, nil)
		})
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		args, kwargs, exception := reductionArguments(name, result)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return executeNewObjectReduction(current, instruction, self, args, kwargs)
	})
}

// reductionArguments validates every tuple component while preserving the
// actual argument and keyword objects for the reconstruction helper.
func reductionArguments(name string, result Value) (*tupleValue, Value, *Exception) {
	tuple, ok := result.(*tupleValue)
	if !ok {
		return nil, nil, newException("TypeError", name+" should return a tuple, not '"+result.TypeName()+"'")
	}
	if name == "__getnewargs__" {
		return tuple, None, nil
	}
	if len(tuple.elements) != 2 {
		return nil, nil, newException("ValueError", "__getnewargs_ex__ should return a tuple of length 2, not "+strconv.Itoa(len(tuple.elements)))
	}
	args, ok := tuple.elements[0].(*tupleValue)
	if !ok {
		return nil, nil, newException("TypeError", "first item of the tuple returned by __getnewargs_ex__ must be a tuple, not '"+tuple.elements[0].TypeName()+"'")
	}
	if _, ok := dictionaryStorage(tuple.elements[1]); !ok {
		return nil, nil, newException("TypeError", "second item of the tuple returned by __getnewargs_ex__ must be a dict, not '"+tuple.elements[1].TypeName()+"'")
	}
	return args, tuple.elements[1], nil
}
