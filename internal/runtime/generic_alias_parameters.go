package runtime

type aliasParameterSequence struct {
	owner Value
	items []Value
	index int
}

type aliasParameterCall struct {
	instruction int
	alias       *genericAliasState
	stack       []aliasParameterSequence
	parameters  []Value
}

func executeGenericAliasParameters(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
	if self.alias == nil {
		return raiseOutcome(newException("TypeError", "uninitialized GenericAlias")), nil
	}
	if self.alias.parameters != nil {
		return pushOutcome(caller, instruction, self.alias.parameters)
	}
	call := &aliasParameterCall{instruction: instruction, alias: self.alias}
	call.stack = []aliasParameterSequence{{owner: self.alias.args, items: self.alias.args.elements}}
	return call.advance(caller)
}

// advance traverses sequence arguments with an explicit worklist and resumes
// metadata descriptors through the VM. Only a successful discovery is cached.
func (call *aliasParameterCall) advance(caller *frame) (instructionOutcome, error) {
	for len(call.stack) != 0 {
		last := len(call.stack) - 1
		state := &call.stack[last]
		if state.index == len(state.items) {
			call.stack[last] = aliasParameterSequence{}
			call.stack = call.stack[:last]
			continue
		}
		item := state.items[state.index]
		state.index++
		if isClassValue(item) {
			continue
		}
		if isTypeParameter(item) {
			call.add(item)
			continue
		}
		if items, sequence := aliasSequenceItems(item); sequence {
			if exception := call.enterSequence(item, items); exception != nil {
				return raiseOutcome(exception), nil
			}
			continue
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := call.inspect(caller, item)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
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
	call.alias.parameters = &tupleValue{elements: call.parameters}
	return pushOutcome(caller, call.instruction, call.alias.parameters)
}

// inspect follows the typing protocol: presence of __typing_subst__ identifies
// a parameter; otherwise only a tuple-valued __parameters__ contributes entries.
func (call *aliasParameterCall) inspect(caller *frame, item Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, call.instruction, item, "__typing_subst__")
	}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception == nil {
			call.add(item)
			return pushOutcome(current, call.instruction, None)
		}
		if !isAttributeError(exception) {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, call.instruction, func() (instructionOutcome, error) {
			return executeDynamicAttributeLoad(current, call.instruction, item, "__parameters__")
		}, func(ready *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil && !isAttributeError(exception) {
				return raiseOutcome(exception), nil
			}
			if parameters, ok := result.(*tupleValue); exception == nil && ok {
				for _, parameter := range parameters.elements {
					call.add(parameter)
				}
			}
			return pushOutcome(ready, call.instruction, None)
		})
	})
}

func (call *aliasParameterCall) add(parameter Value) {
	for _, existing := range call.parameters {
		if existing == parameter {
			return
		}
	}
	call.parameters = append(call.parameters, parameter)
}

// enterSequence snapshots list arguments and rejects cyclic or overly nested
// sequence paths without retaining completed siblings on the worklist.
func (call *aliasParameterCall) enterSequence(owner Value, items []Value) *Exception {
	for _, state := range call.stack {
		if state.owner == owner || len(call.stack) >= 1000 {
			return newException("RecursionError", "maximum recursion depth exceeded in __parameter__ calculation")
		}
	}
	call.stack = append(call.stack, aliasParameterSequence{owner: owner, items: append([]Value(nil), items...)})
	return nil
}

func aliasSequenceItems(value Value) ([]Value, bool) {
	switch value := value.(type) {
	case *tupleValue:
		return value.elements, true
	case *listValue:
		return value.elements, true
	}
	return nil, false
}
