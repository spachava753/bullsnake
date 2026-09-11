package runtime

import "strconv"

type valueIterator interface {
	Value
	next() (Value, bool, *Exception)
}

type iterationCallKind uint8

const (
	iterationGetIterator iterationCallKind = iota
	iterationForNext
	iterationBuiltinNext
	iterationCollectionIterator
	iterationCollectionNext
	iterationEnumerateIterator
	iterationEnumerateNext
	iterationTruthAggregateIterator
	iterationTruthAggregateNext
	iterationMapIterator
	iterationMapNext
	iterationFilterIterator
	iterationFilterNext
	iterationZipIterator
	iterationZipNext
)

type iterationCall struct {
	kind           iterationCallKind
	instruction    int
	target         int
	iterator       Value
	defaultValue   Value
	hasDefault     bool
	collection     *collectionConstructorCall
	enumeration    *enumerateCall
	aggregate      *truthAggregateCall
	mapping        *mapCall
	filtering      *filterCall
	zipConstructor *zipConstructorCall
	zipping        *zipCall
}

type sequenceIterator struct {
	sequence Value
	index    int
}

func (iterator *sequenceIterator) TypeName() string {
	switch iterator.sequence.(type) {
	case *tupleValue:
		return "tuple_iterator"
	case *listValue:
		return "list_iterator"
	default:
		return "iterator"
	}
}
func (iterator *sequenceIterator) Repr() string {
	return "<" + iterator.TypeName() + " object>"
}
func (*sequenceIterator) isValue() {}

func (iterator *sequenceIterator) next() (Value, bool, *Exception) {
	var elements []Value
	switch sequence := iterator.sequence.(type) {
	case *tupleValue:
		elements = sequence.elements
	case *listValue:
		elements = sequence.elements
	default:
		return nil, false, nil
	}
	if iterator.index >= len(elements) {
		return nil, false, nil
	}
	value := elements[iterator.index]
	iterator.index++
	return value, true, nil
}

type textIterator struct {
	text   Value
	offset int
}

func (iterator *textIterator) TypeName() string {
	if _, ok := iterator.text.(*bytearrayValue); ok {
		return "bytearray_iterator"
	}
	if _, ok := iterator.text.(*stringValue); ok {
		return "str_iterator"
	}
	return "bytes_iterator"
}
func (iterator *textIterator) Repr() string {
	return "<" + iterator.TypeName() + " object>"
}
func (*textIterator) isValue() {}

// next advances by a code point for strings and by one byte for binary values,
// consulting mutable bytearray storage on each request.
func (iterator *textIterator) next() (Value, bool, *Exception) {
	switch text := iterator.text.(type) {
	case *stringValue:
		if iterator.offset >= len(text.value) {
			return nil, false, nil
		}
		_, size, _ := decodeStringRune(text.value[iterator.offset:])
		start := iterator.offset
		iterator.offset += size
		return &stringValue{value: text.value[start:iterator.offset]}, true, nil
	case *bytearrayValue:
		if iterator.offset >= len(text.buffer.data) {
			return nil, false, nil
		}
		result := newByteInteger(text.buffer.data[iterator.offset])
		iterator.offset++
		return result, true, nil
	case *bytesValue:
		if iterator.offset >= len(text.value) {
			return nil, false, nil
		}
		value := newByteInteger(text.value[iterator.offset])
		iterator.offset++
		return value, true, nil
	default:
		return nil, false, nil
	}
}

type collectionIterator struct {
	collection Value
	index      int
	length     int
	version    uint64
	failure    *Exception
}

func (iterator *collectionIterator) TypeName() string {
	switch iterator.collection.(type) {
	case *dictValue:
		return "dict_keyiterator"
	case *setValue:
		return "set_iterator"
	case *frozenSetValue:
		return "set_iterator"
	default:
		return "iterator"
	}
}
func (iterator *collectionIterator) Repr() string {
	return "<" + iterator.TypeName() + " object>"
}
func (*collectionIterator) isValue() {}

