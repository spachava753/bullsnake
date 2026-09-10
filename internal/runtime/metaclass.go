package runtime

// selectMetaclass resolves the most-derived compatible type before the class
// body runs. Non-type factories retain Python's callable-metaclass path.
func selectMetaclass(explicit Value, bases []Value) (Value, *Exception) {
	winner := explicit
	if winner == nil {
		winner = typeNativeType
		if len(bases) != 0 {
			winner, _ = typeOf(bases[0])
		}
	}
	if !isClassValue(winner) {
		return winner, nil
	}
	for _, base := range bases {
		if !isClassValue(base) {
			return nil, newException("TypeError", "class base is not a type")
		}
		candidate, exception := typeOf(base)
		if exception != nil {
			return nil, exception
		}
		subtype, _ := subclassMatchesClass(winner, candidate)
		if subtype {
			continue
		}
		subtype, _ = subclassMatchesClass(candidate, winner)
		if subtype {
			winner = candidate
			continue
		}
		return nil, newException("TypeError", "metaclass conflict: the metaclass of a derived class must be a (non-strict) subclass of the metaclasses of all its bases")
	}
	return winner, nil
}

// finishClassBody passes the completed namespace to its selected factory and
// verifies that a compiler-created class cell refers to the returned class.
func finishClassBody(caller *frame, build *classBuild, cell Value) (instructionOutcome, error) {
	dictionary := build.dictionary
	if dictionary == nil {
		dictionary = &dictValue{}
	}
	if _, ok := cell.(*cellValue); ok {
		dictionary.set(&stringValue{value: "__classcell__"}, cell)
	}
	arguments := []Value{&stringValue{value: build.name}, &tupleValue{elements: build.baseValues}, dictionary}
	return continueNativeOperation(caller, build.instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, build.instruction, len(caller.stack), build.metaclass, arguments, build.keywords)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if class, ok := result.(*typeValue); ok {
			build.namespace.dictionary = nil
			build.namespace.values = class.namespace.values
		}
		if classCell, ok := cell.(*cellValue); ok && isClassValue(result) {
			if classCell.value == nil {
				return raiseOutcome(newException("RuntimeError", "__class__ not set defining '"+build.name+"'. Was __classcell__ propagated to type.__new__?")), nil
			}
			if classCell.value != result {
				return raiseOutcome(newException("TypeError", "__class__ set to a different class defining '"+build.name+"'")), nil
			}
		}
		return pushOutcome(current, build.instruction, result)
	})
}

// executeMetaclassCall invokes __new__ without instance binding, then invokes
// __init__ only when the returned object is an instance of the metaclass.
func executeMetaclassCall(caller *frame, instruction int, metaclass *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	constructor, found := metaclass.lookup("__new__")
	if !found {
		constructor, _ = nativeMetaclassMethod("__new__")
	}
	if wrapper, ok := constructor.(*staticMethodValue); ok {
		constructor = wrapper.callable
	}
	newArguments := append([]Value{metaclass}, arguments...)
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), constructor, newArguments, keywords)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		matches, _ := instanceMatchesClass(result, metaclass)
		if !matches {
			return pushOutcome(current, instruction, result)
		}
		actual, _ := typeOf(result)
		initializer, found := actual.(*typeValue).lookup("__init__")
		if !found {
			initializer, _ = nativeMetaclassMethod("__init__")
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, instruction, len(current.stack), initializer, append([]Value{result}, arguments...), keywords)
		}, func(current *frame, initialized Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if initialized != None {
				return raiseOutcome(newException("TypeError", "__init__() should return None, not '"+initialized.TypeName()+"'")), nil
			}
			return pushOutcome(current, instruction, result)
		})
	})
}

// nativeMetaclassMethod supplies the supported type descriptors for direct
// access and inherited metaclass operations.
func nativeMetaclassMethod(name string) (Value, bool) {
	switch name {
	case "__subclasses__":
		return &builtinFunctionValue{name: "type.__subclasses__", call: userSubclasses}, true
	case "__instancecheck__", "__subclasscheck__":
		return &builtinFunctionValue{name: "type." + name, call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
			return nativeTypeCheck(arguments, keywords, name == "__subclasscheck__")
		}}, true
	case "__prepare__":
		return &builtinFunctionValue{name: "type.__prepare__", call: typePrepare}, true
	case "__new__":
		return &builtinFunctionValue{name: "type.__new__", frameCall: executeTypeNew}, true
	case "__init__":
		return &builtinFunctionValue{name: "type.__init__", call: typeInitialize}, true
	}
	return nil, false
}

// executeTypeNew validates the explicit metaclass before sharing the dynamic
// class builder with type(name, bases, namespace).
func executeTypeNew(caller *frame, instruction int, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	if len(arguments) != 4 {
		return raiseOutcome(newException("TypeError", "type.__new__() takes exactly 3 arguments")), nil
	}
	var metaclass *typeValue
	if arguments[0] != typeNativeType {
		var ok bool
		metaclass, ok = arguments[0].(*typeValue)
		if !ok || !metaclass.isSubclassOfNative(typeNativeType) {
			return raiseOutcome(newException("TypeError", "type.__new__() argument 1 must be a subtype of type")), nil
		}
	}
	if keywords != nil && len(keywords.entries) != 0 {
		return raiseOutcome(newException("TypeError", "class keyword arguments are not supported")), nil
	}
	return executeDynamicTypeCall(caller, instruction, len(caller.stack), arguments[1:], metaclass)
}

func typeInitialize(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if len(arguments) != 4 {
		return nil, newException("TypeError", "type.__init__() takes 1 or 3 arguments")
	}
	if !isClassValue(arguments[0]) {
		return nil, newException("TypeError", "descriptor '__init__' requires a 'type' object")
	}
	return None, nil
}

// prepareClassBody resolves __prepare__ before starting the class body, retaining
// the exact returned dictionary for the metaclass's later constructor call.
func prepareClassBody(caller, child *frame) (instructionOutcome, error) {
	build := child.classBuild
	return continueNativeOperation(caller, build.instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, build.instruction, build.metaclass, "__prepare__")
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if !isAttributeError(exception) {
				return raiseOutcome(exception), nil
			}
			return startPreparedClass(child, &dictValue{})
		}
		return continueNativeOperation(current, build.instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, build.instruction, len(current.stack), method,
				[]Value{&stringValue{value: build.name}, &tupleValue{elements: build.baseValues}}, build.keywords)
		}, func(current *frame, namespace Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			dictionary, ok := namespace.(*dictValue)
			if !ok {
				return raiseOutcome(newException("TypeError", "__prepare__() must return a dict in this runtime")), nil
			}
			return startPreparedClass(child, dictionary)
		})
	})
}

func startPreparedClass(child *frame, dictionary *dictValue) (instructionOutcome, error) {
	build := child.classBuild
	build.dictionary = dictionary
	child.locals.dictionary = dictionary
	for _, entry := range dictionary.entries {
		key, ok := entry.key.(*stringValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "type namespace keys must be strings")), nil
		}
		child.locals.values[key.value] = entry.value
		build.recordStore(key.value)
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

func typePrepare(arguments []Value, keywords *dictValue) (Value, *Exception) {
	return &dictValue{}, nil
}
