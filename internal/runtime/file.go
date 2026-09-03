package runtime

import (
	"strings"

	"github.com/spachava753/bullsnake/host"
)

type fileValue struct {
	name       string
	data       string
	position   int
	binary     bool
	closed     bool
	attributes *Namespace
}

func (*fileValue) TypeName() string { return "file" }
func (file *fileValue) Repr() string {
	modeName := "TextIOWrapper"
	if file.binary {
		modeName = "BufferedReader"
	}
	return "<_io." + modeName + " name='" + file.name + "'>"
}
func (*fileValue) isValue() {}

// attribute exposes read-only file operations over host-provided contents.
func (file *fileValue) attribute(name string) (Value, bool) {
	switch name {
	case "name":
		return &stringValue{value: file.name}, true
	case "closed":
		return pythonBool(file.closed), true
	case "read":
		return nativeFunctionNamed("file.read", 0, 1, file.read), true
	case "readline":
		return nativeFunctionNamed("file.readline", 0, 1, file.readline), true
	case "readlines":
		return nativeFunctionNamed("file.readlines", 0, 1, file.readlines), true
	case "close":
		return nativeFunctionNamed("file.close", 0, 0, file.close), true
	case "seek":
		return nativeFunctionNamed("file.seek", 1, 2, file.seek), true
	case "tell":
		return nativeFunctionNamed("file.tell", 0, 0, file.tell), true
	case "__enter__":
		return nativeFunctionNamed("file.__enter__", 0, 0, file.enter), true
	case "__exit__":
		return nativeFunctionNamed("file.__exit__", 3, 3, file.exit), true
	case "__iter__":
		return nativeFunctionNamed("file.__iter__", 0, 0, file.enter), true
	case "__next__":
		return nativeFunctionNamed("file.__next__", 0, 0, file.nextMethod), true
	default:
		if file.attributes == nil {
			return nil, false
		}
		return file.attributes.get(name)
	}
}

// builtinOpen validates a read mode and obtains bytes through the host filesystem capability.
func (runtimeState *Runtime) builtinOpen(_ *frame, arguments []Value) (Value, *Exception, error) {
	name, exception := pathArgument("open", arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	mode := "r"
	if len(arguments) >= 2 {
		modeValue, ok := arguments[1].(*stringValue)
		if !ok {
			return nil, newException("TypeError", "open mode must be str"), nil
		}
		mode = modeValue.value
	}
	if strings.ContainsAny(mode, "wax+") {
		return nil, hostFailure("open", host.ErrDenied), nil
	}
	if runtimeState.host.Files == nil {
		return nil, hostFailure("open", host.ErrDenied), nil
	}
	data, err := runtimeState.host.Files.ReadFile(name)
	if err != nil {
		return nil, hostFailure("open", err), nil
	}
	return &fileValue{
		name: name, data: string(data), binary: strings.Contains(mode, "b"),
		attributes: newNamespace(),
	}, nil, nil
}

// read consumes the requested number of bytes or all remaining content.
func (file *fileValue) read(_ *frame, arguments []Value) (Value, *Exception, error) {
	if file.closed {
		return nil, newException("ValueError", "I/O operation on closed file"), nil
	}
	end := len(file.data)
	if len(arguments) == 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "read size must be an integer"), nil
		}
		if size := integer.Int64(); size >= 0 && size < int64(end-file.position) {
			end = file.position + int(size)
		}
	}
	data := file.data[file.position:end]
	file.position = end
	if file.binary {
		return &bytesValue{value: data}, nil, nil
	}
	return &stringValue{value: data}, nil, nil
}

// readline consumes at most one line while respecting an optional byte limit.
func (file *fileValue) readline(_ *frame, arguments []Value) (Value, *Exception, error) {
	if file.closed {
		return nil, newException("ValueError", "I/O operation on closed file"), nil
	}
	end := len(file.data)
	if newline := strings.IndexByte(file.data[file.position:], '\n'); newline >= 0 {
		end = file.position + newline + 1
	}
	if len(arguments) == 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "readline size must be an integer"), nil
		}
		if size := integer.Int64(); size >= 0 && size < int64(end-file.position) {
			end = file.position + int(size)
		}
	}
	data := file.data[file.position:end]
	file.position = end
	if file.binary {
		return &bytesValue{value: data}, nil, nil
	}
	return &stringValue{value: data}, nil, nil
}

func (file *fileValue) readlines(_ *frame, _ []Value) (Value, *Exception, error) {
	lines := make([]Value, 0)
	for file.position < len(file.data) {
		line, exception, err := file.readline(nil, nil)
		if exception != nil || err != nil {
			return nil, exception, err
		}
		lines = append(lines, line)
	}
	return &listValue{elements: lines}, nil, nil
}

func (file *fileValue) close(_ *frame, _ []Value) (Value, *Exception, error) {
	file.closed = true
	return None, nil, nil
}

// seek updates the in-memory cursor relative to the selected origin.
func (file *fileValue) seek(_ *frame, arguments []Value) (Value, *Exception, error) {
	integer, ok := integerOperand(arguments[0])
	if !ok || !integer.IsInt64() {
		return nil, newException("TypeError", "seek offset must be an integer"), nil
	}
	position := integer.Int64()
	if len(arguments) == 2 {
		whence, whenceOK := integerOperand(arguments[1])
		if !whenceOK || !whence.IsInt64() {
			return nil, newException("TypeError", "whence must be an integer"), nil
		}
		switch whence.Int64() {
		case 1:
			position += int64(file.position)
		case 2:
			position += int64(len(file.data))
		}
	}
	if position < 0 || position > int64(len(file.data)) {
		return nil, newException("ValueError", "invalid seek position"), nil
	}
	file.position = int(position)
	return newInt64(position), nil, nil
}

func (file *fileValue) tell(_ *frame, _ []Value) (Value, *Exception, error) {
	return newInt64(int64(file.position)), nil, nil
}

func (file *fileValue) enter(_ *frame, _ []Value) (Value, *Exception, error) {
	return file, nil, nil
}

func (file *fileValue) exit(_ *frame, _ []Value) (Value, *Exception, error) {
	file.closed = true
	return falseSingleton, nil, nil
}

func (file *fileValue) nextMethod(_ *frame, _ []Value) (Value, *Exception, error) {
	if file.position >= len(file.data) {
		return nil, newException("StopIteration", ""), nil
	}
	value, exception, _ := file.readline(nil, nil)
	if exception != nil {
		return nil, exception, nil
	}
	return value, nil, nil
}

func (file *fileValue) next() (Value, bool, *Exception) {
	if file.position >= len(file.data) {
		return nil, false, nil
	}
	value, exception, _ := file.readline(nil, nil)
	return value, exception == nil, exception
}

var _ valueIterator = (*fileValue)(nil)

func newTextIOWrapper(_ *frame, arguments []Value) (Value, *Exception, error) {
	file, ok := arguments[0].(*fileValue)
	if !ok {
		return nil, newException("TypeError", "TextIOWrapper requires a buffered file"), nil
	}
	file.binary = false
	return file, nil, nil
}
