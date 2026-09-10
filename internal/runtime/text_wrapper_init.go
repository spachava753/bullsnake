package runtime

import "strings"

// initializeTextStream validates explicit codec/newline options, then evaluates
// Python truth flags and binary capabilities in their constructor order.
func initializeTextStream(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	values, exception := bindIOArguments("TextIOWrapper", arguments, keywords, []string{"buffer", "encoding", "errors", "newline", "line_buffering", "write_through"}, []Value{nil, None, None, None, falseSingleton, falseSingleton})
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	stream := &textWrapper{buffer: values[0], encoding: "utf-8", errors: "strict"}
	for field, target := range []*string{&stream.encoding, &stream.errors} {
		index := field + 1
		if values[index] == None {
			continue
		}
		value, ok := values[index].(*stringValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "encoding and errors must be str or None")), nil
		}
		if strings.ContainsRune(value.value, 0) {
			return raiseOutcome(newException("ValueError", "embedded null character")), nil
		}
		*target = value.value
	}
	stream.codec, exception = textCodec(stream.encoding)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if values[3] == None {
		stream.universal, stream.translate = true, true
	} else {
		newline, ok := values[3].(*stringValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "newline must be str or None")), nil
		}
		switch newline.value {
		case "", "\n", "\r", "\r\n":
			stream.newline = newline.value
			stream.universal = newline.value == ""
		default:
			return raiseOutcome(newException("ValueError", "illegal newline value: "+newline.Repr())), nil
		}
	}
	call := &textInitialization{self: self, stream: stream, values: values, instruction: instruction}
	return call.advance(caller)
}

type textInitialization struct {
	self              *instanceValue
	stream            *textWrapper
	values            []Value
	instruction, step int
}

// advance evaluates the two truth flags and three capability methods through
// VM continuations, installing state only after all constructor callbacks pass.
func (call *textInitialization) advance(caller *frame) (instructionOutcome, error) {
	if call.step == 5 {
		call.self.io.wrapper = call.stream
		return pushOutcome(caller, call.instruction, None)
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		if call.step < 2 {
			return executeBuiltinBool(caller, call.instruction, len(caller.stack), []Value{call.values[call.step+4]}, nil)
		}
		return executeMethodCall(caller, call.instruction, call.stream.buffer, []string{"readable", "writable", "seekable"}[call.step-2], nil)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, call.instruction, func() (instructionOutcome, error) {
			return executeBuiltinBool(current, call.instruction, len(current.stack), []Value{result}, nil)
		}, func(resumed *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			fields := []*bool{&call.stream.lineBuffering, &call.stream.writeThrough, &call.stream.readable, &call.stream.writable, &call.stream.seekable}
			*fields[call.step] = result == trueSingleton
			call.step++
			return call.advance(resumed)
		})
	})
}
