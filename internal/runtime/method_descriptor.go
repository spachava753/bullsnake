package runtime

import "strconv"

type classMethodValue struct {
	descriptorState
	callable Value
}

func (value *classMethodValue) TypeName() string {
	if value.class != nil {
		return value.class.name
	}
	return "classmethod"
}
func (method *classMethodValue) Repr() string {
	return "<classmethod(" + method.callable.Repr() + ")>"
}
func (*classMethodValue) isValue() {}

type staticMethodValue struct {
	descriptorState
	callable Value
}

func (value *staticMethodValue) TypeName() string {
	if value.class != nil {
		return value.class.name
	}
	return "staticmethod"
}
func (method *staticMethodValue) Repr() string {
	return "<staticmethod(" + method.callable.Repr() + ")>"
}
func (*staticMethodValue) isValue() {}

func newMethodDescriptor(
	name string,
	arguments []Value,
	keywords *dictValue,
	classBinding bool,
) (Value, *Exception) {
	if keywords != nil && len(keywords.entries) != 0 {
		return nil, newException("TypeError", name+"() takes no keyword arguments")
	}
	if len(arguments) != 1 {
		return nil, newException(
			"TypeError",
			name+" expected 1 argument, got "+strconv.Itoa(len(arguments)),
		)
	}
	if classBinding {
		return &classMethodValue{callable: arguments[0]}, nil
	}
	return &staticMethodValue{callable: arguments[0]}, nil
}

func bindMethodDescriptor(value Value, owner *typeValue) (Value, bool) {
	switch value := value.(type) {
	case *classMethodValue:
		return &boundMethodValue{callable: value.callable, self: owner}, true
	case *staticMethodValue:
		return value.callable, true
	default:
		return nil, false
	}
}

func executeMethodDescriptorAttributeLoad(
	frame *frame,
	instruction int,
	owner Value,
	callable Value,
	name string,
) (instructionOutcome, error) {
	if name == "__isabstractmethod__" {
		return executeAbstractMarker(frame, instruction, callable)
	}
	if name == "__func__" || name == "__wrapped__" {
		return pushOutcome(frame, instruction, callable)
	}
	return raiseOutcome(newException(
		"AttributeError",
		"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
	)), nil
}
