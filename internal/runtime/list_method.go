package runtime

import "strconv"

type listAppendMethod struct {
	list *listValue
}

func (*listAppendMethod) TypeName() string { return "builtin_function_or_method" }
func (*listAppendMethod) Repr() string {
	return "<built-in method append of list object>"
}
func (*listAppendMethod) isValue() {}

func executeListAttributeLoad(
	frame *frame,
	instruction int,
	list *listValue,
	name string,
) (instructionOutcome, error) {
	if name == "append" {
		return pushOutcome(frame, instruction, &listAppendMethod{list: list})
	}
	return raiseOutcome(newException(
		"AttributeError",
		"'list' object has no attribute '"+name+"'",
	)), nil
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
