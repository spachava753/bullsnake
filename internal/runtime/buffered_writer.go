package runtime

// executeBufferedWriter snapshots writable input before callbacks, keeps small
// writes pending, and flushes output before position-changing raw operations.
func executeBufferedWriter(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if name == "__init__" {
		return initializeBuffered(caller, instruction, self, "BufferedWriter", "writable", arguments, keywords)
	}
	minimum, maximum := streamMethodArity(name)
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	var data []byte
	if name == "write" {
		value, exception := binaryData(arguments[0])
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		data = append([]byte(nil), value...)
	}
	stream := self.io.buffered
	if exception := checkBufferedAttached(stream); exception != nil {
		return raiseOutcome(exception), nil
	}
	switch name {
	case "write", "flush":
		return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
			return withIOGuard(current, instruction, &stream.busy, func() (instructionOutcome, error) {
				call := &bufferedWriteCall{stream: stream, instruction: instruction, input: data, writing: name == "write"}
				if name == "write" && len(data) < stream.size && len(data) <= stream.size-len(stream.write) {
					stream.write = append(stream.write, data...)
					return pushOutcome(current, instruction, integerFromInt64(int64(len(data))))
				}
				return call.advance(current)
			})
		})
	case "seek", "truncate":
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeMethodCall(caller, instruction, self, "flush", nil) }, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if name == "seek" {
				return seekBuffered(current, instruction, self, stream, arguments)
			}
			return executeMethodCall(current, instruction, stream.raw, name, arguments)
		})
	case "tell":
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeBufferedReader(caller, instruction, self, name, arguments, keywords)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			value, _ := integerOperand(result)
			value.Add(&value, &integerFromInt64(int64(len(stream.write))).value)
			return pushOutcome(current, instruction, &intValue{value: value})
		})
	case "readable", "writable":
		return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
			if name == "readable" {
				return pushOutcome(current, instruction, falseSingleton)
			}
			return executeMethodCall(current, instruction, stream.raw, name, nil)
		})
	default:
		return executeBufferedReader(caller, instruction, self, name, arguments, keywords)
	}
}

type bufferedWriteCall struct {
	stream      *bufferedStream
	instruction int
	input       []byte
	accepted    int
	writing     bool
	direct      bool
}

// advance first drains pending bytes, then writes large inputs directly and
// retains the tail. Native short writes loop without growing the Go call stack.
func (call *bufferedWriteCall) advance(caller *frame) (instructionOutcome, error) {
	for {
		if len(call.stream.write) == 0 {
			call.direct = true
			if !call.writing {
				return pushOutcome(caller, call.instruction, None)
			}
			if len(call.input)-call.accepted < call.stream.size {
				call.stream.write = append(call.stream.write, call.input[call.accepted:]...)
				return pushOutcome(caller, call.instruction, integerFromInt64(int64(len(call.input))))
			}
		}
		data := call.stream.write
		if call.direct {
			data = call.input[call.accepted:]
		}
		size := len(data)
		buffer := &byteBuffer{data: append([]byte(nil), data...)}
		view := newViewInstance(caller.runtime.memoryViewClass, memoryView{buffer: buffer, owner: &bytesValue{value: string(data)}, length: size, stride: 1, readonly: true})
		suspended := false
		blocked := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := executeRawCount(caller, call.instruction, call.stream.raw, "write", []Value{view})
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				if !isIOInterrupted(exception) {
					return raiseOutcome(exception), nil
				}
			} else if result == None {
				blocked = true
				return call.blocked(current)
			} else {
				count, ok := integerOperand(result)
				if !ok || !count.IsInt64() || count.Sign() < 0 || count.Int64() > int64(size) {
					return raiseOutcome(newException("OSError", "raw write() returned invalid length")), nil
				}
				if call.direct {
					call.accepted += int(count.Int64())
				} else {
					call.stream.write = call.stream.write[int(count.Int64()):]
				}
			}
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance || blocked {
			return outcome, err
		}
		caller.pop()
	}
}

// blocked preserves pending output and accepts only the space left in the
// buffer, reporting exactly how many bytes from this write the caller may skip.
func (call *bufferedWriteCall) blocked(caller *frame) (instructionOutcome, error) {
	if call.writing {
		count := min(call.stream.size-len(call.stream.write), len(call.input)-call.accepted)
		call.stream.write = append(call.stream.write, call.input[call.accepted:call.accepted+count]...)
		call.accepted += count
		if call.accepted == len(call.input) {
			return pushOutcome(caller, call.instruction, integerFromInt64(int64(call.accepted)))
		}
	}
	exception := newException("BlockingIOError", "write could not complete without blocking")
	arguments := []Value{integerFromInt64(11), &stringValue{value: "write could not complete without blocking"}, integerFromInt64(int64(call.accepted))}
	exception.args = &tupleValue{elements: arguments}
	exception.initializeOSError(arguments)
	return raiseOutcome(exception), nil
}
