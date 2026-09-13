package runtime

// executeSetPop removes an arbitrary stored member without copying its value or
// invoking equality. Empty sets and malformed calls leave the set unchanged.
func executeSetPop(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("pop", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	set := self.(*setValue)
	if len(set.entries) == 0 {
		return raiseOutcome(newException("KeyError", "pop from an empty set")), nil
	}
	last := len(set.entries) - 1
	result := set.entries[last]
	set.entries[last] = nil
	set.entries = set.entries[:last]
	return pushOutcome(caller, instruction, result)
}
