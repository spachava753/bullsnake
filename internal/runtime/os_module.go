package runtime

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spachava753/bullsnake/host"
)

// newOSModule builds the interpreter-local OS facade and connects every host
// read or mutation to its separately configured capability.
func (runtimeState *Runtime) newOSModule() *Module {
	module := newSystemModule("os", "")
	pathModule, _ := runtimeState.loadSystemModule("os.path")
	module.globals.values["path"] = pathModule
	name := "posix"
	linesep := "\n"
	if runtime.GOOS == "windows" {
		name = "nt"
		linesep = "\r\n"
	}
	module.globals.values["name"] = &stringValue{value: name}
	module.globals.values["curdir"] = &stringValue{value: "."}
	module.globals.values["pardir"] = &stringValue{value: ".."}
	module.globals.values["sep"] = &stringValue{value: string(filepath.Separator)}
	module.globals.values["extsep"] = &stringValue{value: "."}
	module.globals.values["pathsep"] = &stringValue{value: string(filepath.ListSeparator)}
	module.globals.values["linesep"] = &stringValue{value: linesep}
	module.globals.values["F_OK"] = newInt64(0)
	module.globals.values["X_OK"] = newInt64(1)
	module.globals.values["W_OK"] = newInt64(2)
	module.globals.values["R_OK"] = newInt64(4)
	module.globals.values["O_RDONLY"] = newInt64(int64(os.O_RDONLY))
	module.globals.values["O_WRONLY"] = newInt64(int64(os.O_WRONLY))
	module.globals.values["O_RDWR"] = newInt64(int64(os.O_RDWR))
	module.globals.values["O_CREAT"] = newInt64(int64(os.O_CREATE))
	module.globals.values["O_EXCL"] = newInt64(int64(os.O_EXCL))
	module.globals.values["O_TRUNC"] = newInt64(int64(os.O_TRUNC))
	module.globals.values["O_APPEND"] = newInt64(int64(os.O_APPEND))
	module.globals.values["environ"] = runtimeState.environment()
	module.globals.values["stat_result"] = &builtinTypeValue{
		name: "stat_result",
		matches: func(value Value) bool {
			_, ok := value.(*statResultValue)
			return ok
		},
	}
	module.globals.values["supports_dir_fd"] = &setValue{}
	module.globals.values["supports_fd"] = &setValue{}
	module.globals.values["supports_follow_symlinks"] = &setValue{}
	for _, functionName := range []string{"open", "close", "scandir"} {
		setNativeFunction(module, functionName, 0, -1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return nil, newException("NotImplementedError", "file descriptor operation is not implemented"), nil
			})
	}
	setNativeFunction(module, "getcwd", 0, 0, runtimeState.getcwd)
	setNativeFunction(module, "urandom", 1, 1, runtimeState.urandom)
	setNativeFunction(module, "chdir", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			name, exception := pathArgument("chdir", arguments[0])
			if exception != nil {
				return nil, exception, nil
			}
			if runtimeState.host.WorkingDirectory == nil {
				return nil, hostFailure("chdir", host.ErrDenied), nil
			}
			if err := runtimeState.host.WorkingDirectory.Chdir(name); err != nil {
				return nil, hostFailure("chdir", err), nil
			}
			return None, nil, nil
		})
	setNativeFunction(module, "terminal_size", 1, 1, newTerminalSize)
	setNativeFunction(module, "get_terminal_size", 0, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &terminalSizeValue{columns: 80, lines: 24}, nil, nil
		})
	setNativeFunction(module, "listdir", 0, 1, runtimeState.listdir)
	setNativeFunction(module, "stat", 1, 1, runtimeState.stat)
	setNativeFunction(module, "lstat", 1, 1, runtimeState.stat)
	setNativeFunction(module, "remove", 1, 1, runtimeState.remove)
	setNativeFunction(module, "unlink", 1, 1, runtimeState.remove)
	setNativeFunction(module, "rmdir", 1, 1, runtimeState.removeDir)
	setNativeFunction(module, "fspath", 1, 1, osFspath)
	setNativeFunction(module, "getpid", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return newInt64(1), nil, nil
		})
	setNativeFunction(module, "kill", 2, 2, runtimeState.kill)
	setNativeFunction(module, "fsdecode", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			switch value := arguments[0].(type) {
			case *stringValue:
				return value, nil, nil
			case *bytesValue:
				return &stringValue{value: value.value}, nil, nil
			default:
				return nil, newException("TypeError", "expected str or bytes"), nil
			}
		})
	setNativeFunction(module, "fsencode", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			switch value := arguments[0].(type) {
			case *bytesValue:
				return value, nil, nil
			case *stringValue:
				return &bytesValue{value: value.value}, nil, nil
			default:
				return nil, newException("TypeError", "expected str or bytes"), nil
			}
		})
	return module
}

