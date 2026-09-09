package runtime

// initializeSys installs copied arguments and absent stream defaults without
// reading process-global state.
func initializeSys(runtime *Runtime, module *Module) (*Exception, error) {
	args := make([]Value, len(runtime.args))
	for index, arg := range runtime.args {
		args[index] = &stringValue{value: arg}
	}
	module.globals.values["argv"] = &listValue{elements: args}
	for _, name := range []string{"stdin", "stdout", "stderr"} {
		module.globals.values[name] = None
		module.globals.values["__"+name+"__"] = None
	}
	module.globals.values["exit"] = &builtinFunctionValue{name: "exit", call: sysExit}
	return nil, nil
}

// sysExit raises an ordinary Python exception, never a process exit request.
func sysExit(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if exception := checkNativeArguments("exit", arguments, keywords, 0, 1); exception != nil {
		return nil, exception
	}
	exception := newExceptionOfType(systemExitType, "")
	exception.setArguments(arguments)
	return nil, exception
}
