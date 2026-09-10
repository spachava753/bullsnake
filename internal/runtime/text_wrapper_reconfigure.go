package runtime

import "strings"

// reconfigureTextWrapper validates keyword-only options and protects decoded
// input. Omitted newline differs from explicit None, which restores universal mode.
func reconfigureTextWrapper(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if len(arguments) != 0 {
		return raiseOutcome(newException("TypeError", "reconfigure() takes no positional arguments")), nil
	}
	missing := &dictValue{}
	values, exception := bindIOArguments("reconfigure", nil, keywords, []string{"encoding", "errors", "newline", "line_buffering", "write_through"}, []Value{None, None, missing, None, None})
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	stream := self.io.wrapper
	if stream == nil {
		return raiseOutcome(newException("ValueError", "I/O operation on uninitialized object")), nil
	}
	if stream.buffer == nil {
		return raiseOutcome(newException("ValueError", "underlying buffer has been detached")), nil
	}
	for _, value := range values[:3] {
		if value == None || value == missing {
			continue
		}
		text, ok := value.(*stringValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "encoding, errors, and newline must be str or None")), nil
		}
		if strings.ContainsRune(text.value, 0) {
			return raiseOutcome(newException("ValueError", "embedded null character")), nil
		}
	}
	if stream.readStarted && (values[0] != None || values[1] != None || values[2] != missing) {
		return raiseOutcome(unsupportedStreamOperation("It is not possible to set the encoding or newline of stream after the first read")), nil
	}
	updated := *stream
	if values[0] != None {
		updated.encoding, updated.errors = values[0].(*stringValue).value, "strict"
	}
	if values[1] != None {
		updated.errors = values[1].(*stringValue).value
	}
	if values[2] != missing {
		updated.newline, updated.universal, updated.translate = "", true, values[2] == None
		if values[2] != None {
			newline := values[2].(*stringValue).value
			switch newline {
			case "", "\n", "\r", "\r\n":
				updated.newline, updated.universal = newline, newline == ""
			default:
				return raiseOutcome(newException("ValueError", "illegal newline value")), nil
			}
		}
	}
	call := &textReconfiguration{self: self, stream: stream, updated: &updated, values: values, instruction: instruction, changed: values[0] != None || values[1] != None || values[2] != missing}
	return call.advance(caller, 0)
}

type textReconfiguration struct {
	self            *instanceValue
	stream, updated *textWrapper
	values          []Value
	instruction     int
	changed         bool
}

// advance converts integer-style optional flags before flushing. Configuration
// changes apply only after that flush, preserving queued bytes' original codec.
func (call *textReconfiguration) advance(caller *frame, step int) (instructionOutcome, error) {
	if step < 2 {
		if call.values[step+3] == None {
			return call.advance(caller, step+1)
		}
		return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			return executeIOIndex(caller, call.instruction, call.values[step+3])
		}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			flag := value.(*intValue).value.Sign() != 0
			if step == 0 {
				call.updated.lineBuffering = flag
			} else {
				call.updated.writeThrough = flag
			}
			return call.advance(current, step+1)
		})
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeMethodCall(caller, call.instruction, call.self, "flush", nil)
	}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		codec, exception := textCodec(call.updated.encoding)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		stream, updated := call.stream, call.updated
		stream.encoding, stream.codec, stream.errors = updated.encoding, codec, updated.errors
		stream.newline, stream.universal, stream.translate = updated.newline, updated.universal, updated.translate
		stream.lineBuffering, stream.writeThrough = updated.lineBuffering, updated.writeThrough
		if call.changed {
			stream.resetInput()
		}
		return pushOutcome(current, call.instruction, None)
	})
}
