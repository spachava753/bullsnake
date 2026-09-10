package runtime

import "strings"

var fileOpenFunction = &builtinFunctionValue{name: "open", frameCall: executeFileOpen}

// initializeIOPermissions exposes real denial boundaries. No caller-supplied
// opener, process descriptor, source loader, or ambient filesystem grants access.
func initializeIOPermissions(module *Module) {
	module.globals.values["open"] = fileOpenFunction
	module.globals.values["open_code"] = &builtinFunctionValue{name: "open_code", call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
		values, exception := bindIOArguments("open_code", arguments, keywords, []string{"path"}, []Value{nil})
		if exception != nil {
			return nil, exception
		}
		if _, ok := values[0].(*stringValue); !ok {
			return nil, newException("TypeError", "open_code() argument 'path' must be str")
		}
		return nil, deniedFilesystem()
	}}
	module.globals.values["text_encoding"] = &builtinFunctionValue{name: "text_encoding", frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		if exception := checkNativeArguments("text_encoding", arguments, keywords, 1, 2); exception != nil {
			return raiseOutcome(exception), nil
		}
		var level Value = integerFromInt64(2)
		if len(arguments) == 2 {
			level = arguments[1]
		}
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeIOIndex(caller, instruction, level) }, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			level := result.(*intValue).value.Int64()
			if level < -1<<31 || level > 1<<31-1 {
				return raiseOutcome(newException("OverflowError", "Python int too large to convert to C int")), nil
			}
			encoding := arguments[0]
			if encoding == None {
				encoding = &stringValue{value: "utf-8"}
			}
			return pushOutcome(current, instruction, encoding)
		})
	}}
	class := newIOClass("FileIO", module.globals.values["_RawIOBase"].(*typeValue))
	module.globals.values[class.name] = class
	class.setAttribute("__init__", ioMethod(class, "__init__", func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		values, exception := bindIOArguments("FileIO", arguments, keywords, []string{"file", "mode", "closefd", "opener"}, []Value{nil, &stringValue{value: "r"}, trueSingleton, None})
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return validateFileRequest(caller, instruction, values[0], values[1], values[2], true)
	}))
	for _, name := range []string{"read", "readall", "readinto", "write", "seek", "tell", "truncate", "flush", "close", "fileno", "isatty", "readable", "writable", "seekable"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			minimum, maximum := streamMethodArity(name)
			if name == "readinto" {
				minimum, maximum = 1, 1
			}
			if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
				return raiseOutcome(exception), nil
			}
			if name == "close" {
				return pushOutcome(caller, instruction, None)
			}
			return raiseOutcome(newException("ValueError", "I/O operation on closed file")), nil
		}))
	}
	for _, name := range []string{"closed", "closefd"} {
		class.setAttribute(name, &propertyValue{doc: None, getter: ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
			return pushOutcome(caller, instruction, trueSingleton)
		})})
	}
}

func deniedFilesystem() *Exception {
	return newException("PermissionError", "filesystem access is not configured")
}

// executeFileOpen checks text/binary combinations and converts buffering before
// validating the path and denying access. It never invokes the optional opener.
func executeFileOpen(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	values, exception := bindIOArguments("open", arguments, keywords, []string{"file", "mode", "buffering", "encoding", "errors", "newline", "closefd", "opener"}, []Value{nil, &stringValue{value: "r"}, integerFromInt64(-1), None, None, None, trueSingleton, None})
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	mode, exception := validateFileMode(values[1], false)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	for index, name := range []string{"encoding", "errors", "newline"} {
		value := values[index+3]
		if value == None {
			continue
		}
		if _, ok := value.(*stringValue); !ok {
			return raiseOutcome(newException("TypeError", name+" must be str or None")), nil
		}
		if strings.Contains(mode, "b") {
			return raiseOutcome(newException("ValueError", "binary mode doesn't take an "+name+" argument")), nil
		}
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeIOIndex(caller, instruction, values[2]) }, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		buffering := result.(*intValue).value.Int64()
		if buffering < -1<<31 || buffering > 1<<31-1 {
			return raiseOutcome(newException("OverflowError", "Python int too large to convert to C int")), nil
		}
		return validateFileRequest(current, instruction, values[0], values[1], values[6], false)
	})
}

