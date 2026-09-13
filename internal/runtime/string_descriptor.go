package runtime

import "strings"

// addStringDescriptors exposes the native text operations already implemented
// by the runtime, sharing method calls rather than creating discovery markers.
func addStringDescriptors(class *nativeTypeValue, namespace *dictValue) {
	if class != stringNativeType {
		return
	}
	for _, name := range []string{"__repr__", "__str__", "__format__", "__getitem__", "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__"} {
		kind := nativeWrapperDescriptor
		if name == "__format__" {
			kind = nativeMethodDescriptor
		}
		namespace.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, kind: kind, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeStringSlot(caller, instruction, self, name, arguments, keywords)
		}})
	}
	for _, name := range []string{"capitalize", "count", "endswith", "format", "lower", "removeprefix", "replace", "split", "splitlines", "startswith", "strip"} {
		namespace.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, kind: nativeMethodDescriptor, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
				text, _ := stringStorage(self)
				return executeStringAttributeLoad(caller, instruction, text, name)
			}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return executeFunctionCall(current, instruction, len(current.stack), method, arguments, keywords)
			})
		}})
	}
}

// executeStringSlot applies native text state directly, preserving identity for
// unchanged formatting and declining non-string comparison operands.
func executeStringSlot(caller *frame, instruction int, owner Value, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	self, _ := stringStorage(owner)
	arity := 1
	if name == "__repr__" || name == "__str__" {
		arity = 0
	}
	if exception := checkNativeArguments(name, arguments, keywords, arity, arity); exception != nil {
		return raiseOutcome(exception), nil
	}
	switch name {
	case "__str__":
		return pushOutcome(caller, instruction, self)
	case "__repr__":
		return pushOutcome(caller, instruction, &stringValue{value: self.Repr()})
	case "__getitem__":
		return executeSubscriptValue(caller, instruction, self, arguments[0])
	case "__format__":
		spec, ok := stringStorage(arguments[0])
		if !ok {
			return raiseOutcome(newException("TypeError", "__format__() argument must be str, not "+arguments[0].TypeName())), nil
		}
		if spec.value == "" {
			return executeString(caller, instruction, owner)
		}
		text, exception := formatStringValue(self.value, spec.value)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if text == self.value {
			return pushOutcome(caller, instruction, self)
		}
		return pushOutcome(caller, instruction, &stringValue{value: text})
	default:
		right, ok := stringStorage(arguments[0])
		if !ok {
			return pushOutcome(caller, instruction, notImplementedSingleton)
		}
		comparison := strings.Compare(self.value, right.value)
		return pushOutcome(caller, instruction, booleanValue(name == "__eq__" && comparison == 0 || name == "__ne__" && comparison != 0 || name == "__lt__" && comparison < 0 || name == "__le__" && comparison <= 0 || name == "__gt__" && comparison > 0 || name == "__ge__" && comparison >= 0))
	}
}
