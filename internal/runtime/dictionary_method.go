package runtime

import "strconv"

type dictionaryPopMethod struct {
	dictionary *dictValue
}

type dictionaryGetMethod struct {
	dictionary *dictValue
}

type dictionaryItemsMethod struct {
	dictionary *dictValue
}

type dictionaryKeysMethod struct {
	dictionary *dictValue
}

func (*dictionaryPopMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryPopMethod) Repr() string {
	return "<built-in method pop of dict object>"
}
func (*dictionaryPopMethod) isValue() {}

func (*dictionaryGetMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryGetMethod) Repr() string {
	return "<built-in method get of dict object>"
}
func (*dictionaryGetMethod) isValue() {}

func (*dictionaryItemsMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryItemsMethod) Repr() string {
	return "<built-in method items of dict object>"
}
func (*dictionaryItemsMethod) isValue() {}

func (*dictionaryKeysMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryKeysMethod) Repr() string {
	return "<built-in method keys of dict object>"
}
func (*dictionaryKeysMethod) isValue() {}

// executeDictionaryAttributeLoad returns the implemented bound method for one
// native dictionary or the normal missing-attribute error.
func executeDictionaryAttributeLoad(
	frame *frame,
	instruction int,
	dictionary *dictValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "clear":
		return pushOutcome(
			frame,
			instruction,
			&dictionaryClearMethod{dictionary: dictionary},
		)
	case "pop":
		return pushOutcome(
			frame,
			instruction,
			&dictionaryPopMethod{dictionary: dictionary},
		)
	case "get":
		return pushOutcome(
			frame,
			instruction,
			&dictionaryGetMethod{dictionary: dictionary},
		)
	case "items":
		return pushOutcome(
			frame,
			instruction,
			&dictionaryItemsMethod{dictionary: dictionary},
		)
	case "keys":
		return pushOutcome(
			frame,
			instruction,
			&dictionaryKeysMethod{dictionary: dictionary},
		)
	case "update":
		return pushOutcome(
			frame,
			instruction,
			&dictionaryUpdateMethod{dictionary: dictionary},
		)
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'dict' object has no attribute '"+name+"'",
		)), nil
	}
}

func executeDictionaryItemsCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryItemsMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.items() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 0 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.items() takes no arguments ("+count+" given)",
		)), nil
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &dictionaryItemsView{
		dictionary: method.dictionary,
	})
}

func executeDictionaryKeysCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryKeysMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.keys() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 0 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.keys() takes no arguments ("+count+" given)",
		)), nil
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &dictionaryKeysView{
		dictionary: method.dictionary,
	})
}

// executeDictionaryGetCall returns an existing value or the optional default
// without changing dictionary storage or insertion order.
func executeDictionaryGetCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryGetMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.get() takes no keyword arguments",
		)), nil
	}
	if len(arguments) < 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"get expected at least 1 argument, got 0",
		)), nil
	}
	if len(arguments) > 2 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"get expected at most 2 arguments, got "+count,
		)), nil
	}

	key := arguments[0]
	defaultValue := Value(None)
	if len(arguments) == 2 {
		defaultValue = arguments[1]
	}
	value, found, exception := method.dictionary.get(key)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	if !found {
		value = defaultValue
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, value)
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