// validateFileMode enforces one access mode and compatible text/binary flags.
// Raw FileIO follows CPython in allowing repeated b but never a text flag.
func validateFileMode(value Value, raw bool) (string, *Exception) {
	mode, ok := value.(*stringValue)
	if !ok {
		return "", newException("TypeError", "mode must be str")
	}
	access := 0
	seen := map[rune]bool{}
	for _, character := range mode.value {
		if !strings.ContainsRune("rwax+bt", character) || raw && character == 't' || seen[character] && !(raw && character == 'b') {
			return "", newException("ValueError", "invalid mode: "+mode.Repr())
		}
		seen[character] = true
		if strings.ContainsRune("rwax", character) {
			access++
		}
	}
	if access != 1 || seen['b'] && seen['t'] {
		return "", newException("ValueError", "must have exactly one of create/read/write/append mode")
	}
	return mode.value, nil
}

// validateFileRequest invokes only Python conversion protocols before denial.
// Numeric descriptors are denied too; closefd=False never grants ambient access.
func validateFileRequest(caller *frame, instruction int, file, mode, closefd Value, raw bool) (instructionOutcome, error) {
	if _, exception := validateFileMode(mode, raw); exception != nil {
		return raiseOutcome(exception), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeBuiltinBool(caller, instruction, len(caller.stack), []Value{closefd}, nil)
	}, func(current *frame, closing Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) { return convertFileTarget(current, instruction, file) }, func(resumed *frame, target Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if number, ok := integerOperand(target); ok {
				if number.Sign() < 0 {
					return raiseOutcome(newException("ValueError", "negative file descriptor")), nil
				}
				if !number.IsInt64() || number.Int64() > 1<<31-1 {
					return raiseOutcome(newException("OverflowError", "Python int too large to convert to C int")), nil
				}
			} else {
				path := ""
				switch value := target.(type) {
				case *stringValue:
					path = value.value
				case *bytesValue:
					path = value.value
				}
				if strings.ContainsRune(path, 0) {
					return raiseOutcome(newException("ValueError", "embedded null character")), nil
				}
				if closing == falseSingleton {
					return raiseOutcome(newException("ValueError", "Cannot use closefd=False with file name")), nil
				}
			}
			return raiseOutcome(deniedFilesystem()), nil
		})
	})
}

// convertFileTarget accepts path strings, descriptors, and Python __fspath__ or
// __index__ protocols without consulting the filesystem or source-module loader.
func convertFileTarget(caller *frame, instruction int, value Value) (instructionOutcome, error) {
	switch value.(type) {
	case *stringValue, *bytesValue, *intValue, *boolValue:
		return pushOutcome(caller, instruction, value)
	}
	instance, ok := value.(*instanceValue)
	if !ok {
		return raiseOutcome(newException("TypeError", "expected str, bytes or os.PathLike object, not "+value.TypeName())), nil
	}
	if _, found := lookupInstanceSpecial(instance, "__index__"); found {
		return executeIOIndex(caller, instruction, value)
	}
	method, found := lookupInstanceSpecial(instance, "__fspath__")
	if !found {
		return raiseOutcome(newException("TypeError", "expected str, bytes or os.PathLike object, not "+value.TypeName())), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeFunctionCall(caller, instruction, len(caller.stack), method, nil, nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		switch result.(type) {
		case *stringValue, *bytesValue:
			return pushOutcome(current, instruction, result)
		}
		return raiseOutcome(newException("TypeError", "__fspath__() must return str or bytes")), nil
	})
}
