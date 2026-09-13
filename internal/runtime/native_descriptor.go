package runtime

type nativeDescriptorKind uint8

const (
	nativeWrapperDescriptor nativeDescriptorKind = iota
	nativeMethodDescriptor
)

// nativeDescriptorValue exposes an implemented native slot or regular method.
// Binding retains its descriptor kind rather than creating a Python method.
type nativeDescriptorValue struct {
	kind  nativeDescriptorKind
	class *nativeTypeValue
	name  string
	call  func(*frame, int, Value, []Value, *dictValue) (instructionOutcome, error)
}

func (value *nativeDescriptorValue) TypeName() string {
	if value.kind == nativeMethodDescriptor {
		return "method_descriptor"
	}
	return "wrapper_descriptor"
}
func (value *nativeDescriptorValue) Repr() string {
	if value.kind == nativeMethodDescriptor {
		return "<method '" + value.name + "' of '" + value.class.name + "' objects>"
	}
	return "<slot wrapper '" + value.name + "' of '" + value.class.name + "' objects>"
}
func (*nativeDescriptorValue) isValue() {}

type boundNativeDescriptorValue struct {
	descriptor *nativeDescriptorValue
	self       Value
}

func (value *boundNativeDescriptorValue) TypeName() string {
	if value.descriptor.kind == nativeMethodDescriptor {
		return "builtin_function_or_method"
	}
	return "method-wrapper"
}
func (value *boundNativeDescriptorValue) Repr() string {
	if value.descriptor.kind == nativeMethodDescriptor {
		return "<built-in method " + value.descriptor.name + " of " + value.self.TypeName() + " object>"
	}
	return "<method-wrapper '" + value.descriptor.name + "' of " + value.self.TypeName() + " object>"
}
func (*boundNativeDescriptorValue) isValue() {}

func executeNativeDescriptorCall(caller *frame, instruction, base int, descriptor *nativeDescriptorValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
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

// executeNativeDescriptorAttributeLoad exposes slot metadata and real descriptor binding.
// A None instance returns the descriptor; other receivers must match its owner.
func executeNativeDescriptorAttributeLoad(caller *frame, instruction int, descriptor *nativeDescriptorValue, self Value, name string) (instructionOutcome, error) {
	switch name {
	case "__name__":
		return pushOutcome(caller, instruction, &stringValue{value: descriptor.name})
	case "__qualname__":
		return pushOutcome(caller, instruction, &stringValue{value: descriptor.class.qualname + "." + descriptor.name})
	case "__objclass__":
		if self == nil || descriptor.kind == nativeWrapperDescriptor {
			return pushOutcome(caller, instruction, descriptor.class)
		}
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
				return &boundNativeDescriptorValue{descriptor: descriptor, self: arguments[0]}, nil
			}})
		}
	}
	return raiseOutcome(newException("AttributeError", "native wrapper has no attribute '"+name+"'")), nil
}
