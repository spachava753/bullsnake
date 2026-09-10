package runtime

import (
	"strconv"
	"strings"
)

// executePositionCall binds sizes in source order and retains the stream while
// Python index methods run. Missing truncate size snapshots the current cursor.
func (stream *stringIOValue) executePositionCall(caller *frame, instruction, base int, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	minimum, maximum := streamMethodArity(name)
	if name == "readlines" {
		maximum = 1
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	size := -1
	if name == "truncate" {
		size = stream.position
	}
	if len(arguments) == 0 || (name != "seek" && arguments[0] == None) {
		return stream.finishPositionCall(caller, instruction, name, size, 0)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, arguments[0])
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		size := int(result.(*intValue).value.Int64())
		if len(arguments) < 2 {
			return stream.finishPositionCall(current, instruction, name, size, 0)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeIOIndex(current, instruction, arguments[1])
		}, func(resumed *frame, value Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			whence := value.(*intValue).value.Int64()
			if whence < -1<<31 || whence > 1<<31-1 {
				return raiseOutcome(newException("OverflowError", "Python int too large to convert to C int")), nil
			}
			return stream.finishPositionCall(resumed, instruction, name, size, int(whence))
		})
	})
}

// finishPositionCall validates positions before mutation. Truncating beyond EOF
// leaves text unchanged, and reads beyond EOF leave the cursor where it was.
func (stream *stringIOValue) finishPositionCall(caller *frame, instruction int, name string, size, whence int) (instructionOutcome, error) {
	if stream.closed {
		return raiseOutcome(newException("ValueError", "I/O operation on closed file")), nil
	}
	offsets := stringCodepointOffsets(stream.value)
	length := len(offsets) - 1
	switch name {
	case "readlines":
		result, exception := stream.readlines(size)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, result)
	case "read", "readline":
		return pushOutcome(caller, instruction, stream.read(size, name == "readline"))
	case "truncate":
		if size < 0 {
			return raiseOutcome(newException("ValueError", "Negative size value "+strconv.Itoa(size))), nil
		}
		if size < length {
			stream.value = stream.value[:offsets[size]]
		}
	case "seek":
		if whence < 0 || whence > 2 {
			return raiseOutcome(newException("ValueError", "Invalid whence ("+strconv.Itoa(whence)+", should be 0, 1 or 2)")), nil
		}
		if whence == 0 && size < 0 {
			return raiseOutcome(newException("ValueError", "Negative seek position "+strconv.Itoa(size))), nil
		}
		if whence != 0 && size != 0 {
			return raiseOutcome(newException("OSError", "Can't do nonzero cur-relative seeks")), nil
		}
		if whence == 1 {
			size = stream.position
		} else if whence == 2 {
			size = length
		}
		stream.position = size
	}
	return pushOutcome(caller, instruction, integerFromInt64(int64(size)))
}

// read clamps character counts to the remaining text before applying an
// optional newline boundary, then advances by the characters actually returned.
func (stream *stringIOValue) read(size int, line bool) *stringValue {
	offsets := stringCodepointOffsets(stream.value)
	remaining := len(offsets) - 1 - stream.position
	if remaining <= 0 || size == 0 {
		return &stringValue{}
	}
	if size < 0 || size > remaining {
		size = remaining
	}
	text := stream.value[offsets[stream.position]:offsets[stream.position+size]]
	if line {
		text = text[:stream.lineEnd(text)]
	}
	stream.position += len(stringCodepointOffsets(text)) - 1
	return &stringValue{value: text}
}

// lineEnd recognizes only the configured line boundary, including CRLF when
// both characters fall within the read limit. Unicode line separators are text.
func (stream *stringIOValue) lineEnd(text string) int {
	if stream.universal && !stream.translate {
		if index := strings.IndexAny(text, "\r\n"); index >= 0 {
			end := index + 1
			if text[index] == '\r' && end < len(text) && text[end] == '\n' {
				end++
			}
			return end
		}
	} else {
		newline := stream.newline
		if stream.translate {
			newline = "\n"
		}
		if index := strings.Index(text, newline); index >= 0 {
			return index + len(newline)
		}
	}
	return len(text)
}
