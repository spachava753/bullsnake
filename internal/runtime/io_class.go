package runtime

// ioState is private instance storage for Go-backed I/O classes. Class identity,
// attributes, C3 lookup, and descriptors use the ordinary Python object model.
type ioState struct {
	wrapper  *textWrapper
	pair     *bufferedPair
	buffered *bufferedStream
	decoder  *newlineDecoder
	binary   *bytesIOState
	closed   bool
	text     *stringIOValue
	view     *memoryView
}

func newIOClass(name string, base *typeValue) *typeValue {
	class := newBuiltinClass("_io", name, base)
	class.ioClass = true
	class.setAttribute("__doc__", &stringValue{value: "Base class for I/O streams."})
	return class
}

// executeIOTypeCall allocates private state before invoking the selected
// initializer through normal descriptor lookup, including Python overrides.
func executeIOTypeCall(caller *frame, instruction, base int, class *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	instance := &instanceValue{class: class, attributes: newNamespace(), io: &ioState{}}
	for _, parent := range class.mro {
		if parent.bytesIOClass {
			instance.io.binary = &bytesIOState{buffer: &byteBuffer{}}
			break
		}
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, instance, "__init__")
	}, func(current *frame, initializer Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, instruction, len(current.stack), initializer, arguments, keywords)
		}, func(resumed *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if result != None {
				return raiseOutcome(newException("TypeError", "__init__() should return None, not '"+result.TypeName()+"'")), nil
			}
			return pushOutcome(resumed, instruction, instance)
		})
	})
}

// ioMethod creates an actual instance method descriptor. It validates the
// receiver before passing instance state to the implementation.
func ioMethod(class *typeValue, name string, call func(*frame, int, *instanceValue, []Value, *dictValue) (instructionOutcome, error)) Value {
	return nativeInstanceMethod(class, name, call)
}

func bindInstanceFunction(value, receiver Value) Value {
	if native, ok := value.(*builtinFunctionValue); ok && native.method {
		return &boundMethodValue{callable: value, self: receiver}
	}
	if _, ok := value.(*functionValue); ok {
		return &boundMethodValue{callable: value, self: receiver}
	}
	return value
}
