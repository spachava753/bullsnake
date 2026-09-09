package runtime

import "fmt"

type propertyValue struct {
	getter  Value
	setter  Value
	deleter Value
	doc     Value
	name    string
}

func (*propertyValue) TypeName() string { return "property" }
func (*propertyValue) Repr() string     { return "<property object>" }
func (*propertyValue) isValue()         {}

type propertyAccessorKind uint8

const (
	propertyGetterCopy propertyAccessorKind = iota
	propertySetterCopy
	propertyDeleterCopy
)

type propertyAccessorMethod struct {
	property *propertyValue
	kind     propertyAccessorKind
}

func (*propertyAccessorMethod) TypeName() string { return "builtin_function_or_method" }
func (method *propertyAccessorMethod) Repr() string {
	name := "getter"
	if method.kind == propertySetterCopy {
		name = "setter"
	} else if method.kind == propertyDeleterCopy {
		name = "deleter"
	}
	return "<built-in method " + name + " of property object>"
}
func (*propertyAccessorMethod) isValue() {}

// builtinProperty binds the four public constructor fields while preserving
// None as an absent accessor and an ordinary value for explicit documentation.
func builtinProperty(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if len(arguments) > 4 {
		return nil, newException(
			"TypeError",
			fmt.Sprintf("property expected at most 4 arguments, got %d", len(arguments)),
		)
	}
	values := make([]Value, 4)
	copy(values, arguments)
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "property() keywords must be strings")
			}
			index := propertyArgumentIndex(name.value)
			if index < 0 {
				return nil, newException(
					"TypeError",
					"property() got an unexpected keyword argument '"+name.value+"'",
				)
			}
			if index < len(arguments) || values[index] != nil {
				return nil, newException(
					"TypeError",
					"property() got multiple values for argument '"+name.value+"'",
				)
			}
			values[index] = entry.value
		}
	}
	for index := 0; index < 3; index++ {
		if values[index] == None {
			values[index] = nil
		}
	}
	doc := values[3]
	if doc == nil {
		doc = None
	}
	return &propertyValue{
		getter:  values[0],
		setter:  values[1],
		deleter: values[2],
		doc:     doc,
	}, nil
}

func propertyArgumentIndex(name string) int {
	switch name {
	case "fget":
		return 0
	case "fset":
		return 1
	case "fdel":
		return 2
	case "doc":
		return 3
	default:
		return -1
	}
}

// executePropertyAttributeLoad exposes accessor copies and the metadata that the
// current property implementation can produce without general descriptors.
func executePropertyAttributeLoad(
	frame *frame,
	instruction int,
	property *propertyValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "__isabstractmethod__":
		return executePropertyAbstractMarker(frame, instruction, []Value{property.getter, property.setter, property.deleter})

	case "fget":
		return pushOutcome(frame, instruction, propertyValueOrNone(property.getter))
	case "fset":
		return pushOutcome(frame, instruction, propertyValueOrNone(property.setter))
	case "fdel":
		return pushOutcome(frame, instruction, propertyValueOrNone(property.deleter))
	case "__doc__":
		return pushOutcome(frame, instruction, property.doc)
	case "__name__":
		if name := property.displayName(); name != "" {
			return pushOutcome(frame, instruction, &stringValue{value: name})
		}
	case "getter":
		return pushOutcome(frame, instruction, &propertyAccessorMethod{
			property: property,
			kind:     propertyGetterCopy,
		})
	case "setter":
		return pushOutcome(frame, instruction, &propertyAccessorMethod{
			property: property,
			kind:     propertySetterCopy,
		})
	case "deleter":
		return pushOutcome(frame, instruction, &propertyAccessorMethod{
			property: property,
			kind:     propertyDeleterCopy,
		})
	}
	return instructionOutcome{
		kind: raised,
		exception: newException(
			"AttributeError",
			"'property' object has no attribute '"+name+"'",
		),
	}, nil
}

func propertyValueOrNone(value Value) Value {
	if value == nil {
		return None
	}
	return value
}

func (property *propertyValue) displayName() string {
	if property.name != "" {
		return property.name
	}
	if getter, ok := property.getter.(*functionValue); ok {
		return getter.code.code.Name()
	}
	return ""
}

// executePropertyAccessorCall returns a property copy with one accessor changed,
// leaving the decorated property untouched as Python decorators expect.
func executePropertyAccessorCall(
	caller *frame,
	instruction int,
	base int,
	method *propertyAccessorMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"property accessor methods take no keyword arguments",
			),
		}, nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				fmt.Sprintf("property accessor method requires 1 argument, got %d", len(arguments)),
			),
		}, nil
	}
	copy := *method.property
	accessor := arguments[0]
	if accessor != None {
		switch method.kind {
		case propertyGetterCopy:
			copy.getter = accessor
		case propertySetterCopy:
			copy.setter = accessor
		case propertyDeleterCopy:
			copy.deleter = accessor
		}
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &copy)
}

// executePropertyDescriptorCall selects one property accessor, reports a
// missing half, or invokes the selected Python callable through the frame loop.
func executePropertyDescriptorCall(
	frame *frame,
	instruction int,
	kind attributeCallKind,
	property *propertyValue,
	owner *instanceValue,
	value Value,
) (instructionOutcome, error) {
	accessor := property.getter
	operation := "getter"
	arguments := []Value{owner}
	if kind == attributeSet {
		accessor = property.setter
		operation = "setter"
		arguments = []Value{owner, value}
	} else if kind == attributeDelete {
		accessor = property.deleter
		operation = "deleter"
	}
	if accessor == nil {
		return propertyAccessFailure(property, owner.class, operation), nil
	}
	return executeAttributeCallable(
		frame,
		instruction,
		kind,
		accessor,
		arguments,
	)
}

func propertyAccessFailure(
	property *propertyValue,
	owner *typeValue,
	operation string,
) instructionOutcome {
	name := property.displayName()
	message := "property of '" + owner.qualifiedName + "' object has no " + operation
	if name != "" {
		message = "property '" + name + "' of '" + owner.qualifiedName +
			"' object has no " + operation
	}
	return instructionOutcome{
		kind:      raised,
		exception: newException("AttributeError", message),
	}
}

var _ Value = (*propertyValue)(nil)
var _ Value = (*propertyAccessorMethod)(nil)
