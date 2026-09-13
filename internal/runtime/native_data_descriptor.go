package runtime

type nativeDataDescriptorValue struct {
	class  *nativeTypeValue
	name   string
	member bool
	get    func(*frame, int, Value) (instructionOutcome, error)
	set    func(Value, Value) (instructionOutcome, error)
}

func (descriptor *nativeDataDescriptorValue) TypeName() string {
	if descriptor.member {
		return "member_descriptor"
	}
	return "getset_descriptor"
}
func (descriptor *nativeDataDescriptorValue) Repr() string {
	kind := "attribute"
	if descriptor.member {
		kind = "member"
	}
	return "<" + kind + " '" + descriptor.name + "' of '" + descriptor.class.name + "' objects>"
}
func (*nativeDataDescriptorValue) isValue() {}

// executeNativeDataDescriptorAttribute supplies receiver-checked access to the
// same native field operations used by ordinary attribute instructions.
func executeNativeDataDescriptorAttribute(caller *frame, instruction int, descriptor *nativeDataDescriptorValue, name string) (instructionOutcome, error) {
	switch name {
	case "__name__":
		return pushOutcome(caller, instruction, &stringValue{value: descriptor.name})
	case "__objclass__":
		return pushOutcome(caller, instruction, descriptor.class)
	case "__get__", "__set__", "__delete__":
		return pushOutcome(caller, instruction, &builtinFunctionValue{name: name, frameCall: func(current *frame, at, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			minimum, maximum := 1, 2
			if name == "__set__" {
				minimum = 2
			} else if name == "__delete__" {
				maximum = 1
			}
			if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
				discardCallSegment(current, base)
				return raiseOutcome(exception), nil
			}
			arguments = append([]Value(nil), arguments...)
			discardCallSegment(current, base)
			if name == "__get__" && arguments[0] == None {
				if len(arguments) == 1 || arguments[1] == None {
					return raiseOutcome(newException("TypeError", "__get__(None, None) is invalid")), nil
				}
				return pushOutcome(current, at, descriptor)
			}
			if !nativeReceiverMatches(arguments[0], descriptor.class) {
				return raiseOutcome(newException("TypeError", "descriptor '"+descriptor.name+"' requires a '"+descriptor.class.name+"' object")), nil
			}
			if name == "__get__" {
				return descriptor.load(current, at, arguments[0])
			}
			var value Value
			if name == "__set__" {
				value = arguments[1]
			}
			outcome, err := descriptor.store(arguments[0], value)
			if err != nil || outcome.kind != advance {
				return outcome, err
			}
			return pushOutcome(current, at, None)
		}})
	}
	return raiseOutcome(newException("AttributeError", "descriptor has no attribute '"+name+"'")), nil
}

func (descriptor *nativeDataDescriptorValue) load(caller *frame, instruction int, receiver Value) (instructionOutcome, error) {
	if !nativeReceiverMatches(receiver, descriptor.class) {
		return raiseOutcome(newException("TypeError", "descriptor '"+descriptor.name+"' requires a '"+descriptor.class.name+"' object")), nil
	}
	return descriptor.get(caller, instruction, receiver)
}

func (descriptor *nativeDataDescriptorValue) store(receiver, value Value) (instructionOutcome, error) {
	if !nativeReceiverMatches(receiver, descriptor.class) {
		return raiseOutcome(newException("TypeError", "descriptor '"+descriptor.name+"' requires a '"+descriptor.class.name+"' object")), nil
	}
	if descriptor.set == nil {
		return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
	}
	return descriptor.set(receiver, value)
}

// addNativeDataDescriptors publishes actual function, cell, and class-annotation
// fields. Fields without implemented mutation expose read-only descriptor setters.
func addNativeDataDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	if class == functionNativeType {
		dictionary.set(&stringValue{value: "__doc__"}, &nativeDataDescriptorValue{class: class, name: "__doc__", member: true, get: func(caller *frame, instruction int, value Value) (instructionOutcome, error) {
			return executeFunctionAttributeLoad(caller, instruction, value.(*functionValue), "__doc__")
		}, set: func(receiver, value Value) (instructionOutcome, error) {
			return storeFunctionDocstring(receiver.(*functionValue), value)
		}})
		for _, name := range []string{"__code__", "__globals__", "__closure__", "__annotations__", "__annotate__", "__type_params__"} {
			dictionary.set(&stringValue{value: name}, &nativeDataDescriptorValue{class: class, name: name, member: name == "__globals__" || name == "__closure__", get: func(caller *frame, instruction int, value Value) (instructionOutcome, error) {
				return executeFunctionAttributeLoad(caller, instruction, value.(*functionValue), name)
			}})
		}
	}
	if class == nativeTypesByRuntimeName["cell"] {
		dictionary.set(&stringValue{value: "cell_contents"}, &nativeDataDescriptorValue{class: class, name: "cell_contents", get: func(caller *frame, instruction int, value Value) (instructionOutcome, error) {
			return executeCellAttributeLoad(caller, instruction, value.(*cellValue), "cell_contents")
		}, set: func(receiver, value Value) (instructionOutcome, error) {
			return executeCellAttributeStore(receiver.(*cellValue), "cell_contents", value)
		}})
	}
	if class == typeNativeType {
		dictionary.set(&stringValue{value: "__annotations__"}, &nativeDataDescriptorValue{class: class, name: "__annotations__", get: func(caller *frame, instruction int, value Value) (instructionOutcome, error) {
			if class, ok := value.(*typeValue); ok {
				return executeClassAnnotationsLoad(caller, instruction, class)
			}
			return raiseOutcome(newException("AttributeError", "__annotations__")), nil
		}})
	}
}
