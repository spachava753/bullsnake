package runtime

// initializeABC exposes the implemented class-setup helper. Registry and cache
// helpers remain absent until their weak-reference contracts are implemented.
func initializeABC(runtime *Runtime, module *Module) (*Exception, error) {
	module.globals.values["_abc_init"] = &builtinFunctionValue{name: "_abc_init", frameCall: executeABCInit}
	return nil, nil
}

type abcComputation struct {
	instruction int
	class       *typeValue
	entries     []dictEntry
	index       int
	bases       []*typeValue
	baseIndex   int
	iterator    Value
	sentinel    Value
	names       *setValue
}

// executeABCInit snapshots direct attributes before invoking their marker
// descriptors, then computes inherited abstract methods in direct-base order.
func executeABCInit(caller *frame, instruction int, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	if len(arguments) != 1 || (keywords != nil && len(keywords.entries) != 0) {
		return raiseOutcome(newException("TypeError", "_abc_init() takes exactly one positional argument")), nil
	}
	class, ok := arguments[0].(*typeValue)
	if !ok {
		return raiseOutcome(newException("TypeError", "_abc_init() requires a user class")), nil
	}
	call := &abcComputation{instruction: instruction, class: class, bases: class.bases, names: &setValue{}, sentinel: &dictValue{}}
	latest := make(map[string]int)
	for index, name := range class.namespaceOrder {
		latest[name] = index
	}
	for index, name := range class.namespaceOrder {
		if latest[name] != index {
			continue
		}
		if value, exists := class.namespace.get(name); exists {
			call.entries = append(call.entries, dictEntry{key: &stringValue{value: name}, value: value})
		}
	}
	return call.next(caller)
}

// next scans direct attributes before advancing each base's live iterator.
// The result is committed only after every attribute and truth callback succeeds.
func (call *abcComputation) next(caller *frame) (instructionOutcome, error) {
	if call.index < len(call.entries) {
		entry := call.entries[call.index]
		call.index++
		return call.check(caller, entry.key, entry.value)
	}
	if call.iterator != nil {
		return call.nextInherited(caller)
	}
	for call.baseIndex < len(call.bases) {
		base := call.bases[call.baseIndex]
		call.baseIndex++
		methods, exists := base.namespace.get("__abstractmethods__")
		if !exists {
			continue
		}
		return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			return executeBuiltinIter(caller, call.instruction, len(caller.stack), []Value{methods}, nil)
		}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			call.iterator = iterator
			return call.nextInherited(current)
		})
	}
	methods := &frozenSetValue{entries: call.names.entries}
	call.class.setAttribute("__abstractmethods__", methods)
	call.class.abstract = len(methods.entries) != 0
	return pushOutcome(caller, call.instruction, None)
}

func (call *abcComputation) check(caller *frame, name, value Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeAbstractMarker(caller, call.instruction, value)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == trueSingleton {
			if exception := call.names.add(name); exception != nil {
				return raiseOutcome(exception), nil
			}
		}
		return call.next(current)
	})
}

// nextInherited resolves one inherited name against the newly constructed class
// before requesting another name, preserving iterator and descriptor side effects.
func (call *abcComputation) nextInherited(caller *frame) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeBuiltinNext(caller, call.instruction, len(caller.stack), []Value{call.iterator, call.sentinel}, nil)
	}, func(current *frame, name Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if name == call.sentinel {
			call.iterator = nil
			return call.next(current)
		}
		text, ok := name.(*stringValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "attribute name must be string, not '"+name.TypeName()+"'")), nil
		}
		return continueNativeOperation(current, call.instruction, func() (instructionOutcome, error) {
			return executeDynamicAttributeLoad(current, call.instruction, call.class, text.value)
		}, func(resumed *frame, value Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				if isAttributeError(exception) {
					return call.next(resumed)
				}
				return raiseOutcome(exception), nil
			}
			return call.check(resumed, name, value)
		})
	})
}
