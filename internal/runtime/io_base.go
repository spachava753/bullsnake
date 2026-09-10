package runtime

func initializeIOBases(module *Module) {
	base := newIOClass("_IOBase", nil)
	module.globals.values["_IOBase"] = base
	for _, name := range []string{"readline", "readlines", "writelines", "__init__", "close", "flush", "__enter__", "__exit__", "__iter__", "__next__", "fileno", "seek", "tell", "truncate", "readable", "writable", "seekable", "isatty", "_checkClosed", "_checkReadable", "_checkWritable", "_checkSeekable"} {
		base.setAttribute(name, ioMethod(base, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeIOBaseMethod(caller, instruction, self, name, arguments, keywords)
		}))
	}
	base.setAttribute("closed", &propertyValue{doc: None, getter: ioMethod(base, "closed", func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
		return pushOutcome(caller, instruction, booleanValue(self.io.closed))
	})})
	text := newIOClass("_TextIOBase", base)
	module.globals.values["_TextIOBase"] = text
	for _, name := range []string{"read", "readline", "write", "detach"} {
		text.setAttribute(name, ioMethod(text, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			minimum, maximum := streamMethodArity(name)
			if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
				return raiseOutcome(exception), nil
			}
			return raiseOutcome(unsupportedStreamOperation(name)), nil
		}))
	}
	for _, name := range []string{"encoding", "errors", "newlines"} {
		text.setAttribute(name, &propertyValue{doc: None, getter: ioMethod(text, name, func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
			return pushOutcome(caller, instruction, None)
		})})
	}
}

// executeIOBaseMethod implements base defaults and dispatches overridable
// operations through Python. A failed flush still closes private base state.
func executeIOBaseMethod(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if name == "readline" || name == "readlines" || name == "writelines" {
		return executeIOLines(caller, instruction, self, name, arguments, keywords)
	}
	if name == "__init__" {
		return pushOutcome(caller, instruction, None)
	}
	minimum, maximum := streamMethodArity(name)
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	switch name {
	case "seek", "truncate", "fileno":
		return raiseOutcome(unsupportedStreamOperation(name)), nil
	case "readable", "writable", "seekable":
		return pushOutcome(caller, instruction, falseSingleton)
	case "tell":
		return executeMethodCall(caller, instruction, self, "seek", []Value{integerFromInt64(0), integerFromInt64(1)})
	case "__exit__":
		return executeMethodCall(caller, instruction, self, "close", nil)
	case "close":
		if self.io.closed {
			return pushOutcome(caller, instruction, None)
		}
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(caller, instruction, self, "flush", nil)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			self.io.closed = true
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(current, instruction, None)
		})
	case "flush":
		if self.io.closed {
			return raiseOutcome(newException("ValueError", "I/O operation on closed file.")), nil
		}
		return pushOutcome(caller, instruction, None)
	case "_checkReadable", "_checkWritable", "_checkSeekable":
		method := map[string]string{"_checkReadable": "readable", "_checkWritable": "writable", "_checkSeekable": "seekable"}[name]
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(caller, instruction, self, method, nil)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if result != trueSingleton {
				return raiseOutcome(unsupportedStreamOperation("File or stream is not " + method + ".")), nil
			}
			return pushOutcome(current, instruction, result)
		})
	case "__next__":
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(caller, instruction, self, "readline", nil)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
				return executeBuiltinLen(current, instruction, len(current.stack), []Value{result}, nil)
			}, func(resumed *frame, length Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				if length.(*intValue).value.Sign() == 0 {
					return raiseOutcome(newException("StopIteration", "")), nil
				}
				return pushOutcome(resumed, instruction, result)
			})
		})
	default:
		return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
			if name == "__enter__" || name == "__iter__" {
				return pushOutcome(current, instruction, self)
			}
			if name == "isatty" {
				return pushOutcome(current, instruction, falseSingleton)
			}
			return pushOutcome(current, instruction, None)
		})
	}
}

// withIOOpen reads the Python closed property and converts its truth value
// before continuing, preserving subclass property and __bool__ callbacks.
func withIOOpen(caller *frame, instruction int, self *instanceValue, resume func(*frame) (instructionOutcome, error)) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, self, "closed")
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeBuiltinBool(current, instruction, len(current.stack), []Value{result}, nil)
		}, func(resumed *frame, closed Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if closed == trueSingleton {
				return raiseOutcome(newException("ValueError", "I/O operation on closed file.")), nil
			}
			return resume(resumed)
		})
	})
}
