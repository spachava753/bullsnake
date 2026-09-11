package runtime

import "bufio"

// initializeSys installs copied arguments and borrowed stream wrappers without
// reading process-global state.
func initializeSys(runtime *Runtime, module *Module) (*Exception, error) {
	args := make([]Value, len(runtime.args))
	for index, arg := range runtime.args {
		args[index] = &stringValue{value: arg}
	}
	module.globals.values["maxsize"] = integerFromInt64(int64(^uint(0) >> 1))
	module.globals.values["argv"] = &listValue{elements: args}
	stdin := Value(None)
	if runtime.stdin != nil {
		terminal, _ := runtime.stdin.(Terminal)
		stdin = &hostTextStream{reader: bufio.NewReader(runtime.stdin), terminal: terminal}
	}
	streams := map[string]Value{
		"stdin":  stdin,
		"stdout": newHostOutput(runtime.stdout),
		"stderr": newHostOutput(runtime.stderr),
	}
	for name, stream := range streams {
		module.globals.values[name] = stream
		module.globals.values["__"+name+"__"] = stream
	}
	module.globals.values["_getframe"] = &builtinFunctionValue{name: "_getframe", frameCall: executeGetFrame}
	module.globals.values["exit"] = &builtinFunctionValue{name: "exit", call: sysExit}
	return nil, nil
}

// sysExit raises an ordinary Python exception, never a process exit request.
func sysExit(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if exception := checkNativeArguments("exit", arguments, keywords, 0, 1); exception != nil {
		return nil, exception
	}
	if len(arguments) == 1 {
		if arguments[0] == None {
			arguments = nil
		} else if tuple, ok := arguments[0].(*tupleValue); ok {
			arguments = tuple.elements
		} else if exception, ok := arguments[0].(*Exception); ok && exception.class.isSubclassOf(systemExitType) {
			return nil, exception
		}
	}
	exception := newExceptionOfType(systemExitType, "")
	exception.setArguments(arguments)
	return nil, exception
}
