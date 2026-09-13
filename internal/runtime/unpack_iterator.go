package runtime

type unpackCall struct {
	instruction int
	before      int
	after       int
	starred     bool
	iterator    Value
	elements    []Value
}

// start resolves the iterable without rewriting errors raised by a user hook.
// Only an absent or disabled iteration slot gets the unpack-specific diagnostic.
func (call *unpackCall) start(caller *frame, source Value) (instructionOutcome, error) {
	if iterator, native := newIterator(source); native {
		call.iterator = iterator
		return call.advance(caller)
	}
	method, found := lookupIterationSpecial(source, "__iter__")
	if !found || method == None {
		return raiseOutcome(newException("TypeError", "cannot unpack non-iterable "+source.TypeName()+" object")), nil
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeIterationSpecial(caller, method, &iterationCall{kind: iterationGetIterator, instruction: call.instruction})
	}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call.iterator = iterator
		return call.advance(current)
	})
}

// advance pulls only the required fixed prefix and one excess item for ordinary
// unpacking. Native steps loop; suspended Python steps resume on the VM frame.
func (call *unpackCall) advance(caller *frame) (instructionOutcome, error) {
	for {
		if call.starred && len(call.elements) == call.before {
			return call.collectRemainder(caller)
		}
		suspended, complete := false, false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := executeBuiltinNext(caller, call.instruction, len(caller.stack), []Value{call.iterator}, nil)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				if !isStopIteration(exception) {
					return raiseOutcome(exception), nil
				}
				complete = true
				return call.finish(current)
			}
			call.elements = append(call.elements, value)
			if !call.starred && len(call.elements) > call.before {
				complete = true
				return call.finish(current)
			}
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance || complete {
			return outcome, err
		}
		caller.pop()
	}
}

// collectRemainder uses ordinary list consumption after the leading targets,
// including a second __iter__ call on the retained iterator for starred unpacking.
func (call *unpackCall) collectRemainder(caller *frame) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return startCollectionConstructor(caller, &collectionConstructorCall{instruction: call.instruction, kind: collectionList, iterable: call.iterator})
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call.elements = append(call.elements, value.(*listValue).elements...)
		return call.finish(current)
	})
}

// finish reuses exact-sequence validation and stack publication. No assignment
// target has been stored while iterator calls or arity checks can still fail.
func (call *unpackCall) finish(caller *frame) (instructionOutcome, error) {
	if !caller.push(&tupleValue{elements: call.elements}) {
		return instructionOutcome{}, caller.failure(call.instruction, "operand stack overflow")
	}
	if call.starred {
		return executeUnpackEx(caller, call.instruction, call.before, call.after)
	}
	return executeUnpackSequence(caller, call.instruction, call.before)
}
