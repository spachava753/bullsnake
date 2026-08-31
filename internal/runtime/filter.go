package runtime

import "strconv"

type filterValue struct {
	predicate Value
	iterator  Value
}

func (*filterValue) TypeName() string { return "filter" }
func (*filterValue) Repr() string     { return "<filter object>" }
func (*filterValue) isValue()         {}

type filterCall struct {
	filtering *filterValue
	request   *iterationCall
	item      Value
}

// executeFilterTypeCall creates a lazy predicate and source iterator pair,
// resolving user __iter__ through the ordinary iteration continuation.
func executeFilterTypeCall(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"filter() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 2 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"filter expected 2 arguments, got "+strconv.Itoa(len(arguments)),
		)), nil
	}
	filtering := &filterValue{predicate: arguments[0]}
	iterable := arguments[1]
	discardCallSegment(caller, base)
	if iterator, builtin := newIterator(iterable); builtin {
		filtering.iterator = iterator
		return pushOutcome(caller, instruction, filtering)
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
		kind:        iterationFilterIterator,
		instruction: instruction,
		filtering:   &filterCall{filtering: filtering},
	})
}

func finishFilterIterator(
	frame *frame,
	call *filterCall,
	iterator Value,
) (instructionOutcome, error) {
	if call == nil || call.filtering == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"filter iterator lookup has no constructor state",
		)
	}
	if !isIteratorValue(iterator) {
		return raiseOutcome(newException(
			"TypeError",
			"iter() returned non-iterator of type '"+iterator.TypeName()+"'",
		)), nil
	}
	call.filtering.iterator = iterator
	return pushOutcome(frame, frame.instruction-1, call.filtering)
}

// executeFilterNext pulls candidates until one passes or source execution must
// suspend, preserving the outer iterator request across every skipped item.
func executeFilterNext(frame *frame, call *filterCall) (instructionOutcome, error) {
	if call == nil || call.filtering == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"filter next has incomplete continuation state",
		)
	}
	switch iterator := call.filtering.iterator.(type) {
	case valueIterator:
		for {
			value, present, exception := iterator.next()
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !present {
				return finishFilterStop(frame, call)
			}
			if call.filtering.predicate == None && value != notImplementedSingleton {
				if truth, immediate := immediateTruth(value); immediate {
					if truth {
						return finishIterationCall(frame, call.request, value)
					}
					continue
				}
			}
			return executeFilterItem(frame, call, value)
		}
	case *enumerateValue:
		return executeEnumerateNext(frame, &enumerateCall{
			enumeration: iterator,
			request: &iterationCall{
				kind:        iterationFilterNext,
				instruction: call.request.instruction,
				filtering:   call,
			},
		})
	case *mapValue:
		return executeMapNext(frame, &mapCall{
			mapping: iterator,
			request: &iterationCall{
				kind:        iterationFilterNext,
				instruction: call.request.instruction,
				filtering:   call,
			},
		})
	case *filterValue:
		return executeFilterNext(frame, &filterCall{
			filtering: iterator,
			request: &iterationCall{
				kind:        iterationFilterNext,
				instruction: call.request.instruction,
				filtering:   call,
			},
		})
	case *zipValue:
		return executeZipNext(frame, &zipCall{
			zipper: iterator,
			request: &iterationCall{
				kind:        iterationFilterNext,
				instruction: call.request.instruction,
				filtering:   call,
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
			return finishFilterStop(frame, call)
		}
		return resumeGenerator(
			frame,
			call.request.instruction,
			iterator,
			None,
			generatorResume{
				kind:        generatorFilter,
				instruction: call.request.instruction,
				filtering:   call,
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
			kind:        iterationFilterNext,
			instruction: call.request.instruction,
			filtering:   call,
		})
	default:
		return instructionOutcome{}, frame.failure(
			call.request.instruction,
			"filter retained a non-iterator",
		)
	}
}

// executeFilterItem retains one candidate, evaluates the identity or callable
// predicate, and sends the predicate result through resumable truth testing.
func executeFilterItem(
	frame *frame,
	call *filterCall,
	item Value,
) (instructionOutcome, error) {
	call.item = item
	if call.filtering.predicate == None {
		return finishFilterPredicate(frame, call, item)
	}
	outcome, err := executeFunctionCall(
		frame,
		call.request.instruction,
		len(frame.stack),
		call.filtering.predicate,
		[]Value{item},
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.filtering = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			call.request.instruction,
			"filter predicate returned without a value",
		)
	}
	return finishFilterPredicate(frame, call, result)
}

func finishFilterPredicate(
	frame *frame,
	call *filterCall,
	result Value,
) (instructionOutcome, error) {
	return executeTruthWithCall(frame, result, &truthCall{
		instruction: call.request.instruction,
		original:    result,
		filtering:   call,
	})
}

func finishFilterTruth(
	frame *frame,
	call *filterCall,
	truth bool,
) (instructionOutcome, error) {
	if truth {
		item := call.item
		call.item = nil
		return finishIterationCall(frame, call.request, item)
	}
	call.item = nil
	return executeFilterNext(frame, call)
}

func finishFilterStop(
	frame *frame,
	call *filterCall,
) (instructionOutcome, error) {
	if call == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"filter exhaustion has no outer request",
		)
	}
	return finishIterationStop(frame, call.request, newStopIteration(None))
}

var _ Value = (*filterValue)(nil)
