package runtime

// executeObjectFormat validates text specs before invoking the actual string
// conversion; nonempty specs are unsupported even if the receiver overrides format.
func executeObjectFormat(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__format__", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	spec, ok := arguments[0].(*stringValue)
	if !ok {
		return raiseOutcome(newException("TypeError", "__format__() argument must be str, not "+arguments[0].TypeName())), nil
	}
	if spec.value != "" {
		return raiseOutcome(newException("TypeError", "unsupported format string passed to "+self.TypeName()+".__format__")), nil
	}
	return executeString(caller, instruction, self)
}
