package runtime

import "strconv"

type dictionaryCopyMethod struct {
	dictionary *dictValue
}

func (*dictionaryCopyMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryCopyMethod) Repr() string {
	return "<built-in method copy of dict object>"
}
func (*dictionaryCopyMethod) isValue() {}

// executeDictionaryCopyCall clones ordered entry storage while retaining the
// original key and value objects.
func executeDictionaryCopyCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryCopyMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.copy() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 0 {
		count := len(arguments)
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.copy() takes no arguments ("+strconv.Itoa(count)+" given)",
		)), nil
	}
	entries := make([]dictEntry, len(method.dictionary.entries))
	copy(entries, method.dictionary.entries)
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &dictValue{entries: entries})
}
