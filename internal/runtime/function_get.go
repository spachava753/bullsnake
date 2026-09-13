package runtime

// executeFunctionGet binds a Python function to any non-None receiver. The
// optional owner only distinguishes unbound access from an invalid empty pair.
func executeFunctionGet(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__get__", arguments, keywords, 1, 2); exception != nil {
		return raiseOutcome(exception), nil
	}
	if arguments[0] == None {
		if len(arguments) == 1 || arguments[1] == None {
			return raiseOutcome(newException("TypeError", "__get__(None, None) is invalid")), nil
		}
		return pushOutcome(caller, instruction, self)
	}
	return pushOutcome(caller, instruction, &boundMethodValue{callable: self, self: arguments[0]})
}
