package runtime

import "bytes"

type bufferedStream struct {
	write []byte
	raw   Value
	size  int
	read  []byte
	busy  bool
}

// initializeBufferedType installs common lifecycle methods and raw metadata
// descriptors with the concrete class's read/write method dispatcher.
func initializeBufferedType(module *Module, className string, methods []string, dispatch func(*frame, int, *instanceValue, string, []Value, *dictValue) (instructionOutcome, error)) {
	class := newIOClass(className, module.globals.values["_BufferedIOBase"].(*typeValue))
	module.globals.values[class.name] = class
	for _, name := range append(methods, "__init__", "seek", "tell", "flush", "close", "detach", "readable", "writable", "seekable", "fileno", "isatty") {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return dispatch(caller, instruction, self, name, arguments, keywords)
		}))
	}
	for _, name := range []string{"raw", "closed", "name", "mode"} {
		class.setAttribute(name, &propertyValue{doc: None, getter: ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
			stream := self.io.buffered
			if name == "raw" {
				if stream == nil || stream.raw == nil {
					return pushOutcome(caller, instruction, None)
				}
				return pushOutcome(caller, instruction, stream.raw)
			}
			if exception := checkBufferedAttached(stream); exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeDynamicAttributeLoad(caller, instruction, stream.raw, name)
		})})
	}
}

func checkBufferedAttached(stream *bufferedStream) *Exception {
	if stream == nil {
		return newException("ValueError", "I/O operation on uninitialized object")
	}
	if stream.raw == nil {
		return newException("ValueError", "raw stream has been detached")
	}
	return nil
}

// initializeBuffered validates index conversion and the raw capability contract
// before installing an empty read-ahead buffer. It does not acquire host access.
func initializeBuffered(caller *frame, instruction int, self *instanceValue, className, capability string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	values, exception := bindIOArguments(className, arguments, keywords, []string{"raw", "buffer_size"}, []Value{nil, integerFromInt64(ioDefaultBufferSize)})
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, values[1])
	}, func(current *frame, count Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		size := int(count.(*intValue).value.Int64())
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(current, instruction, values[0], capability, nil)
		}, func(resumed *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if result != trueSingleton {
				return raiseOutcome(unsupportedStreamOperation("File or stream is not " + capability + ".")), nil
			}
			if size <= 0 {
				return raiseOutcome(newException("ValueError", "buffer size must be strictly positive")), nil
			}
			if self.io.buffered != nil && self.io.buffered.busy {
				return raiseOutcome(newException("RuntimeError", "reentrant call inside buffered stream")), nil
			}
			self.io.buffered = &bufferedStream{raw: values[0], size: size}
			return pushOutcome(resumed, instruction, None)
		})
	})
}

// executeBufferedReader binds sizes before closed checks and holds the buffer
// guard across raw callbacks. Lifecycle operations retain normal override lookup.
func executeBufferedReader(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if name == "__init__" {
		return initializeBuffered(caller, instruction, self, "BufferedReader", "readable", arguments, keywords)
	}
	if name == "readinto" || name == "readinto1" {
		return executeConcreteReadInto(caller, instruction, self, name, arguments, keywords)
	}
	minimum, maximum := streamMethodArity(name)
	if name == "read1" || name == "peek" {
		maximum = 1
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	stream := self.io.buffered
	if exception := checkBufferedAttached(stream); exception != nil {
		return raiseOutcome(exception), nil
	}
	if name == "close" {
		return closeBuffered(caller, instruction, self, stream)
	}
	if name == "detach" {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeMethodCall(caller, instruction, self, "flush", nil) }, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			raw := stream.raw
			stream.raw, stream.read = nil, nil
			return pushOutcome(current, instruction, raw)
		})
	}
	if name == "seek" {
		return seekBuffered(caller, instruction, self, stream, arguments)
	}
	if name == "read" || name == "read1" || name == "readline" || name == "peek" {
		var size Value = integerFromInt64(-1)
		if len(arguments) > 0 && arguments[0] != None {
			size = arguments[0]
		}
		if len(arguments) > 0 && arguments[0] == None && (name == "read1" || name == "peek") {
			size = None
		}
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeIOIndex(caller, instruction, size) }, func(current *frame, count Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			limit := int(count.(*intValue).value.Int64())
			return withIOOpen(current, instruction, self, func(resumed *frame) (instructionOutcome, error) {
				return withIOGuard(resumed, instruction, &stream.busy, func() (instructionOutcome, error) {
					if name == "read" && limit < -1 {
						return raiseOutcome(newException("ValueError", "read length must be non-negative or -1")), nil
					}
					if name == "read1" && limit < 0 {
						limit = stream.size
					}
					call := &bufferedReadCall{stream: stream, instruction: instruction, name: name, limit: limit}
					return call.advance(resumed)
				})
			})
		})
	}
	return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
		switch name {
		case "flush":
			return pushOutcome(current, instruction, None)
		case "writable":
			return pushOutcome(current, instruction, falseSingleton)
		case "tell":
			return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
				return executeMethodCall(current, instruction, stream.raw, name, nil)
			}, func(resumed *frame, position Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				value, ok := integerOperand(position)
				if !ok {
					return raiseOutcome(newException("TypeError", "an integer is required")), nil
				}
				if value.Sign() < 0 {
					return raiseOutcome(newException("OSError", "Raw stream returned invalid position")), nil
				}
				value.Sub(&value, &integerFromInt64(int64(len(stream.read))).value)
				return pushOutcome(resumed, instruction, &intValue{value: value})
			})
		default:
			return executeMethodCall(current, instruction, stream.raw, name, nil)
		}
	})
}

