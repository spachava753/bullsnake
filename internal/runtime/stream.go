package runtime

import (
	"io"
	"math/big"

	"github.com/spachava753/bullsnake/host"
)

type streamValue struct {
	name   string
	reader io.Reader
	writer io.Writer
}

func (*streamValue) TypeName() string { return "TextIOWrapper" }
func (stream *streamValue) Repr() string {
	return "<host stream " + stream.name + ">"
}
func (*streamValue) isValue() {}

func (stream *streamValue) attribute(name string) (Value, bool) {
	switch name {
	case "write":
		return nativeFunctionNamed(stream.name+".write", 1, 1, stream.write), true
	case "flush":
		return nativeFunctionNamed(stream.name+".flush", 0, 0, stream.flush), true
	case "isatty":
		return nativeFunctionNamed(stream.name+".isatty", 0, 0, stream.isatty), true
	default:
		return nil, false
	}
}

func (stream *streamValue) write(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException(
			"TypeError",
			"write() argument must be str, not "+arguments[0].TypeName(),
		), nil
	}
	if stream.writer == nil {
		return nil, hostFailure(stream.name+".write", host.ErrDenied), nil
	}
	written, err := io.WriteString(stream.writer, text.value)
	if err != nil {
		return nil, hostFailure(stream.name+".write", err), nil
	}
	var value big.Int
	value.SetInt64(int64(written))
	return &intValue{value: value}, nil, nil
}

func (stream *streamValue) flush(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	if stream.writer == nil {
		return nil, hostFailure(stream.name+".flush", host.ErrDenied), nil
	}
	if flusher, ok := stream.writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return nil, hostFailure(stream.name+".flush", err), nil
		}
	}
	return None, nil, nil
}

func (*streamValue) isatty(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	return falseSingleton, nil, nil
}
