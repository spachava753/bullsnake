package runtime

type mapValue struct {
	callable Value
	iterator Value
}

func (*mapValue) TypeName() string { return "map" }
func (*mapValue) Repr() string     { return "<map object>" }
func (*mapValue) isValue()         {}

type mapCall struct {
	mapping *mapValue
	request *iterationCall
}

// executeMapTypeCall creates the currently supported one-iterable lazy map and
// resolves its source iterator before returning the map object.
func executeMapTypeCall(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"NotImplementedError",
			"map keyword arguments are not supported",
		)), nil
	}
	if len(arguments) < 2 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"map() must have at least two arguments.",
		)), nil
	}
	if len(arguments) > 2 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"NotImplementedError",
			"map with multiple iterables is not supported",
		)), nil
	}
	mapping := &mapValue{callable: arguments[0]}
	iterable := arguments[1]
	discardCallSegment(caller, base)
	if iterator, builtin := newIterator(iterable); builtin {
		mapping.iterator = iterator
		return pushOutcome(caller, instruction, mapping)
	}
	instance, ok := iterable.(*instanceValue)
	if !ok {
		return raiseOutcome(newException(
			"TypeError",
			"'"+iterable.TypeName()+"' object is not iterable",
		)), nil
	}
	method, found := lookupInstanceSpecial(instance, "__iter__")
	if !found || method == None {
		return raiseOutcome(newException(
			"TypeError",
			"'"+iterable.TypeName()+"' object is not iterable",
		)), nil
	}
	return executeIterationSpecial(caller, method, &iterationCall{
		kind:        iterationMapIterator,
		instruction: instruction,
		mapping:     &mapCall{mapping: mapping},
	})
}

func finishMapIterator(
	frame *frame,
	call *mapCall,
	iterator Value,
) (instructionOutcome, error) {
	if call == nil || call.mapping == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"map iterator lookup has no constructor state",
		)
	}
	if !isIteratorValue(iterator) {
		return raiseOutcome(newException(
			"TypeError",
			"iter() returned non-iterator of type '"+iterator.TypeName()+"'",
		)), nil
	}
	call.mapping.iterator = iterator
	return pushOutcome(frame, frame.instruction-1, call.mapping)
}

// executeMapNext pulls one source value and runs the mapped callable only after
// the source reports a successful item.
func executeMapNext(frame *frame, call *mapCall) (instructionOutcome, error) {
	if call == nil || call.mapping == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"map next has incomplete continuation state",
		)
	}
	switch iterator := call.mapping.iterator.(type) {
	case valueIterator:
		value, present, exception := iterator.next()
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if !present {
			return finishMapStop(frame, call)
		}
		return executeMappedCall(frame, call, value)
	case *enumerateValue:
		return executeEnumerateNext(frame, &enumerateCall{
			enumeration: iterator,
			request: &iterationCall{
				kind:        iterationMapNext,
				instruction: call.request.instruction,
				mapping:     call,
			},
		})
	case *mapValue:
		return executeMapNext(frame, &mapCall{
			mapping: iterator,
			request: &iterationCall{
				kind:        iterationMapNext,
				instruction: call.request.instruction,
				mapping:     call,
			},
		})
	case *filterValue:
		return executeFilterNext(frame, &filterCall{
			filtering: iterator,
			request: &iterationCall{
				kind:        iterationMapNext,
				instruction: call.request.instruction,
				mapping:     call,
			},
		})
	case *zipValue:
		return executeZipNext(frame, &zipCall{
			zipper: iterator,
			request: &iterationCall{
				kind:        iterationMapNext,
				instruction: call.request.instruction,
				mapping:     call,
			},
		})
	case *generatorValue:
		if iterator.kind != generatorObject {
			return raiseOutcome(newException(
				"TypeError",
				"'"+iterator.TypeName()+"' object is not an iterator",
			)), nil
		}
		if iterator.state == generatorCompleted {
			return finishMapStop(frame, call)
		}
		return resumeGenerator(
			frame,
			call.request.instruction,
			iterator,
			None,
			generatorResume{
				kind:        generatorMap,
				instruction: call.request.instruction,
				mapping:     call,
			},
		)
	case *instanceValue:
		method, found := lookupInstanceSpecial(iterator, "__next__")
		if !found || method == None {
			return raiseOutcome(newException(
				"TypeError",
				"'"+iterator.TypeName()+"' object is not an iterator",
			)), nil
		}
		return executeIterationSpecial(frame, method, &iterationCall{
			kind:        iterationMapNext,
			instruction: call.request.instruction,
			mapping:     call,
		})
	default:
		return instructionOutcome{}, frame.failure(
			call.request.instruction,
			"map retained a non-iterator",
		)
	}
}

func executeMappedCall(
	frame *frame,
	call *mapCall,
	value Value,
) (instructionOutcome, error) {
	outcome, err := executeFunctionCall(
		frame,
		call.request.instruction,
		len(frame.stack),
		call.mapping.callable,
		[]Value{value},
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.mapping = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			call.request.instruction,
			"map callable returned without a value",
		)
	}
	return finishMapItem(frame, call, result)
}

func finishMapItem(
	frame *frame,
	call *mapCall,
	result Value,
) (instructionOutcome, error) {
	if call == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"map result has no outer request",
		)
	}
	return finishIterationCall(frame, call.request, result)
}

func finishMapStop(
	frame *frame,
	call *mapCall,
) (instructionOutcome, error) {
	if call == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"map exhaustion has no outer request",
		)
	}
	return finishIterationStop(frame, call.request, newStopIteration(None))
}

var _ Value = (*mapValue)(nil)
