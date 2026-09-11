package runtime

import "strconv"

type dictionaryClearMethod struct {
	dictionary *dictValue
}

func (*dictionaryClearMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryClearMethod) Repr() string {
	return "<built-in method clear of dict object>"
}
func (*dictionaryClearMethod) isValue() {}

// executeDictionaryClearCall drops all retained entries and records a key-set
// mutation only when the dictionary was nonempty.
func executeDictionaryClearCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryClearMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.clear() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 0 {
		count := len(arguments)
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.clear() takes no arguments ("+strconv.Itoa(count)+" given)",
		)), nil
	}
	if len(method.dictionary.entries) != 0 {
		for index := range method.dictionary.entries {
			method.dictionary.entries[index] = dictEntry{}
		}
		if method.dictionary.namespace != nil {
			clear(method.dictionary.namespace.values)
		}
		method.dictionary.entries = nil
		method.dictionary.version++
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, None)
}
