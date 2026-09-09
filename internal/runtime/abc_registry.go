package runtime

import "strconv"

// abcData belongs to a class's _abc_impl attribute. All three sets are weak;
// only a successful new registration advances the owning runtime's token.
type abcData struct {
	registry weakClassSet
	positive weakClassSet
	negative weakClassSet
	version  uint64
}

func (*abcData) TypeName() string { return "_abc._abc_data" }
func (*abcData) Repr() string     { return "<_abc._abc_data object>" }
func (*abcData) isValue()         {}

// abcHelper checks each private helper's positional shape before dispatching.
// Arguments are retained before the caller's operand segment is discarded.
func abcHelper(name string, count int, call func(*frame, int, []Value) (instructionOutcome, error)) *builtinFunctionValue {
	return &builtinFunctionValue{name: name, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		if len(arguments) != count || (keywords != nil && len(keywords.entries) != 0) {
			return raiseOutcome(newException("TypeError", name+"() takes exactly "+strconv.Itoa(count)+" positional arguments")), nil
		}
		return call(caller, instruction, arguments)
	}}
}

// withABCData uses ordinary lookup so inherited, replaced, and descriptor-valued
// _abc_impl attributes follow the same rules as other Python attributes.
func withABCData(caller *frame, instruction int, class Value, resume func(*frame, *abcData) (instructionOutcome, error)) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, class, "_abc_impl")
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		data, ok := value.(*abcData)
		if !ok {
			return raiseOutcome(newException("TypeError", "_abc_impl is set to a wrong type")), nil
		}
		return resume(current, data)
	})
}

// abcMethod resolves and calls a method through ordinary descriptor continuations.
func abcMethod(caller *frame, instruction int, receiver Value, name string, arguments []Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, receiver, name)
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return executeFunctionCall(current, instruction, len(current.stack), method, arguments, nil)
	})
}

func abcIsSubclass(caller *frame, instruction int, child, parent Value) (instructionOutcome, error) {
	return executeClassCheck(caller, instruction, len(caller.stack), []Value{child, parent}, nil, true)
}

// registerABC first tests existing membership, then reverse membership to reject
// inheritance cycles. Both checks honor Python metaclass overrides.
func registerABC(caller *frame, instruction int, arguments []Value) (instructionOutcome, error) {
	class, candidate := arguments[0], arguments[1]
	if !isClassValue(candidate) {
		return raiseOutcome(newException("TypeError", "Can only register classes")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return abcIsSubclass(caller, instruction, candidate, class)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == trueSingleton {
			return pushOutcome(current, instruction, candidate)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return abcIsSubclass(current, instruction, class, candidate)
		}, func(resumed *frame, reverse Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if reverse == trueSingleton {
				return raiseOutcome(newException("RuntimeError", "Refusing to create an inheritance cycle")), nil
			}
			return withABCData(resumed, instruction, class, func(ready *frame, data *abcData) (instructionOutcome, error) {
				data.registry.add(candidate)
				ready.runtime.abcToken++
				return pushOutcome(ready, instruction, candidate)
			})
		})
	})
}

// checkABCSubclass follows CPython's cache, hook, nominal MRO, registry, and
// immediate-subclass order. A hook must return an exact bool or NotImplemented.
func checkABCSubclass(caller *frame, instruction int, arguments []Value) (instructionOutcome, error) {
	class, candidate := arguments[0], arguments[1]
	if !isClassValue(candidate) {
		return raiseOutcome(newException("TypeError", "issubclass() arg 1 must be a class")), nil
	}
	return withABCData(caller, instruction, class, func(current *frame, data *abcData) (instructionOutcome, error) {
		if data.positive.contains(candidate) {
			return pushOutcome(current, instruction, trueSingleton)
		}
		if data.version != current.runtime.abcToken {
			data.negative.entries = nil
			data.version = current.runtime.abcToken
		} else if data.negative.contains(candidate) {
			return pushOutcome(current, instruction, falseSingleton)
		}
		call := &abcSubclassCall{instruction: instruction, class: class, candidate: candidate, data: data}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return abcMethod(current, instruction, class, "__subclasshook__", []Value{candidate})
		}, call.afterHook)
	})
}

type abcSubclassCall struct {
	instruction int
	class       Value
	candidate   Value
	data        *abcData
	registry    []weakClass
	index       int
	children    *listValue
}

