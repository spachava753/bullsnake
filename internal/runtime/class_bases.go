package runtime

type classBasesCall struct {
	child    *frame
	original *tupleValue
	resolved []Value
	index    int
	changed  bool
}

// advance visits original bases once, skipping class objects and splicing each
// non-class hook result without recursively rewriting the returned bases.
func (call *classBasesCall) advance(caller *frame) (instructionOutcome, error) {
	instruction := call.child.classBuild.instruction
	for call.index < len(call.original.elements) {
		base := call.original.elements[call.index]
		call.index++
		if isClassValue(base) {
			call.resolved = append(call.resolved, base)
			continue
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			outcome, err := call.rewrite(caller, base)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	return call.finish(caller)
}

// rewrite resolves __mro_entries__ as an ordinary attribute, preserving Python
// descriptor failures and requiring an exact supported tuple result.
func (call *classBasesCall) rewrite(caller *frame, base Value) (instructionOutcome, error) {
	instruction := call.child.classBuild.instruction
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, base, "__mro_entries__")
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if !isAttributeError(exception) {
				return raiseOutcome(exception), nil
			}
			call.resolved = append(call.resolved, base)
			return pushOutcome(current, instruction, None)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, instruction, len(current.stack), method, []Value{call.original}, nil)
		}, func(ready *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			bases, ok := result.(*tupleValue)
			if !ok {
				return raiseOutcome(newException("TypeError", "__mro_entries__ must return a tuple")), nil
			}
			call.resolved = append(call.resolved, bases.elements...)
			call.changed = true
			return pushOutcome(ready, instruction, None)
		})
	})
}

func (call *classBasesCall) finish(caller *frame) (instructionOutcome, error) {
	build := call.child.classBuild
	bases, exceptionBase, nativeBase, objectBase, exception := resolveClassBases(call.resolved)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	metaclass, exception := selectMetaclass(build.metaclass, call.resolved)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	build.baseValues, build.bases = call.resolved, bases
	build.exceptionBase, build.nativeBase, build.objectBase = exceptionBase, nativeBase, objectBase
	build.metaclass = metaclass
	if call.changed {
		build.originalBases = call.original
	}
	return prepareClassBody(caller, call.child)
}
