package runtime

// addDictionaryDescriptors exposes existing mapping operations through genuine
// receiver-checked slots and methods, rather than type-discovery placeholders.
func addDictionaryDescriptors(class *nativeTypeValue, namespace *dictValue) {
	if class != dictNativeType {
		return
	}
	namespace.set(&stringValue{value: "__new__"}, &builtinFunctionValue{name: "dict.__new__", call: func(arguments []Value, _ *dictValue) (Value, *Exception) {
		if len(arguments) == 0 {
			return nil, newException("TypeError", "dict.__new__ requires a dict subtype")
		}
		if arguments[0] == dictNativeType {
			return &dictValue{}, nil
		}
		if subclass, ok := arguments[0].(*typeValue); ok && subclass.isSubclassOfNative(dictNativeType) {
			return &instanceValue{class: subclass, attributes: newNamespace(), dictionary: &dictValue{}}, nil
		}
		return nil, newException("TypeError", "dict.__new__ requires a dict subtype")
	}})
	for _, name := range []string{"__init__", "__getitem__", "__setitem__", "__delitem__", "__repr__", "__eq__", "__ne__", "setdefault", "clear", "copy", "get", "pop", "items", "keys", "values", "update"} {
		kind := nativeMethodDescriptor
		if name != "__getitem__" && len(name) > 4 && name[:2] == "__" {
			kind = nativeWrapperDescriptor
		}
		namespace.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, kind: kind, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			storage, ok := dictionaryStorage(self)
			if !ok {
				return raiseOutcome(newException("TypeError", "uninitialized dict subtype")), nil
			}
			if name == "__getitem__" {
				return executeDictionaryGetitem(caller, instruction, self, storage, arguments, keywords)
			}
			return executeDictionaryDescriptor(caller, instruction, storage, name, arguments, keywords)
		}})
	}
}

// executeDictionaryDescriptor handles mapping slots directly and forwards
// ordinary methods to their shared implementations through the VM call path.
func executeDictionaryDescriptor(caller *frame, instruction int, dictionary *dictValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	switch name {
	case "__getitem__", "__setitem__", "__delitem__", "setdefault":
		return executeDictionaryItemDescriptor(caller, instruction, dictionary, name, arguments, keywords)
	case "__repr__":
		if exception := checkNativeArguments(name, arguments, keywords, 0, 0); exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, &stringValue{value: dictionary.Repr()})
	case "__eq__", "__ne__":
		if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
			return raiseOutcome(exception), nil
		}
		other, ok := dictionaryStorage(arguments[0])
		if !ok {
			return pushOutcome(caller, instruction, notImplementedSingleton)
		}
		return executeDictionaryEquality(caller, instruction, dictionary, other, name == "__ne__", nil)
	case "__init__":
		name = "update"
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDictionaryAttributeLoad(caller, instruction, dictionary, name)
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return executeFunctionCall(current, instruction, len(current.stack), method, arguments, keywords)
	})
}

// executeDictionaryItemDescriptor validates arity before accessing keys and
// preserves insertion order and shared values across direct slot mutations.
func executeDictionaryItemDescriptor(caller *frame, instruction int, dictionary *dictValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	minimum, maximum := 1, 1
	if name == "__setitem__" {
		minimum, maximum = 2, 2
	} else if name == "setdefault" {
		maximum = 2
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	key := arguments[0]
	result := Value(None)
	var exception *Exception
	switch name {
	case "__setitem__":
		exception = dictionary.set(key, arguments[1])
	case "__delitem__":
		var found bool
		found, exception = dictionary.delete(key)
		if exception == nil && !found {
			exception = newException("KeyError", key.Repr())
		}
	default:
		var found bool
		result, found, exception = dictionary.get(key)
		if exception == nil && !found {
			if name == "__getitem__" {
				exception = newException("KeyError", key.Repr())
			} else {
				result = None
				if len(arguments) == 2 {
					result = arguments[1]
				}
				exception = dictionary.set(key, result)
			}
		}
	}
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, result)
}
