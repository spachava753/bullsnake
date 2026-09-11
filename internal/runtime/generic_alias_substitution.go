package runtime

import "fmt"

type aliasSubstitutionCall struct {
	instruction int
	self        *instanceValue
	parameters  []Value
	items       []Value
	next        int
}

// executeGenericAliasSubscription discovers parameters before preparing each
// replacement, then reconstructs an exact GenericAlias with substituted args.
func executeGenericAliasSubscription(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__getitem__", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeGenericAliasParameters(caller, instruction, self, nil, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		parameters := result.(*tupleValue).elements
		if len(parameters) == 0 {
			return raiseOutcome(newException("TypeError", self.alias.repr()+" is not a generic class")), nil
		}
		items := []Value{arguments[0]}
		if tuple, ok := arguments[0].(*tupleValue); ok {
			items = tuple.elements
		}
		call := &aliasSubstitutionCall{instruction: instruction, self: self, parameters: parameters, items: items}
		return call.prepare(current)
	})
}

// prepare runs typing preparation hooks in parameter order, retaining each
// returned argument sequence before the next hook and checking final arity.
func (call *aliasSubstitutionCall) prepare(caller *frame) (instructionOutcome, error) {
	for call.next < len(call.parameters) {
		index := call.next
		call.next++
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := call.prepareOne(caller, index)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			call.items = []Value{result}
			if tuple, ok := result.(*tupleValue); ok {
				call.items = tuple.elements
			}
			if suspended {
				return call.prepare(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	if len(call.items) != len(call.parameters) {
		quantity := "few"
		if len(call.items) > len(call.parameters) {
			quantity = "many"
		}
		return raiseOutcome(newException("TypeError", fmt.Sprintf("Too %s arguments for %s; actual %d, expected %d", quantity, call.self.alias.repr(), len(call.items), len(call.parameters)))), nil
	}
	walk := &aliasSubstitutionWalk{call: call, stack: []aliasSubstitutionSequence{{owner: call.self.alias.args, items: call.self.alias.args.elements}}}
	return walk.advance(caller)
}

// prepareOne handles native TypeVar defaults without importing typing and
// otherwise invokes the parameter's optional Python preparation method.
func (call *aliasSubstitutionCall) prepareOne(caller *frame, index int) (instructionOutcome, error) {
	parameter := call.parameters[index]
	args := &tupleValue{elements: call.items}
	if _, ordinary := parameter.(*typeVarValue); ordinary {
		if index < len(call.items) {
			return pushOutcome(caller, call.instruction, args)
		}
		return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			return executeTypeVarLoad(caller, call.instruction, parameter.(*typeVarValue), typeVarDefaultLoad)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if result == noDefaultSingleton || index != len(call.items) {
				return raiseOutcome(newException("TypeError", fmt.Sprintf("Too few arguments for %s; actual %d, expected at least %d", call.self.alias.repr(), len(call.items), index+1))), nil
			}
			return pushOutcome(current, call.instruction, &tupleValue{elements: append(append([]Value(nil), call.items...), result)})
		})
	}
	if isTypeParameter(parameter) {
		return raiseOutcome(newException("NotImplementedError", "variadic type parameter substitution is not supported")), nil
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeBuiltinGetattr(caller, call.instruction, len(caller.stack), []Value{parameter, &stringValue{value: "__typing_prepare_subst__"}, None}, nil)
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if method == None {
			return pushOutcome(current, call.instruction, args)
		}
		return executeFunctionCall(current, call.instruction, len(current.stack), method, []Value{call.self, args}, nil)
	})
}

func (call *aliasSubstitutionCall) replacement(parameter Value) (Value, bool) {
	for index, candidate := range call.parameters {
		if candidate == parameter {
			return call.items[index], true
		}
	}
	return parameter, false
}
