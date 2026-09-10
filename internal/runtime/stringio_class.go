package runtime

// initializeStringIOClass installs native buffer methods and properties on the
// ordinary text I/O hierarchy, with subclass-aware line iteration.
func initializeStringIOClass(module *Module) {
	class := newIOClass("StringIO", module.globals.values["_TextIOBase"].(*typeValue))
	module.globals.values["StringIO"] = class
	for _, name := range []string{"__init__", "write", "getvalue", "flush", "close", "isatty", "tell", "readable", "writable", "seekable", "read", "readline", "seek", "truncate"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeStringIOMethod(caller, instruction, self, name, arguments, keywords)
		}))
	}
	for _, name := range []string{"closed", "newlines", "line_buffering"} {
		class.setAttribute(name, &propertyValue{doc: None, getter: ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
			stream := self.io.text
			if stream == nil {
				return raiseOutcome(newException("ValueError", "I/O operation on uninitialized object")), nil
			}
			if name == "closed" {
				return pushOutcome(caller, instruction, booleanValue(stream.closed))
			}
			if stream.closed {
				return raiseOutcome(newException("ValueError", "I/O operation on closed file")), nil
			}
			if name == "line_buffering" {
				return pushOutcome(caller, instruction, falseSingleton)
			}
			return pushOutcome(caller, instruction, stream.newlines())
		})})
	}
	class.setAttribute("__next__", ioMethod(class, "__next__", func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		if exception := checkNativeArguments("__next__", arguments, keywords, 0, 0); exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			if self.class == class {
				return executeStringIOMethod(caller, instruction, self, "readline", nil, nil)
			}
			return executeMethodCall(caller, instruction, self, "readline", nil)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			text, ok := result.(*stringValue)
			if !ok {
				return raiseOutcome(newException("OSError", "readline() should have returned a str object, not '"+result.TypeName()+"'")), nil
			}
			if text.value == "" {
				return raiseOutcome(newException("StopIteration", "")), nil
			}
			return pushOutcome(current, instruction, result)
		})
	}))
}

// executeStringIOMethod retains native buffer storage across Python overrides.
// Reinitialization replaces content in place so pending index callbacks see it.
func executeStringIOMethod(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if name == "__init__" {
		stream, exception := newStringIO(arguments, keywords)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if self.io.text == nil {
			self.io.text = stream
		} else {
			*self.io.text = *stream
		}
		return pushOutcome(caller, instruction, None)
	}
	stream := self.io.text
	if stream == nil {
		return raiseOutcome(newException("ValueError", "I/O operation on uninitialized object")), nil
	}
	switch name {
	case "read", "readline", "seek", "truncate":
		return stream.executePositionCall(caller, instruction, len(caller.stack), name, arguments, keywords)
	default:
		result, exception := stream.call(name, arguments, keywords)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, result)
	}
}
