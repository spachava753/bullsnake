package runtime

type objectValue struct {
	identity byte
}

func (*objectValue) TypeName() string { return "object" }
func (*objectValue) Repr() string     { return "<object object>" }
func (*objectValue) isValue()         {}

func executeObjectTypeCall(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if len(arguments) != 0 || keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException("TypeError", "object() takes no arguments")), nil
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &objectValue{})
}

var _ Value = (*objectValue)(nil)
