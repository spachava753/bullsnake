package runtime

func isObjectMethodName(name string) bool {
	return name == "__init__" || name == "__repr__" || name == "__str__"
}

// executeObjectString calls the actual repr slot, not __str__ or an instance
// attribute. A direct slot call preserves a non-string result for its caller.
func executeObjectString(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__str__", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	if instance, ok := self.(*instanceValue); ok {
		if method, found := lookupInstanceSpecial(instance, "__repr__"); found {
			return executeFunctionCall(caller, instruction, len(caller.stack), method, nil, nil)
		}
	}
	return pushOutcome(caller, instruction, &stringValue{value: self.Repr()})
}

// executeObjectRepresentation bypasses the receiver's repr override and emits
// the runtime's address-free root object representation using its actual class.
func executeObjectRepresentation(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__repr__", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	class, exception := typeOf(self)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	name := self.TypeName()
	if class, ok := class.(*typeValue); ok {
		name = class.qualifiedName
		if class.module != "" && class.module != "builtins" {
			name = class.module + "." + name
		}
	}
	return pushOutcome(caller, instruction, &stringValue{value: "<" + name + " object>"})
}
