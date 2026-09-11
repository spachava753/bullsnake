package runtime

// initializeSimpleNamespaceClass supplies real mutable attribute namespaces
// using ordinary native class construction, dictionaries, and subclass slots.
func initializeSimpleNamespaceClass() *typeValue {
	class := newBuiltinClass("types", "SimpleNamespace", nil)
	class.simpleNamespaceClass = true
	class.setAttribute("__hash__", None)
	class.setAttribute("__new__", &builtinFunctionValue{name: "SimpleNamespace.__new__", frameCall: func(caller *frame, instruction, base int, arguments []Value, _ *dictValue) (instructionOutcome, error) {
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		if len(arguments) == 0 {
			return raiseOutcome(newException("TypeError", "SimpleNamespace.__new__ requires a type")), nil
		}
		subclass, ok := arguments[0].(*typeValue)
		if !ok || !subclass.isSubclassOf(class) {
			return raiseOutcome(newException("TypeError", "SimpleNamespace.__new__ requires a SimpleNamespace subtype")), nil
		}
		value := &instanceValue{class: subclass, attributes: newNamespace()}
		value.attributes.asDictionary()
		return pushOutcome(caller, instruction, value)
	}})
	class.setAttribute("__init__", nativeInstanceMethod(class, "__init__", executeSimpleNamespaceInit))
	class.setAttribute("__dict__", &propertyValue{doc: None, getter: nativeInstanceMethod(class, "__dict__", func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
		return pushOutcome(caller, instruction, self.attributes.asDictionary())
	})})
	return class
}

// executeSimpleNamespaceInit converts the optional positional source through
// dict's existing iterator path, validates keys, then updates before keywords.
func executeSimpleNamespaceInit(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if len(arguments) > 1 {
		return raiseOutcome(newException("TypeError", "SimpleNamespace expected at most 1 argument")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), dictNativeType, arguments, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if exception := updateSimpleNamespace(self, result.(*dictValue)); exception != nil {
			return raiseOutcome(exception), nil
		}
		if keywords != nil {
			if exception := updateSimpleNamespace(self, keywords); exception != nil {
				return raiseOutcome(exception), nil
			}
		}
		return pushOutcome(current, instruction, None)
	})
}

func updateSimpleNamespace(self *instanceValue, dictionary *dictValue) *Exception {
	for _, entry := range dictionary.entries {
		if _, ok := entry.key.(*stringValue); !ok {
			return newException("TypeError", "keywords must be strings")
		}
	}
	for _, entry := range dictionary.entries {
		self.attributes.store(entry.key.(*stringValue).value, entry.value)
	}
	return nil
}