// urandom obtains exactly the requested bytes through the configured entropy
// capability and translates host policy failures into Python exceptions.
func (runtimeState *Runtime) urandom(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	size, ok := integerOperand(arguments[0])
	if !ok || !size.IsInt64() {
		return nil, newException("TypeError", "'size' must be an integer"), nil
	}
	if size.Sign() < 0 {
		return nil, newException("ValueError", "negative argument not allowed"), nil
	}
	if runtimeState.host.Entropy == nil {
		return nil, hostFailure("urandom", host.ErrDenied), nil
	}
	buffer := make([]byte, int(size.Int64()))
	if _, err := io.ReadFull(runtimeState.host.Entropy, buffer); err != nil {
		return nil, hostFailure("urandom", err), nil
	}
	return &bytesValue{value: string(buffer)}, nil, nil
}

// kill delivers supported signals inside this interpreter without changing process handlers.
func (runtimeState *Runtime) kill(
	caller *frame,
	arguments []Value,
) (Value, *Exception, error) {
	pid, pidOK := integerOperand(arguments[0])
	signalNumber, signalOK := integerOperand(arguments[1])
	if !pidOK || !signalOK || !pid.IsInt64() || !signalNumber.IsInt64() {
		return nil, newException("TypeError", "pid and signal number must be integers"), nil
	}
	if pid.Int64() != 1 {
		return nil, newException("ProcessLookupError", "no such process"), nil
	}
	handler, found := runtimeState.signalHandlers[signalNumber.Int64()]
	if !found || handler == falseSingleton || valuesEqual(handler, newInt64(0)) {
		if signalNumber.Int64() == 2 {
			return nil, newException("KeyboardInterrupt", ""), nil
		}
		return None, nil, nil
	}
	if handler == trueSingleton || valuesEqual(handler, newInt64(1)) {
		return None, nil, nil
	}
	return callValueSynchronously(caller, handler, []Value{arguments[1], None})
}

type terminalSizeValue struct {
	columns int64
	lines   int64
}

func (*terminalSizeValue) TypeName() string { return "terminal_size" }
func (value *terminalSizeValue) Repr() string {
	return "os.terminal_size(columns=" + newInt64(value.columns).Repr() +
		", lines=" + newInt64(value.lines).Repr() + ")"
}
func (*terminalSizeValue) isValue() {}
func (value *terminalSizeValue) attribute(name string) (Value, bool) {
	switch name {
	case "columns":
		return newInt64(value.columns), true
	case "lines":
		return newInt64(value.lines), true
	default:
		return nil, false
	}
}

// newTerminalSize constructs the attribute-bearing pair returned by terminal queries.
func newTerminalSize(_ *frame, arguments []Value) (Value, *Exception, error) {
	columns, exception := directSubscript(arguments[0], newInt64(0))
	if exception != nil {
		return nil, exception, nil
	}
	lines, exception := directSubscript(arguments[0], newInt64(1))
	if exception != nil {
		return nil, exception, nil
	}
	columnInteger, columnsOK := integerOperand(columns)
	lineInteger, linesOK := integerOperand(lines)
	if !columnsOK || !linesOK || !columnInteger.IsInt64() || !lineInteger.IsInt64() {
		return nil, newException("TypeError", "terminal_size values must be integers"), nil
	}
	return &terminalSizeValue{columns: columnInteger.Int64(), lines: lineInteger.Int64()}, nil, nil
}

var _ Value = (*terminalSizeValue)(nil)

type statResultValue struct {
	info fs.FileInfo
}

