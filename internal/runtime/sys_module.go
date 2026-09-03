package runtime

import (
	"encoding/binary"
	"runtime"
)

const (
	pythonReferenceVersion = "3.14.7"
	pythonReferenceCommit  = "823f0323ee6ec1402088b73bce1a38473cac36dc"
)

// newSysModule exposes interpreter metadata and capability-backed standard streams.
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
	module.globals.values["prefix"] = &stringValue{}
	module.globals.values["base_prefix"] = &stringValue{}
	module.globals.values["exec_prefix"] = &stringValue{}
	module.globals.values["base_exec_prefix"] = &stringValue{}
	module.globals.values["byteorder"] = &stringValue{value: nativeByteOrder()}
	builtinNames := stringList(systemModuleNames)
	module.globals.values["builtin_module_names"] = &tupleValue{
		elements: builtinNames.elements,
	}
	module.globals.values["warnoptions"] = &listValue{}
	module.globals.values["flags"] = &sysFlagsValue{}
	module.globals.values["implementation"] = &sysImplementationValue{}
	module.globals.values["hash_info"] = &sysHashInfoValue{}
	module.globals.values["_jit"] = &sysJITValue{}
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
	setNativeFunction(module, "exception", 0, 0, sysException)
	setNativeFunction(module, "getrefcount", 1, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return newInt64(2), nil, nil
		})
	setNativeFunction(module, "exit", 0, 1, sysExit)
	module.globals.values["excepthook"] = nativeFunctionNamed(
		"excepthook", 3, 3,
		func(_ *frame, _ []Value) (Value, *Exception, error) { return None, nil, nil },
	)
	for _, name := range []string{"setprofile", "settrace", "_setprofileallthreads", "_settraceallthreads"} {
		setNativeFunction(module, name, 1, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			})
	}
	for _, name := range []string{"getprofile", "gettrace"} {
		setNativeFunction(module, name, 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			})
	}
	setNativeFunction(module, "_getframe", 0, 1, sysGetFrame)
	setNativeFunction(module, "intern", 1, 1, sysIntern)
	setNativeFunction(module, "getrecursionlimit", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return newInt64(1000), nil, nil
		})
	setNativeFunction(module, "setrecursionlimit", 1, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		})
	setNativeFunction(module, "getswitchinterval", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &floatValue{value: 0.005}, nil, nil
		})
	setNativeFunction(module, "setswitchinterval", 1, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		})
	for _, name := range []string{"getfilesystemencoding", "getdefaultencoding"} {
		setNativeFunction(module, name, 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &stringValue{value: "utf-8"}, nil, nil
			})
	}
	setNativeFunction(module, "getfilesystemencodeerrors", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &stringValue{value: "surrogateescape"}, nil, nil
		})
	setNativeFunction(module, "_getframemodulename", 0, 1,
		func(caller *frame, arguments []Value) (Value, *Exception, error) {
			selected, exception, err := sysGetFrame(caller, arguments)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			frame := selected.(*frameValue).frame
			if name, found := frame.globals.get("__name__"); found {
				return name, nil, nil
			}
			return None, nil, nil
		})
	return module
}

type sysFlagsValue struct{}

func (*sysFlagsValue) TypeName() string { return "flags" }
func (*sysFlagsValue) Repr() string     { return "sys.flags()" }
func (*sysFlagsValue) isValue()         {}
func (*sysFlagsValue) attribute(_ string) (Value, bool) {
	return newInt64(0), true
}

type sysImplementationValue struct{}

func (*sysImplementationValue) TypeName() string { return "SimpleNamespace" }
func (*sysImplementationValue) Repr() string     { return "namespace(name='bullsnake')" }
func (*sysImplementationValue) isValue()         {}
func (*sysImplementationValue) attribute(name string) (Value, bool) {
	switch name {
	case "name":
		return &stringValue{value: "bullsnake"}, true
	case "cache_tag":
		return None, true
	case "hexversion":
		return newInt64(0x030e07f0), true
	default:
		return nil, false
	}
}

type sysHashInfoValue struct{}

func (*sysHashInfoValue) TypeName() string { return "hash_info" }
func (*sysHashInfoValue) Repr() string     { return "sys.hash_info(width=64)" }
func (*sysHashInfoValue) isValue()         {}
func (*sysHashInfoValue) attribute(name string) (Value, bool) {
	if name == "width" {
		return newInt64(64), true
	}
	return newInt64(0), true
}

type sysJITValue struct{}

