package runtime

import "strconv"

type truthAggregateCall struct {
	instruction int
	iterable    Value
	iterator    Value
	stopTruth   bool
	exhausted   bool
}

// executeBuiltinAll starts an aggregate that returns False on the first false
// item and True when its iterator is exhausted.
func executeBuiltinAll(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	return executeBuiltinTruthAggregate(
		caller,
		instruction,
		base,
		arguments,
		keywords,
		"all",
		false,
		true,
	)
}

// executeBuiltinAny starts an aggregate that returns True on the first true
// item and False when its iterator is exhausted.
func executeBuiltinAny(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	return executeBuiltinTruthAggregate(
		caller,
		instruction,
		base,
		arguments,
		keywords,
		"any",
		true,
		false,
	)
}

func executeBuiltinTruthAggregate(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
	name string,
	stopTruth bool,
	exhausted bool,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			name+"() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			name+"() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	call := &truthAggregateCall{
		instruction: instruction,
		iterable:    arguments[0],
		stopTruth:   stopTruth,
		exhausted:   exhausted,
	}
	discardCallSegment(caller, base)
	return startTruthAggregate(caller, call)
}

func startTruthAggregate(
	frame *frame,
	call *truthAggregateCall,
) (instructionOutcome, error) {
	if iterator, builtin := newIterator(call.iterable); builtin {
		call.iterator = iterator
		call.iterable = nil
		return continueTruthAggregate(frame, call)
	}
	instance, ok := call.iterable.(*instanceValue)
	if !ok {
		return raiseOutcome(newException(
			"TypeError",
			"'"+call.iterable.TypeName()+"' object is not iterable",
		)), nil
	}
	method, found := lookupInstanceSpecial(instance, "__iter__")
	if !found || method == None {
		return raiseOutcome(newException(
			"TypeError",
			"'"+call.iterable.TypeName()+"' object is not iterable",
		)), nil
	}
	call.iterable = nil
	return executeIterationSpecial(frame, method, &iterationCall{
		kind:        iterationTruthAggregateIterator,
		instruction: call.instruction,
		aggregate:   call,
	})
}

func finishTruthAggregateIterator(
	frame *frame,
	call *truthAggregateCall,
	iterator Value,
) (instructionOutcome, error) {
	if call == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"truth aggregate iterator lookup has no state",
		)
	}
	if !isIteratorValue(iterator) {
		return raiseOutcome(newException(
			"TypeError",
			"iter() returned non-iterator of type '"+iterator.TypeName()+"'",
		)), nil
	}
	call.iterator = iterator
	return continueTruthAggregate(frame, call)
}

// continueTruthAggregate drains native iterators in one Go loop and suspends
// only when the inner iterator or an item's truth method runs Python code.
func continueTruthAggregate(
	frame *frame,
	call *truthAggregateCall,
) (instructionOutcome, error) {
	switch iterator := call.iterator.(type) {
	case valueIterator:
		for {
			value, present, exception := iterator.next()
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !present {
				return finishTruthAggregateStop(frame, call)
			}
			if value == notImplementedSingleton {
				return raiseOutcome(newException(
					"TypeError",
					"NotImplemented should not be used in a boolean context",
				)), nil
			}
			if truth, immediate := immediateTruth(value); immediate {
				if truth == call.stopTruth {
					return finishTruthAggregateTruth(frame, call, truth)
				}
				continue
			}
			return executeTruthAggregateItem(frame, call, value)
		}
	case *filterValue:
		return executeFilterNext(frame, &filterCall{
			filtering: iterator,
			request: &iterationCall{
				kind:        iterationTruthAggregateNext,
				instruction: call.instruction,
				aggregate:   call,
			},
		})
	case *mapValue:
		return executeMapNext(frame, &mapCall{
			mapping: iterator,
			request: &iterationCall{
				kind:        iterationTruthAggregateNext,
				instruction: call.instruction,
				aggregate:   call,
			},
		})
	case *zipValue:
		return executeZipNext(frame, &zipCall{
			zipper: iterator,
			request: &iterationCall{
				kind:        iterationTruthAggregateNext,
				instruction: call.instruction,
				aggregate:   call,
			},
		})
	case *enumerateValue:
		return executeEnumerateNext(frame, &enumerateCall{
			enumeration: iterator,
			request: &iterationCall{
				kind:        iterationTruthAggregateNext,
				instruction: call.instruction,
				aggregate:   call,
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
			return finishTruthAggregateStop(frame, call)
		}
		return resumeGenerator(
			frame,
			call.instruction,
			iterator,
			None,
			generatorResume{
				kind:        generatorTruthAggregate,
				instruction: call.instruction,
				aggregate:   call,
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
			kind:        iterationTruthAggregateNext,
			instruction: call.instruction,
			aggregate:   call,
		})
	default:
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"truth aggregate retained a non-iterator",
		)
	}
}

func executeTruthAggregateItem(
	frame *frame,
	call *truthAggregateCall,
	value Value,
) (instructionOutcome, error) {
	return executeTruthWithCall(frame, value, &truthCall{
		instruction: call.instruction,
		original:    value,
		aggregate:   call,
	})
}

func finishTruthAggregateTruth(
	frame *frame,
	call *truthAggregateCall,
	truth bool,
) (instructionOutcome, error) {
	if truth == call.stopTruth {
		return pushOutcome(frame, call.instruction, booleanValue(truth))
	}
	return continueTruthAggregate(frame, call)
}

func finishTruthAggregateStop(
	frame *frame,
	call *truthAggregateCall,
) (instructionOutcome, error) {
	if call == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"truth aggregate exhaustion has no state",
		)
	}
	return pushOutcome(frame, call.instruction, booleanValue(call.exhausted))
}

func booleanValue(value bool) Value {
	if value {
		return trueSingleton
	}
	return falseSingleton
}
