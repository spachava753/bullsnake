package runtime

import "strings"

type ioLinesCall struct {
	self        *instanceValue
	instruction int
	name        string
	stage       string
	input       Value
	iterator    Value
	sentinel    Value
	line        Value
	peek        Value
	limit       int
	limited     bool
	readSize    int
	buffer      strings.Builder
	lines       *listValue
	done        bool
}

// executeIOLines binds one positional size or iterable before starting a
// resumable line operation. Size conversion uses Python's index protocol.
func executeIOLines(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	minimum := 0
	if name == "writelines" {
		minimum = 1
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	call := &ioLinesCall{self: self, instruction: instruction, name: name, limit: -1, sentinel: &dictValue{}, lines: &listValue{}}
	if name == "writelines" {
		call.input, call.stage = arguments[0], "iter"
		return withIOOpen(caller, instruction, self, call.advance)
	}
	call.input, call.stage = self, "iter"
	if name == "readline" {
		call.stage = "lookup peek"
	}
	if len(arguments) == 0 || arguments[0] == None {
		return call.advance(caller)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, arguments[0])
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call.limit = int(result.(*intValue).value.Int64())
		call.limited = call.limit > 0
		return call.advance(current)
	})
}

// advance loops over immediately completed native operations. Python calls
// resume it from their frame continuation, so input size does not grow Go stack.
func (call *ioLinesCall) advance(caller *frame) (instructionOutcome, error) {
	for !call.done {
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := call.operation(caller)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception := call.accept(result, exception); exception != nil {
				return raiseOutcome(exception), nil
			}
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	switch call.name {
	case "readlines":
		return pushOutcome(caller, call.instruction, call.lines)
	case "readline":
		return pushOutcome(caller, call.instruction, &bytesValue{value: call.buffer.String()})
	default:
		return pushOutcome(caller, call.instruction, None)
	}
}

// operation performs exactly one callback or iterator request for the current
// stage; accept alone updates progress after that operation finishes.
func (call *ioLinesCall) operation(caller *frame) (instructionOutcome, error) {
	switch call.stage {
	case "iter":
		return executeIteratorLookup(caller, call.instruction, call.input)
	case "next":
		return executeBuiltinNext(caller, call.instruction, len(caller.stack), []Value{call.iterator, call.sentinel}, nil)
	case "length":
		return executeBuiltinLen(caller, call.instruction, len(caller.stack), []Value{call.line}, nil)
	case "write":
		return executeMethodCall(caller, call.instruction, call.self, "write", []Value{call.line})
	case "lookup peek":
		return executeDynamicAttributeLoad(caller, call.instruction, call.self, "peek")
	case "peek":
		return executeFunctionCall(caller, call.instruction, len(caller.stack), call.peek, []Value{integerFromInt64(1)}, nil)
	default:
		return executeMethodCall(caller, call.instruction, call.self, "read", []Value{integerFromInt64(int64(call.readSize))})
	}
}

// accept checks callback results and preserves partial work. EINTR retries only
// the interrupted read, peek, or write; iterator exceptions always propagate.
func (call *ioLinesCall) accept(result Value, exception *Exception) *Exception {
	if exception != nil {
		if call.stage == "lookup peek" && isAttributeError(exception) {
			call.startRead()
			return nil
		}
		if exception.fields != nil && (call.stage == "read" || call.stage == "peek" || call.stage == "write") {
			if number, found := exception.fields.get("errno"); found && exception.class.isSubclassOf(osErrorType) {
				if errno, ok := integerOperand(number); ok && errno.IsInt64() && errno.Int64() == 4 {
					return nil
				}
			}
		}
		return exception
	}
	switch call.stage {
	case "iter":
		call.iterator, call.stage = result, "next"
	case "next":
		if result == call.sentinel {
			call.done = true
		} else {
			call.line = result
			if call.name == "writelines" {
				call.stage = "write"
			} else {
				call.lines.elements = append(call.lines.elements, result)
				if call.limited {
					call.stage = "length"
				}
			}
		}
	case "length":
		length := &result.(*intValue).value
		if !length.IsInt64() || length.Int64() > int64(call.limit) {
			call.done = true
		} else {
			call.limit -= int(length.Int64())
			call.stage = "next"
		}
	case "write":
		call.stage = "next"
	case "lookup peek":
		call.peek = result
		call.startRead()
	case "peek":
		text, ok := result.(*bytesValue)
		if !ok {
			return newException("OSError", "peek() should have returned a bytes object, not '"+result.TypeName()+"'")
		}
		call.readSize = 1
		if len(text.value) > 0 {
			call.readSize = len(text.value)
			if index := strings.IndexByte(text.value, '\n'); index >= 0 {
				call.readSize = index + 1
			}
			if call.limit >= 0 && call.readSize > call.limit {
				call.readSize = call.limit
			}
		}
		call.stage = "read"
	case "read":
		text, ok := result.(*bytesValue)
		if !ok {
			return newException("OSError", "read() should have returned a bytes object, not '"+result.TypeName()+"'")
		}
		call.buffer.WriteString(text.value)
		if text.value == "" || strings.HasSuffix(text.value, "\n") {
			call.done = true
		} else {
			call.startRead()
		}
	}
	return nil
}

func (call *ioLinesCall) startRead() {
	if call.limit >= 0 && call.buffer.Len() >= call.limit {
		call.done = true
	} else if call.peek != nil {
		call.stage = "peek"
	} else {
		call.stage, call.readSize = "read", 1
	}
}
