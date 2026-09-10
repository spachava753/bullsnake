package runtime

// printCall retains the selected stream and original values across Python
// conversion, write, and flush calls. Output is deliberately incremental.
type printCall struct {
	instruction int
	values      []Value
	file        Value
	sep         Value
	end         Value
	flush       Value
	index       int
}

// executeBuiltinPrint binds keyword-only controls and resolves flush truth before
// selecting stdout, matching CPython's argument-conversion order.
func executeBuiltinPrint(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	call := &printCall{instruction: instruction, values: append([]Value(nil), arguments...), file: None, sep: None, end: None, flush: falseSingleton}
	discardCallSegment(caller, base)
	if keywords != nil {
		for _, entry := range keywords.entries {
			switch entry.key.(*stringValue).value {
			case "sep":
				call.sep = entry.value
			case "end":
				call.end = entry.value
			case "file":
				call.file = entry.value
			case "flush":
				call.flush = entry.value
			default:
				return raiseOutcome(newException("TypeError", "print() got an unexpected keyword argument '"+entry.key.(*stringValue).value+"'")), nil
			}
		}
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeBuiltinBool(caller, instruction, len(caller.stack), []Value{call.flush}, nil)
	}, func(current *frame, truth Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call.flush = truth
		return call.start(current)
	})
}

// start resolves the current Python stream once. A None stdout is denied under
// Bullsnake's host policy, rather than CPython's disconnected-stream no-op.
func (call *printCall) start(caller *frame) (instructionOutcome, error) {
	if call.file == None {
		module, exception, err := caller.runtime.initializeModule("sys", caller.runtime.constructors["sys"])
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		var exists bool
		call.file, exists = module.globals.get("stdout")
		if !exists {
			return raiseOutcome(newException("RuntimeError", "lost sys.stdout")), nil
		}
		if call.file == None {
			return raiseOutcome(newException("PermissionError", "standard output is not configured")), nil
		}
	}
	for _, option := range []struct {
		name     string
		value    *Value
		fallback string
	}{
		{"sep", &call.sep, " "}, {"end", &call.end, "\n"},
	} {
		if *option.value == None {
			*option.value = &stringValue{value: option.fallback}
		}
		if _, ok := (*option.value).(*stringValue); !ok {
			return raiseOutcome(newException("TypeError", option.name+" must be None or a string, not "+(*option.value).TypeName())), nil
		}
	}
	return call.next(caller)
}

// next writes values, separators, and the ending separately. It resolves write
// before converting each value, so descriptors and partial failures stay ordered.
func (call *printCall) next(caller *frame) (instructionOutcome, error) {
	if call.index > 2*len(call.values) {
		if call.flush != trueSingleton {
			return pushOutcome(caller, call.instruction, None)
		}
		return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			return executeMethodCall(caller, call.instruction, call.file, "flush", nil)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(current, call.instruction, None)
		})
	}
	var value Value
	if call.index >= 2*len(call.values)-1 {
		value = call.end
		call.index = 2*len(call.values) + 1
	} else {
		if call.index%2 == 0 {
			value = call.values[call.index/2]
		} else {
			value = call.sep
		}
		call.index++
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, call.instruction, call.file, "write")
	}, func(current *frame, writer Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, call.instruction, func() (instructionOutcome, error) {
			return executeString(current, call.instruction, value)
		}, func(converted *frame, text Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return continueNativeOperation(converted, call.instruction, func() (instructionOutcome, error) {
				return executeFunctionCall(converted, call.instruction, len(converted.stack), writer, []Value{text}, nil)
			}, func(written *frame, result Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return call.next(written)
			})
		})
	})
}
