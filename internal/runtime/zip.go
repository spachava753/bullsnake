package runtime

type zipValue struct {
	iterators []Value
}

func (*zipValue) TypeName() string { return "zip" }
func (*zipValue) Repr() string     { return "<zip object>" }
func (*zipValue) isValue()         {}

type zipConstructorCall struct {
	zipper      *zipValue
	iterables   []Value
	index       int
	instruction int
}

type zipCall struct {
	zipper  *zipValue
	request *iterationCall
	index   int
	items   []Value
}

// executeZipTypeCall copies all positional sources before resolving their
// iterators from left to right. Strict mode remains a separate feature.
func executeZipTypeCall(
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
			"zip keyword arguments are not supported",
		)), nil
	}
	iterables := append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	return continueZipConstruction(caller, &zipConstructorCall{
		zipper:      &zipValue{},
		iterables:   iterables,
		instruction: instruction,
	})
}

// continueZipConstruction resolves immediate iterators in a loop and suspends
// only when a user __iter__ method must run.
func continueZipConstruction(
	frame *frame,
	call *zipConstructorCall,
) (instructionOutcome, error) {
	for call.index < len(call.iterables) {
		iterable := call.iterables[call.index]
		if iterator, builtin := newIterator(iterable); builtin {
			call.zipper.iterators = append(call.zipper.iterators, iterator)
			call.index++
			continue
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
		return executeIterationSpecial(frame, method, &iterationCall{
			kind:           iterationZipIterator,
			instruction:    call.instruction,
			zipConstructor: call,
		})
	}
	return pushOutcome(frame, call.instruction, call.zipper)
}

func finishZipIterator(
	frame *frame,
	call *zipConstructorCall,
	iterator Value,
) (instructionOutcome, error) {
	if call == nil || call.zipper == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"zip iterator lookup has no constructor state",
		)
	}
	if !isIteratorValue(iterator) {
		return raiseOutcome(newException(
			"TypeError",
			"iter() returned non-iterator of type '"+iterator.TypeName()+"'",
		)), nil
	}
	call.zipper.iterators = append(call.zipper.iterators, iterator)
	call.index++
	return continueZipConstruction(frame, call)
}

// executeZipNext pulls one item from each retained iterator in source order.
// Exhausting any source discards a partial tuple and exhausts this request.
func executeZipNext(frame *frame, call *zipCall) (instructionOutcome, error) {
	if call == nil || call.zipper == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"zip next has incomplete continuation state",
		)
	}
	if len(call.zipper.iterators) == 0 {
		return finishZipStop(frame, call)
	}
	for call.index < len(call.zipper.iterators) {
		iterator := call.zipper.iterators[call.index]
		switch iterator := iterator.(type) {
		case valueIterator:
			value, present, exception := iterator.next()
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !present {
				return finishZipStop(frame, call)
			}
			call.items = append(call.items, value)
			call.index++
		case *enumerateValue:
			return executeEnumerateNext(frame, &enumerateCall{
				enumeration: iterator,
				request:     zipIterationRequest(call),
			})
		case *mapValue:
			return executeMapNext(frame, &mapCall{
				mapping: iterator,
				request: zipIterationRequest(call),
			})
		case *filterValue:
			return executeFilterNext(frame, &filterCall{
				filtering: iterator,
				request:   zipIterationRequest(call),
			})
		case *zipValue:
			return executeZipNext(frame, &zipCall{
				zipper:  iterator,
				request: zipIterationRequest(call),
			})
		case *generatorValue:
			if iterator.kind != generatorObject {
				return raiseOutcome(newException(
					"TypeError",
					"'"+iterator.TypeName()+"' object is not an iterator",
				)), nil
			}
			if iterator.state == generatorCompleted {
				return finishZipStop(frame, call)
			}
			return resumeGenerator(
				frame,
				call.request.instruction,
				iterator,
				None,
				generatorResume{
					kind:        generatorZip,
					instruction: call.request.instruction,
					zipping:     call,
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
			return executeIterationSpecial(frame, method, zipIterationRequest(call))
		default:
			return instructionOutcome{}, frame.failure(
				call.request.instruction,
				"zip retained a non-iterator",
			)
		}
	}
	items := append([]Value(nil), call.items...)
	return finishIterationCall(frame, call.request, &tupleValue{elements: items})
}

func zipIterationRequest(call *zipCall) *iterationCall {
	return &iterationCall{
		kind:        iterationZipNext,
		instruction: call.request.instruction,
		zipping:     call,
	}
}

func finishZipItem(
	frame *frame,
	call *zipCall,
	value Value,
) (instructionOutcome, error) {
	if call == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"zip item has no continuation state",
		)
	}
	call.items = append(call.items, value)
	call.index++
	return executeZipNext(frame, call)
}

func finishZipStop(
	frame *frame,
	call *zipCall,
) (instructionOutcome, error) {
	if call == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"zip exhaustion has no outer request",
		)
	}
	return finishIterationStop(frame, call.request, newStopIteration(None))
}

var _ Value = (*zipValue)(nil)
