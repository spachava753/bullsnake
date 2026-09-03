package runtime

import (
	"encoding/binary"
	"runtime"
)

const (
	pythonReferenceVersion = "3.14.7"
	pythonReferenceCommit  = "823f0323ee6ec1402088b73bce1a38473cac36dc"
)

func (runtimeState *Runtime) newSysModule() *Module {
	module := newSystemModule("sys", "")
	module.globals.values["version"] = &stringValue{
		value: pythonReferenceVersion + " (Bullsnake; CPython reference " +
			pythonReferenceCommit + ")",
	}
	module.globals.values["version_info"] = &tupleValue{elements: []Value{
		newInt64(3),
		newInt64(14),
		newInt64(7),
		&stringValue{value: "final"},
		newInt64(0),
	}}
	module.globals.values["hexversion"] = newInt64(0x030e07f0)
	module.globals.values["platform"] = &stringValue{value: pythonPlatform()}
	module.globals.values["maxsize"] = newInt64(int64(^uint(0) >> 1))
	module.globals.values["byteorder"] = &stringValue{value: nativeByteOrder()}
	builtinNames := stringList(systemModuleNames)
	module.globals.values["builtin_module_names"] = &tupleValue{
		elements: builtinNames.elements,
	}
	module.globals.values["warnoptions"] = &listValue{}
	module.globals.values["path"] = stringList(runtimeState.path)
	module.globals.values["stdin"] = &streamValue{
		name:   "sys.stdin",
		reader: runtimeState.host.Stdin,
	}
	module.globals.values["stdout"] = &streamValue{
		name:   "sys.stdout",
		writer: runtimeState.host.Stdout,
	}
	module.globals.values["stderr"] = &streamValue{
		name:   "sys.stderr",
		writer: runtimeState.host.Stderr,
	}

	var arguments []string
	if runtimeState.host.Process != nil {
		arguments = runtimeState.host.Process.Args()
		if executable, err := runtimeState.host.Process.Executable(); err == nil {
			module.globals.values["executable"] = &stringValue{value: executable}
		}
	}
	if _, found := module.globals.values["executable"]; !found {
		module.globals.values["executable"] = &stringValue{}
	}
	module.globals.values["argv"] = stringList(arguments)

	setNativeFunction(module, "exc_info", 0, 0, sysExcInfo)
	setNativeFunction(module, "exit", 0, 1, sysExit)
	return module
}

func sysExcInfo(caller *frame, _ []Value) (Value, *Exception, error) {
	exception := activeHandledException(caller, caller.instruction-1)
	if exception == nil {
		return &tupleValue{elements: []Value{None, None, None}}, nil, nil
	}
	var exceptionType Value = exception.class
	if exception.userClass != nil {
		exceptionType = exception.userClass
	}
	return &tupleValue{elements: []Value{exceptionType, exception, None}}, nil, nil
}

func sysExit(_ *frame, arguments []Value) (Value, *Exception, error) {
	exception := newExceptionOfType(systemExitType, exceptionMessage(arguments))
	if len(arguments) == 0 {
		exception.code = None
	} else {
		exception.code = arguments[0]
	}
	return nil, exception, nil
}

func pythonPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}

func nativeByteOrder() string {
	var value uint16 = 1
	bytes := [2]byte{}
	binary.NativeEndian.PutUint16(bytes[:], value)
	if bytes[0] == 1 {
		return "little"
	}
	return "big"
}
