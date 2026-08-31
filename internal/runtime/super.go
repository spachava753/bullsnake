package runtime

import "fmt"

type superValue struct {
	start        *typeValue
	receiver     Value
	receiverType *typeValue
}

func (*superValue) TypeName() string { return "super" }
func (value *superValue) Repr() string {
	if value.receiverType == nil {
		return "<super: <class '" + value.start.qualifiedName + "'>, NULL>"
	}
	return "<super: <class '" + value.start.qualifiedName + "'>, <" +
		value.receiverType.qualifiedName + " object>>"
}
func (*superValue) isValue() {}

// executeBuiltinSuper constructs explicit super values or derives the starting
// class and receiver from the current frame for the zero-argument form.
func executeBuiltinSuper(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "super() takes no keyword arguments"),
		}, nil
	}
	if len(arguments) > 2 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				fmt.Sprintf("super() expected at most 2 arguments, got %d", len(arguments)),
			),
		}, nil
	}
	var start *typeValue
	var receiver Value
	var exception *Exception
	if len(arguments) == 0 {
		start, receiver, exception = zeroArgumentSuper(caller)
	} else {
		var ok bool
		start, ok = arguments[0].(*typeValue)
		if !ok {
			exception = newException(
				"TypeError",
				"super() argument 1 must be a type, not "+arguments[0].TypeName(),
			)
		}
		if len(arguments) == 2 && arguments[1] != None {
			receiver = arguments[1]
		}
	}
	if exception != nil {
		discardCallSegment(caller, base)
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	receiverType, exception := validateSuperReceiver(start, receiver)
	discardCallSegment(caller, base)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return pushOutcome(caller, instruction, &superValue{
		start:        start,
		receiver:     receiver,
		receiverType: receiverType,
	})
}

// zeroArgumentSuper reads argument zero and the compiler-provided __class__
// closure from the frame that is executing the super call.
func zeroArgumentSuper(frame *frame) (*typeValue, Value, *Exception) {
	if frame.code.code.PositionalCount() == 0 {
		return nil, nil, newException("RuntimeError", "super(): no arguments")
	}
	receiver := frame.fastLocals[0]
	for cellIndex, localIndex := range frame.code.cellLocals {
		if localIndex == 0 {
			receiver = frame.deref[cellIndex].value
			break
		}
	}
	if receiver == nil {
		return nil, nil, newException("RuntimeError", "super(): arg[0] deleted")
	}
	for index := range frame.deref {
		if derefName(frame.code, index) != "__class__" {
			continue
		}
		classValue := frame.deref[index].value
		if classValue == nil {
			return nil, nil, newException("RuntimeError", "super(): empty __class__ cell")
		}
		class, ok := classValue.(*typeValue)
		if !ok {
			return nil, nil, newException(
				"RuntimeError",
				"super(): __class__ is not a type ("+classValue.TypeName()+")",
			)
		}
		return class, receiver, nil
	}
	return nil, nil, newException("RuntimeError", "super(): __class__ cell not found")
}

// validateSuperReceiver accepts an instance or class whose method resolution
// order contains the requested starting class.
func validateSuperReceiver(start *typeValue, receiver Value) (*typeValue, *Exception) {
	if receiver == nil {
		return nil, nil
	}
	var receiverType *typeValue
	description := "instance of " + receiver.TypeName()
	switch receiver := receiver.(type) {
	case *instanceValue:
		receiverType = receiver.class
	case *typeValue:
		receiverType = receiver
		description = "type " + receiver.name
	}
	if receiverType != nil && receiverType.isSubclassOf(start) {
		return receiverType, nil
	}
	return nil, newException(
		"TypeError",
		"super(type, obj): obj ("+description+") is not an instance or subtype of type ("+
			start.name+").",
	)
}

// executeSuperAttributeLoad starts after the requested class, ignores the
// receiver namespace, and applies a found method or descriptor to the receiver.
func executeSuperAttributeLoad(
	frame *frame,
	instruction int,
	value *superValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "__thisclass__":
		return pushOutcome(frame, instruction, value.start)
	case "__self__":
		receiver := value.receiver
		if receiver == nil {
			receiver = None
		}
		return pushOutcome(frame, instruction, receiver)
	case "__self_class__":
		if value.receiverType == nil {
			return pushOutcome(frame, instruction, None)
		}
		return pushOutcome(frame, instruction, value.receiverType)
	}
	if value.receiverType == nil {
		return missingSuperAttribute(name), nil
	}
	classValue, found := lookupAfterClass(value.receiverType, value.start, name)
	if !found {
		return missingSuperAttribute(name), nil
	}
	_, classMode := value.receiver.(*typeValue)
	if property, ok := classValue.(*propertyValue); ok {
		if classMode {
			return pushOutcome(frame, instruction, property)
		}
		return executePropertyDescriptorCall(
			frame,
			instruction,
			attributeGet,
			property,
			value.receiver.(*instanceValue),
			nil,
		)
	}
	if descriptor, ok := classValue.(*instanceValue); ok &&
		descriptorHasSpecial(descriptor, "__get__") {
		receiver := value.receiver
		if classMode {
			receiver = None
		}
		return executeDescriptorCall(
			frame,
			instruction,
			attributeGet,
			descriptor,
			[]Value{receiver, value.receiverType},
		)
	}
	if function, ok := classValue.(*functionValue); ok && !classMode {
		classValue = &boundMethodValue{
			function: function,
			self:     value.receiver.(*instanceValue),
		}
	}
	return pushOutcome(frame, instruction, classValue)
}

// lookupAfterClass finds the start in the receiver's C3 order, then searches
// only class namespaces that follow it.
func lookupAfterClass(receiverType, start *typeValue, name string) (Value, bool) {
	startIndex := -1
	for index, class := range receiverType.mro {
		if class == start {
			startIndex = index
			break
		}
	}
	if startIndex < 0 {
		return nil, false
	}
	for _, class := range receiverType.mro[startIndex+1:] {
		if value, found := class.namespace.get(name); found {
			return value, true
		}
	}
	return nil, false
}

func missingSuperAttribute(name string) instructionOutcome {
	return instructionOutcome{
		kind: raised,
		exception: newException(
			"AttributeError",
			"'super' object has no attribute '"+name+"'",
		),
	}
}

var _ Value = (*superValue)(nil)
