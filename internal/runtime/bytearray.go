package runtime

import "strings"

type bytearrayValue struct {
	value string
}

func (*bytearrayValue) TypeName() string { return "bytearray" }
func (value *bytearrayValue) Repr() string {
	return "bytearray(" + (&bytesValue{value: value.value}).Repr() + ")"
}
func (*bytearrayValue) isValue() {}

// attribute exposes the mutable bytes operations shared with Python bytearray.
func (value *bytearrayValue) attribute(name string) (Value, bool) {
	if name != "startswith" && name != "endswith" {
		return nil, false
	}
	return nativeFunctionNamed("bytearray."+name, 1, 3,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			candidates := []Value{arguments[0]}
			if tuple, ok := arguments[0].(*tupleValue); ok {
				candidates = tuple.elements
			}
			for _, choice := range candidates {
				candidate, ok := choice.(*bytesValue)
				if !ok {
					return nil, newException("TypeError", name+" first arg must be bytes"), nil
				}
				matched := strings.HasPrefix(value.value, candidate.value)
				if name == "endswith" {
					matched = strings.HasSuffix(value.value, candidate.value)
				}
				if matched {
					return trueSingleton, nil, nil
				}
			}
			return falseSingleton, nil, nil
		}), true
}

var _ Value = (*bytearrayValue)(nil)
