package runtime

import "strings"

type stringIOValue struct {
	contents []rune
	position int
	closed   bool
}

func (*stringIOValue) TypeName() string { return "StringIO" }
func (*stringIOValue) Repr() string     { return "<_io.StringIO object>" }
func (*stringIOValue) isValue()         {}

// attribute exposes the state and bound operations of a mutable text buffer.
func (stream *stringIOValue) attribute(name string) (Value, bool) {
	switch name {
	case "closed":
		return pythonBool(stream.closed), true
	case "write":
		return nativeFunctionNamed("StringIO.write", 1, 1, stream.write), true
	case "getvalue":
		return nativeFunctionNamed("StringIO.getvalue", 0, 0, stream.getvalue), true
	case "read":
		return nativeFunctionNamed("StringIO.read", 0, 1, stream.read), true
	case "readline":
		return nativeFunctionNamed("StringIO.readline", 0, 1, stream.readline), true
	case "readlines":
		return nativeFunctionNamed("StringIO.readlines", 0, 1, stream.readlines), true
	case "__iter__":
		return nativeFunctionNamed("StringIO.__iter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) { return stream, nil, nil }), true
	case "__next__":
		return nativeFunctionNamed("StringIO.__next__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				value, available, exception := stream.next()
				if exception != nil {
					return nil, exception, nil
				}
				if !available {
					return nil, newException("StopIteration", ""), nil
				}
				return value, nil, nil
			}), true
	case "flush":
		return nativeFunctionNamed("StringIO.flush", 0, 0, stream.flush), true
	case "isatty":
		return nativeFunctionNamed("StringIO.isatty", 0, 0, stream.isatty), true
	case "tell":
		return nativeFunctionNamed("StringIO.tell", 0, 0, stream.tell), true
	case "seek":
		return nativeFunctionNamed("StringIO.seek", 1, 2, stream.seek), true
	case "truncate":
		return nativeFunctionNamed("StringIO.truncate", 0, 1, stream.truncate), true
	case "close":
		return nativeFunctionNamed("StringIO.close", 0, 0, stream.close), true
	default:
		return nil, false
	}
}

// read returns the requested text window and advances the stream position.
func (stream *stringIOValue) read(_ *frame, arguments []Value) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	end := len(stream.contents)
	if len(arguments) == 1 {
		size, ok := integerOperand(arguments[0])
		if !ok || !size.IsInt64() {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		if size.Int64() >= 0 && int64(stream.position)+size.Int64() < int64(end) {
			end = stream.position + int(size.Int64())
		}
	}
	value := string(stream.contents[stream.position:end])
	stream.position = end
	return &stringValue{value: value}, nil, nil
}

// readline returns one bounded logical text line and advances the stream.
func (stream *stringIOValue) readline(_ *frame, arguments []Value) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	end := len(stream.contents)
	for index := stream.position; index < len(stream.contents); index++ {
		if stream.contents[index] == '\n' {
			end = index + 1
			break
		}
	}
	if len(arguments) == 1 {
		size, ok := integerOperand(arguments[0])
		if !ok || !size.IsInt64() {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		if size.Int64() >= 0 && int64(stream.position)+size.Int64() < int64(end) {
			end = stream.position + int(size.Int64())
		}
	}
	value := string(stream.contents[stream.position:end])
	stream.position = end
	return &stringValue{value: value}, nil, nil
}

func (stream *stringIOValue) readlines(caller *frame, arguments []Value) (Value, *Exception, error) {
	var lines []Value
	for stream.position < len(stream.contents) {
		line, exception, err := stream.readline(caller, nil)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		lines = append(lines, line)
	}
	return &listValue{elements: lines}, nil, nil
}

func (stream *stringIOValue) next() (Value, bool, *Exception) {
	if exception := stream.closedError(); exception != nil {
		return nil, false, exception
	}
	if stream.position >= len(stream.contents) {
		return nil, false, nil
	}
	value, exception, _ := stream.readline(nil, nil)
	return value, exception == nil, exception
}

func newStringIO(_ *frame, arguments []Value) (Value, *Exception, error) {
	stream := &stringIOValue{}
	if len(arguments) == 0 {
		return stream, nil, nil
	}
	if arguments[0] == None {
		return stream, nil, nil
	}
	initial, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException(
			"TypeError",
			"initial_value must be str or None, not "+arguments[0].TypeName(),
		), nil
	}
	stream.contents = []rune(initial.value)
	return stream, nil, nil
}

func (stream *stringIOValue) write(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException(
			"TypeError",
			"string argument expected, got '"+arguments[0].TypeName()+"'",
		), nil
	}
	written := []rune(text.value)
	end := stream.position + len(written)
	if end > len(stream.contents) {
		stream.contents = append(stream.contents, make([]rune, end-len(stream.contents))...)
	}
	copy(stream.contents[stream.position:end], written)
	stream.position = end
	return newInt64(int64(len(written))), nil, nil
}

func (stream *stringIOValue) getvalue(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	return &stringValue{value: string(stream.contents)}, nil, nil
}

func (stream *stringIOValue) flush(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	return None, nil, nil
}

func (stream *stringIOValue) isatty(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	return falseSingleton, nil, nil
}

func (stream *stringIOValue) tell(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	return newInt64(int64(stream.position)), nil, nil
}

