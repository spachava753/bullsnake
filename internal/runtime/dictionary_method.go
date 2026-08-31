package runtime

import "strconv"

type dictionaryPopMethod struct {
	dictionary *dictValue
}

func (*dictionaryPopMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryPopMethod) Repr() string {
	return "<built-in method pop of dict object>"
}
func (*dictionaryPopMethod) isValue() {}

func executeDictionaryAttributeLoad(
	frame *frame,
	instruction int,
	dictionary *dictValue,
	name string,
) (instructionOutcome, error) {
	if name == "pop" {
		return pushOutcome(
			frame,
			instruction,
			&dictionaryPopMethod{dictionary: dictionary},
		)
	}
	return raiseOutcome(newException(
		"AttributeError",
		"'dict' object has no attribute '"+name+"'",
	)), nil
}

// executeDictionaryPopCall returns and removes a key, or returns the optional
// default without mutating the dictionary when the key is absent.
func executeDictionaryPopCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryPopMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.pop() takes no keyword arguments",
		)), nil
	}
	if len(arguments) < 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"pop expected at least 1 argument, got 0",
		)), nil
	}
	if len(arguments) > 2 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"pop expected at most 2 arguments, got "+count,
		)), nil
	}

	key := arguments[0]
	var defaultValue Value
	if len(arguments) == 2 {
		defaultValue = arguments[1]
	}
	value, found, exception := method.dictionary.get(key)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	if found {
		_, exception = method.dictionary.delete(key)
		if exception != nil {
			discardCallSegment(caller, base)
			return raiseOutcome(exception), nil
		}
		discardCallSegment(caller, base)
		return pushOutcome(caller, instruction, value)
	}
	if len(arguments) == 2 {
		discardCallSegment(caller, base)
		return pushOutcome(caller, instruction, defaultValue)
	}

	message := key.Repr()
	discardCallSegment(caller, base)
	return raiseOutcome(newException("KeyError", message)), nil
}
