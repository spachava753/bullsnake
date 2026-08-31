package runtime

import "strconv"

type collectionConstructorCall struct {
	instruction int
	tuple       bool
	iterable    Value
	iterator    Value
	elements    []Value
}

// executeSequenceTypeCall validates list or tuple construction before starting
// an iterable collection that may suspend in Python iterator code.
func executeSequenceTypeCall(
	caller *frame,
	instruction int,
	base int,
	class *nativeTypeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			class.name+"() takes no keyword arguments",
		)), nil
	}
	if len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			class.name+" expected at most 1 argument, got "+strconv.Itoa(len(arguments)),
		)), nil
	}
	tuple := class == tupleNativeType
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return finishCollectionConstructor(caller, &collectionConstructorCall{
			instruction: instruction,
			tuple:       tuple,
		})
	}
	iterable := arguments[0]
	if tuple {
		if existing, sameType := iterable.(*tupleValue); sameType {
			discardCallSegment(caller, base)
			return pushOutcome(caller, instruction, existing)
		}
	}
	discardCallSegment(caller, base)
	return startCollectionConstructor(caller, &collectionConstructorCall{
		instruction: instruction,
		tuple:       tuple,
		iterable:    iterable,
	})
}

func startCollectionConstructor(
	frame *frame,
	call *collectionConstructorCall,
) (instructionOutcome, error) {
	if iterator, builtin := newIterator(call.iterable); builtin {
		call.iterator = iterator
		call.iterable = nil
		return continueCollectionConstructor(frame, call)
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
		kind:        iterationCollectionIterator,
		instruction: call.instruction,
		collection:  call,
	})
}

// continueCollectionConstructor drains native iterators directly and suspends
// for generator or user __next__ execution when Python code must run.
func continueCollectionConstructor(
	frame *frame,
	call *collectionConstructorCall,
) (instructionOutcome, error) {
	switch iterator := call.iterator.(type) {
	case valueIterator:
		for {
			value, present, exception := iterator.next()
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !present {
				return finishCollectionConstructor(frame, call)
			}
			call.elements = append(call.elements, value)
		}
	case *generatorValue:
		if iterator.kind != generatorObject {
			return raiseOutcome(newException(
				"TypeError",
				"'"+iterator.TypeName()+"' object is not an iterator",
			)), nil
		}
		if iterator.state == generatorCompleted {
			return finishCollectionConstructor(frame, call)
		}
		return resumeGenerator(
			frame,
			call.instruction,
			iterator,
			None,
			generatorResume{
				kind:        generatorCollection,
				instruction: call.instruction,
				collection:  call,
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
			kind:        iterationCollectionNext,
			instruction: call.instruction,
			collection:  call,
		})
	default:
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"collection constructor retained a non-iterator",
		)
	}
}

func finishCollectionConstructor(
	frame *frame,
	call *collectionConstructorCall,
) (instructionOutcome, error) {
	elements := make([]Value, len(call.elements))
	copy(elements, call.elements)
	if call.tuple {
		return pushOutcome(frame, call.instruction, &tupleValue{elements: elements})
	}
	return pushOutcome(frame, call.instruction, &listValue{elements: elements})
}
