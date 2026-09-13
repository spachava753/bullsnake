package runtime

import "strconv"

// executeBuiltinGlobals returns the calling frame's defining module dictionary,
// not its locals or its caller's namespace. The namespace owns the stable view.
func executeBuiltinGlobals(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	discardCallSegment(caller, base)
	if keywords != nil && len(keywords.entries) != 0 {
		return raiseOutcome(newException("TypeError", "globals() takes no keyword arguments")), nil
	}
	if len(arguments) != 0 {
		return raiseOutcome(newException("TypeError", "globals() takes no arguments ("+strconv.Itoa(len(arguments))+" given)")), nil
	}
	return pushOutcome(caller, instruction, caller.globals.asDictionary())
}
