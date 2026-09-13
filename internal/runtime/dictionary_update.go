package runtime

import "strconv"

type dictionaryUpdateMethod struct {
	dictionary *dictValue
}

func (*dictionaryUpdateMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryUpdateMethod) Repr() string {
	return "<built-in method update of dict object>"
}
func (*dictionaryUpdateMethod) isValue() {}

// executeDictionaryUpdateCall validates call shape before consuming mappings or
// iterable pairs. Target mutations are incremental, and keywords follow source.
func executeDictionaryUpdateCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryUpdateMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if len(arguments) > 1 {
		count := len(arguments)
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"update expected at most 1 argument, got "+strconv.Itoa(count),
		)), nil
	}
	call := &dictionaryInputCall{instruction: instruction, target: method.dictionary, sentinel: &dictValue{}}
	if len(arguments) == 1 {
		call.source = arguments[0]
	}
	if keywords != nil {
		call.keywords = append([]dictEntry(nil), keywords.entries...)
	}
	discardCallSegment(caller, base)
	return call.start(caller)
}