// closeBuffered always attempts the raw close after flushing, even on a Python
// flush failure. Borrowed host adapters themselves retain provider ownership.
func closeBuffered(caller *frame, instruction int, self *instanceValue, stream *bufferedStream) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, stream.raw, "closed")
	}, func(current *frame, closed Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if closed == trueSingleton {
			return pushOutcome(current, instruction, None)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) { return executeMethodCall(current, instruction, self, "flush", nil) }, func(resumed *frame, _ Value, flushError *Exception) (instructionOutcome, error) {
			return continueNativeOperation(resumed, instruction, func() (instructionOutcome, error) {
				return executeMethodCall(resumed, instruction, stream.raw, "close", nil)
			}, func(finished *frame, result Value, closeError *Exception) (instructionOutcome, error) {
				if closeError != nil {
					return raiseOutcome(closeError), nil
				}
				if flushError != nil {
					return raiseOutcome(flushError), nil
				}
				return pushOutcome(finished, instruction, result)
			})
		})
	})
}

// seekBuffered adjusts relative offsets for read-ahead and discards buffered
// bytes only after a successful raw seek, preserving them on failure.
func seekBuffered(caller *frame, instruction int, self *instanceValue, stream *bufferedStream, arguments []Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeIOIndex(caller, instruction, arguments[0]) }, func(current *frame, offset Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		var origin Value = integerFromInt64(0)
		if len(arguments) > 1 {
			origin = arguments[1]
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) { return executeIOIndex(current, instruction, origin) }, func(resumed *frame, whence Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			mode := whence.(*intValue).value.Int64()
			if mode < 0 || mode > 2 {
				return raiseOutcome(newException("ValueError", "unsupported whence")), nil
			}
			return withIOOpen(resumed, instruction, self, func(opened *frame) (instructionOutcome, error) {
				return withIOGuard(opened, instruction, &stream.busy, func() (instructionOutcome, error) {
					value, _ := integerOperand(offset)
					if mode == 1 {
						value.Sub(&value, &integerFromInt64(int64(len(stream.read))).value)
					}
					return continueNativeOperation(opened, instruction, func() (instructionOutcome, error) {
						return executeMethodCall(opened, instruction, stream.raw, "seek", []Value{&intValue{value: value}, whence})
					}, func(finished *frame, result Value, exception *Exception) (instructionOutcome, error) {
						if exception != nil {
							return raiseOutcome(exception), nil
						}
						position, ok := integerOperand(result)
						if !ok || position.Sign() < 0 {
							return raiseOutcome(newException("OSError", "Raw stream returned invalid position")), nil
						}
						stream.read = nil
						return pushOutcome(finished, instruction, &intValue{value: position})
					})
				})
			})
		})
	})
}

type bufferedReadCall struct {
	noneOnBlock   bool
	rawData       []byte
	stream        *bufferedStream
	instruction   int
	name          string
	limit         int
	data          []byte
	target        *instanceValue
	stage         string
	readall       Value
	done, blocked bool
}

