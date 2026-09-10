package runtime

// executeTextNext calls subclass readline overrides but keeps exact native
// iteration independent of instance attributes. Only exhaustion restores tell.
func executeTextNext(caller *frame, instruction int, self *instanceValue, class *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__next__", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	stream := self.io.wrapper
	if stream == nil {
		return raiseOutcome(newException("ValueError", "I/O operation on uninitialized object")), nil
	}
	if stream.buffer == nil {
		return raiseOutcome(newException("ValueError", "underlying buffer has been detached")), nil
	}
	stream.telling = false
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		if self.class == class {
			return executeTextRead(caller, instruction, self, "readline", nil, nil)
		}
		return executeMethodCall(caller, instruction, self, "readline", nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		text, ok := result.(*stringValue)
		if !ok {
			return raiseOutcome(newException("OSError", "readline() should have returned a str object, not '"+result.TypeName()+"'")), nil
		}
		if text.value == "" {
			stream.telling = stream.seekable
			return raiseOutcome(newException("StopIteration", "")), nil
		}
		return pushOutcome(current, instruction, result)
	})
}
