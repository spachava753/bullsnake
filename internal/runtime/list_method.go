package runtime

import "strconv"

type listAppendMethod struct {
	list *listValue
}

type listPopMethod struct {
	list *listValue
}

func (*listAppendMethod) TypeName() string { return "builtin_function_or_method" }
func (*listAppendMethod) Repr() string {
	return "<built-in method append of list object>"
}
func (*listAppendMethod) isValue() {}

func (*listPopMethod) TypeName() string { return "builtin_function_or_method" }
func (*listPopMethod) Repr() string {
	return "<built-in method pop of list object>"
}
func (*listPopMethod) isValue() {}

func executeListAttributeLoad(
	frame *frame,
	instruction int,
	list *listValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "append":
		return pushOutcome(frame, instruction, &listAppendMethod{list: list})
	case "pop":
		return pushOutcome(frame, instruction, &listPopMethod{list: list})
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'list' object has no attribute '"+name+"'",
		)), nil
	}
}

func executeListAppendCall(
	caller *frame,
	instruction int,
	base int,
	method *listAppendMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.append() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.append() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	method.list.elements = append(method.list.elements, arguments[0])
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, None)
}

// executeListPopCall removes one selected element after applying Python's
// positional-only integer index rules.
func executeListPopCall(
	caller *frame,
	instruction int,
	base int,
	method *listPopMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.pop() takes no keyword arguments",
		)), nil
	}
	if len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"pop expected at most 1 argument, got "+strconv.Itoa(len(arguments)),
		)), nil
	}

	index := -1
	if len(arguments) == 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok {
			message := "'" + arguments[0].TypeName() +
				"' object cannot be interpreted as an integer"
			discardCallSegment(caller, base)
			return raiseOutcome(newException("TypeError", message)), nil
		}
		if !integer.IsInt64() || int64(int(integer.Int64())) != integer.Int64() {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"OverflowError",
				"Python int too large to convert to C ssize_t",
			)), nil
		}
		index = int(integer.Int64())
	}

	if len(method.list.elements) == 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException("IndexError", "pop from empty list")), nil
	}
	if index < 0 {
		index += len(method.list.elements)
	}
	if index < 0 || index >= len(method.list.elements) {
		discardCallSegment(caller, base)
		return raiseOutcome(newException("IndexError", "pop index out of range")), nil
	}

	value := method.list.elements[index]
	copy(method.list.elements[index:], method.list.elements[index+1:])
	last := len(method.list.elements) - 1
	method.list.elements[last] = nil
	method.list.elements = method.list.elements[:last]
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, value)
}
