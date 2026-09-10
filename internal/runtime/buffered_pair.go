package runtime

type bufferedPair struct{ reader, writer Value }

func initializeBufferedPair(module *Module) {
	class := newIOClass("BufferedRWPair", module.globals.values["_BufferedIOBase"].(*typeValue))
	module.globals.values[class.name] = class
	readerClass, writerClass := module.globals.values["BufferedReader"], module.globals.values["BufferedWriter"]
	class.setAttribute("__init__", ioMethod(class, "__init__", func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		return initializePair(caller, instruction, self, readerClass, writerClass, arguments, keywords)
	}))
	for _, name := range []string{"read", "read1", "readinto", "readinto1", "peek", "write", "flush", "close", "readable", "writable", "isatty"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executePair(caller, instruction, self, name, arguments, keywords)
		}))
	}
	class.setAttribute("closed", &propertyValue{doc: None, getter: ioMethod(class, "closed", func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		return executePair(caller, instruction, self, "closed", arguments, keywords)
	})})
}

// initializePair retains the original native wrapper classes independently of
// mutable module exports and constructs both sides through ordinary class calls.
func initializePair(caller *frame, instruction int, self *instanceValue, readerClass, writerClass Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("BufferedRWPair", arguments, keywords, 2, 3); exception != nil {
		return raiseOutcome(exception), nil
	}
	var size Value = integerFromInt64(ioDefaultBufferSize)
	if len(arguments) == 3 {
		size = arguments[2]
	}
	reader, writer := arguments[0], arguments[1]
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, size)
	}, func(current *frame, size Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		pair := &pairInitialization{self: self, reader: reader, writer: writer, readerClass: readerClass, writerClass: writerClass, size: size, instruction: instruction}
		return pair.check(current, false)
	})
}

type pairInitialization struct {
	self                                           *instanceValue
	reader, writer, readerClass, writerClass, size Value
	instruction                                    int
}

// check validates both raw capabilities before allocating either child, then
// lets the concrete constructors perform their own checks as CPython does.
func (pair *pairInitialization) check(caller *frame, writing bool) (instructionOutcome, error) {
	raw, method := pair.reader, "readable"
	if writing {
		raw, method = pair.writer, "writable"
	}
	return continueNativeOperation(caller, pair.instruction, func() (instructionOutcome, error) {
		return executeMethodCall(caller, pair.instruction, raw, method, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result != trueSingleton {
			return raiseOutcome(unsupportedStreamOperation("File or stream is not " + method + ".")), nil
		}
		if !writing {
			return pair.check(current, true)
		}
		return pair.construct(current)
	})
}

func (pair *pairInitialization) construct(caller *frame) (instructionOutcome, error) {
	instruction := pair.instruction
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), pair.readerClass, []Value{pair.reader, pair.size}, nil)
	}, func(current *frame, reader Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, instruction, len(current.stack), pair.writerClass, []Value{pair.writer, pair.size}, nil)
		}, func(resumed *frame, writer Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			pair.self.io.pair = &bufferedPair{reader: reader, writer: writer}
			return pushOutcome(resumed, instruction, None)
		})
	})
}

// executePair forwards to the appropriate child. Close always visits both sides;
// isatty consults the reader only when the writer returned exactly False.
func executePair(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	minimum, maximum := streamMethodArity(name)
	if name == "read1" || name == "peek" {
		maximum = 1
	}
	if name == "readinto" || name == "readinto1" {
		minimum, maximum = 1, 1
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	pair := self.io.pair
	if pair == nil {
		return raiseOutcome(newException("ValueError", "I/O operation on uninitialized object")), nil
	}
	if name == "closed" {
		return executeDynamicAttributeLoad(caller, instruction, pair.writer, name)
	}
	if name == "close" || name == "isatty" {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(caller, instruction, pair.writer, name, nil)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if name == "isatty" && (exception != nil || result != falseSingleton) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return pushOutcome(current, instruction, result)
			}
			return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
				return executeMethodCall(current, instruction, pair.reader, name, nil)
			}, func(resumed *frame, result Value, readerError *Exception) (instructionOutcome, error) {
				if readerError != nil {
					return raiseOutcome(readerError), nil
				}
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return pushOutcome(resumed, instruction, result)
			})
		})
	}
	target := pair.reader
	if name == "write" || name == "flush" || name == "writable" {
		target = pair.writer
	}
	return executeMethodCall(caller, instruction, target, name, arguments)
}
