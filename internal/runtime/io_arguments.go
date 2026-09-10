package runtime

import "slices"

// bindIOArguments combines positional and named parameters, retaining nil for
// required arguments and rejecting duplicate, unknown, and missing assignments.
func bindIOArguments(name string, arguments []Value, keywords *dictValue, names []string, defaults []Value) ([]Value, *Exception) {
	values := append([]Value(nil), defaults...)
	if len(arguments) > len(values) {
		return nil, newException("TypeError", name+"() received too many arguments")
	}
	copy(values, arguments)
	if keywords != nil {
		for _, entry := range keywords.entries {
			key := entry.key.(*stringValue).value
			index := slices.Index(names, key)
			if index < 0 {
				return nil, newException("TypeError", name+"() got an unexpected keyword argument '"+key+"'")
			}
			if index < len(arguments) {
				return nil, newException("TypeError", name+"() got multiple values for argument '"+key+"'")
			}
			values[index] = entry.value
		}
	}
	for index, value := range values {
		if value == nil {
			return nil, newException("TypeError", name+"() missing required argument '"+names[index]+"'")
		}
	}
	return values, nil
}
