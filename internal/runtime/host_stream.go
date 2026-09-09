package runtime

import (
	"bufio"
	"io"
)

var hostTextStreamType = nativeType("bullsnake", "HostTextStream")

// hostTextStream borrows exactly one direction of Go I/O. Only output Flush
// and either direction's IsTerminal are recognized as optional capabilities.
type hostTextStream struct {
	reader       *bufio.Reader
	writer       io.Writer
	terminal     Terminal
	closed       bool
	pendingError error
}

func (*hostTextStream) TypeName() string { return "bullsnake.HostTextStream" }
func (*hostTextStream) Repr() string     { return "<bullsnake.HostTextStream>" }
func (*hostTextStream) isValue()         {}

func newHostOutput(writer io.Writer) Value {
	if writer == nil {
		return None
	}
	terminal, _ := writer.(Terminal)
	return &hostTextStream{writer: writer, terminal: terminal}
}

// executeHostStreamAttributeLoad exposes a fixed Python surface, independent of
// any unrelated interfaces implemented by a supplied Go provider.
func executeHostStreamAttributeLoad(frame *frame, instruction int, stream *hostTextStream, name string) (instructionOutcome, error) {
	switch name {
	case "closed":
		return pushOutcome(frame, instruction, booleanValue(stream.closed))
	case "encoding":
		return pushOutcome(frame, instruction, &stringValue{value: "utf-8"})
	case "errors":
		return pushOutcome(frame, instruction, &stringValue{value: "strict"})
	case "read", "readline", "write", "flush", "close", "readable", "writable", "seekable", "isatty", "seek", "tell", "truncate", "fileno", "__enter__", "__exit__":
		method := &builtinFunctionValue{name: name, call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
			return stream.call(name, arguments, keywords)
		}}
		return pushOutcome(frame, instruction, method)
	default:
		return raiseOutcome(newException("AttributeError", "'bullsnake.HostTextStream' object has no attribute '"+name+"'")), nil
	}
}

// call validates each method before inspecting closed state or invoking a
// provider. Close is idempotent and closes the wrapper even when flush fails.
func (stream *hostTextStream) call(name string, arguments []Value, keywords *dictValue) (Value, *Exception) {
	minimum, maximum := streamMethodArity(name)
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return nil, exception
	}
	if name == "close" || name == "__exit__" {
		if stream.closed {
			return None, nil
		}
		exception := stream.flush()
		stream.closed = true
		return None, exception
	}
	if stream.closed {
		return nil, newException("ValueError", "I/O operation on closed file.")
	}
	switch name {
	case "write":
		return stream.write(arguments[0])
	case "read", "readline":
		return stream.read(arguments, name == "readline")
	case "flush":
		return None, stream.flush()
	case "readable":
		return booleanValue(stream.reader != nil), nil
	case "writable":
		return booleanValue(stream.writer != nil), nil
	case "seekable":
		return falseSingleton, nil
	case "isatty":
		return booleanValue(stream.terminal != nil && stream.terminal.IsTerminal()), nil
	case "__enter__":
		return stream, nil
	default:
		return nil, unsupportedStreamOperation(name)
	}
}

func streamMethodArity(name string) (int, int) {
	switch name {
	case "write":
		return 1, 1
	case "read", "readline", "truncate":
		return 0, 1
	case "seek":
		return 1, 2
	case "__exit__":
		return 3, 3
	default:
		return 0, 0
	}
}

func unsupportedStreamOperation(name string) *Exception {
	return newExceptionOfType(unsupportedOperationType, name)
}

func (stream *hostTextStream) flush() *Exception {
	if flusher, ok := stream.writer.(Flusher); ok {
		if err := flusher.Flush(); err != nil {
			return providerException(err, nil)
		}
	}
	return nil
}
