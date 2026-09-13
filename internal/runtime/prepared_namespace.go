package runtime

// executeNamespaceLookup calls a prepared dictionary subtype's lookup before
// falling back on KeyError. Other callback failures propagate unchanged.
func executeNamespaceLookup(caller *frame, instruction int, namespace *Namespace, name string, fallback func() (instructionOutcome, error)) (instructionOutcome, error) {
	if namespace.prepared == nil {
		if value, found := namespace.get(name); found {
			return pushOutcome(caller, instruction, value)
		}
		return fallback()
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return callPreparedNamespaceMethod(caller, instruction, namespace.prepared, "__getitem__", []Value{&stringValue{value: name}})
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if exception.class.isSubclassOf(keyErrorType) {
				return fallback()
			}
			return raiseOutcome(exception), nil
		}
		return pushOutcome(current, instruction, value)
	})
}

func executeGlobalName(caller *frame, instruction int, name string) (instructionOutcome, error) {
	value, found := caller.globals.get(name)
	if !found {
		value, found = caller.builtins.get(name)
	}
	if !found {
		return raiseOutcome(newException("NameError", "name '"+name+"' is not defined")), nil
	}
	return pushOutcome(caller, instruction, value)
}

func callPreparedNamespaceMethod(caller *frame, instruction int, namespace *instanceValue, name string, arguments []Value) (instructionOutcome, error) {
	method, found := lookupInstanceSpecial(namespace, name)
	if !found {
		return raiseOutcome(newException("TypeError", "prepared dictionary lacks "+name)), nil
	}
	return executeFunctionCall(caller, instruction, len(caller.stack), method, arguments, nil)
}

// executePreparedMutation discards the mapping method's return value and maps a
// missing deletion to NameError without pre-reading or duplicating a mutation.
func executePreparedMutation(caller *frame, instruction int, name string, value Value, deleting bool) (instructionOutcome, error) {
	method := "__setitem__"
	arguments := []Value{&stringValue{value: name}, value}
	if deleting {
		method = "__delitem__"
		arguments = arguments[:1]
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return callPreparedNamespaceMethod(caller, instruction, caller.locals.prepared, method, arguments)
	}, func(_ *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if deleting && exception.class.isSubclassOf(keyErrorType) {
				exception = newException("NameError", "name '"+name+"' is not defined")
			}
			return raiseOutcome(exception), nil
		}
		return instructionOutcome{kind: advance}, nil
	})
}

// finishClassBody publishes the two possible compiler bindings through the same
// namespace slots before invoking the metaclass with the original mapping.
func finishClassBody(caller *frame, build *classBuild, cell Value) (instructionOutcome, error) {
	var entries []dictEntry
	if build.originalBases != nil {
		entries = append(entries, dictEntry{key: &stringValue{value: "__orig_bases__"}, value: build.originalBases})
	}
	if _, ok := cell.(*cellValue); ok {
		entries = append(entries, dictEntry{key: &stringValue{value: "__classcell__"}, value: cell})
	}
	return finishPreparedBindings(caller, build, cell, entries)
}

// finishPreparedBindings has at most two immediate steps; Python setters still
// suspend through VM continuations instead of recursing on the Go stack.
func finishPreparedBindings(caller *frame, build *classBuild, cell Value, entries []dictEntry) (instructionOutcome, error) {
	if len(entries) == 0 {
		return callPreparedClass(caller, build, cell)
	}
	entry := entries[0]
	return continueNativeOperation(caller, build.instruction, func() (instructionOutcome, error) {
		if namespace, ok := build.prepared.(*instanceValue); ok {
			return callPreparedNamespaceMethod(caller, build.instruction, namespace, "__setitem__", []Value{entry.key, entry.value})
		}
		if exception := build.dictionary.set(entry.key, entry.value); exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, build.instruction, None)
	}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return finishPreparedBindings(current, build, cell, entries[1:])
	})
}