func (*statResultValue) TypeName() string { return "stat_result" }
func (result *statResultValue) Repr() string {
	return "os.stat_result(st_size=" + newInt64(result.info.Size()).Repr() + ")"
}
func (*statResultValue) isValue() {}
func (result *statResultValue) attribute(name string) (Value, bool) {
	switch name {
	case "st_size":
		return newInt64(result.info.Size()), true
	case "st_mtime":
		return &floatValue{value: float64(result.info.ModTime().UnixNano()) / 1e9}, true
	case "st_mtime_ns":
		return newInt64(result.info.ModTime().UnixNano()), true
	case "st_mode":
		return newInt64(int64(result.info.Mode())), true
	default:
		return nil, false
	}
}

func (runtimeState *Runtime) stat(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	name, exception := pathArgument("stat", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	if runtimeState.host.Files == nil {
		return nil, hostFailure("os.stat", host.ErrDenied), nil
	}
	info, err := runtimeState.host.Files.Stat(name)
	if err != nil {
		return nil, hostFailure("os.stat", err), nil
	}
	return &statResultValue{info: info}, nil, nil
}

var _ Value = (*statResultValue)(nil)

func (runtimeState *Runtime) removeDir(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	name, exception := pathArgument("rmdir", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	if runtimeState.host.FileMutator == nil {
		return nil, hostFailure("os.rmdir", host.ErrDenied), nil
	}
	if err := runtimeState.host.FileMutator.RemoveDir(name); err != nil {
		return nil, hostFailure("os.rmdir", err), nil
	}
	return None, nil, nil
}

func (runtimeState *Runtime) remove(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	name, exception := pathArgument("remove", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	if runtimeState.host.FileMutator == nil {
		return nil, hostFailure("os.remove", host.ErrDenied), nil
	}
	if err := runtimeState.host.FileMutator.Remove(name); err != nil {
		return nil, hostFailure("os.remove", err), nil
	}
	return None, nil, nil
}

func (runtimeState *Runtime) newOSPathModule() *Module {
	module := newSystemModule("os.path", "os")
	module.globals.values["curdir"] = &stringValue{value: "."}
	module.globals.values["pardir"] = &stringValue{value: ".."}
	module.globals.values["sep"] = &stringValue{value: string(filepath.Separator)}
	module.globals.values["pathsep"] = &stringValue{value: string(filepath.ListSeparator)}
	module.globals.values["extsep"] = &stringValue{value: "."}
	setNativeFunction(module, "join", 1, -1, pathJoin)
	setNativeFunction(module, "basename", 1, 1, pathBase)
	setNativeFunction(module, "dirname", 1, 1, pathDir)
	setNativeFunction(module, "splitext", 1, 1, pathSplitExt)
	setNativeFunction(module, "splitdrive", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			path, ok := arguments[0].(*stringValue)
			if !ok {
				return nil, newException("TypeError", "path must be str"), nil
			}
			volume := filepath.VolumeName(path.value)
			return &tupleValue{elements: []Value{
				&stringValue{value: volume},
				&stringValue{value: strings.TrimPrefix(path.value, volume)},
			}}, nil, nil
		})
	setNativeFunction(module, "normpath", 1, 1, pathClean)
	setNativeFunction(module, "normcase", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			return arguments[0], nil, nil
		})
	setNativeFunction(module, "isabs", 1, 1, pathIsAbs)
	setNativeFunction(module, "abspath", 1, 1, runtimeState.pathAbs)
	setNativeFunction(module, "relpath", 1, 2, runtimeState.pathRel)
	setNativeFunction(module, "realpath", 1, 1, runtimeState.pathReal)
	setNativeFunction(module, "exists", 1, 1, runtimeState.pathExists)
	setNativeFunction(module, "isfile", 1, 1, runtimeState.pathIsFile)
	setNativeFunction(module, "isdir", 1, 1, runtimeState.pathIsDir)
	setNativeFunction(module, "commonprefix", 1, 1, pathCommonPrefix)
	return module
}

func (runtimeState *Runtime) environment() *dictValue {
	environment := &dictValue{}
	if runtimeState.host.Process == nil {
		return environment
	}
	for _, entry := range runtimeState.host.Process.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if exception := environment.set(
			&stringValue{value: name},
			&stringValue{value: value},
		); exception != nil {
			panic("runtime: environment name is not hashable")
		}
	}
	return environment
}