// seek computes one absolute rune position from the selected origin without
// changing buffer contents.
func (stream *stringIOValue) seek(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	offset, ok := integerOperand(arguments[0])
	if !ok || !offset.IsInt64() {
		return nil, newException("TypeError", "an integer is required"), nil
	}
	whence := int64(0)
	if len(arguments) == 2 {
		value, valid := integerOperand(arguments[1])
		if !valid || !value.IsInt64() {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		whence = value.Int64()
	}
	position := offset.Int64()
	switch whence {
	case 0:
	case 1:
		position += int64(stream.position)
	case 2:
		position += int64(len(stream.contents))
	default:
		return nil, newException("ValueError", "Invalid whence"), nil
	}
	if position < 0 || int64(int(position)) != position {
		return nil, newException("ValueError", "Negative seek position"), nil
	}
	stream.position = int(position)
	return newInt64(position), nil, nil
}

// truncate validates an optional size and shortens the buffer when needed.
func (stream *stringIOValue) truncate(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	if exception := stream.closedError(); exception != nil {
		return nil, exception, nil
	}
	size := int64(stream.position)
	if len(arguments) == 1 {
		value, ok := integerOperand(arguments[0])
		if !ok || !value.IsInt64() {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		size = value.Int64()
	}
	if size < 0 || int64(int(size)) != size {
		return nil, newException("ValueError", "Negative size value"), nil
	}
	if int(size) < len(stream.contents) {
		stream.contents = stream.contents[:int(size)]
	}
	return newInt64(size), nil, nil
}

func (stream *stringIOValue) close(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	stream.closed = true
	stream.contents = nil
	return None, nil, nil
}

func (stream *stringIOValue) closedError() *Exception {
	if !stream.closed {
		return nil
	}
	return newException("ValueError", "I/O operation on closed file")
}

type bytesIOValue struct {
	contents string
	position int
}

func (*bytesIOValue) TypeName() string { return "BytesIO" }
func (*bytesIOValue) Repr() string     { return "<_io.BytesIO object>" }
func (*bytesIOValue) isValue()         {}

func newBytesIO(_ *frame, arguments []Value) (Value, *Exception, error) {
	stream := &bytesIOValue{}
	if len(arguments) == 0 {
		return stream, nil, nil
	}
	value, ok := arguments[0].(*bytesValue)
	if !ok {
		return nil, newException("TypeError", "initial_value must be bytes"), nil
	}
	stream.contents = value.value
	return stream, nil, nil
}

// attribute exposes the byte-buffer methods needed by unittest streams and
// iterator consumers.
func (stream *bytesIOValue) attribute(name string) (Value, bool) {
	switch name {
	case "read":
		return nativeFunctionNamed("BytesIO.read", 0, 1, stream.read), true
	case "readline":
		return nativeFunctionNamed("BytesIO.readline", 0, 1, stream.readline), true
	case "readlines":
		return nativeFunctionNamed("BytesIO.readlines", 0, 1, stream.readlines), true
	case "seek":
		return nativeFunctionNamed("BytesIO.seek", 1, 2,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				position, ok := integerOperand(arguments[0])
				if !ok || !position.IsInt64() || position.Sign() < 0 {
					return nil, newException("ValueError", "negative seek value"), nil
				}
				stream.position = int(position.Int64())
				return newInt64(position.Int64()), nil, nil
			}), true
	case "__iter__":
		return nativeFunctionNamed("BytesIO.__iter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) { return stream, nil, nil }), true
	case "__next__":
		return nativeFunctionNamed("BytesIO.__next__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				value, available, exception := stream.next()
				if exception != nil {
					return nil, exception, nil
				}
				if !available {
					return nil, newException("StopIteration", ""), nil
				}
				return value, nil, nil
			}), true
	default:
		return nil, false
	}
}

// read returns the requested byte window and advances the stream position.
func (stream *bytesIOValue) read(_ *frame, arguments []Value) (Value, *Exception, error) {
	end := len(stream.contents)
	if len(arguments) == 1 {
		size, ok := integerOperand(arguments[0])
		if !ok || !size.IsInt64() {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		if size.Int64() >= 0 && int64(stream.position)+size.Int64() < int64(end) {
			end = stream.position + int(size.Int64())
		}
	}
	value := stream.contents[stream.position:end]
	stream.position = end
	return &bytesValue{value: value}, nil, nil
}

// readline returns one bounded byte line and advances the stream position.
func (stream *bytesIOValue) readline(_ *frame, arguments []Value) (Value, *Exception, error) {
	end := len(stream.contents)
	if index := strings.IndexByte(stream.contents[stream.position:], '\n'); index >= 0 {
		end = stream.position + index + 1
	}
	if len(arguments) == 1 {
		size, ok := integerOperand(arguments[0])
		if !ok || !size.IsInt64() {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		if size.Int64() >= 0 && int64(stream.position)+size.Int64() < int64(end) {
			end = stream.position + int(size.Int64())
		}
	}
	value := stream.contents[stream.position:end]
	stream.position = end
	return &bytesValue{value: value}, nil, nil
}

func (stream *bytesIOValue) readlines(caller *frame, _ []Value) (Value, *Exception, error) {
	var lines []Value
	for stream.position < len(stream.contents) {
		line, exception, err := stream.readline(caller, nil)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		lines = append(lines, line)
	}
	return &listValue{elements: lines}, nil, nil
}

func (stream *bytesIOValue) next() (Value, bool, *Exception) {
	if stream.position >= len(stream.contents) {
		return nil, false, nil
	}
	value, exception, _ := stream.readline(nil, nil)
	if exception != nil {
		return nil, false, exception
	}
	return value, true, nil
}