// next checks structural collection mutations before yielding the next stored
// key or element, and keeps mutation failures sticky for later calls.
func (iterator *collectionIterator) next() (Value, bool, *Exception) {
	if iterator.failure != nil {
		return nil, false, iterator.failure
	}
	switch collection := iterator.collection.(type) {
	case *dictValue:
		if len(collection.entries) != iterator.length {
			iterator.failure = newException(
				"RuntimeError",
				"dictionary changed size during iteration",
			)
			return nil, false, iterator.failure
		}
		if collection.version != iterator.version {
			iterator.failure = newException(
				"RuntimeError",
				"dictionary keys changed during iteration",
			)
			return nil, false, iterator.failure
		}
		if iterator.index >= len(collection.entries) {
			return nil, false, nil
		}
		value := collection.entries[iterator.index].key
		iterator.index++
		return value, true, nil
	case *setValue:
		if len(collection.entries) != iterator.length {
			iterator.failure = newException(
				"RuntimeError",
				"Set changed size during iteration",
			)
			return nil, false, iterator.failure
		}
		if iterator.index >= len(collection.entries) {
			return nil, false, nil
		}
		value := collection.entries[iterator.index]
		iterator.index++
		return value, true, nil
	case *frozenSetValue:
		if iterator.index >= len(collection.entries) {
			return nil, false, nil
		}
		value := collection.entries[iterator.index]
		iterator.index++
		return value, true, nil
	default:
		return nil, false, nil
	}
}

// newIterator returns self-iterators unchanged and selects the concrete iterator
// whose element contract matches each currently iterable built-in value.
func newIterator(value Value) (Value, bool) {
	switch value := value.(type) {
	case valueIterator:
		return value, true
	case *enumerateValue:
		return value, true
	case *mapValue:
		return value, true
	case *filterValue:
		return value, true
	case *zipValue:
		return value, true
	case *generatorValue:
		if value.kind == generatorObject {
			return value, true
		}
		return nil, false
	case *frameLocalsProxy:
		snapshot := value.snapshot()
		return &collectionIterator{collection: snapshot, length: len(snapshot.entries), version: snapshot.version}, true
	case *mappingProxyValue:
		return &collectionIterator{collection: value.dictionary, length: len(value.dictionary.entries), version: value.dictionary.version}, true
	case *dictionaryKeysView:
		return &collectionIterator{
			collection: value.dictionary,
			length:     len(value.dictionary.entries),
			version:    value.dictionary.version,
		}, true
	case *dictionaryItemsView:
		return &dictionaryItemsIterator{
			dictionary: value.dictionary,
			length:     len(value.dictionary.entries),
			version:    value.dictionary.version,
		}, true
	case *dictionaryValuesView:
		return &dictionaryValuesIterator{
			dictionary: value.dictionary,
			length:     len(value.dictionary.entries),
			version:    value.dictionary.version,
		}, true
	case *tupleValue, *listValue:
		return &sequenceIterator{sequence: value}, true
	case *rangeValue:
		return newRangeIterator(value), true
	case *stringValue, *bytesValue, *bytearrayValue:
		return &textIterator{text: value}, true
	case *templateValue:
		return &templateIterator{template: value}, true
	case *dictValue:
		return &collectionIterator{
			collection: value,
			length:     len(value.entries),
			version:    value.version,
		}, true
	case *setValue:
		return &collectionIterator{
			collection: value,
			length:     len(value.entries),
		}, true
	case *frozenSetValue:
		return &collectionIterator{
			collection: value,
			length:     len(value.entries),
		}, true
	default:
		return nil, false
	}
}

// executeBuiltinIter exposes the existing iterator lookup through the one-
// argument builtin while reserving callable-sentinel iteration for a later slice.
func executeBuiltinIter(
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
			"iter() takes no keyword arguments",
		)), nil
	}
	if len(arguments) < 1 || len(arguments) > 2 {
		message := "iter expected at least 1 argument, got 0"
		if len(arguments) > 2 {
			message = "iter expected at most 2 arguments, got " +
				strconv.Itoa(len(arguments))
		}
		discardCallSegment(caller, base)
		return raiseOutcome(newException("TypeError", message)), nil
	}
	if len(arguments) == 2 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"NotImplementedError",
			"iter() callable-sentinel form is not supported",
		)), nil
	}
	iterable := arguments[0]
	discardCallSegment(caller, base)
	return executeIteratorLookup(caller, instruction, iterable)
}

