package runtime

import "strings"

type textWrapper struct {
	input                          []byte
	decoded                        []textUnit
	decodeOffset, skipped          int
	pendingCR                      *textUnit
	seen                           uint8
	hasRead1, readStarted, telling bool
	buffer                         Value
	encoding, codec, errors        string
	newline                        string
	universal, translate           bool
	lineBuffering, writeThrough    bool
	readable, writable, seekable   bool
	pending                        []byte
}

func initializeTextWrapper(module *Module) {
	class := newIOClass("TextIOWrapper", module.globals.values["_TextIOBase"].(*typeValue))
	module.globals.values[class.name] = class
	for _, name := range []string{"__init__", "reconfigure", "read", "readline", "__next__", "seek", "tell", "truncate", "write", "flush", "close", "detach", "readable", "writable", "seekable", "fileno", "isatty"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			if name == "__next__" {
				return executeTextNext(caller, instruction, self, class, arguments, keywords)
			}
			return executeTextWrapper(caller, instruction, self, name, arguments, keywords)
		}))
	}
	for _, name := range []string{"buffer", "closed", "name", "encoding", "errors", "line_buffering", "write_through", "newlines"} {
		class.setAttribute(name, &propertyValue{doc: None, getter: ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
			return textWrapperAttribute(caller, instruction, self.io.wrapper, name)
		})})
	}
}

// textWrapperAttribute exposes codec configuration and delegates live closed/name
// metadata to the supplied binary object. Detached buffers remain inspectable.
func textWrapperAttribute(caller *frame, instruction int, stream *textWrapper, name string) (instructionOutcome, error) {
	if stream == nil {
		return raiseOutcome(newException("ValueError", "I/O operation on uninitialized object")), nil
	}
	var value Value
	switch name {
	case "buffer":
		value = stream.buffer
		if value == nil {
			value = None
		}
	case "encoding":
		value = &stringValue{value: stream.encoding}
	case "errors":
		value = &stringValue{value: stream.errors}
	case "line_buffering":
		value = booleanValue(stream.lineBuffering)
	case "write_through":
		value = booleanValue(stream.writeThrough)
	case "newlines":
		state := stringIOValue{seen: stream.seen}
		value = state.newlines()
	default:
		if stream.buffer == nil {
			return raiseOutcome(newException("ValueError", "underlying buffer has been detached")), nil
		}
		return executeDynamicAttributeLoad(caller, instruction, stream.buffer, name)
	}
	return pushOutcome(caller, instruction, value)
}

// executeTextWrapper validates text before stream operations, buffers encoded
// bytes, and delegates binary callbacks without acquiring any host capability.
func executeTextWrapper(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if name == "__init__" {
		return initializeTextStream(caller, instruction, self, arguments, keywords)
	}
	if name == "read" || name == "readline" {
		return executeTextRead(caller, instruction, self, name, arguments, keywords)
	}
	if name == "reconfigure" {
		return reconfigureTextWrapper(caller, instruction, self, arguments, keywords)
	}
	minimum, maximum := streamMethodArity(name)
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	if name == "write" {
		if _, ok := arguments[0].(*stringValue); !ok {
			return raiseOutcome(newException("TypeError", "write() argument must be str, not "+arguments[0].TypeName())), nil
		}
	}
	stream := self.io.wrapper
	if stream == nil {
		return raiseOutcome(newException("ValueError", "I/O operation on uninitialized object")), nil
	}
	if stream.buffer == nil {
		return raiseOutcome(newException("ValueError", "underlying buffer has been detached")), nil
	}
	if name == "seek" || name == "tell" || name == "truncate" {
		return executeTextPosition(caller, instruction, self, name, arguments)
	}
	if name == "readable" || name == "writable" || name == "seekable" || name == "isatty" || name == "fileno" {
		return executeMethodCall(caller, instruction, stream.buffer, name, nil)
	}
	if name == "close" {
		return closeBuffered(caller, instruction, self, &bufferedStream{raw: stream.buffer})
	}
	if name == "detach" {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) { return executeMethodCall(caller, instruction, self, "flush", nil) }, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			buffer := stream.buffer
			stream.buffer = nil
			return pushOutcome(current, instruction, buffer)
		})
	}
	return withIOOpen(caller, instruction, self, func(current *frame) (instructionOutcome, error) {
		switch name {
		case "write":
			return writeTextWrapper(current, instruction, stream, arguments[0].(*stringValue).value)
		case "flush":
			stream.telling = stream.seekable
			return continueNativeOperation(current, instruction, func() (instructionOutcome, error) { return flushTextBytes(current, instruction, stream) }, func(resumed *frame, _ Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return executeMethodCall(resumed, instruction, stream.buffer, "flush", nil)
			})
		default:
			return executeMethodCall(current, instruction, stream.buffer, name, arguments)
		}
	})
}

// writeTextWrapper counts original characters, then translates and encodes.
// Write-through drains text bytes; line buffering additionally flushes the buffer.
func writeTextWrapper(caller *frame, instruction int, stream *textWrapper, text string) (instructionOutcome, error) {
	if !stream.writable {
		return raiseOutcome(unsupportedStreamOperation("not writable")), nil
	}
	count := len(stringCodepointOffsets(text)) - 1
	line := stream.lineBuffering && strings.ContainsAny(text, "\r\n")
	if stream.newline == "\r" || stream.newline == "\r\n" {
		text = strings.ReplaceAll(text, "\n", stream.newline)
	}
	data, exception := encodeText(text, stream.codec, stream.errors)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	stream.pending = append(stream.pending, data...)
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		if stream.writeThrough || line || len(stream.pending) >= 8192 {
			return flushTextBytes(caller, instruction, stream)
		}
		return pushOutcome(caller, instruction, None)
	}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			if line {
				return executeMethodCall(current, instruction, stream.buffer, "flush", nil)
			}
			return pushOutcome(current, instruction, None)
		}, func(resumed *frame, _ Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			stream.resetInput()
			return pushOutcome(resumed, instruction, integerFromInt64(int64(count)))
		})
	})
}

// flushTextBytes clears pending bytes before the binary write, matching CPython
// when a failing callback leaves its partial output count unknown.
func flushTextBytes(caller *frame, instruction int, stream *textWrapper) (instructionOutcome, error) {
	if len(stream.pending) == 0 {
		return pushOutcome(caller, instruction, None)
	}
	data := &bytesValue{value: string(stream.pending)}
	stream.pending = nil
	return executeRetryIOCall(caller, instruction, stream.buffer, "write", []Value{data})
}
