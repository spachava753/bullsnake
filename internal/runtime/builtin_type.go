package runtime

type builtinTypeValue struct {
	name    string
	matches func(Value) bool
}

func (*builtinTypeValue) TypeName() string { return "type" }
func (class *builtinTypeValue) Repr() string {
	return "<class '" + class.name + "'>"
}
func (*builtinTypeValue) isValue() {}

var builtinTypes = []*builtinTypeValue{
	{name: "object", matches: func(Value) bool { return true }},
	{name: "str", matches: func(value Value) bool { _, ok := value.(*stringValue); return ok }},
	{name: "bytes", matches: func(value Value) bool { _, ok := value.(*bytesValue); return ok }},
	{name: "bool", matches: func(value Value) bool { _, ok := value.(*boolValue); return ok }},
	{name: "int", matches: func(value Value) bool {
		switch value.(type) {
		case *intValue, *boolValue:
			return true
		default:
			return false
		}
	}},
	{name: "float", matches: func(value Value) bool { _, ok := value.(*floatValue); return ok }},
	{name: "complex", matches: func(value Value) bool { _, ok := value.(*complexValue); return ok }},
	{name: "tuple", matches: func(value Value) bool { _, ok := value.(*tupleValue); return ok }},
	{name: "list", matches: func(value Value) bool { _, ok := value.(*listValue); return ok }},
	{name: "dict", matches: func(value Value) bool { _, ok := value.(*dictValue); return ok }},
	{name: "set", matches: func(value Value) bool { _, ok := value.(*setValue); return ok }},
	{name: "type", matches: func(value Value) bool {
		switch value.(type) {
		case *builtinTypeValue, *exceptionTypeValue, *typeValue:
			return true
		default:
			return false
		}
	}},
}

// valueIsInstance matches built-in markers, user classes, exception classes,
// and recursive tuples while retaining invalid class information as TypeError.
func valueIsInstance(value Value, classInfo Value) (bool, *Exception) {
	switch classInfo := classInfo.(type) {
	case *builtinTypeValue:
		return classInfo.matches(value), nil
	case *typeValue:
		instance, ok := value.(*instanceValue)
		return ok && instance.class.isSubclassOf(classInfo), nil
	case *exceptionTypeValue:
		exception, ok := value.(*Exception)
		if !ok {
			return false, nil
		}
		if exception.userClass != nil {
			base := exception.userClass.builtinExceptionBase()
			return base != nil && base.isSubclassOf(classInfo), nil
		}
		return exception.class.isSubclassOf(classInfo), nil
	case *tupleValue:
		for _, candidate := range classInfo.elements {
			matches, exception := valueIsInstance(value, candidate)
			if exception != nil || matches {
				return matches, exception
			}
		}
		return false, nil
	default:
		return false, newException(
			"TypeError",
			"isinstance() arg 2 must be a type, a tuple of types, or a union",
		)
	}
}

var _ Value = (*builtinTypeValue)(nil)
