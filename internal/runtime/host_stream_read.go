package runtime

import (
	"io"
	"strings"
	"unicode/utf8"
)

// read counts decoded characters and leaves bytes from partial provider reads
// buffered. A failure after returned text is retained for the next read call.
// Newlines are preserved, with LF alone delimiting readline results.
func (stream *hostTextStream) read(arguments []Value, line bool) (Value, *Exception) {
	if stream.reader == nil {
		return nil, unsupportedStreamOperation("not readable")
	}
	size := int64(-1)
	if len(arguments) == 1 && (arguments[0] != None || line) {
		number, ok := integerOperand(arguments[0])
		if !ok {
			return nil, newException("TypeError", "read size must be an integer or None")
		}
		if !number.IsInt64() {
			return nil, newException("OverflowError", "cannot fit read size into an index-sized integer")
		}
		size = number.Int64()
	}
	if size == 0 {
		return &stringValue{}, nil
	}
	if stream.pendingError != nil {
		err := stream.pendingError
		stream.pendingError = nil
		return nil, providerException(err, nil)
	}
	var text strings.Builder
	for count := int64(0); size < 0 || count < size; count++ {
		prefix, err := stream.reader.Peek(1)
		var first byte
		var current rune
		var width int
		if len(prefix) != 0 {
			first = prefix[0]
			current, width, err = stream.reader.ReadRune()
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			if text.Len() == 0 {
				return nil, providerException(err, nil)
			}
			stream.pendingError = err
			break
		}
		if current == utf8.RuneError && width == 1 {
			return nil, streamUnicodeError(unicodeDecodeErrorType, &bytesValue{value: string([]byte{first})}, 0, "invalid UTF-8 sequence")
		}
		text.WriteRune(current)
		if line && current == '\n' {
			break
		}
	}
	return &stringValue{value: text.String()}, nil
}