// executeIteratorLookup creates built-in iterators immediately or suspends for
// a class __iter__ method before validating the returned iterator.
func executeIteratorLookup(
	frame *frame,
	index int,
	iterable Value,
) (instructionOutcome, error) {
	if iterator, builtin := newIterator(iterable); builtin {
		return pushOutcome(frame, index, iterator)
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
	return executeIterationSpecial(
		frame,
		method,
		&iterationCall{kind: iterationGetIterator, instruction: index},
	)
}

// executeForIter retains the iterator while yielding and removes it before the
// exhaustion jump, matching the compiler's two stack-depth edges.
func executeForIter(
	frame *frame,
	index int,
	target int,
) (instructionOutcome, error) {
	if len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	value := frame.stack[len(frame.stack)-1]
	if filtering, ok := value.(*filterValue); ok {
		frame.pop()
		request := &iterationCall{
			kind:        iterationForNext,
			instruction: index,
			target:      target,
			iterator:    filtering,
		}
		return executeFilterNext(frame, &filterCall{
			filtering: filtering,
			request:   request,
		})
	}
	if mapping, ok := value.(*mapValue); ok {
		frame.pop()
		request := &iterationCall{
			kind:        iterationForNext,
			instruction: index,
			target:      target,
			iterator:    mapping,
		}
		return executeMapNext(frame, &mapCall{
			mapping: mapping,
			request: request,
		})
	}
	if zipping, ok := value.(*zipValue); ok {
		frame.pop()
		request := &iterationCall{
			kind:        iterationForNext,
			instruction: index,
			target:      target,
			iterator:    zipping,
		}
		return executeZipNext(frame, &zipCall{
			zipper:  zipping,
			request: request,
		})
	}
	if enumeration, ok := value.(*enumerateValue); ok {
		frame.pop()
		request := &iterationCall{
			kind:        iterationForNext,
			instruction: index,
			target:      target,
			iterator:    enumeration,
		}
		return executeEnumerateNext(frame, &enumerateCall{
			enumeration: enumeration,
			request:     request,
		})
	}
	if generator, ok := value.(*generatorValue); ok {
		return resumeGeneratorIteration(frame, index, target, generator)
	}
	if iterator, ok := value.(valueIterator); ok {
		next, present, exception := iterator.next()
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !present {
			frame.pop()
			frame.instruction = target
			return instructionOutcome{kind: advance}, nil
		}
		return pushOutcome(frame, index, next)
	}
	instance, ok := value.(*instanceValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+value.TypeName()+"' object is not an iterator",
			),
		}, nil
	}
	frame.pop()
	method, found := lookupInstanceSpecial(instance, "__next__")
	if !found || method == None {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+value.TypeName()+"' object is not an iterator",
			),
		}, nil
	}
	return executeIterationSpecial(
		frame,
		method,
		&iterationCall{
			kind:        iterationForNext,
			instruction: index,
			target:      target,
			iterator:    value,
		},
	)
}

// executeIterationSpecial consumes the completed special-method operation,
// including any native continuations, before applying iteration semantics.
func executeIterationSpecial(caller *frame, method Value, call *iterationCall) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, call.instruction, len(caller.stack), method, nil, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			switch call.kind {
			case iterationGetIterator, iterationEnumerateIterator, iterationTruthAggregateIterator,
				iterationMapIterator, iterationFilterIterator, iterationZipIterator:
			default:
				if isStopIteration(exception) {
					return finishIterationStop(current, call, exception)
				}
			}
			return raiseOutcome(exception), nil
		}
		return finishIterationCall(current, call, result)
	})
}

