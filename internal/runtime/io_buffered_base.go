package runtime

import "strconv"

func initializeBufferedIOClass(module *Module) {
	class := newIOClass("_BufferedIOBase", module.globals.values["_IOBase"].(*typeValue))
	module.globals.values["_BufferedIOBase"] = class
	for _, name := range []string{"read", "read1", "write", "detach"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			minimum, maximum := streamMethodArity(name)
			if name == "read1" {
				maximum = 1
			}
			if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
				return raiseOutcome(exception), nil
			}
			return raiseOutcome(unsupportedStreamOperation(name)), nil
		}))
	}
	for _, name := range []string{"readinto", "readinto1"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeBufferedBaseReadInto(caller, instruction, self, name, arguments, keywords)
		}))
	}
}

// executeBufferedBaseReadInto keeps the target pinned until a Python read/read1
// finishes and validates its bytes result before copying any returned data.
func executeBufferedBaseReadInto(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	lease, exception := acquireWritableBuffer(caller, arguments[0])
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	method := "read"
	if name == "readinto1" {
		method = "read1"
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeMethodCall(caller, instruction, self, method, []Value{integerFromInt64(int64(len(lease.data)))})
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		defer releaseBufferLease(current, lease)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		data, ok := result.(*bytesValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "read() should return bytes")), nil
		}
		if len(data.value) > len(lease.data) {
			return raiseOutcome(newException("ValueError", "read() returned too much data: "+strconv.Itoa(len(lease.data))+" bytes requested, "+strconv.Itoa(len(data.value))+" returned")), nil
		}
		copy(lease.data, data.value)
		return pushOutcome(current, instruction, integerFromInt64(int64(len(data.value))))
	})
}
