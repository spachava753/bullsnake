package runtime

import (
	"fmt"
	"io"
	"unicode/utf8"
)

// write reports Python character counts only after a complete successful write.
// Short writes are failures, and borrowed providers are never retried or closed.
func (stream *hostTextStream) write(value Value) (Value, *Exception) {
	if stream.writer == nil {
		return nil, unsupportedStreamOperation("not writable")
	}
	text, ok := value.(*stringValue)
	if !ok {
		return nil, newException("TypeError", "write() argument must be str, not "+value.TypeName())
	}
	if !utf8.ValidString(text.value) {
		offset, position := 0, int64(0)
		for offset < len(text.value) {
			current, width, valid := decodeStringRune(text.value[offset:])
			if !valid || current >= 0xd800 && current <= 0xdfff {
				break
			}
			offset += width
			position++
		}
		return nil, streamUnicodeError(unicodeEncodeErrorType, text, position, "surrogates not allowed")
	}
	data := []byte(text.value)
	written, err := stream.writer.Write(data)
	if written < 0 || written > len(data) {
		return nil, providerException(fmt.Errorf("writer returned invalid byte count %d", written), nil)
	}
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		exception := providerException(err, nil)
		if exception.class == blockingIOErrorType {
			complete := written
			for complete > 0 && !utf8.Valid(data[:complete]) {
				complete--
			}
			exception.fields.values["characters_written"] = integerFromInt64(int64(utf8.RuneCount(data[:complete])))
		}
		return nil, exception
	}
	return integerFromInt64(int64(utf8.RuneCountInString(text.value))), nil
}
