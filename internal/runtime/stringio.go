package runtime

import "strings"

var stringIOType = nativeType("_io", "StringIO")

// stringIOValue owns its text independently of host streams. Positions count
// Python code points, including surrogate code points encoded internally as WTF-8.
type stringIOValue struct {
	value     string
	position  int
	closed    bool
	newline   string
	universal bool
	translate bool
	seen      uint8
}

func (*stringIOValue) TypeName() string { return "_io.StringIO" }
func (*stringIOValue) Repr() string     { return "<_io.StringIO object>" }
func (*stringIOValue) isValue()         {}

// newStringIO validates newline before initial_value, then applies the write
// translation rules to initial text and resets the cursor to zero.
func newStringIO(arguments []Value, keywords *dictValue) (Value, *Exception) {
	values, exception := bindStringIO(arguments, keywords)
	if exception != nil {
		return nil, exception
	}
	stream := &stringIOValue{}
	if values[1] == None {
		stream.universal, stream.translate = true, true
	} else {
		newline, ok := values[1].(*stringValue)
		if !ok {
			return nil, newException("TypeError", "newline must be str or None, not "+values[1].TypeName())
		}
		switch newline.value {
		case "", "\n", "\r", "\r\n":
			stream.newline = newline.value
			stream.universal = newline.value == ""
		default:
			return nil, newException("ValueError", "illegal newline value: "+newline.Repr())
		}
	}
	if values[0] != None {
		initial, ok := values[0].(*stringValue)
		if !ok {
			return nil, newException("TypeError", "initial_value must be str or None, not "+values[0].TypeName())
		}
		stream.value = stream.translateNewlines(initial.value)
	}
	return stream, nil
}

// bindStringIO combines positional and named constructor values, rejecting
// unknown keywords and duplicate assignments before validating their types.
func bindStringIO(arguments []Value, keywords *dictValue) ([2]Value, *Exception) {
	values := [2]Value{None, &stringValue{value: "\n"}}
	if len(arguments) > len(values) {
		return values, newException("TypeError", "StringIO() takes at most 2 arguments")
	}
	copy(values[:], arguments)
	if keywords == nil {
		return values, nil
	}
	for _, entry := range keywords.entries {
		name, ok := entry.key.(*stringValue)
		if !ok {
			return values, newException("TypeError", "keywords must be strings")
		}
		index := 0
		switch name.value {
		case "initial_value":
		case "newline":
			index = 1
		default:
			return values, newException("TypeError", "'"+name.value+"' is an invalid keyword argument for StringIO()")
		}
		if index < len(arguments) {
			return values, newException("TypeError", "StringIO() got multiple values for argument '"+name.value+"'")
		}
		values[index] = entry.value
	}
	return values, nil
}

func executeStringIOAttributeLoad(frame *frame, instruction int, stream *stringIOValue, name string) (instructionOutcome, error) {
	switch name {
	case "closed":
		return pushOutcome(frame, instruction, booleanValue(stream.closed))
	case "newlines":
		if stream.closed {
			return raiseOutcome(newException("ValueError", "I/O operation on closed file")), nil
		}
		return pushOutcome(frame, instruction, stream.newlines())
	case "write", "getvalue", "flush", "close", "isatty", "__enter__", "__exit__":
		return pushOutcome(frame, instruction, &builtinFunctionValue{name: name, call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
			return stream.call(name, arguments, keywords)
		}})
	default:
		return raiseOutcome(newException("AttributeError", "'_io.StringIO' object has no attribute '"+name+"'")), nil
	}
}

// call checks method arguments before closed state. Close releases only this
// buffer, while all other operations require an open stream.
func (stream *stringIOValue) call(name string, arguments []Value, keywords *dictValue) (Value, *Exception) {
	minimum, maximum := streamMethodArity(name)
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return nil, exception
	}
	if name == "write" {
		if _, ok := arguments[0].(*stringValue); !ok {
			return nil, newException("TypeError", "string argument expected, got '"+arguments[0].TypeName()+"'")
		}
	}
	if name == "close" || name == "__exit__" {
		stream.closed, stream.value = true, ""
		return None, nil
	}
	if stream.closed {
		return nil, newException("ValueError", "I/O operation on closed file")
	}
	switch name {
	case "write":
		return stream.write(arguments[0].(*stringValue).value), nil
	case "getvalue":
		return &stringValue{value: stream.value}, nil
	case "isatty":
		return falseSingleton, nil
	case "__enter__":
		return stream, nil
	default:
		return None, nil
	}
}

// write overwrites at character boundaries without mutating previous getvalue
// results. Its return count measures input before newline translation.
func (stream *stringIOValue) write(text string) Value {
	count := len(stringCodepointOffsets(text)) - 1
	if count == 0 {
		return integerFromInt64(0)
	}
	text = stream.translateNewlines(text)
	offsets := stringCodepointOffsets(stream.value)
	end := stream.position + len(stringCodepointOffsets(text)) - 1
	suffix := ""
	if end < len(offsets)-1 {
		suffix = stream.value[offsets[end]:]
	}
	stream.value = stream.value[:offsets[stream.position]] + text + suffix
	stream.position = end
	return integerFromInt64(int64(count))
}

// translateNewlines finishes universal decoding on every write, matching
// StringIO's final=True decoder calls rather than carrying pending CR state.
func (stream *stringIOValue) translateNewlines(text string) string {
	if !stream.universal {
		if stream.newline == "\r" || stream.newline == "\r\n" {
			return strings.ReplaceAll(text, "\n", stream.newline)
		}
		return text
	}
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\r':
			if index+1 < len(text) && text[index+1] == '\n' {
				stream.seen |= 4
				index++
			} else {
				stream.seen |= 1
			}
		case '\n':
			stream.seen |= 2
		}
	}
	if stream.translate {
		return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	}
	return text
}

func (stream *stringIOValue) newlines() Value {
	var values []Value
	for index, newline := range []string{"\r", "\n", "\r\n"} {
		if stream.seen&(1<<index) != 0 {
			values = append(values, &stringValue{value: newline})
		}
	}
	if len(values) == 0 {
		return None
	}
	if len(values) == 1 {
		return values[0]
	}
	return &tupleValue{elements: values}
}
