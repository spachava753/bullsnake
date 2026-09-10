package runtime

// resetInput discards decode read-ahead after a write or successful seek. Codec
// state is runtime-owned and reset without touching host state or GC callbacks.
func (stream *textWrapper) resetInput() {
	stream.input, stream.decoded, stream.pendingCR = nil, nil, nil
	stream.decodeOffset, stream.skipped, stream.seen = 0, 0, 0
	stream.readStarted = false
}

// executeTextPosition uses byte offsets as restore cookies for the implemented
// stateless codecs. Relative seeks are restricted to zero as Python requires.
func executeTextPosition(caller *frame, instruction int, self *instanceValue, name string, arguments []Value) (instructionOutcome, error) {
	stream := self.io.wrapper
	return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
		if name == "truncate" {
			return continueNativeOperation(current, instruction, func() (instructionOutcome, error) { return executeMethodCall(current, instruction, self, "flush", nil) }, func(resumed *frame, _ Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				var position Value = None
				if len(arguments) > 0 {
					position = arguments[0]
				}
				return executeMethodCall(resumed, instruction, stream.buffer, "truncate", []Value{position})
			})
		}
		if !stream.seekable {
			return raiseOutcome(unsupportedStreamOperation("underlying stream is not seekable")), nil
		}
		if name == "tell" {
			return tellTextWrapper(current, instruction, stream)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) { return executeIOIndex(current, instruction, arguments[0]) }, func(resumed *frame, offset Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			var origin Value = integerFromInt64(0)
			if len(arguments) > 1 {
				origin = arguments[1]
			}
			return continueNativeOperation(resumed, instruction, func() (instructionOutcome, error) { return executeIOIndex(resumed, instruction, origin) }, func(converted *frame, whence Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				mode := whence.(*intValue).value.Int64()
				if mode < 0 || mode > 2 {
					return raiseOutcome(newException("ValueError", "invalid whence")), nil
				}
				if mode != 0 && offset.(*intValue).value.Sign() != 0 {
					return raiseOutcome(unsupportedStreamOperation("can't do nonzero relative seeks")), nil
				}
				if offset.(*intValue).value.Sign() < 0 {
					return raiseOutcome(newException("ValueError", "negative seek position")), nil
				}
				if mode == 1 {
					return continueNativeOperation(converted, instruction, func() (instructionOutcome, error) {
						return executeMethodCall(converted, instruction, self, "tell", nil)
					}, func(told *frame, offset Value, exception *Exception) (instructionOutcome, error) {
						if exception != nil {
							return raiseOutcome(exception), nil
						}
						return seekTextWrapper(told, instruction, self, stream, offset, integerFromInt64(0))
					})
				}
				return seekTextWrapper(converted, instruction, self, stream, offset, whence)
			})
		})
	})
}

// tellTextWrapper flushes encoded output and subtracts unread source bytes from
// the binary position, rejecting disabled telling and invalid raw positions.
func tellTextWrapper(caller *frame, instruction int, stream *textWrapper) (instructionOutcome, error) {
	if !stream.telling {
		return raiseOutcome(newException("OSError", "telling position disabled by next() call")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return flushTextBytes(caller, instruction, stream) }, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(current, instruction, stream.buffer, "tell", nil)
		}, func(resumed *frame, position Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			value, ok := integerOperand(position)
			if !ok {
				return raiseOutcome(newException("TypeError", "an integer is required")), nil
			}
			value.Sub(&value, &integerFromInt64(int64(len(stream.input))).value)
			if value.Sign() < 0 {
				return raiseOutcome(newException("OSError", "underlying buffer returned an invalid position")), nil
			}
			return pushOutcome(resumed, instruction, &intValue{value: value})
		})
	})
}

// seekTextWrapper flushes before moving the binary cursor and drops decoded
// read-ahead only on success, allowing a failed seek to retain usable state.
func seekTextWrapper(caller *frame, instruction int, self *instanceValue, stream *textWrapper, offset, whence Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeMethodCall(caller, instruction, self, "flush", nil) }, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(current, instruction, stream.buffer, "seek", []Value{offset, whence})
		}, func(resumed *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			stream.resetInput()
			stream.telling = stream.seekable
			return pushOutcome(resumed, instruction, result)
		})
	})
}
