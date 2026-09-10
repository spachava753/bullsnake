package runtime

func initializeBytesIOClass(module *Module) {
	class := newIOClass("BytesIO", module.globals.values["_BufferedIOBase"].(*typeValue))
	class.bytesIOClass = true
	module.globals.values["BytesIO"] = class
	for _, name := range []string{"__init__", "write", "writelines", "read", "read1", "readline", "readlines", "readinto", "seek", "tell", "truncate", "getvalue", "getbuffer", "close", "flush", "readable", "writable", "seekable", "__next__"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeBytesIOMethod(caller, instruction, self, name, arguments, keywords)
		}))
	}
	class.setAttribute("closed", &propertyValue{doc: None, getter: ioMethod(class, "closed", func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
		return pushOutcome(caller, instruction, booleanValue(self.io.binary.buffer == nil))
	})})
}

// executeBytesIOMethod dispatches native buffer operations independently of
// subclass overrides, while inherited base methods remain overridable.
func executeBytesIOMethod(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	stream := self.io.binary
	if name == "__init__" {
		if exception := initializeBytesIO(stream, arguments, keywords); exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, None)
	}
	minimum, maximum := streamMethodArity(name)
	switch name {
	case "readinto", "writelines":
		minimum, maximum = 1, 1
	case "read1", "readlines":
		maximum = 1
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	switch name {
	case "read", "read1", "readline", "seek", "truncate", "readlines":
		return executeBytesIOPosition(caller, instruction, self, name, arguments)
	case "readinto":
		lease, exception := acquireWritableBuffer(caller, arguments[0])
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		defer releaseBufferLease(caller, lease)
		if stream.buffer == nil {
			return raiseOutcome(bytesIOClosedError()), nil
		}
		size := min(len(lease.data), max(0, stream.size-stream.position))
		if size > 0 {
			copy(lease.data, stream.buffer.data[stream.position:stream.position+size])
			stream.position += size
		}
		return pushOutcome(caller, instruction, integerFromInt64(int64(size)))
	case "write":
		result, exception := stream.write(arguments[0])
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, result)
	case "close":
		if stream.buffer != nil && stream.buffer.hasViews() {
			return raiseOutcome(bufferExportError()), nil
		}
		stream.buffer = nil
		return pushOutcome(caller, instruction, None)
	}
	if stream.buffer == nil {
		return raiseOutcome(bytesIOClosedError()), nil
	}
	switch name {
	case "writelines":
		call := &ioLinesCall{self: self, instruction: instruction, name: "writelines", stage: "iter", input: arguments[0], sentinel: &dictValue{}, nativeBytesWrite: true}
		return call.advance(caller)
	case "getbuffer":
		view := memoryView{buffer: stream.buffer, owner: &bytesIOBuffer{stream: self}, length: stream.size, stride: 1}
		return pushOutcome(caller, instruction, newViewInstance(caller.runtime.memoryViewClass, view))
	case "getvalue":
		return pushOutcome(caller, instruction, &bytesValue{value: string(stream.buffer.data[:stream.size])})
	case "tell":
		return pushOutcome(caller, instruction, integerFromInt64(int64(stream.position)))
	case "readable", "writable", "seekable":
		return pushOutcome(caller, instruction, trueSingleton)
	case "__next__":
		line := stream.read(-1, true)
		if line.value == "" {
			return raiseOutcome(newException("StopIteration", "")), nil
		}
		return pushOutcome(caller, instruction, line)
	default:
		return pushOutcome(caller, instruction, None)
	}
}
