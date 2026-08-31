package runtime

type valueIterator interface {
	Value
	next() (Value, bool, *Exception)
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
	if _, ok := iterator.text.(*stringValue); ok {
		return "str_iterator"
	}
	return "bytes_iterator"
}
func (iterator *textIterator) Repr() string {
	return "<" + iterator.TypeName() + " object>"
}
func (*textIterator) isValue() {}

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
	case *generatorValue:
		if value.kind == generatorObject {
			return value, true
		}
		return nil, false
	case *tupleValue, *listValue:
		return &sequenceIterator{sequence: value}, true
	case *stringValue, *bytesValue:
		return &textIterator{text: value}, true
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
	default:
		return nil, false
	}
}

func executeGetIter(frame *frame, index int) (instructionOutcome, error) {
	iterable, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	iterator, ok := newIterator(iterable)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+iterable.TypeName()+"' object is not iterable",
			),
		}, nil
	}
	return pushOutcome(frame, index, iterator)
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
	if generator, ok := value.(*generatorValue); ok {
		return resumeGeneratorIteration(frame, index, target, generator)
	}
	iterator, ok := value.(valueIterator)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+value.TypeName()+"' object is not an iterator",
			),
		}, nil
	}
	next, ok, exception := iterator.next()
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if !ok {
		frame.pop()
		frame.instruction = target
		return instructionOutcome{kind: advance}, nil
	}
	return pushOutcome(frame, index, next)
}
