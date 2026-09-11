package runtime

// prepareParamSpec applies a lazy default before normalizing PEP 612's single
// parameter shorthand and list arguments, without modifying the supplied list.
func (call *aliasSubstitutionCall) prepareParamSpec(caller *frame, index int, parameter *paramSpecValue) (instructionOutcome, error) {
	if index < len(call.items) {
		return call.finishParamSpecPreparation(caller, index, call.items)
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeVariadicTypeParameterDefaultLoad(caller, call.instruction, parameter)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == noDefaultSingleton || index != len(call.items) {
			return raiseOutcome(newException("TypeError", "Too few arguments for "+call.self.alias.repr())), nil
		}
		items := append(append([]Value(nil), call.items...), result)
		return call.finishParamSpecPreparation(current, index, items)
	})
}

func (call *aliasSubstitutionCall) finishParamSpecPreparation(caller *frame, index int, items []Value) (instructionOutcome, error) {
	if len(call.parameters) == 1 && !nativeParamExpression(items[0]) {
		return pushOutcome(caller, call.instruction, &tupleValue{elements: []Value{&tupleValue{elements: items}}})
	}
	if list, ok := items[index].(*listValue); ok {
		items = append([]Value(nil), items...)
		items[index] = &tupleValue{elements: append([]Value(nil), list.elements...)}
	}
	return pushOutcome(caller, call.instruction, &tupleValue{elements: items})
}

func nativeParamExpression(value Value) bool {
	switch value.(type) {
	case *ellipsisValue, *paramSpecValue, *tupleValue, *listValue:
		return true
	}
	return false
}

// checkedParamSpecArgument converts a parameter list into an immutable tuple
// of checked arguments and otherwise accepts only supported parameter expressions.
func checkedParamSpecArgument(value Value) (Value, *Exception) {
	if items, sequence := aliasSequenceItems(value); sequence {
		result := make([]Value, len(items))
		for index, item := range items {
			checked, exception := checkedTypeArgument(item)
			if exception != nil {
				return nil, exception
			}
			result[index] = checked
		}
		return &tupleValue{elements: result}, nil
	}
	if !nativeParamExpression(value) {
		return nil, newException("TypeError", "Expected a list of types, an ellipsis, ParamSpec, or Concatenate. Got "+value.Repr())
	}
	return value, nil
}
