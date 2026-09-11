package runtime

// addNativeHashCallDescriptors publishes executable slots and explicit disabled
// hashes. Native classes inherit object hashing only when they have no override.
func addNativeHashCallDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	switch class.name {
	case "list", "dict", "set", "bytearray", "dict_keys", "dict_items", "dict_values", "FrameLocalsProxy", "slice":
		dictionary.set(&stringValue{value: "__hash__"}, None)
	case "object", "NoneType", "int", "float", "complex", "str", "bytes", "tuple", "frozenset", "range", "mappingproxy":
		dictionary.set(&stringValue{value: "__hash__"}, nativeHashDescriptor(class))
	}
	switch class.name {
	case "function", "builtin_function_or_method", "method", "method_descriptor", "type":
		dictionary.set(&stringValue{value: "__call__"}, nativeCallDescriptor(class))
	}
}

func nativeReceiverMatches(receiver Value, class *nativeTypeValue) bool {
	actual, exception := typeOf(receiver)
	if exception != nil {
		return false
	}
	switch actual := actual.(type) {
	case *nativeTypeValue:
		return actual.isSubclassOf(class)
	case *typeValue:
		return actual.isSubclassOfNative(class)
	case *exceptionTypeValue:
		return class == objectNativeType
	}
	return false
}

func nativeHashDescriptor(class *nativeTypeValue) Value {
	return &builtinFunctionValue{name: "__hash__", method: true, call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
		if exception := checkNativeArguments("__hash__", arguments, keywords, 1, 1); exception != nil {
			return nil, exception
		}
		self := arguments[0]
		if !nativeReceiverMatches(self, class) {
			return nil, newException("TypeError", "descriptor '__hash__' requires a '"+class.name+"' object")
		}
		if class == objectNativeType {
			return hashIntegerValue(stableTextHash(self.TypeName(), self.Repr())), nil
		}
		hash, exception, _ := fixedValueHash(self)
		if exception != nil {
			return nil, exception
		}
		return hashIntegerValue(hash), nil
	}}
}

func nativeCallDescriptor(class *nativeTypeValue) Value {
	return &builtinFunctionValue{name: "__call__", method: true, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		if len(arguments) == 0 {
			discardCallSegment(caller, base)
			return raiseOutcome(newException("TypeError", "unbound method __call__() needs an argument")), nil
		}
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		if !nativeReceiverMatches(arguments[0], class) {
			return raiseOutcome(newException("TypeError", "descriptor '__call__' requires a '"+class.name+"' object")), nil
		}
		return executeFunctionCall(caller, instruction, len(caller.stack), arguments[0], arguments[1:], keywords)
	}}
}

// boundNativeHashCall resolves inherited native slots without intercepting
// class attribute lookup or user-instance special-method precedence.
func (runtime *Runtime) boundNativeHashCall(owner Value, name string) (Value, bool) {
	if (name != "__hash__" && name != "__call__") || isClassValue(owner) {
		return nil, false
	}
	actual, exception := typeOf(owner)
	class, native := actual.(*nativeTypeValue)
	if exception != nil || !native {
		return nil, false
	}
	method, found := runtime.nativeClassAttribute(class, name)
	if !found || method == None {
		return method, found
	}
	return &boundMethodValue{callable: method, self: owner}, true
}