func (runtimeState *Runtime) getcwd(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	if runtimeState.host.Process == nil {
		return nil, hostFailure("os.getcwd", host.ErrDenied), nil
	}
	directory, err := runtimeState.host.Process.Getwd()
	if err != nil {
		return nil, hostFailure("os.getcwd", err), nil
	}
	return &stringValue{value: directory}, nil, nil
}

// listdir validates an optional path and returns names from the configured
// filesystem capability.
func (runtimeState *Runtime) listdir(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	name := "."
	if len(arguments) != 0 {
		var exception *Exception
		name, exception = pathArgument("listdir", arguments[0])
		if exception != nil {
			return nil, exception, nil
		}
	}
	if runtimeState.host.Files == nil {
		return nil, hostFailure("os.listdir", host.ErrDenied), nil
	}
	entries, err := runtimeState.host.Files.ReadDir(name)
	if err != nil {
		return nil, hostFailure("os.listdir", err), nil
	}
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name()
	}
	return stringList(names), nil, nil
}

// osFspath accepts native path values or dispatches __fspath__ while enforcing
// its string-or-bytes return contract.
func osFspath(caller *frame, arguments []Value) (Value, *Exception, error) {
	if _, ok := arguments[0].(*stringValue); ok {
		return arguments[0], nil, nil
	}
	if _, ok := arguments[0].(*bytesValue); ok {
		return arguments[0], nil, nil
	}
	if instance, ok := arguments[0].(*instanceValue); ok {
		method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__fspath__")
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if found {
			result, exception, err := callValueSynchronously(caller, method, nil)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			switch result.(type) {
			case *stringValue, *bytesValue:
				return result, nil, nil
			default:
				return nil, newException("TypeError", "__fspath__ returned non-string"), nil
			}
		}
	}
	{
		return nil, newException(
			"TypeError",
			"expected str, bytes or os.PathLike object, not "+arguments[0].TypeName(),
		), nil
	}
}

func pathJoin(_ *frame, arguments []Value) (Value, *Exception, error) {
	parts := make([]string, len(arguments))
	for index, argument := range arguments {
		part, exception := pathArgument("join", argument)
		if exception != nil {
			return nil, exception, nil
		}
		parts[index] = part
	}
	return &stringValue{value: filepath.Join(parts...)}, nil, nil
}

