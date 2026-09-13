package runtime

// executeClassSlotBinding performs descriptor binding on an already selected
// class slot, including resumable Python descriptors, without reading instance state.
func executeClassSlotBinding(caller *frame, instruction int, self Value, method Value) (instructionOutcome, error) {
	class, _ := typeOf(self)
	switch descriptor := method.(type) {
	case *nativeDescriptorValue:
		return executeNativeDescriptorBinding(caller, instruction, descriptor, self, class)
	case *nativeDataDescriptorValue:
		return descriptor.load(caller, instruction, self)
	case *propertyValue:
		if descriptor.getter == nil {
			return propertyAccessFailure(descriptor, class.(*typeValue), "getter"), nil
		}
		return executeAttributeCallable(caller, instruction, attributeGet, descriptor.getter, []Value{self})
	case *instanceValue:
		if descriptorHasSpecial(descriptor, "__get__") {
			return executeDescriptorCall(caller, instruction, attributeGet, descriptor, []Value{self, class})
		}
	case *classMethodValue:
		return pushOutcome(caller, instruction, &boundMethodValue{callable: descriptor.callable, self: class})
	case *staticMethodValue:
		return pushOutcome(caller, instruction, descriptor.callable)
	}
	return pushOutcome(caller, instruction, bindInstanceFunction(method, self))
}
