package runtime

// wrapperDescriptorValue exposes an implemented native special-method slot.
// It binds to methodWrapperValue without becoming a Python function or method.
type wrapperDescriptorValue struct {
	class *nativeTypeValue
	name  string
	call  func(*frame, int, Value, []Value, *dictValue) (instructionOutcome, error)
}

func (*wrapperDescriptorValue) TypeName() string { return "wrapper_descriptor" }
func (value *wrapperDescriptorValue) Repr() string {
	return "<slot wrapper '" + value.name + "' of '" + value.class.name + "' objects>"
}
func (*wrapperDescriptorValue) isValue() {}

type methodWrapperValue struct {
	descriptor *wrapperDescriptorValue
	self       Value
}

func (*methodWrapperValue) TypeName() string { return "method-wrapper" }
func (value *methodWrapperValue) Repr() string {
	return "<method-wrapper '" + value.descriptor.name + "' of " + value.self.TypeName() + " object>"
}
func (*methodWrapperValue) isValue() {}

func executeWrapperCall(caller *frame, instruction, base int, descriptor *wrapperDescriptorValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	if len(arguments) == 0 {
		return raiseOutcome(newException("TypeError", "descriptor '"+descriptor.name+"' of '"+descriptor.class.name+"' object needs an argument")), nil
	}
	if !nativeReceiverMatches(arguments[0], descriptor.class) {
		return raiseOutcome(newException("TypeError", "descriptor '"+descriptor.name+"' requires a '"+descriptor.class.name+"' object")), nil
	}
	return descriptor.call(caller, instruction, arguments[0], arguments[1:], keywords)
}

// executeWrapperAttributeLoad exposes slot metadata and real descriptor binding.
// A None instance returns the descriptor; other receivers must match its owner.
func executeWrapperAttributeLoad(caller *frame, instruction int, descriptor *wrapperDescriptorValue, self Value, name string) (instructionOutcome, error) {
	switch name {
	case "__name__":
		return pushOutcome(caller, instruction, &stringValue{value: descriptor.name})
	case "__qualname__":
		return pushOutcome(caller, instruction, &stringValue{value: descriptor.class.qualname + "." + descriptor.name})
	case "__objclass__":
		return pushOutcome(caller, instruction, descriptor.class)
	case "__self__":
		if self != nil {
			return pushOutcome(caller, instruction, self)
		}
	case "__get__":
		if self == nil {
			return pushOutcome(caller, instruction, &builtinFunctionValue{name: "__get__", call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
				if exception := checkNativeArguments("__get__", arguments, keywords, 1, 2); exception != nil {
					return nil, exception
				}
				if arguments[0] == None {
					return descriptor, nil
				}
				if !nativeReceiverMatches(arguments[0], descriptor.class) {
					return nil, newException("TypeError", "descriptor requires a '"+descriptor.class.name+"' object")
				}
				return &methodWrapperValue{descriptor: descriptor, self: arguments[0]}, nil
			}})
		}
	}
	return raiseOutcome(newException("AttributeError", "native wrapper has no attribute '"+name+"'")), nil
}
