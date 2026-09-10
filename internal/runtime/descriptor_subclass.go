package runtime

// descriptorState keeps user-subclass identity and attributes on native wrapper
// values, so descriptor binding and Python construction share one object.
type descriptorState struct {
	class      *typeValue
	attributes *Namespace
}

func descriptorIdentity(value Value) *descriptorState {
	switch value := value.(type) {
	case *propertyValue:
		return &value.descriptorState
	case *classMethodValue:
		return &value.descriptorState
	case *staticMethodValue:
		return &value.descriptorState
	}
	return nil
}

// executeDescriptorSubclassCall allocates native storage and invokes either the
// inherited native initializer or a Python initializer through VM continuations.
func executeDescriptorSubclassCall(caller *frame, instruction, base int, class *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	var wrapper Value
	switch class.nativeClassBase() {
	case propertyNativeType:
		wrapper = &propertyValue{doc: None}
	case classMethodNativeType:
		wrapper = &classMethodValue{callable: None}
	case staticMethodNativeType:
		wrapper = &staticMethodValue{callable: None}
	default:
		return raiseOutcome(newException("TypeError", "unsupported descriptor base")), nil
	}
	state := descriptorIdentity(wrapper)
	state.class, state.attributes = class, newNamespace()
	initializer, exists := class.lookup("__init__")
	if !exists {
		_, exception := initializeDescriptor(append([]Value{wrapper}, arguments...), keywords)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, wrapper)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), &boundMethodValue{callable: initializer, self: wrapper}, arguments, keywords)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result != None {
			return raiseOutcome(newException("TypeError", "__init__() should return None, not '"+result.TypeName()+"'")), nil
		}
		return pushOutcome(current, instruction, wrapper)
	})
}

// initializeDescriptor implements native __init__ on already allocated storage,
// including calls made by ABC descriptor subclasses through super.
func initializeDescriptor(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if len(arguments) == 0 {
		return nil, newException("TypeError", "descriptor initializer requires a receiver")
	}
	switch target := arguments[0].(type) {
	case *propertyValue:
		value, exception := builtinProperty(arguments[1:], keywords)
		if exception != nil {
			return nil, exception
		}
		property := value.(*propertyValue)
		property.descriptorState = target.descriptorState
		*target = *property
	case *classMethodValue:
		value, exception := newMethodDescriptor("classmethod", arguments[1:], keywords, true)
		if exception != nil {
			return nil, exception
		}
		target.callable = value.(*classMethodValue).callable
	case *staticMethodValue:
		value, exception := newMethodDescriptor("staticmethod", arguments[1:], keywords, false)
		if exception != nil {
			return nil, exception
		}
		target.callable = value.(*staticMethodValue).callable
	default:
		return nil, newException("TypeError", "descriptor initializer requires a descriptor receiver")
	}
	return None, nil
}

// executeDescriptorSubclassAttribute applies user-class overrides before the
// inherited native wrapper fields, retaining ordinary function binding.
func executeDescriptorSubclassAttribute(caller *frame, instruction int, owner Value, name string) (instructionOutcome, bool, error) {
	state := descriptorIdentity(owner)
	if state == nil || state.class == nil {
		return instructionOutcome{}, false, nil
	}
	value, found := state.class.lookup(name)
	if found {
		if descriptor, ok := value.(*instanceValue); ok && descriptorHasSpecial(descriptor, "__get__") {
			outcome, err := executeDescriptorCall(caller, instruction, attributeGet, descriptor, []Value{owner, state.class})
			return outcome, true, err
		}
		if property, ok := value.(*propertyValue); ok && property.getter != nil {
			outcome, err := executeAttributeCallable(caller, instruction, attributeGet, property.getter, []Value{owner})
			return outcome, true, err
		}
	}
	if attribute, exists := state.attributes.get(name); exists {
		value, found = attribute, true
	} else if found {
		if function, ok := value.(*functionValue); ok {
			value = &boundMethodValue{callable: function, self: owner}
		}
		if bound, ok := bindMethodDescriptor(value, state.class); ok {
			value = bound
		}
	}
	if !found {
		return instructionOutcome{}, false, nil
	}
	outcome, err := pushOutcome(caller, instruction, value)
	return outcome, true, err
}