func (*sysJITValue) TypeName() string { return "_jit" }
func (*sysJITValue) Repr() string     { return "<sys._jit>" }
func (*sysJITValue) isValue()         {}
func (*sysJITValue) attribute(name string) (Value, bool) {
	if name != "is_enabled" {
		return nil, false
	}
	return nativeFunctionNamed("sys._jit.is_enabled", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return falseSingleton, nil, nil
		}), true
}

func sysIntern(_ *frame, arguments []Value) (Value, *Exception, error) {
	if _, ok := arguments[0].(*stringValue); !ok {
		return nil, newException("TypeError", "intern() argument must be str"), nil
	}
	return arguments[0], nil, nil
}

type frameValue struct {
	frame *frame
}

func (*frameValue) TypeName() string { return "frame" }
func (*frameValue) Repr() string     { return "<frame object>" }
func (*frameValue) isValue()         {}

// attribute exposes the code, namespaces, line, and caller link consumed by
// inspect and traceback without leaking a mutable Go frame.
func (value *frameValue) attribute(name string) (Value, bool) {
	switch name {
	case "f_locals":
		return &namespaceValue{namespace: value.frame.locals}, true
	case "f_globals":
		return &namespaceValue{namespace: value.frame.globals}, true
	case "f_code":
		return &codeValue{code: value.frame.code}, true
	case "f_lineno":
		position := value.frame.position(max(0, value.frame.instruction-1))
		return newInt64(int64(position.Start.Line)), true
	case "f_back":
		if value.frame.previous == nil {
			return None, true
		}
		return &frameValue{frame: value.frame.previous}, true
	case "clear":
		return nativeFunctionNamed("frame.clear", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			}), true
	default:
		return nil, false
	}
}

// sysGetFrame validates an optional depth and walks suspended callers while
// retaining the selected execution frame through a Python-visible wrapper.
func sysGetFrame(caller *frame, arguments []Value) (Value, *Exception, error) {
	depth := int64(0)
	if len(arguments) == 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		depth = integer.Int64()
	}
	selected := caller
	for depth > 0 && selected != nil {
		if selected.previous != nil {
			selected = selected.previous
		} else {
			selected = selected.logicalPrevious
		}
		depth--
	}
	if depth != 0 || selected == nil {
		return nil, newException("ValueError", "call stack is not deep enough"), nil
	}
	return &frameValue{frame: selected}, nil, nil
}

var _ Value = (*frameValue)(nil)

func sysExcInfo(caller *frame, _ []Value) (Value, *Exception, error) {
	exception := activeHandledException(caller, caller.instruction-1)
	if exception == nil {
		return &tupleValue{elements: []Value{None, None, None}}, nil, nil
	}
	var exceptionType Value = exception.class
	if exception.userClass != nil {
		exceptionType = exception.userClass
	}
	return &tupleValue{elements: []Value{
		exceptionType, exception, materializeTraceback(exception.traceback),
	}}, nil, nil
}

func sysException(caller *frame, _ []Value) (Value, *Exception, error) {
	exception := activeHandledException(caller, caller.instruction-1)
	if exception == nil {
		return None, nil, nil
	}
	return exception, nil, nil
}

type tracebackValue struct {
	entry tracebackEntry
	next  Value
}

func (*tracebackValue) TypeName() string { return "traceback" }
func (*tracebackValue) Repr() string     { return "<traceback object>" }
func (*tracebackValue) isValue()         {}
func (traceback *tracebackValue) attribute(name string) (Value, bool) {
	switch name {
	case "tb_frame":
		return &frameValue{frame: traceback.entry.frame}, true
	case "tb_lineno":
		position := traceback.entry.frame.position(traceback.entry.instruction)
		return newInt64(int64(position.Start.Line)), true
	case "tb_lasti":
		// A negative offset asks traceback.py to use tb_lineno directly and
		// avoids claiming CPython bytecode offsets for Bullsnake instructions.
		return newInt64(-1), true
	case "tb_next":
		return traceback.next, true
	default:
		return nil, false
	}
}

func materializeTraceback(entries []tracebackEntry) Value {
	var next Value = None
	for _, entry := range entries {
		next = &tracebackValue{entry: entry, next: next}
	}
	return next
}

var _ Value = (*tracebackValue)(nil)

func sysExit(_ *frame, arguments []Value) (Value, *Exception, error) {
	exception := newExceptionOfType(systemExitType, exceptionMessage(arguments))
	exception.args = &tupleValue{elements: append([]Value(nil), arguments...)}
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
