package runtime

func (runtime *Runtime) newABCModule() *Module {
	module := newSystemModule("_abc", "")
	setNativeFunction(module, "get_cache_token", 0, 0, runtime.abcCacheToken)
	setNativeFunction(module, "_abc_init", 1, 1, abcInit)
	setNativeFunction(module, "_abc_register", 2, 2, runtime.abcRegister)
	setNativeFunction(module, "_abc_instancecheck", 2, 2, runtime.abcInstanceCheck)
	setNativeFunction(module, "_abc_subclasscheck", 2, 2, runtime.abcSubclassCheck)
	setNativeFunction(module, "_get_dump", 1, 1, runtime.abcDump)
	setNativeFunction(module, "_reset_registry", 1, 1, runtime.abcResetRegistry)
	setNativeFunction(module, "_reset_caches", 1, 1, abcResetCaches)
	return module
}

func (runtime *Runtime) abcCacheToken(_ *frame, _ []Value) (Value, *Exception, error) {
	return newInt64(runtime.abcToken), nil, nil
}

func abcInit(_ *frame, arguments []Value) (Value, *Exception, error) {
	if _, ok := arguments[0].(*typeValue); !ok {
		return nil, newException("TypeError", "_abc_init() argument must be a type"), nil
	}
	return None, nil, nil
}

func (runtime *Runtime) abcRegister(_ *frame, arguments []Value) (Value, *Exception, error) {
	class, ok := arguments[0].(*typeValue)
	if !ok {
		return nil, newException("TypeError", "_abc_register() argument 1 must be a type"), nil
	}
	if !isClassValue(arguments[1]) {
		return nil, newException("TypeError", "Can only register classes"), nil
	}
	for _, registered := range runtime.abcTypes[class] {
		if registered == arguments[1] {
			return arguments[1], nil, nil
		}
	}
	runtime.abcTypes[class] = append(runtime.abcTypes[class], arguments[1])
	runtime.abcToken++
	return arguments[1], nil, nil
}

func (runtime *Runtime) abcInstanceCheck(_ *frame, arguments []Value) (Value, *Exception, error) {
	class, ok := arguments[0].(*typeValue)
	if !ok {
		return nil, newException("TypeError", "_abc_instancecheck() argument 1 must be a type"), nil
	}
	matches := runtime.abcInstanceMatches(class, arguments[1])
	return pythonBool(matches), nil, nil
}

func (runtime *Runtime) abcSubclassCheck(_ *frame, arguments []Value) (Value, *Exception, error) {
	class, ok := arguments[0].(*typeValue)
	if !ok {
		return nil, newException("TypeError", "_abc_subclasscheck() argument 1 must be a type"), nil
	}
	if !isClassValue(arguments[1]) {
		return nil, newException("TypeError", "issubclass() arg 1 must be a class"), nil
	}
	return pythonBool(runtime.abcSubclassMatches(class, arguments[1])), nil, nil
}

func (runtime *Runtime) abcDump(_ *frame, arguments []Value) (Value, *Exception, error) {
	class, ok := arguments[0].(*typeValue)
	if !ok {
		return nil, newException("TypeError", "_get_dump() argument must be a type"), nil
	}
	registered := &setValue{}
	for _, value := range runtime.abcTypes[class] {
		registered.entries = append(registered.entries, value)
	}
	return &tupleValue{elements: []Value{
		registered,
		&setValue{},
		&setValue{},
		newInt64(runtime.abcToken),
	}}, nil, nil
}

func (runtime *Runtime) abcResetRegistry(_ *frame, arguments []Value) (Value, *Exception, error) {
	class, ok := arguments[0].(*typeValue)
	if !ok {
		return nil, newException("TypeError", "_reset_registry() argument must be a type"), nil
	}
	delete(runtime.abcTypes, class)
	runtime.abcToken++
	return None, nil, nil
}

func abcResetCaches(_ *frame, _ []Value) (Value, *Exception, error) {
	return None, nil, nil
}

func isClassValue(value Value) bool {
	switch value.(type) {
	case *typeValue, *builtinTypeValue, *exceptionTypeValue:
		return true
	default:
		return false
	}
}

// abcInstanceMatches checks direct Python inheritance and the runtime-local
// virtual classes registered for one ABC.
func (runtime *Runtime) abcInstanceMatches(class *typeValue, value Value) bool {
	if class.name == "Mapping" || class.name == "MutableMapping" {
		switch value := value.(type) {
		case *dictValue, *namespaceValue:
			return true
		case *instanceValue:
			return value.mapping != nil
		default:
			return false
		}
	}
	if instance, ok := value.(*instanceValue); ok && instance.class.isSubclassOf(class) {
		return true
	}
	if exception, ok := value.(*Exception); ok && exception.userClass != nil &&
		exception.userClass.isSubclassOf(class) {
		return true
	}
	if runtimeClass, ok := runtimeTypeOf(value).(*typeValue); ok && runtimeClass.isSubclassOf(class) {
		return true
	}
	for _, registered := range runtime.abcTypes[class] {
		if builtin, ok := registered.(*builtinTypeValue); ok && builtin.matches(value) {
			return true
		}
		if registeredClass, ok := registered.(*typeValue); ok {
			instance, isInstance := value.(*instanceValue)
			if isInstance && instance.class.isSubclassOf(registeredClass) {
				return true
			}
		}
	}
	return false
}

// abcSubclassMatches checks direct Python inheritance and registered virtual
// base relationships without invoking Python subclass hooks.
func (runtime *Runtime) abcSubclassMatches(class *typeValue, candidate Value) bool {
	if candidateClass, ok := candidate.(*typeValue); ok && candidateClass.isSubclassOf(class) {
		return true
	}
	for _, registered := range runtime.abcTypes[class] {
		if registered == candidate {
			return true
		}
		if candidateClass, candidateOK := candidate.(*typeValue); candidateOK {
			if registeredClass, registeredOK := registered.(*typeValue); registeredOK &&
				candidateClass.isSubclassOf(registeredClass) {
				return true
			}
		}
	}
	return false
}

// lookupTypeAttribute searches the metaclass and class MRO while reporting
// whether descriptor binding must use the metaclass receiver.
func lookupTypeAttribute(class *typeValue, name string) (Value, bool, bool) {
	if value, found := class.lookup(name); found {
		return value, true, false
	}
	if base := class.inheritedBuiltinBase(); base != nil && base.name == "list" {
		if _, found := listMethod(&listValue{}, name); found {
			method := nativeKeywordAwareFunctionNamed("list."+name, 1, -1,
				func(caller *frame, arguments []Value, keywords *dictValue) (Value, *Exception, error) {
					var list *listValue
					switch receiver := arguments[0].(type) {
					case *listValue:
						list = receiver
					case *instanceValue:
						list = receiver.sequence
					}
					if list == nil {
						return nil, newException("TypeError", "descriptor requires a list instance"), nil
					}
					bound, _ := listMethod(list, name)
					return callValueSynchronouslyWithKeywords(caller, bound, arguments[1:], keywords)
				})
			return method, true, false
		}
	}
	if class.metaclass == nil {
		return nil, false, false
	}
	value, found := class.metaclass.lookup(name)
	return value, found, found
}
