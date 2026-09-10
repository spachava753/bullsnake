package runtime

import "strconv"

const ioDefaultBufferSize = 128 * 1024

func initializeRawIOClass(module *Module) {
	class := newIOClass("_RawIOBase", module.globals.values["_IOBase"].(*typeValue))
	module.globals.values["_RawIOBase"] = class
	module.globals.values["DEFAULT_BUFFER_SIZE"] = integerFromInt64(ioDefaultBufferSize)
	class.setAttribute("read", ioMethod(class, "read", executeRawRead))
	class.setAttribute("readall", ioMethod(class, "readall", func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		if exception := checkNativeArguments("readall", arguments, keywords, 0, 0); exception != nil {
			return raiseOutcome(exception), nil
		}
		call := &ioLinesCall{self: self, instruction: instruction, name: "readall", stage: "read", readSize: ioDefaultBufferSize}
		return call.advance(caller)
	}))
	for _, name := range []string{"readinto", "write"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			if keywords != nil && len(keywords.entries) != 0 {
				return raiseOutcome(newException("TypeError", name+"() takes no keyword arguments")), nil
			}
			return raiseOutcome(newException("NotImplementedError", "")), nil
		}))
	}
}

func executeRawRead(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("read", arguments, keywords, 0, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	if len(arguments) == 0 {
		return executeMethodCall(caller, instruction, self, "readall", nil)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, arguments[0])
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		size := int(result.(*intValue).value.Int64())
		if size < 0 {
			return executeMethodCall(current, instruction, self, "readall", nil)
		}
		return executeRawReadInto(current, instruction, self, size)
	})
}

// executeRawReadInto gives the Python implementation a new writable buffer,
// validates its count, then copies exactly the filled bytes into a snapshot.
func executeRawReadInto(caller *frame, instruction int, self *instanceValue, size int) (instructionOutcome, error) {
	buffer := &bytearrayValue{buffer: &byteBuffer{data: make([]byte, size)}}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeMethodCall(caller, instruction, self, "readinto", []Value{buffer})
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == None {
			return pushOutcome(current, instruction, None)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeIOIndex(current, instruction, result)
		}, func(resumed *frame, count Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				if exception.class == overflowErrorType {
					exception = newException("ValueError", exception.Message())
				}
				return raiseOutcome(exception), nil
			}
			filled := int(count.(*intValue).value.Int64())
			if filled < 0 || filled > size || filled > len(buffer.buffer.data) {
				return raiseOutcome(newException("ValueError", "readinto returned "+strconv.Itoa(filled)+" outside buffer size "+strconv.Itoa(size))), nil
			}
			return pushOutcome(resumed, instruction, &bytesValue{value: string(buffer.buffer.data[:filled])})
		})
	})
}