// finishIterationCall checks __iter__ results or restores the stack result for
// a suspended FOR_ITER or next() operation.
func finishIterationCall(
	frame *frame,
	call *iterationCall,
	result Value,
) (instructionOutcome, error) {
	switch call.kind {
	case iterationGetIterator:
		if !isIteratorValue(result) {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					"iter() returned non-iterator of type '"+result.TypeName()+"'",
				),
			}, nil
		}
		return pushOutcome(frame, call.instruction, result)
	case iterationForNext:
		if len(frame.stack)+2 > frame.code.stackSize {
			return instructionOutcome{}, frame.failure(
				call.instruction,
				"operand stack overflow while returning iterator item",
			)
		}
		frame.stack = append(frame.stack, call.iterator, result)
		return instructionOutcome{kind: advance}, nil
	case iterationBuiltinNext:
		return pushOutcome(frame, call.instruction, result)
	case iterationCollectionIterator:
		if call.collection == nil {
			return instructionOutcome{}, frame.failure(
				call.instruction,
				"collection iterator lookup has no constructor state",
			)
		}
		if !isIteratorValue(result) {
			return raiseOutcome(newException(
				"TypeError",
				"iter() returned non-iterator of type '"+result.TypeName()+"'",
			)), nil
		}
		call.collection.iterator = result
		return continueCollectionConstructor(frame, call.collection)
	case iterationCollectionNext:
		if call.collection == nil {
			return instructionOutcome{}, frame.failure(
				call.instruction,
				"collection next call has no constructor state",
			)
		}
		appendCollectionElement(call.collection, result)
		return continueCollectionConstructor(frame, call.collection)
	case iterationEnumerateIterator:
		return finishEnumerateIterator(frame, call.enumeration, result)
	case iterationEnumerateNext:
		return finishEnumerateItem(frame, call.enumeration, result)
	case iterationTruthAggregateIterator:
		return finishTruthAggregateIterator(frame, call.aggregate, result)
	case iterationTruthAggregateNext:
		return executeTruthAggregateItem(frame, call.aggregate, result)
	case iterationMapIterator:
		return finishMapIterator(frame, call.mapping, result)
	case iterationMapNext:
		return executeMappedCall(frame, call.mapping, result)
	case iterationFilterIterator:
		return finishFilterIterator(frame, call.filtering, result)
	case iterationFilterNext:
		return executeFilterItem(frame, call.filtering, result)
	case iterationZipIterator:
		return finishZipIterator(frame, call.zipConstructor, result)
	case iterationZipNext:
		return finishZipItem(frame, call.zipping, result)
	default:
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"unknown iteration continuation",
		)
	}
}

// finishIterationStop applies loop, next builtin, or collection-constructor
// exhaustion semantics to one StopIteration from a user __next__ call.
func finishIterationStop(
	frame *frame,
	call *iterationCall,
	exception *Exception,
) (instructionOutcome, error) {
	switch call.kind {
	case iterationForNext:
		frame.instruction = call.target
		return instructionOutcome{kind: advance}, nil
	case iterationBuiltinNext:
		if call.hasDefault {
			return pushOutcome(frame, call.instruction, call.defaultValue)
		}
		return instructionOutcome{kind: raised, exception: exception}, nil
	case iterationCollectionNext:
		if call.collection == nil {
			return instructionOutcome{}, frame.failure(
				call.instruction,
				"collection exhaustion has no constructor state",
			)
		}
		return finishCollectionConstructor(frame, call.collection)
	case iterationEnumerateNext:
		return finishEnumerateStop(frame, call.enumeration)
	case iterationTruthAggregateNext:
		return finishTruthAggregateStop(frame, call.aggregate)
	case iterationMapNext:
		return finishMapStop(frame, call.mapping)
	case iterationFilterNext:
		return finishFilterStop(frame, call.filtering)
	case iterationZipNext:
		return finishZipStop(frame, call.zipping)
	default:
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
}

// isIteratorValue recognizes native iterators, synchronous generators, and user
// instances whose class provides an enabled __next__ method.
func isIteratorValue(value Value) bool {
	switch value := value.(type) {
	case valueIterator:
		return true
	case *enumerateValue:
		return true
	case *mapValue:
		return true
	case *filterValue:
		return true
	case *zipValue:
		return true
	case *generatorValue:
		return value.kind == generatorObject
	case *instanceValue:
		next, found := value.class.lookup("__next__")
		return found && next != None
	default:
		return false
	}
}
