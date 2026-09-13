package runtime

// executeObjectInitSubclass is the cooperative endpoint for class initialization:
// it accepts no arguments and rejects any unconsumed class keywords.
func executeObjectInitSubclass(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments(allocationClassName(self)+".__init_subclass__", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, None)
}
