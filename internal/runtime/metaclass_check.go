package runtime

// executeClassCheck validates call shape before evaluating tuple candidates and
// metaclass hooks. Native fallback retains the existing ancestry rules.
func executeClassCheck(caller *frame, instruction, base int, arguments []Value, keywords *dictValue, subclass bool) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	if len(arguments) != 2 || (keywords != nil && len(keywords.entries) != 0) {
		var exception *Exception
		if subclass {
			_, exception = builtinIsSubclass(arguments, keywords)
		} else {
			_, exception = builtinIsInstance(arguments, keywords)
		}
		return raiseOutcome(exception), nil
	}
	call := &metaclassCheck{instruction: instruction, subject: arguments[0], pending: []Value{arguments[1]}, subclass: subclass}
	return call.next(caller)
}

type metaclassCheck struct {
	instruction int
	subject     Value
	pending     []Value
	subclass    bool
}

// next evaluates nested tuples lazily and dispatches only hooks on the
// candidate's metaclass, ignoring same-named attributes on the candidate itself.
func (call *metaclassCheck) next(caller *frame) (instructionOutcome, error) {
	for len(call.pending) != 0 {
		candidate := call.pending[0]
		call.pending = call.pending[1:]
		if tuple, ok := candidate.(*tupleValue); ok {
			call.pending = append(append([]Value(nil), tuple.elements...), call.pending...)
			continue
		}
		if !call.subclass {
			actual, _ := typeOf(call.subject)
			if actual == candidate {
				return pushOutcome(caller, call.instruction, trueSingleton)
			}
		}
		name := "__instancecheck__"
		if call.subclass {
			name = "__subclasscheck__"
		}
		if class, ok := candidate.(*typeValue); ok && class.metaclass != nil {
			if method, exists := class.metaclass.lookup(name); exists {
				bound := Value(&boundMethodValue{callable: method, self: class})
				if descriptor, ok := bindMethodDescriptor(method, class.metaclass); ok {
					bound = descriptor
				}
				return call.invoke(caller, bound)
			}
		}
		var result Value
		var exception *Exception
		if call.subclass {
			result, exception = builtinIsSubclass([]Value{call.subject, candidate}, nil)
		} else {
			result, exception = builtinIsInstance([]Value{call.subject, candidate}, nil)
		}
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == trueSingleton {
			return pushOutcome(caller, call.instruction, result)
		}
	}
	return pushOutcome(caller, call.instruction, falseSingleton)
}

func (call *metaclassCheck) invoke(caller *frame, method Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, call.instruction, len(caller.stack), method, []Value{call.subject}, nil)
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, call.instruction, func() (instructionOutcome, error) {
			return executeBuiltinBool(current, call.instruction, len(current.stack), []Value{value}, nil)
		}, func(resumed *frame, truth Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if truth == trueSingleton {
				return pushOutcome(resumed, call.instruction, truth)
			}
			return call.next(resumed)
		})
	})
}

// nativeTypeCheck implements the unbound type descriptors without redispatching
// to the override that may have called them through super.
func nativeTypeCheck(arguments []Value, keywords *dictValue, subclass bool) (Value, *Exception) {
	if len(arguments) != 2 || (keywords != nil && len(keywords.entries) != 0) {
		return nil, newException("TypeError", "type check requires two positional arguments")
	}
	if !isClassValue(arguments[0]) {
		return nil, newException("TypeError", "type check requires a type receiver")
	}
	if subclass {
		return builtinIsSubclass([]Value{arguments[1], arguments[0]}, nil)
	}
	return builtinIsInstance([]Value{arguments[1], arguments[0]}, nil)
}