func pathBase(_ *frame, arguments []Value) (Value, *Exception, error) {
	name, exception := pathArgument("basename", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return &stringValue{value: filepath.Base(name)}, nil, nil
}

func pathDir(_ *frame, arguments []Value) (Value, *Exception, error) {
	name, exception := pathArgument("dirname", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return &stringValue{value: filepath.Dir(name)}, nil, nil
}

func pathSplitExt(_ *frame, arguments []Value) (Value, *Exception, error) {
	name, exception := pathArgument("splitext", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	extension := filepath.Ext(name)
	root := strings.TrimSuffix(name, extension)
	return &tupleValue{elements: []Value{
		&stringValue{value: root},
		&stringValue{value: extension},
	}}, nil, nil
}

func pathClean(_ *frame, arguments []Value) (Value, *Exception, error) {
	name, exception := pathArgument("normpath", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return &stringValue{value: filepath.Clean(name)}, nil, nil
}

func pathIsAbs(_ *frame, arguments []Value) (Value, *Exception, error) {
	name, exception := pathArgument("isabs", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return pythonBool(filepath.IsAbs(name)), nil, nil
}

func (runtimeState *Runtime) pathAbs(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	name, exception := pathArgument("abspath", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	absolute, exception := runtimeState.absolutePath(name)
	if exception != nil {
		return nil, exception, nil
	}
	return &stringValue{value: absolute}, nil, nil
}

// pathRel resolves both operands through the configured working directory
// before calculating their lexical relative path.
func (runtimeState *Runtime) pathRel(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	name, exception := pathArgument("relpath", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	start := "."
	if len(arguments) == 2 {
		start, exception = pathArgument("relpath", arguments[1])
		if exception != nil {
			return nil, exception, nil
		}
	}
	absoluteName, exception := runtimeState.absolutePath(name)
	if exception != nil {
		return nil, exception, nil
	}
	absoluteStart, exception := runtimeState.absolutePath(start)
	if exception != nil {
		return nil, exception, nil
	}
	relative, err := filepath.Rel(absoluteStart, absoluteName)
	if err != nil {
		return nil, hostFailure("os.path.relpath", err), nil
	}
	return &stringValue{value: relative}, nil, nil
}

// pathReal resolves host-backed paths and lexically normalizes nonexistent test paths.
func (runtimeState *Runtime) pathReal(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	name, exception := pathArgument("realpath", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	if runtimeState.host.Files == nil {
		return nil, hostFailure("os.path.realpath", host.ErrDenied), nil
	}
	absolute, exception := runtimeState.absolutePath(name)
	if exception != nil {
		return nil, exception, nil
	}
	resolved, err := runtimeState.host.Files.RealPath(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		return &stringValue{value: filepath.Clean(absolute)}, nil, nil
	}
	if err != nil {
		return nil, hostFailure("os.path.realpath", err), nil
	}
	return &stringValue{value: resolved}, nil, nil
}

func (runtimeState *Runtime) pathExists(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	_, exists, exception := runtimeState.pathInfo("exists", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return pythonBool(exists), nil, nil
}

func (runtimeState *Runtime) pathIsFile(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	info, exists, exception := runtimeState.pathInfo("isfile", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return pythonBool(exists && info.Mode().IsRegular()), nil, nil
}

func (runtimeState *Runtime) pathIsDir(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	info, exists, exception := runtimeState.pathInfo("isdir", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return pythonBool(exists && info.IsDir()), nil, nil
}

func (runtimeState *Runtime) pathInfo(
	operation string,
	argument Value,
) (fs.FileInfo, bool, *Exception) {
	name, exception := pathArgument(operation, argument)
	if exception != nil {
		return nil, false, exception
	}
	if runtimeState.host.Files == nil {
		return nil, false, hostFailure("os.path."+operation, host.ErrDenied)
	}
	info, err := runtimeState.host.Files.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, hostFailure("os.path."+operation, err)
	}
	return info, true, nil
}

// pathCommonPrefix validates one string sequence and compares Unicode code
// points until all entries stop sharing a prefix.
func pathCommonPrefix(_ *frame, arguments []Value) (Value, *Exception, error) {
	var elements []Value
	switch values := arguments[0].(type) {
	case *listValue:
		elements = values.elements
	case *tupleValue:
		elements = values.elements
	default:
		return nil, newException(
			"TypeError",
			"commonprefix() argument must be a list or tuple, not "+
				arguments[0].TypeName(),
		), nil
	}
	if len(elements) == 0 {
		return &stringValue{}, nil, nil
	}
	values := make([][]rune, len(elements))
	for index, element := range elements {
		value, exception := pathArgument("commonprefix", element)
		if exception != nil {
			return nil, exception, nil
		}
		values[index] = []rune(value)
	}
	prefix := values[0]
	for _, value := range values[1:] {
		limit := min(len(prefix), len(value))
		index := 0
		for index < limit && prefix[index] == value[index] {
			index++
		}
		prefix = prefix[:index]
	}
	return &stringValue{value: string(prefix)}, nil, nil
}

func (runtimeState *Runtime) absolutePath(name string) (string, *Exception) {
	if filepath.IsAbs(name) {
		return filepath.Clean(name), nil
	}
	if runtimeState.host.Process == nil {
		return "", hostFailure("os.path.abspath", host.ErrDenied)
	}
	directory, err := runtimeState.host.Process.Getwd()
	if err != nil {
		return "", hostFailure("os.path.abspath", err)
	}
	return filepath.Join(directory, name), nil
}

func pathArgument(function string, value Value) (string, *Exception) {
	text, ok := value.(*stringValue)
	if !ok {
		return "", newException(
			"TypeError",
			function+"() argument must be str, not "+value.TypeName(),
		)
	}
	return text.value, nil
}

func pythonBool(value bool) Value {
	if value {
		return trueSingleton
	}
	return falseSingleton
}
