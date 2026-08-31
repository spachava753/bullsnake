package runtime

import (
	"math/big"
	"strconv"
)

type enumerateValue struct {
	iterator Value
	index    big.Int
}

func (*enumerateValue) TypeName() string { return "enumerate" }
func (*enumerateValue) Repr() string     { return "<enumerate object>" }
func (*enumerateValue) isValue()         {}

type enumerateCall struct {
	enumeration *enumerateValue
	request     *iterationCall
	instruction int
}

// executeEnumerateTypeCall binds the iterable and optional start argument,
// resolves the inner iterator, and retains arbitrary-precision indexes.
func executeEnumerateTypeCall(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	iterable, start, exception := bindEnumerateArguments(arguments, keywords)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	index, ok := integerOperand(start)
	if !ok {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"'"+start.TypeName()+"' object cannot be interpreted as an integer",
		)), nil
	}

	enumeration := &enumerateValue{index: index}
	discardCallSegment(caller, base)
	if iterator, builtin := newIterator(iterable); builtin {
		enumeration.iterator = iterator
		return pushOutcome(caller, instruction, enumeration)
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
		kind:        iterationEnumerateIterator,
		instruction: instruction,
		enumeration: &enumerateCall{
			enumeration: enumeration,
			instruction: instruction,
		},
	})
}

// bindEnumerateArguments accepts the two Argument Clinic names and reports
// duplicate, unknown, missing, and excessive arguments before iterator lookup.
func bindEnumerateArguments(
	arguments []Value,
	keywords *dictValue,
) (Value, Value, *Exception) {
	total := len(arguments)
	if keywords != nil {
		total += len(keywords.entries)
	}
	if total > 2 {
		return nil, nil, newException(
			"TypeError",
			"enumerate() takes at most 2 arguments ("+strconv.Itoa(total)+" given)",
		)
	}

	var iterable Value
	start := Value(&intValue{})
	iterableSet := false
	startSet := false
	if len(arguments) >= 1 {
		iterable = arguments[0]
		iterableSet = true
	}
	if len(arguments) == 2 {
		start = arguments[1]
		startSet = true
	}
	var entries []dictEntry
	if keywords != nil {
		entries = keywords.entries
	}
	for _, entry := range entries {
		name := entry.key.(*stringValue).value
		switch name {
		case "iterable":
			if iterableSet {
				return nil, nil, newException(
					"TypeError",
					"enumerate() got multiple values for argument 'iterable'",
				)
			}
			iterable = entry.value
			iterableSet = true
		case "start":
			if startSet {
				return nil, nil, newException(
					"TypeError",
					"enumerate() got multiple values for argument 'start'",
				)
			}
			start = entry.value
			startSet = true
		default:
			return nil, nil, newException(
				"TypeError",
				"'"+name+"' is an invalid keyword argument for enumerate()",
			)
		}
	}
	if !iterableSet {
		return nil, nil, newException(
			"TypeError",
			"enumerate() missing required argument 'iterable'",
		)
	}
	return iterable, start, nil
}

func finishEnumerateIterator(
	frame *frame,
	call *enumerateCall,
	iterator Value,
) (instructionOutcome, error) {
	if call == nil || call.enumeration == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"enumerate iterator lookup has no constructor state",
		)
	}
	if !isIteratorValue(iterator) {
		return raiseOutcome(newException(
			"TypeError",
			"iter() returned non-iterator of type '"+iterator.TypeName()+"'",
		)), nil
	}
	call.enumeration.iterator = iterator
	return pushOutcome(frame, call.instruction, call.enumeration)
}

// executeEnumerateNext advances the retained inner iterator and finishes the
// outer for-loop or next() request only after an item or exhaustion is known.
func executeEnumerateNext(
	frame *frame,
	call *enumerateCall,
) (instructionOutcome, error) {
	if call == nil || call.enumeration == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"enumerate next has incomplete continuation state",
		)
	}
	switch iterator := call.enumeration.iterator.(type) {
	case valueIterator:
		for {
			value, present, exception := iterator.next()
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !present {
				return finishEnumerateStop(frame, call)
			}
			pair := call.enumeration.nextPair(value)
			if call.request.kind != iterationCollectionNext {
				return finishIterationCall(frame, call.request, pair)
			}
			if call.request.collection == nil {
				return instructionOutcome{}, frame.failure(
					call.request.instruction,
					"enumerate collection has no constructor state",
				)
			}
			appendCollectionElement(call.request.collection, pair)
		}
	case *generatorValue:
		if iterator.kind != generatorObject {
			return raiseOutcome(newException(
				"TypeError",
				"'"+iterator.TypeName()+"' object is not an iterator",
			)), nil
		}
		if iterator.state == generatorCompleted {
			return finishEnumerateStop(frame, call)
		}
		return resumeGenerator(
			frame,
			call.request.instruction,
			iterator,
			None,
			generatorResume{
				kind:        generatorEnumerate,
				instruction: call.request.instruction,
				enumeration: call,
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
			kind:        iterationEnumerateNext,
			instruction: call.request.instruction,
			enumeration: call,
		})
	default:
		return instructionOutcome{}, frame.failure(
			call.request.instruction,
			"enumerate retained a non-iterator",
		)
	}
}

func finishEnumerateItem(
	frame *frame,
	call *enumerateCall,
	value Value,
) (instructionOutcome, error) {
	if call == nil || call.enumeration == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"enumerate item has incomplete continuation state",
		)
	}
	pair := call.enumeration.nextPair(value)
	return finishIterationCall(frame, call.request, pair)
}

func (enumeration *enumerateValue) nextPair(value Value) *tupleValue {
	var index big.Int
	index.Set(&enumeration.index)
	enumeration.index.Add(&enumeration.index, big.NewInt(1))
	return &tupleValue{elements: []Value{&intValue{value: index}, value}}
}

func finishEnumerateStop(
	frame *frame,
	call *enumerateCall,
) (instructionOutcome, error) {
	if call == nil || call.request == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"enumerate exhaustion has no outer request",
		)
	}
	return finishIterationStop(frame, call.request, newStopIteration(None))
}

var _ Value = (*enumerateValue)(nil)
