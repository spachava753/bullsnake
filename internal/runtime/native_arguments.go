package runtime

import "fmt"

// checkNativeArguments enforces positional-only arity before any provider call.
func checkNativeArguments(name string, arguments []Value, keywords *dictValue, minimum, maximum int) *Exception {
	if keywords != nil && len(keywords.entries) != 0 {
		return newException("TypeError", name+"() takes no keyword arguments")
	}
	if len(arguments) < minimum || len(arguments) > maximum {
		return newException("TypeError", fmt.Sprintf("%s() takes %d to %d arguments (%d given)", name, minimum, maximum, len(arguments)))
	}
	return nil
}
