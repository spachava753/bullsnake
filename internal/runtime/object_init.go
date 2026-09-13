package runtime

// executeObjectInit permits excess arguments only when initialization is still
// object.__init__ and allocation is customized, matching the native slot rules.
func executeObjectInit(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if len(arguments) == 0 && (keywords == nil || len(keywords.entries) == 0) {
		return pushOutcome(caller, instruction, None)
	}
	name := "object"
	overridesInit, overridesNew := objectConstructionOverrides(self)
	if !overridesInit {
		if overridesNew {
			return pushOutcome(caller, instruction, None)
		}
		name = self.TypeName()
	}
	return raiseOutcome(newException("TypeError", name+".__init__() takes exactly one argument (the instance to initialize)")), nil
}

// objectConstructionOverrides distinguishes Python-defined slots and the
// implemented native constructor families without invoking either constructor.
func objectConstructionOverrides(self Value) (initialization, allocation bool) {
	if instance, ok := self.(*instanceValue); ok {
		initializer, found := instance.class.lookup("__init__")
		wrapper, wrapped := initializer.(*wrapperDescriptorValue)
		if found && (!wrapped || wrapper.class != objectNativeType || wrapper.name != "__init__") {
			return true, true
		}
		if _, found := instance.class.lookup("__new__"); found {
			return false, true
		}
		if native := instance.class.nativeClassBase(); native != nil {
			return nativeHasOwnInitializer(native), true
		}
		for _, base := range instance.class.mro {
			if base.genericAliasClass || base.bufferViewClass {
				return false, true
			}
		}
		return false, false
	}
	class, _ := typeOf(self)
	if native, ok := class.(*nativeTypeValue); ok {
		return nativeHasOwnInitializer(native), native != objectNativeType
	}
	return true, true
}

func nativeHasOwnInitializer(class *nativeTypeValue) bool {
	switch class {
	case listNativeType, dictNativeType, setNativeType, bytearrayNativeType,
		moduleNativeType, typeNativeType, superNativeType, propertyNativeType,
		classMethodNativeType, staticMethodNativeType:
		return true
	}
	return false
}
