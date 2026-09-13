package runtime

// rootAllocatableClass identifies ordinary instance layouts. Native storage and
// exception layouts must use their own allocators rather than empty instances.
func rootAllocatableClass(class *typeValue) bool {
	if class.nativeClassBase() != nil || class.isExceptionClass() {
		return false
	}
	for _, base := range class.mro {
		if base.ioClass || base.genericAliasClass || base.simpleNamespaceClass || base.bufferViewClass {
			return false
		}
	}
	return true
}

func allocationClassName(class Value) string {
	switch class := class.(type) {
	case *typeValue:
		return class.name
	case *nativeTypeValue:
		return class.name
	case *exceptionTypeValue:
		return class.name
	}
	return class.TypeName()
}

// addNativeAllocators publishes actual static __new__ functions for object and
// singleton types. Each runtime retains one callable identity per native class.
func addNativeAllocators(class *nativeTypeValue, dictionary *dictValue) {
	if class != objectNativeType && class != intNativeType && singletonForClass(class) == nil {
		return
	}
	dictionary.set(&stringValue{value: "__new__"}, &builtinFunctionValue{self: class, name: class.name + ".__new__", frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		if len(arguments) == 0 {
			return raiseOutcome(newException("TypeError", class.name+".__new__(): not enough arguments")), nil
		}
		target := arguments[0]
		if !isClassValue(target) {
			return raiseOutcome(newException("TypeError", class.name+".__new__(X): X is not a type object ("+target.TypeName()+")")), nil
		}
		if class == objectNativeType {
			return executeObjectAllocation(caller, instruction, target, arguments[1:], keywords)
		}
		if class == intNativeType && target == boolNativeType {
			return raiseOutcome(newException("TypeError", "int.__new__(bool) is not safe, use bool.__new__()")), nil
		}
		if target != class {
			name := allocationClassName(target)
			return raiseOutcome(newException("TypeError", class.name+".__new__("+name+"): "+name+" is not a subtype of "+class.name)), nil
		}
		if class == intNativeType {
			value, exception := builtinInt(arguments[1:], keywords)
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(caller, instruction, value)
		}
		return executeSingletonTypeCall(caller, instruction, len(caller.stack), class, arguments[1:], keywords)
	}})
}

func singletonForClass(class *nativeTypeValue) Value {
	switch class {
	case noneNativeType:
		return None
	case ellipsisNativeType:
		return ellipsisSingleton
	case notImplementedNativeType:
		return notImplementedSingleton
	}
	return nil
}

func executeSingletonTypeCall(caller *frame, instruction, base int, class *nativeTypeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	discardCallSegment(caller, base)
	if len(arguments) != 0 || keywords != nil && len(keywords.entries) != 0 {
		return raiseOutcome(newException("TypeError", class.name+" takes no arguments")), nil
	}
	return pushOutcome(caller, instruction, singletonForClass(class))
}

// executeObjectAllocation enforces layout safety and the excess-argument rules,
// then allocates without calling __init__. Abstract allocation shares diagnostics.
func executeObjectAllocation(caller *frame, instruction int, target Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	class, user := target.(*typeValue)
	if target != objectNativeType && (!user || !rootAllocatableClass(class)) {
		name := allocationClassName(target)
		return raiseOutcome(newException("TypeError", "object.__new__("+name+") is not safe, use "+name+".__new__()")), nil
	}
	if len(arguments) != 0 || keywords != nil && len(keywords.entries) != 0 {
		customInit := false
		if user {
			allocator, found := class.lookup("__new__")
			if static, ok := allocator.(*staticMethodValue); ok {
				allocator = static.callable
			}
			root, _ := caller.runtime.nativeClassAttribute(objectNativeType, "__new__")
			if found && allocator != root {
				return raiseOutcome(newException("TypeError", "object.__new__() takes exactly one argument (the type to instantiate)")), nil
			}
			initializer, found := class.lookup("__init__")
			wrapper, native := initializer.(*nativeDescriptorValue)
			customInit = found && (!native || wrapper.class != objectNativeType || wrapper.name != "__init__")
		}
		if !customInit {
			return raiseOutcome(newException("TypeError", allocationClassName(target)+"() takes no arguments")), nil
		}
	}
	if user {
		if class.abstract {
			return executeAbstractAllocation(caller, instruction, class)
		}
		return pushOutcome(caller, instruction, &instanceValue{class: class, attributes: newNamespace()})
	}
	return pushOutcome(caller, instruction, &objectValue{})
}
