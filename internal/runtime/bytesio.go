package runtime

import "bytes"

type bytesIOState struct {
	buffer   *byteBuffer
	position int
	size     int
}

// bytesIOBuffer retains the Python stream for exported views, independently of
// the buffer's weak export table. It grants no host or stream operations.
type bytesIOBuffer struct {
	stream *instanceValue
}

func (*bytesIOBuffer) TypeName() string { return "_io._BytesIOBuffer" }
func (*bytesIOBuffer) Repr() string     { return "<_io._BytesIOBuffer object>" }
func (*bytesIOBuffer) isValue()         {}

func bytesIOClosedError() *Exception {
	return newException("ValueError", "I/O operation on closed file.")
}

// write validates binary data before closed/export checks, then overwrites or
// extends the logical buffer with NUL padding for an earlier seek beyond EOF.
func (stream *bytesIOState) write(value Value) (Value, *Exception) {
	data, exception := binaryData(value)
	if exception != nil {
		return nil, exception
	}
	if stream.buffer == nil {
		return nil, bytesIOClosedError()
	}
	if stream.buffer.hasViews() {
		return nil, bufferExportError()
	}
	if len(data) == 0 {
		return integerFromInt64(0), nil
	}
	if stream.position > int(^uint(0)>>1)-len(data) {
		return nil, newException("OverflowError", "new buffer size too large")
	}
	end := stream.position + len(data)
	if end > len(stream.buffer.data) {
		stream.buffer.data = append(stream.buffer.data, make([]byte, end-len(stream.buffer.data))...)
	}
	if stream.position > stream.size {
		clear(stream.buffer.data[stream.size:stream.position])
	}
	copy(stream.buffer.data[stream.position:end], data)
	stream.position = end
	if end > stream.size {
		stream.size = end
	}
	return integerFromInt64(int64(len(data))), nil
}

// read bounds the request by the remaining bytes and optionally the next LF,
// leaving positions beyond EOF unchanged and returning an independent snapshot.
func (stream *bytesIOState) read(size int, line bool) *bytesValue {
	remaining := stream.size - stream.position
	if remaining <= 0 || size == 0 {
		return &bytesValue{}
	}
	if size < 0 || size > remaining {
		size = remaining
	}
	data := stream.buffer.data[stream.position : stream.position+size]
	if line {
		if end := bytes.IndexByte(data, '\n'); end >= 0 {
			data = data[:end+1]
		}
	}
	stream.position += len(data)
	return &bytesValue{value: string(data)}
}

func (stream *bytesIOState) readlines(hint int) Value {
	result := &listValue{}
	limited := hint > 0
	for {
		line := stream.read(-1, true)
		if line.value == "" {
			return result
		}
		result.elements = append(result.elements, line)
		if limited {
			if len(line.value) >= hint {
				return result
			}
			hint -= len(line.value)
		}
	}
}

// initializeBytesIO resets logical size and position even when an export later
// prevents reinitialization, matching CPython's constructor mutation ordering.
func initializeBytesIO(stream *bytesIOState, arguments []Value, keywords *dictValue) *Exception {
	if len(arguments) > 1 {
		return newException("TypeError", "BytesIO() takes at most 1 argument")
	}
	var value Value = None
	if len(arguments) == 1 {
		value = arguments[0]
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			if entry.key.(*stringValue).value != "initial_bytes" {
				return newException("TypeError", "invalid keyword argument for BytesIO()")
			}
			if len(arguments) != 0 {
				return newException("TypeError", "BytesIO() got multiple values for argument 'initial_bytes'")
			}
			value = entry.value
		}
	}
	stream.position, stream.size = 0, 0
	if stream.buffer != nil && stream.buffer.hasViews() {
		return bufferExportError()
	}
	if value == None {
		return nil
	}
	if data, ok := value.(*bytesValue); ok {
		stream.buffer = &byteBuffer{data: []byte(data.value)}
		stream.size = len(data.value)
		return nil
	}
	_, exception := stream.write(value)
	stream.position = 0
	return exception
}
