package runtime

func storeFunctionDocstring(function *functionValue, value Value) (instructionOutcome, error) {
	if value == nil {
		value = None
	}
	function.docstring = value
	return instructionOutcome{kind: advance}, nil
}
