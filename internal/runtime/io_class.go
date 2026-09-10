package runtime

// ioState is private instance storage for Go-backed I/O classes. Class identity,
// attributes, C3 lookup, and descriptors use the ordinary Python object model.
type ioState struct {
	closed bool
	text   *stringIOValue
}

func newIOClass(name string, base *typeValue) *typeValue {
	class := &typeValue{name: name, qualifiedName: name, module: "_io", namespace: newNamespace(), ioClass: true, immutable: true}
	class.mro = []*typeValue{class}
	if base != nil {
		class.bases = []*typeValue{base}
		class.mro = append(class.mro, base.mro...)
		base.subclasses.entries = append(base.subclasses.entries, makeWeakClass(class))
	} else {
		class.objectBase = true
	}
	class.setAttribute("__doc__", &stringValue{value: "Base class for I/O streams."})
	return class
}

// executeIOTypeCall allocates private state before invoking the selected
// initializer through normal descriptor lookup, including Python overrides.
func executeIOTypeCall(caller *frame, instruction, base int, class *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	instance := &instanceValue{class: class, attributes: newNamespace(), io: &ioState{}}
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
	return &builtinFunctionValue{name: name, method: true, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		if len(arguments) == 0 {
			return raiseOutcome(newException("TypeError", "unbound method "+name+"() needs an argument")), nil
		}
		self, ok := arguments[0].(*instanceValue)
		if !ok || self.io == nil || !self.class.isSubclassOf(class) {
			return raiseOutcome(newException("TypeError", "descriptor '"+name+"' requires a '"+class.name+"' object")), nil
		}
		return call(caller, instruction, self, arguments[1:], keywords)
	}}
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
