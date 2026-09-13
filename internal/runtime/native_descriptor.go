package runtime

type nativeDescriptorKind uint8

const (
	nativeWrapperDescriptor nativeDescriptorKind = iota
	nativeMethodDescriptor
	nativeClassMethodDescriptor
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
	if value.kind == nativeClassMethodDescriptor {
		return "classmethod_descriptor"
	}
	if value.kind == nativeMethodDescriptor {
		return "method_descriptor"
	}
	return "wrapper_descriptor"
}
func (value *nativeDescriptorValue) Repr() string {
	if value.kind != nativeWrapperDescriptor {
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
	if value.descriptor.kind != nativeWrapperDescriptor {
		return "builtin_function_or_method"
	}
	return "method-wrapper"
}
func (value *boundNativeDescriptorValue) Repr() string {
	if value.descriptor.kind != nativeWrapperDescriptor {
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
	if !descriptor.receiverMatches(arguments[0]) {
		return raiseOutcome(newException("TypeError", "descriptor '"+descriptor.name+"' requires a '"+descriptor.class.name+"' object")), nil
	}
	return descriptor.call(caller, instruction, arguments[0], arguments[1:], keywords)
}

func (descriptor *nativeDescriptorValue) receiverMatches(receiver Value) bool {
	if descriptor.kind == nativeClassMethodDescriptor {
		if !isClassValue(receiver) {
			return false
		}
		matched, _ := subclassMatchesClass(receiver, descriptor.class)
		return matched
	}
	return nativeReceiverMatches(receiver, descriptor.class)
}

// bind applies instance or classmethod binding without invoking Python type
// check hooks; explicit owners control classmethod binding when supplied.
func (descriptor *nativeDescriptorValue) bind(receiver, owner Value) (Value, *Exception) {
	if receiver == None && owner == None {
		return nil, newException("TypeError", "__get__(None, None) is invalid")
	}
	if descriptor.kind == nativeClassMethodDescriptor {
		if owner == None {
			var exception *Exception
			owner, exception = typeOf(receiver)
			if exception != nil {
				return nil, exception
			}
		}
		receiver = owner
	} else if receiver == None {
		return descriptor, nil
	}
	if !descriptor.receiverMatches(receiver) {
		return nil, newException("TypeError", "descriptor requires a '"+descriptor.class.name+"' receiver")
	}
	return &boundNativeDescriptorValue{descriptor: descriptor, self: receiver}, nil
}

func executeNativeDescriptorBinding(caller *frame, instruction int, descriptor *nativeDescriptorValue, receiver, owner Value) (instructionOutcome, error) {
	value, exception := descriptor.bind(receiver, owner)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, value)
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
				owner := Value(None)
				if len(arguments) == 2 {
					owner = arguments[1]
				}
				return descriptor.bind(arguments[0], owner)
			}})
		}
	}
	return raiseOutcome(newException("AttributeError", "native wrapper has no attribute '"+name+"'")), nil
}