// advance drains read-ahead before calling raw operations. The trampoline loops
// native short reads without growing the Go stack and resumes Python callbacks.
func (call *bufferedReadCall) advance(caller *frame) (instructionOutcome, error) {
	for !call.done {
		call.consume()
		if call.done {
			break
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := call.operation(caller)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
			if exception := call.accept(value, exception); exception != nil {
				return raiseOutcome(exception), nil
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
	if call.blocked && len(call.data) == 0 && (call.name == "read" || call.noneOnBlock) {
		return pushOutcome(caller, call.instruction, None)
	}
	return pushOutcome(caller, call.instruction, &bytesValue{value: string(call.data)})
}

// consume returns buffered bytes without callbacks, with a line or size bound.
// Peek copies without moving position; read1 stops after any buffered progress.
func (call *bufferedReadCall) consume() {
	if call.name == "peek" {
		if len(call.stream.read) > 0 {
			call.data, call.done = append(call.data, call.stream.read...), true
		}
		return
	}
	size := len(call.stream.read)
	if call.limit >= 0 {
		size = min(size, call.limit-len(call.data))
	}
	if call.name == "readline" {
		if end := bytes.IndexByte(call.stream.read[:size], '\n'); end >= 0 {
			size, call.done = end+1, true
		}
	}
	call.data = append(call.data, call.stream.read[:size]...)
	call.stream.read = call.stream.read[size:]
	if call.limit >= 0 && len(call.data) >= call.limit || call.name == "read1" && size > 0 {
		call.done = true
	}
}

// operation selects readall or its read fallback for unbounded requests;
// bounded reads use a temporary writable view with separately retained storage.
func (call *bufferedReadCall) operation(caller *frame) (instructionOutcome, error) {
	if call.name == "read" && call.limit < 0 {
		switch call.stage {
		case "":
			call.stage = "lookup"
			return executeDynamicAttributeLoad(caller, call.instruction, call.stream.raw, "readall")
		case "readall":
			return executeFunctionCall(caller, call.instruction, len(caller.stack), call.readall, nil, nil)
		default:
			return executeMethodCall(caller, call.instruction, call.stream.raw, "read", nil)
		}
	}
	size := call.stream.size
	if call.name == "read1" {
		size = call.limit
	}
	buffer := &bytearrayValue{buffer: &byteBuffer{data: make([]byte, size)}}
	call.rawData = buffer.buffer.data
	call.target = newViewInstance(caller.runtime.memoryViewClass, memoryView{buffer: buffer.buffer, owner: buffer, length: size, stride: 1})
	return executeMethodCall(caller, call.instruction, call.stream.raw, "readinto", []Value{call.target})
}

// accept validates raw callback results before copying, retries EINTR, and
// distinguishes nonblocking no-progress from partial reads and read1's empty bytes.
func (call *bufferedReadCall) accept(value Value, exception *Exception) *Exception {
	if exception != nil {
		if call.stage == "lookup" && isAttributeError(exception) {
			call.stage = "read"
			return nil
		}
		if isIOInterrupted(exception) {
			return nil
		}
		return exception
	}
	if call.stage == "lookup" {
		call.readall, call.stage = value, "readall"
		return nil
	}
	if value == None {
		call.done, call.blocked = true, true
		return nil
	}
	if call.limit < 0 && call.name == "read" {
		data, ok := value.(*bytesValue)
		if !ok {
			return newException("TypeError", call.stage+"() should return bytes")
		}
		call.data = append(call.data, data.value...)
		call.done = call.stage == "readall" || data.value == ""
		return nil
	}
	count, ok := integerOperand(value)
	if !ok || !count.IsInt64() || count.Sign() < 0 || count.Int64() > int64(len(call.rawData)) {
		return newException("OSError", "raw readinto() returned invalid length")
	}
	size := int(count.Int64())
	call.stream.read = append(call.stream.read, call.rawData[:size]...)
	call.target = nil
	if size == 0 {
		call.done = true
	}
	return nil
}

// executeConcreteReadInto pins the destination across raw reads and preserves
// None on nonblocking no-progress, unlike BufferedIOBase's read override helper.
func executeConcreteReadInto(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	lease, exception := acquireWritableBuffer(caller, arguments[0])
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		stream := self.io.buffered
		if exception := checkBufferedAttached(stream); exception != nil {
			return raiseOutcome(exception), nil
		}
		return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
			return withIOGuard(current, instruction, &stream.busy, func() (instructionOutcome, error) {
				method := "read"
				if name == "readinto1" {
					method = "read1"
				}
				call := &bufferedReadCall{stream: stream, instruction: instruction, name: method, limit: len(lease.data), noneOnBlock: true}
				return call.advance(current)
			})
		})
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		defer releaseBufferLease(current, lease)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if value == None {
			return pushOutcome(current, instruction, None)
		}
		data := value.(*bytesValue).value
		copy(lease.data, data)
		return pushOutcome(current, instruction, integerFromInt64(int64(len(data))))
	})
}

// isIOInterrupted recognizes errno EINTR only on structured OSError instances,
// so unrelated callback errors are never retried.
func isIOInterrupted(exception *Exception) bool {
	if exception == nil || exception.fields == nil || !exception.class.isSubclassOf(osErrorType) {
		return false
	}
	number, found := exception.fields.get("errno")
	if !found {
		return false
	}
	value, ok := integerOperand(number)
	return ok && value.IsInt64() && value.Int64() == 4
}
