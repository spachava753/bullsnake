package runtime

// addContextDescriptors publishes real context-variable and token operations,
// including read-only metadata and Python 3.14's token context manager.
func addContextDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	if class == contextVarNativeType {
		for _, method := range []struct {
			name string
			call func(*frame, int, Value, []Value, *dictValue) (instructionOutcome, error)
		}{
			{"get", executeContextVarGet}, {"set", executeContextVarSet}, {"reset", executeContextVarReset},
		} {
			dictionary.set(&stringValue{value: method.name}, &nativeDescriptorValue{kind: nativeMethodDescriptor, class: class, name: method.name, call: method.call})
		}
		dictionary.set(&stringValue{value: "name"}, &nativeDataDescriptorValue{class: class, name: "name", member: true, get: func(caller *frame, instruction int, self Value) (instructionOutcome, error) {
			return pushOutcome(caller, instruction, self.(*contextVarValue).name)
		}})
		dictionary.set(&stringValue{value: "__hash__"}, nativeHashDescriptor(class))
	}
	if class != contextTokenNativeType {
		return
	}
	dictionary.set(&stringValue{value: "MISSING"}, contextMissingSingleton)
	dictionary.set(&stringValue{value: "__hash__"}, None)
	for _, name := range []string{"var", "old_value"} {
		dictionary.set(&stringValue{value: name}, &nativeDataDescriptorValue{class: class, name: name, get: func(caller *frame, instruction int, self Value) (instructionOutcome, error) {
			token := self.(*contextTokenValue)
			if name == "var" {
				return pushOutcome(caller, instruction, token.variable)
			}
			if token.oldValue == nil {
				return pushOutcome(caller, instruction, contextMissingSingleton)
			}
			return pushOutcome(caller, instruction, token.oldValue)
		}})
	}
	for _, name := range []string{"__enter__", "__exit__"} {
		dictionary.set(&stringValue{value: name}, &nativeDescriptorValue{kind: nativeMethodDescriptor, class: class, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			count := 0
			if name == "__exit__" {
				count = 3
			}
			if exception := checkNativeArguments(name, arguments, keywords, count, count); exception != nil {
				return raiseOutcome(exception), nil
			}
			if name == "__enter__" {
				return pushOutcome(caller, instruction, self)
			}
			token := self.(*contextTokenValue)
			if exception := resetContextToken(caller.runtime, token.variable, token); exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(caller, instruction, None)
		}})
	}
}

func executeContextAttribute(caller *frame, instruction int, self Value, name string) (instructionOutcome, error) {
	class, _ := typeOf(self)
	if attribute, found := caller.runtime.nativeClassAttribute(class.(*nativeTypeValue), name); found {
		switch attribute := attribute.(type) {
		case *nativeDescriptorValue:
			return executeNativeDescriptorBinding(caller, instruction, attribute, self, class)
		case *nativeDataDescriptorValue:
			return attribute.load(caller, instruction, self)
		default:
			return pushOutcome(caller, instruction, attribute)
		}
	}
	return raiseOutcome(newException("AttributeError", "'"+self.TypeName()+"' object has no attribute '"+name+"'")), nil
}
