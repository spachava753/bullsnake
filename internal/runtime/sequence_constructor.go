package runtime

import "strconv"

type collectionConstructorKind uint8

const (
	collectionList collectionConstructorKind = iota
	collectionTuple
	collectionSet
	collectionFrozenSet
	collectionDict
	collectionListExtend
	collectionStringJoin
	collectionSorted
)

type collectionConstructorCall struct {
	instruction int
	kind        collectionConstructorKind
	iterable    Value
	iterator    Value
	elements    []Value
	keywords    *dictValue
	list        *listValue
	separator   *stringValue
	sorting     *sortCall
}

// executeCollectionTypeCall validates list, tuple, set, or dict construction
// before starting an iterable collection that may suspend in Python code.
func executeCollectionTypeCall(
	caller *frame,
	instruction int,
	base int,
	class *nativeTypeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	kind := collectionList
	switch class {
	case tupleNativeType:
		kind = collectionTuple
	case setNativeType:
		kind = collectionSet
	case frozenSetNativeType:
		kind = collectionFrozenSet
	case dictNativeType:
		kind = collectionDict
	}
	if kind != collectionDict && keywords != nil && len(keywords.entries) != 0 {
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
	call := &collectionConstructorCall{
		instruction: instruction,
		kind:        kind,
		keywords:    keywords,
	}
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return finishCollectionConstructor(caller, call)
	}
	iterable := arguments[0]
	if kind == collectionTuple {
		if existing, sameType := iterable.(*tupleValue); sameType {
			discardCallSegment(caller, base)
			return pushOutcome(caller, instruction, existing)
		}
	}
	if kind == collectionFrozenSet {
		if existing, sameType := iterable.(*frozenSetValue); sameType {
			discardCallSegment(caller, base)
			return pushOutcome(caller, instruction, existing)
		}
	}
	if kind == collectionDict {
		if source, mapping := iterable.(*dictValue); mapping {
			for _, entry := range source.entries {
				call.elements = append(call.elements, &tupleValue{
					elements: []Value{entry.key, entry.value},
				})
			}
			discardCallSegment(caller, base)
			return finishCollectionConstructor(caller, call)
		}
	}
	call.iterable = iterable
	discardCallSegment(caller, base)
	return startCollectionConstructor(caller, call)
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
		return raiseOutcome(collectionIterableException(call)), nil
	}
	method, found := lookupInstanceSpecial(instance, "__iter__")
	if !found || method == None {
		return raiseOutcome(collectionIterableException(call)), nil
	}
	call.iterable = nil
	return executeIterationSpecial(frame, method, &iterationCall{
		kind:        iterationCollectionIterator,
		instruction: call.instruction,
		collection:  call,
	})
}

func collectionIterableException(call *collectionConstructorCall) *Exception {
	if call.kind == collectionStringJoin {
		return newException("TypeError", "can only join an iterable")
	}
	return newException(
		"TypeError",
		"'"+call.iterable.TypeName()+"' object is not iterable",
	)
}

func appendCollectionElement(call *collectionConstructorCall, value Value) {
	if call.kind == collectionListExtend {
		call.list.elements = append(call.list.elements, value)
		return
	}
	call.elements = append(call.elements, value)
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
			appendCollectionElement(call, value)
		}
	case *filterValue:
		request := &iterationCall{
			kind:        iterationCollectionNext,
			instruction: call.instruction,
			collection:  call,
		}
		return executeFilterNext(frame, &filterCall{
			filtering: iterator,
			request:   request,
		})
	case *mapValue:
		request := &iterationCall{
			kind:        iterationCollectionNext,
			instruction: call.instruction,
			collection:  call,
		}
		return executeMapNext(frame, &mapCall{
			mapping: iterator,
			request: request,
		})
	case *zipValue:
		request := &iterationCall{
			kind:        iterationCollectionNext,
			instruction: call.instruction,
			collection:  call,
		}
		return executeZipNext(frame, &zipCall{
			zipper:  iterator,
			request: request,
		})
	case *enumerateValue:
		request := &iterationCall{
			kind:        iterationCollectionNext,
			instruction: call.instruction,
			collection:  call,
		}
		return executeEnumerateNext(frame, &enumerateCall{
			enumeration: iterator,
			request:     request,
		})
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

// finishCollectionConstructor copies collected elements into the requested
// concrete collection and delegates dictionary pair validation.
func finishCollectionConstructor(
	frame *frame,
	call *collectionConstructorCall,
) (instructionOutcome, error) {
	elements := make([]Value, len(call.elements))
	copy(elements, call.elements)
	switch call.kind {
	case collectionListExtend:
		return pushOutcome(frame, call.instruction, None)
	case collectionStringJoin:
		return finishStringJoin(frame, call, elements)
	case collectionSorted:
		if call.sorting == nil {
			return instructionOutcome{}, frame.failure(
				call.instruction,
				"sorted collection has no continuation state",
			)
		}
		call.sorting.values = elements
		return executeTruthWithCall(frame, call.sorting.reverseValue, &truthCall{
			instruction: call.instruction,
			sortReverse: call.sorting,
		})
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
	case collectionFrozenSet:
		if len(elements) == 0 {
			return pushOutcome(frame, call.instruction, emptyFrozenSetSingleton)
		}
		set := &setValue{entries: make([]Value, 0, len(elements))}
		for _, element := range elements {
			if exception := set.add(element); exception != nil {
				return raiseOutcome(exception), nil
			}
		}
		return pushOutcome(frame, call.instruction, &frozenSetValue{entries: set.entries})
	case collectionDict:
		return finishDictConstructor(frame, call, elements)
	default:
		return pushOutcome(frame, call.instruction, &listValue{elements: elements})
	}
}

// finishDictConstructor validates each collected pair, applies it in order,
// then overlays keyword entries before returning the new dictionary.
func finishDictConstructor(
	frame *frame,
	call *collectionConstructorCall,
	elements []Value,
) (instructionOutcome, error) {
	dictionary := &dictValue{entries: make([]dictEntry, 0, len(elements))}
	for index, element := range elements {
		var pair []Value
		switch element := element.(type) {
		case *tupleValue:
			pair = element.elements
		case *listValue:
			pair = element.elements
		default:
			return raiseOutcome(newException(
				"TypeError",
				"cannot convert dictionary update sequence element #"+
					strconv.Itoa(index)+" to a sequence",
			)), nil
		}
		if len(pair) != 2 {
			return raiseOutcome(newException(
				"ValueError",
				"dictionary update sequence element #"+strconv.Itoa(index)+
					" has length "+strconv.Itoa(len(pair))+"; 2 is required",
			)), nil
		}
		if exception := dictionary.set(pair[0], pair[1]); exception != nil {
			return raiseOutcome(exception), nil
		}
	}
	if call.keywords != nil {
		for _, entry := range call.keywords.entries {
			if exception := dictionary.set(entry.key, entry.value); exception != nil {
				return raiseOutcome(exception), nil
			}
		}
	}
	return pushOutcome(frame, call.instruction, dictionary)
}
