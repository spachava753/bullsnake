package runtime

import "strings"

// executeTextRead converts character limits before stream checks, flushes pending
// encoded output, and starts incremental decoding through binary callbacks.
func executeTextRead(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments(name, arguments, keywords, 0, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	var size Value = integerFromInt64(-1)
	if len(arguments) != 0 && arguments[0] != None {
		size = arguments[0]
	}
	if name == "readline" && len(arguments) != 0 && arguments[0] == None {
		size = None
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeIOIndex(caller, instruction, size) }, func(current *frame, size Value, exception *Exception) (instructionOutcome, error) {
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
		return withIOOpen(current, instruction, self, func(opened *frame) (instructionOutcome, error) {
			if !stream.readable {
				return raiseOutcome(unsupportedStreamOperation("not readable")), nil
			}
			return continueNativeOperation(opened, instruction, func() (instructionOutcome, error) { return flushTextBytes(opened, instruction, stream) }, func(resumed *frame, _ Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				call := &textReadCall{stream: stream, instruction: instruction, limit: int(size.(*intValue).value.Int64()), line: name != "read"}
				return call.advance(resumed)
			})
		})
	})
}

type textReadCall struct {
	stream                    *textWrapper
	instruction, limit, count int
	line, done, eof           bool
	result                    strings.Builder
}

// advance drains decoded characters then obtains another binary chunk. Native
// short reads loop without Go stack growth; Python reads suspend in the VM.
func (call *textReadCall) advance(caller *frame) (instructionOutcome, error) {
	for {
		// Unbounded reads retain old read-ahead until the final decode succeeds.
		if call.line || call.limit >= 0 || call.eof {
			call.consume()
		}
		if call.done || call.eof {
			return pushOutcome(caller, call.instruction, &stringValue{value: call.result.String()})
		}
		if call.line {
			call.stream.readStarted = false
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			method := "read"
			var arguments []Value
			if call.line || call.limit >= 0 {
				if call.stream.hasRead1 {
					method = "read1"
				}
				arguments = []Value{integerFromInt64(8192)}
			}
			outcome, err := executeRetryIOCall(caller, call.instruction, call.stream.buffer, method, arguments)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !call.line && call.limit < 0 && result == None {
				return raiseOutcome(newException("BlockingIOError", "Read returned None.")), nil
			}
			data, exception := binaryData(result)
			if exception != nil {
				return raiseOutcome(newException("TypeError", "underlying read() should have returned a bytes-like object, not '"+result.TypeName()+"'")), nil
			}
			call.eof = len(data) == 0 || !call.line && call.limit < 0
			if exception := call.stream.decodeChunk(data, call.eof); exception != nil {
				return raiseOutcome(exception), nil
			}
			if call.line || call.limit >= 0 {
				call.stream.readStarted = true
				call.stream.readSnapshot = call.stream.readSnapshot || call.stream.telling
			} else if call.stream.readSnapshot {
				// A read-all clears the saved seekable decoded buffer.
				call.stream.readStarted, call.stream.readSnapshot = false, false
			}
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
}

// consume advances only by complete Python characters and their corresponding
// byte widths. Universal-preserved CRLF forms one line unless the size cuts it.
func (call *textReadCall) consume() {
	stream := call.stream
	for len(stream.decoded) > 0 && (call.limit < 0 || call.count < call.limit) {
		unit := stream.decoded[0]
		stream.decoded = stream.decoded[1:]
		stream.input = stream.input[unit.width:]
		stream.decodeOffset -= unit.width
		call.result.WriteString(unit.text)
		call.count++
		if call.line {
			if stream.universal && unit.text == "\r" && len(stream.decoded) > 0 && stream.decoded[0].text == "\n" && (call.limit < 0 || call.count < call.limit) {
				continue
			}
			ending := stream.newline
			if stream.universal {
				ending = "\n"
			}
			if strings.HasSuffix(call.result.String(), ending) || stream.universal && unit.text == "\r" {
				call.done = true
				return
			}
		}
	}
	if call.limit >= 0 && call.count >= call.limit {
		call.done = true
	}
	if call.eof && len(stream.decoded) == 0 && stream.pendingCR == nil {
		stream.input, stream.decodeOffset, stream.skipped = nil, 0, 0
		if call.line && !call.done {
			stream.readStarted, stream.readSnapshot = false, false
		}
	}
}
