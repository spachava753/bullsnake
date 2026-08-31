package runtime

type attributeCallKind uint8

const (
	attributeGet attributeCallKind = iota
	attributeSet
	attributeDelete
)

type attributeCall struct {
	kind        attributeCallKind
	instruction int
}

func executeTypeAttributeLoad(
	frame *frame,
	instruction int,
	owner *typeValue,
	name string,
) (instructionOutcome, error) {
	value, found := owner.lookup(name)
	if !found {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"AttributeError",
				"type object '"+owner.name+"' has no attribute '"+name+"'",
			),
		}, nil
	}
	if descriptor, ok := value.(*instanceValue); ok &&
		descriptorHasSpecial(descriptor, "__get__") {
		return executeDescriptorCall(
			frame,
			instruction,
			attributeGet,
			descriptor,
			[]Value{None, owner},
		)
	}
	return pushOutcome(frame, instruction, value)
}

// executeInstanceAttributeLoad follows data descriptor, instance dictionary,
// non-data descriptor, and plain class attribute precedence in that order.
func executeInstanceAttributeLoad(
	frame *frame,
	instruction int,
	owner *instanceValue,
	name string,
) (instructionOutcome, error) {
	classValue, classFound := owner.class.lookup(name)
	if property, ok := classValue.(*propertyValue); ok {
		return executePropertyDescriptorCall(
			frame,
			instruction,
			attributeGet,
			property,
			owner,
			nil,
		)
	}
	descriptor, isDescriptor := classValue.(*instanceValue)
	hasGet := isDescriptor && descriptorHasSpecial(descriptor, "__get__")
	if hasGet && descriptorIsData(descriptor) {
		return executeDescriptorCall(
			frame,
			instruction,
			attributeGet,
			descriptor,
			[]Value{owner, owner.class},
		)
	}
	if value, found := owner.attributes.get(name); found {
		return pushOutcome(frame, instruction, value)
	}
	if !classFound {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"AttributeError",
				"'"+owner.class.name+"' object has no attribute '"+name+"'",
			),
		}, nil
	}
	if hasGet {
		return executeDescriptorCall(
			frame,
			instruction,
			attributeGet,
			descriptor,
			[]Value{owner, owner.class},
		)
	}
	if function, bind := classValue.(*functionValue); bind {
		classValue = &boundMethodValue{function: function, self: owner}
	}
	return pushOutcome(frame, instruction, classValue)
}

// executeInstanceAttributeStore sends writes through a property or user data
// descriptor before falling back to the instance namespace.
func executeInstanceAttributeStore(
	frame *frame,
	instruction int,
	owner *instanceValue,
	name string,
	value Value,
) (instructionOutcome, error) {
	if classValue, found := owner.class.lookup(name); found {
		if property, ok := classValue.(*propertyValue); ok {
			return executePropertyDescriptorCall(
				frame,
				instruction,
				attributeSet,
				property,
				owner,
				value,
			)
		}
		if descriptor, ok := classValue.(*instanceValue); ok && descriptorIsData(descriptor) {
			if !descriptorHasSpecial(descriptor, "__set__") {
				return instructionOutcome{
					kind:      raised,
					exception: newException("AttributeError", "__set__"),
				}, nil
			}
			return executeDescriptorCall(
				frame,
				instruction,
				attributeSet,
				descriptor,
				[]Value{owner, value},
			)
		}
	}
	owner.attributes.values[name] = value
	return instructionOutcome{kind: advance}, nil
}

// executeInstanceAttributeDelete gives a data descriptor the first deletion
// attempt and otherwise removes only an existing instance attribute.
func executeInstanceAttributeDelete(
	frame *frame,
	instruction int,
	owner *instanceValue,
	name string,
) (instructionOutcome, error) {
	if classValue, found := owner.class.lookup(name); found {
		if property, ok := classValue.(*propertyValue); ok {
			return executePropertyDescriptorCall(
				frame,
				instruction,
				attributeDelete,
				property,
				owner,
				nil,
			)
		}
		if descriptor, ok := classValue.(*instanceValue); ok && descriptorIsData(descriptor) {
			if !descriptorHasSpecial(descriptor, "__delete__") {
				return instructionOutcome{
					kind:      raised,
					exception: newException("AttributeError", "__delete__"),
				}, nil
			}
			return executeDescriptorCall(
				frame,
				instruction,
				attributeDelete,
				descriptor,
				[]Value{owner},
			)
		}
	}
	if _, found := owner.attributes.values[name]; !found {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"AttributeError",
				"'"+owner.class.name+"' object has no attribute '"+name+"'",
			),
		}, nil
	}
	delete(owner.attributes.values, name)
	return instructionOutcome{kind: advance}, nil
}

func descriptorHasSpecial(descriptor *instanceValue, name string) bool {
	_, found := descriptor.class.lookup(name)
	return found
}

func descriptorIsData(descriptor *instanceValue) bool {
	return descriptorHasSpecial(descriptor, "__set__") ||
		descriptorHasSpecial(descriptor, "__delete__")
}

// executeDescriptorCall invokes the selected descriptor method and records
// whether its result is returned to Python or discarded after mutation.
func executeDescriptorCall(
	frame *frame,
	instruction int,
	kind attributeCallKind,
	descriptor *instanceValue,
	arguments []Value,
) (instructionOutcome, error) {
	name := "__get__"
	if kind == attributeSet {
		name = "__set__"
	} else if kind == attributeDelete {
		name = "__delete__"
	}
	method, found := lookupInstanceSpecial(descriptor, name)
	if !found {
		return instructionOutcome{
			kind:      raised,
			exception: newException("AttributeError", name),
		}, nil
	}
	return executeAttributeCallable(frame, instruction, kind, method, arguments)
}

func executeAttributeCallable(
	frame *frame,
	instruction int,
	kind attributeCallKind,
	callable Value,
	arguments []Value,
) (instructionOutcome, error) {
	call := &attributeCall{kind: kind, instruction: instruction}
	outcome, err := executeFunctionCall(
		frame,
		instruction,
		len(frame.stack),
		callable,
		arguments,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.attribute = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"descriptor special method returned without a value",
		)
	}
	return finishAttributeCall(frame, call, result)
}

func finishAttributeCall(
	frame *frame,
	call *attributeCall,
	result Value,
) (instructionOutcome, error) {
	if call.kind == attributeGet {
		return pushOutcome(frame, call.instruction, result)
	}
	return instructionOutcome{kind: advance}, nil
}