func (call *abcSubclassCall) finish(caller *frame, result bool) (instructionOutcome, error) {
	if result {
		call.data.positive.add(call.candidate)
		return pushOutcome(caller, call.instruction, trueSingleton)
	}
	call.data.negative.add(call.candidate)
	return pushOutcome(caller, call.instruction, falseSingleton)
}

// afterHook caches exact answers and only applies nominal inheritance when the
// hook declines. Registry snapshots retain weak targets across Python callbacks.
func (call *abcSubclassCall) afterHook(caller *frame, result Value, exception *Exception) (instructionOutcome, error) {
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if result == trueSingleton || result == falseSingleton {
		return call.finish(caller, result == trueSingleton)
	}
	if result != notImplementedSingleton {
		return raiseOutcome(newException("AssertionError", "__subclasshook__ must return either False, True, or NotImplemented")), nil
	}
	matched, exception := subclassMatchesClass(call.candidate, call.class)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if matched {
		return call.finish(caller, true)
	}
	if call.data.registry.contains(call.candidate) {
		// CPython's direct registry hit deliberately does not fill the positive cache.
		return pushOutcome(caller, call.instruction, trueSingleton)
	}
	call.registry = call.data.registry.snapshot()
	return call.nextRegistered(caller)
}

// nextRegistered promotes only the current entry, allowing callbacks to change
// the registry without invalidating traversal or retaining every target class.
func (call *abcSubclassCall) nextRegistered(caller *frame) (instructionOutcome, error) {
	for call.index < len(call.registry) {
		registered := call.registry[call.index].value()
		call.index++
		if registered == nil {
			continue
		}
		return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			return abcIsSubclass(caller, call.instruction, call.candidate, registered)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if result == trueSingleton {
				return call.finish(current, true)
			}
			return call.nextRegistered(current)
		})
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return abcMethod(caller, call.instruction, call.class, "__subclasses__", nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		children, ok := result.(*listValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "__subclasses__() must return a list")), nil
		}
		call.children = children
		call.index = 0
		return call.nextChild(current)
	})
}

func (call *abcSubclassCall) nextChild(caller *frame) (instructionOutcome, error) {
	if call.index >= len(call.children.elements) {
		return call.finish(caller, false)
	}
	child := call.children.elements[call.index]
	call.index++
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return abcIsSubclass(caller, call.instruction, call.candidate, child)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == trueSingleton {
			return call.finish(current, true)
		}
		return call.nextChild(current)
	})
}

// checkABCInstance checks the reported __class__ first, then the actual type.
// Subclass overrides keep their return value; the outer isinstance resolves truth.
func checkABCInstance(caller *frame, instruction int, arguments []Value) (instructionOutcome, error) {
	class, instance := arguments[0], arguments[1]
	return withABCData(caller, instruction, class, func(current *frame, data *abcData) (instructionOutcome, error) {
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeDynamicAttributeLoad(current, instruction, instance, "__class__")
		}, func(resumed *frame, reported Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if data.positive.contains(reported) {
				return pushOutcome(resumed, instruction, trueSingleton)
			}
			actual, exception := typeOf(instance)
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if reported == actual {
				if data.version == resumed.runtime.abcToken && data.negative.contains(reported) {
					return pushOutcome(resumed, instruction, falseSingleton)
				}
				return abcMethod(resumed, instruction, class, "__subclasscheck__", []Value{reported})
			}
			return continueNativeOperation(resumed, instruction, func() (instructionOutcome, error) {
				return abcMethod(resumed, instruction, class, "__subclasscheck__", []Value{reported})
			}, func(checked *frame, result Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return continueNativeOperation(checked, instruction, func() (instructionOutcome, error) {
					return executeBuiltinBool(checked, instruction, len(checked.stack), []Value{result}, nil)
				}, func(ready *frame, truth Value, exception *Exception) (instructionOutcome, error) {
					if exception != nil {
						return raiseOutcome(exception), nil
					}
					if truth == trueSingleton {
						return pushOutcome(ready, instruction, result)
					}
					return abcMethod(ready, instruction, class, "__subclasscheck__", []Value{actual})
				})
			})
		})
	})
}

// defaultSubclassHook declines structural matching, as object.__subclasshook__ does.
func defaultSubclassHook() *builtinFunctionValue {
	return &builtinFunctionValue{name: "__subclasshook__", call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
		if keywords != nil && len(keywords.entries) != 0 {
			return nil, newException("TypeError", "__subclasshook__() takes no keyword arguments")
		}
		return notImplementedSingleton, nil
	}}
}
