package runtime

import "strconv"

type setAddMethod struct {
	set *setValue
}

func (*setAddMethod) TypeName() string { return "builtin_function_or_method" }
func (*setAddMethod) Repr() string {
	return "<built-in method add of set object>"
}
func (*setAddMethod) isValue() {}

func executeSetAttributeLoad(
	frame *frame,
	instruction int,
	set *setValue,
	name string,
) (instructionOutcome, error) {
	if name == "add" {
		return pushOutcome(frame, instruction, &setAddMethod{set: set})
	}
	return raiseOutcome(newException(
		"AttributeError",
		"'set' object has no attribute '"+name+"'",
	)), nil
}

func executeSetAddCall(
	caller *frame,
	instruction int,
	base int,
	method *setAddMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"set.add() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"set.add() takes exactly one argument ("+count+" given)",
		)), nil
	}
	exception := method.set.add(arguments[0])
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, None)
}
