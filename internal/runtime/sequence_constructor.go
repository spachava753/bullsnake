package runtime

import "strconv"

type collectionConstructorKind uint8

const (
	collectionList collectionConstructorKind = iota
	collectionTuple
	collectionSet
)

type collectionConstructorCall struct {
	instruction int
	kind        collectionConstructorKind
	iterable    Value
	iterator    Value
	elements    []Value
}

// executeCollectionTypeCall validates list, tuple, or set construction before
// starting an iterable collection that may suspend in Python iterator code.
func executeCollectionTypeCall(
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
	kind := collectionList
	switch class {
	case tupleNativeType:
		kind = collectionTuple
	case setNativeType:
		kind = collectionSet
	}
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return finishCollectionConstructor(caller, &collectionConstructorCall{
			instruction: instruction,
			kind:        kind,
		})
	}
	iterable := arguments[0]
	if kind == collectionTuple {
		if existing, sameType := iterable.(*tupleValue); sameType {
			discardCallSegment(caller, base)
			return pushOutcome(caller, instruction, existing)
		}
	}
	discardCallSegment(caller, base)
	return startCollectionConstructor(caller, &collectionConstructorCall{
		instruction: instruction,
		kind:        kind,
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
	switch call.kind {
	case collectionTuple:
		return pushOutcome(frame, call.instruction, &tupleValue{elements: elements})
	case collectionSet:
		set := &setValue{entries: make([]Value, 0, len(elements))}
		for _, element := range elements {
			if exception := set.add(element); exception != nil {
				return raiseOutcome(exception), nil
			}
		}
		return pushOutcome(frame, call.instruction, set)
	default:
		return pushOutcome(frame, call.instruction, &listValue{elements: elements})
	}
}
