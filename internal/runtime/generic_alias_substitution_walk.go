package runtime

type aliasSubstitutionSequence struct {
	owner  Value
	items  []Value
	result []Value
	index  int
}

type aliasSubstitutionWalk struct {
	call  *aliasSubstitutionCall
	stack []aliasSubstitutionSequence
}

// advance reconstructs lists and tuples with a typed worklist, while nested
// parameterized objects run their own subscription and typing callbacks.
func (walk *aliasSubstitutionWalk) advance(caller *frame) (instructionOutcome, error) {
	call := walk.call
	for len(walk.stack) != 0 {
		last := len(walk.stack) - 1
		state := &walk.stack[last]
		if state.index == len(state.items) {
			var result Value = &tupleValue{elements: state.result}
			if _, list := state.owner.(*listValue); list {
				result = &listValue{elements: state.result}
			}
			walk.stack[last] = aliasSubstitutionSequence{}
			walk.stack = walk.stack[:last]
			if last == 0 {
				return pushOutcome(caller, call.instruction, newGenericAlias(caller.runtime.genericAliasClass, call.self.alias.origin, result))
			}
			walk.append(result)
			continue
		}
		item := state.items[state.index]
		state.index++
		if isClassValue(item) {
			walk.append(item)
			continue
		}
		if items, sequence := aliasSequenceItems(item); sequence {
			if len(walk.stack) >= 1000 {
				return raiseOutcome(newException("RecursionError", "maximum recursion depth exceeded in substitution")), nil
			}
			walk.stack = append(walk.stack, aliasSubstitutionSequence{owner: item, items: append([]Value(nil), items...)})
			continue
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := call.substitute(caller, item)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			walk.append(result)
			if suspended {
				return walk.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	return instructionOutcome{}, caller.failure(call.instruction, "empty substitution worklist")
}

func (walk *aliasSubstitutionWalk) append(value Value) {
	state := &walk.stack[len(walk.stack)-1]
	state.result = append(state.result, value)
}

// substitute replaces direct native parameters, invokes custom typing methods,
// and delegates nested parameterized objects to their ordinary item protocol.
func (call *aliasSubstitutionCall) substitute(caller *frame, item Value) (instructionOutcome, error) {
	if _, parameter := item.(*paramSpecValue); parameter {
		value, _ := call.replacement(item)
		value, exception := checkedParamSpecArgument(value)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, call.instruction, value)
	}
	if _, ordinary := item.(*typeVarValue); ordinary {
		value, _ := call.replacement(item)
		value, exception := checkedTypeArgument(value)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, call.instruction, value)
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, call.instruction, item, "__typing_subst__")
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception == nil {
			value, found := call.replacement(item)
			if !found {
				return raiseOutcome(newException("TypeError", "typing parameter missing from cached __parameters__")), nil
			}
			return executeFunctionCall(current, call.instruction, len(current.stack), method, []Value{value}, nil)
		}
		if !isAttributeError(exception) {
			return raiseOutcome(exception), nil
		}
		return call.substituteNested(current, item)
	})
}

func (call *aliasSubstitutionCall) substituteNested(caller *frame, item Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeBuiltinGetattr(caller, call.instruction, len(caller.stack), []Value{item, &stringValue{value: "__parameters__"}, None}, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		parameters, ok := result.(*tupleValue)
		if !ok || len(parameters.elements) == 0 {
			return pushOutcome(current, call.instruction, item)
		}
		items := make([]Value, len(parameters.elements))
		for index, parameter := range parameters.elements {
			items[index], _ = call.replacement(parameter)
		}
		current.push(item)
		current.push(&tupleValue{elements: items})
		return executeBinarySubscript(current, call.instruction)
	})
}

func checkedTypeArgument(value Value) (Value, *Exception) {
	if value == None {
		return noneNativeType, nil
	}
	switch value.(type) {
	case *tupleValue:
		return nil, newException("TypeError", "Parameters to generic types must be types. Got "+value.Repr()+".")
	case *stringValue:
		return nil, newException("NotImplementedError", "string forward references in type substitution are not supported")
	}
	return value, nil
}
