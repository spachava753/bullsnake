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

// executeDictionaryUpdateCall snapshots a native source and keyword entries,
// then replays them into the target without changing existing key positions.
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
	var sourceEntries []dictEntry
	if len(arguments) == 1 {
		source, ok := arguments[0].(*dictValue)
		if !ok {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"NotImplementedError",
				"dict.update iterable and user mapping inputs are not supported",
			)), nil
		}
		sourceEntries = append(sourceEntries, source.entries...)
	}
	var keywordEntries []dictEntry
	if keywords != nil {
		keywordEntries = append(keywordEntries, keywords.entries...)
	}
	discardCallSegment(caller, base)
	for _, entry := range sourceEntries {
		if exception := method.dictionary.set(entry.key, entry.value); exception != nil {
			return raiseOutcome(exception), nil
		}
	}
	for _, entry := range keywordEntries {
		if exception := method.dictionary.set(entry.key, entry.value); exception != nil {
			return raiseOutcome(exception), nil
		}
	}
	return pushOutcome(caller, instruction, None)
}
