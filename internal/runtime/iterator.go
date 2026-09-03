package runtime

import (
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type valueIterator interface {
	Value
	next() (Value, bool, *Exception)
}

type generatorValue struct {
	frame       *frame
	running     bool
	done        bool
	needsResume bool
}

func (*generatorValue) TypeName() string { return "generator" }
func (*generatorValue) Repr() string     { return "<generator object>" }
func (*generatorValue) isValue()         {}

func (generator *generatorValue) attribute(name string) (Value, bool) {
	switch name {
	case "throw":
		return nativeFunctionNamed("generator.throw", 1, 3,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				exception, ok := arguments[0].(*Exception)
				if !ok {
					return nil, newException("TypeError", "exceptions must derive from BaseException"), nil
				}
				return throwGenerator(caller.runtime, caller, generator, exception)
			}), true
	case "close":
		return nativeFunctionNamed("generator.close", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				generator.running = false
				generator.done = true
				generator.frame.previous = nil
				return None, nil, nil
			}), true
	case "__iter__":
		return nativeFunctionNamed("generator.__iter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return generator, nil, nil
			}), true
	default:
		return nil, false
	}
}

func (*generatorValue) next() (Value, bool, *Exception) {
	return nil, false, newException("RuntimeError", "generator must be resumed by the VM")
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

// next decodes one string code point or returns one integer byte from either
// immutable bytes or a bytearray.
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
	case *bytearrayValue:
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
func newIterator(value Value) (valueIterator, bool) {
	switch value := value.(type) {
	case valueIterator:
		return value, true
	case *tupleValue, *listValue:
		return &sequenceIterator{sequence: value}, true
	case *dequeValue:
		return &sequenceIterator{sequence: &listValue{elements: value.elements}}, true
	case *stringValue, *bytesValue, *bytearrayValue:
		return &textIterator{text: value}, true
	case *dictViewValue:
		return &dictViewIterator{view: value}, true
	case *namespaceValue:
		dictionary := value.dictionary()
		return &collectionIterator{
			collection: dictionary,
			length:     len(dictionary.entries),
			version:    dictionary.version,
		}, true
	case *rangeValue:
		return newRangeIterator(value), true
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
		return &sequenceIterator{
			sequence: &tupleValue{elements: value.entries},
		}, true
	default:
		return nil, false
	}
}

// newIteratorForFrame extends concrete iteration with a synchronous Python
// __iter__ callback for native builtins and GET_ITER.
func newIteratorForFrame(
	caller *frame,
	iterable Value,
) (valueIterator, *Exception, error) {
	if iterator, ok := newIterator(iterable); ok {
		return iterator, nil, nil
	}
	instance, ok := iterable.(*instanceValue)
	if !ok {
		return nil, nil, nil
	}
	if instance.class.builtinBase == builtinTypeNamed("list") {
		if instance.sequence == nil {
			instance.sequence = &listValue{}
		}
		return &sequenceIterator{sequence: instance.sequence}, nil, nil
	}
	if instance.mapping != nil {
		return &collectionIterator{
			collection: instance.mapping,
			length:     len(instance.mapping.entries),
			version:    instance.mapping.version,
		}, nil, nil
	}
	if instance.tuple != nil {
		return &sequenceIterator{sequence: instance.tuple}, nil, nil
	}
	method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__iter__")
	if !found {
		return nil, nil, nil
	}
	if err != nil || exception != nil {
		return nil, exception, err
	}
	result, exception, err := callValueSynchronously(
		caller, method, nil,
	)
	if err != nil || exception != nil {
		return nil, exception, err
	}
	iterator, ok := newIterator(result)
	if !ok {
		if result != iterable {
			if nested, nestedException, nestedErr := newIteratorForFrame(caller, result); nestedErr != nil || nestedException != nil {
				return nil, nestedException, nestedErr
			} else if nested != nil {
				return nested, nil, nil
			}
		}
		return nil, newException("TypeError", "iter() returned non-iterator"), nil
	}
	return iterator, nil, nil
}

func newIteratorForRuntime(
	runtimeState *Runtime,
	iterable Value,
) (valueIterator, *Exception, error) {
	prepared, err := runtimeState.prepare(nativeCallbackCode)
	if err != nil {
		return nil, nil, err
	}
	caller := &frame{
		runtime: runtimeState, code: prepared, instruction: 1,
		locals: newNamespace(), globals: newNamespace(), builtins: runtimeState.builtins,
	}
	return newIteratorForFrame(caller, iterable)
}

func executeGetIter(frame *frame, index int) (instructionOutcome, error) {
	iterable, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	iterator, exception, err := newIteratorForFrame(frame, iterable)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if iterator == nil {
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

var generatorDrainCode = bytecode.NewCode(bytecode.CodeSpec{
	Name:          "<generator drain>",
	QualifiedName: "<generator drain>",
	StackSize:     2,
	Instructions: []bytecode.Instruction{
		{Opcode: bytecode.LoadConst},
		{Opcode: bytecode.ForIter, Operand: 5},
		{Opcode: bytecode.Swap, Operand: 2},
		{Opcode: bytecode.PopTop},
		{Opcode: bytecode.ReturnValue},
		{Opcode: bytecode.LoadConst},
		{Opcode: bytecode.ReturnValue},
	},
	Positions: make([]lexer.Span, 7),
	Constants: []bytecode.Constant{bytecode.None()},
})

// resumeGeneratorFrom drives one generator until its next yield or completion,
// attaching a validated FOR_ITER caller and an optional logical call link.
func resumeGeneratorFrom(
	runtimeState *Runtime,
	logicalCaller *frame,
	generator *generatorValue,
) (Value, bool, *Exception, error) {
	if generator.done {
		return nil, false, nil, nil
	}
	prepared, err := runtimeState.prepare(generatorDrainCode)
	if err != nil {
		return nil, false, nil, err
	}
	collector := &frame{
		runtime:         runtimeState,
		code:            prepared,
		stack:           []Value{generator},
		locals:          newNamespace(),
		globals:         newNamespace(),
		builtins:        runtimeState.builtins,
		instruction:     2,
		logicalPrevious: logicalCaller,
	}
	if generator.needsResume {
		if !generator.frame.push(None) {
			return nil, false, nil, generator.frame.failure(
				generator.frame.instruction, "operand stack overflow while resuming generator",
			)
		}
		generator.needsResume = false
	}
	generator.running = true
	generator.frame.previous = collector
	result, raised, err := execute(&threadState{current: generator.frame})
	if err != nil {
		return nil, false, nil, err
	}
	if raised != nil {
		generator.running = false
		generator.frame.previous = nil
		return nil, false, raised.exception, nil
	}
	if generator.done {
		return nil, false, nil, nil
	}
	return result, true, nil, nil
}

// throwGenerator injects an exception at the suspended yield expression and
// drives the generator until it yields, exits, or propagates an exception.
func throwGenerator(
	runtimeState *Runtime,
	logicalCaller *frame,
	generator *generatorValue,
	exception *Exception,
) (Value, *Exception, error) {
	if generator.done {
		return nil, exception, nil
	}
	if generator.running {
		return nil, newException("ValueError", "generator already executing"), nil
	}
	prepared, err := runtimeState.prepare(generatorDrainCode)
	if err != nil {
		return nil, nil, err
	}
	collector := &frame{
		runtime:         runtimeState,
		code:            prepared,
		stack:           []Value{generator},
		locals:          newNamespace(),
		globals:         newNamespace(),
		builtins:        runtimeState.builtins,
		instruction:     2,
		logicalPrevious: logicalCaller,
	}
	generator.needsResume = false
	generator.running = true
	generator.frame.previous = collector
	thread := &threadState{current: generator.frame}
	unhandled, routeErr := routeException(
		thread, generator.frame, generator.frame.instruction-1, exception, false,
	)
	if routeErr != nil {
		return nil, nil, routeErr
	}
	if unhandled != nil {
		return nil, unhandled.exception, nil
	}
	result, raised, executeErr := execute(thread)
	if executeErr != nil {
		return nil, nil, executeErr
	}
	if raised != nil {
		return nil, raised.exception, nil
	}
	if generator.done {
		return nil, newException("StopIteration", ""), nil
	}
	return result, nil, nil
}

func nextNativeIterator(
	runtimeState *Runtime,
	iterator valueIterator,
) (Value, bool, *Exception, error) {
	if generator, ok := iterator.(*generatorValue); ok {
		return resumeGeneratorFrom(runtimeState, nil, generator)
	}
	value, available, exception := iterator.next()
	return value, available, exception, nil
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
		if generator.done {
			frame.pop()
			frame.instruction = target
			return instructionOutcome{kind: advance}, nil
		}
		if generator.running {
			return instructionOutcome{
				kind:      raised,
				exception: newException("ValueError", "generator already executing"),
			}, nil
		}
		if generator.needsResume {
			if !generator.frame.push(None) {
				return instructionOutcome{}, generator.frame.failure(
					generator.frame.instruction,
					"operand stack overflow while resuming generator",
				)
			}
			generator.needsResume = false
		}
		generator.running = true
		generator.frame.previous = frame
		return instructionOutcome{kind: called, frame: generator.frame}, nil
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

// executeGetAIter calls an instance's __aiter__ special method and leaves its
// asynchronous iterator on the operand stack.
func executeGetAIter(frame *frame, index int) (instructionOutcome, error) {
	iterable, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	instance, ok := iterable.(*instanceValue)
	if !ok {
		return instructionOutcome{kind: raised, exception: newException(
			"TypeError", "'"+iterable.TypeName()+"' object is not an async iterable",
		)}, nil
	}
	method, found, exception, err := lookupBoundSpecialMethod(frame, instance, "__aiter__")
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if !found {
		return instructionOutcome{kind: raised, exception: newException(
			"TypeError", "'"+iterable.TypeName()+"' object is not an async iterable",
		)}, nil
	}
	iterator, exception, err := callValueSynchronously(frame, method, nil)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return pushOutcome(frame, index, iterator)
}

// executeAsyncForIter drives one __anext__ awaitable to completion. The
// interpreter's event loop is deterministic, so the operation can complete
// synchronously while preserving Python coroutine and StopAsyncIteration
// semantics.
func executeAsyncForIter(
	frame *frame,
	index int,
	target int,
) (instructionOutcome, error) {
	if len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	iterator := frame.stack[len(frame.stack)-1]
	instance, ok := iterator.(*instanceValue)
	if !ok {
		return instructionOutcome{kind: raised, exception: newException(
			"TypeError", "async iterator has no __anext__ method",
		)}, nil
	}
	method, found, exception, err := lookupBoundSpecialMethod(frame, instance, "__anext__")
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if !found {
		return instructionOutcome{kind: raised, exception: newException(
			"TypeError", "async iterator has no __anext__ method",
		)}, nil
	}
	awaitable, exception, err := callValueSynchronously(frame, method, nil)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception == nil {
		coroutine, coroutineOK := awaitable.(*coroutineValue)
		if !coroutineOK {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "async for received an invalid object from __anext__",
			)}, nil
		}
		awaitable, exception, err = runCoroutineSynchronously(frame, coroutine)
		if err != nil {
			return instructionOutcome{}, err
		}
	}
	if exception != nil {
		if exception.class == stopAsyncIterationType ||
			(exception.userClass != nil && exception.userClass.builtinExceptionBase() == stopAsyncIterationType) {
			frame.pop()
			frame.instruction = target
			return instructionOutcome{kind: advance}, nil
		}
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return pushOutcome(frame, index, awaitable)
}
