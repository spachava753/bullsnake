package runtime

// executeBufferedRandom shares the reader and writer machinery while keeping a
// single logical position. Reads flush pending writes; writes rewind read-ahead.
func executeBufferedRandom(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if name == "__init__" {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return initializeBuffered(caller, instruction, self, "BufferedRandom", "seekable", arguments, keywords)
		}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return checkRandomCapabilities(current, instruction, self, "readable")
		})
	}
	stream := self.io.buffered
	if exception := checkBufferedAttached(stream); exception != nil {
		return raiseOutcome(exception), nil
	}
	switch name {
	case "read", "read1", "readinto", "readinto1", "readline", "peek":
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeBufferedWriter(caller, instruction, self, "flush", nil, nil)
		}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeBufferedReader(current, instruction, self, name, arguments, keywords)
		})
	case "write":
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return rewindBuffered(caller, instruction, self, stream) }, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeBufferedWriter(current, instruction, self, name, arguments, keywords)
		})
	case "flush":
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeBufferedWriter(caller, instruction, self, name, arguments, keywords)
		}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return rewindBuffered(current, instruction, self, stream)
		})
	case "readable":
		return executeBufferedReader(caller, instruction, self, name, arguments, keywords)
	default:
		return executeBufferedWriter(caller, instruction, self, name, arguments, keywords)
	}
}

// checkRandomCapabilities validates the two remaining capabilities in CPython
// order and discards newly initialized state if either callback rejects access.
func checkRandomCapabilities(caller *frame, instruction int, self *instanceValue, capability string) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeMethodCall(caller, instruction, self.io.buffered.raw, capability, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil || result != trueSingleton {
			self.io.buffered = nil
			if exception == nil {
				exception = unsupportedStreamOperation("File or stream is not " + capability + ".")
			}
			return raiseOutcome(exception), nil
		}
		if capability == "readable" {
			return checkRandomCapabilities(current, instruction, self, "writable")
		}
		return pushOutcome(current, instruction, None)
	})
}

// rewindBuffered returns the raw cursor to the logical cursor before switching
// from reads to writes. Failed seeks leave unread bytes available for retry.
func rewindBuffered(caller *frame, instruction int, self *instanceValue, stream *bufferedStream) (instructionOutcome, error) {
	return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
		return withIOGuard(current, instruction, &stream.busy, func() (instructionOutcome, error) {
			if len(stream.read) == 0 {
				return pushOutcome(current, instruction, None)
			}
			return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
				return executeMethodCall(current, instruction, stream.raw, "seek", []Value{integerFromInt64(-int64(len(stream.read))), integerFromInt64(1)})
			}, func(resumed *frame, result Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				position, ok := integerOperand(result)
				if !ok || position.Sign() < 0 {
					return raiseOutcome(newException("OSError", "Raw stream returned invalid position")), nil
				}
				stream.read = nil
				return pushOutcome(resumed, instruction, None)
			})
		})
	})
}
