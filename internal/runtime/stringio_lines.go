package runtime

func (stream *stringIOValue) next() (Value, bool, *Exception) {
	if stream.closed {
		return nil, false, newException("ValueError", "I/O operation on closed file")
	}
	line := stream.read(-1, true)
	return line, line.value != "", nil
}

// readlines stops after the first line that takes a positive hint over its
// character budget. A nonpositive hint consumes the remaining stream.
func (stream *stringIOValue) readlines(hint int) (Value, *Exception) {
	result := &listValue{}
	limited := hint > 0
	for {
		value, found, exception := stream.next()
		if exception != nil {
			return nil, exception
		}
		if !found {
			return result, nil
		}
		line := value.(*stringValue)
		result.elements = append(result.elements, line)
		if limited {
			count := len(stringCodepointOffsets(line.value)) - 1
			if count > hint {
				return result, nil
			}
			hint -= count
		}
	}
}

type stringIOWriteLines struct {
	stream      *stringIOValue
	iterator    Value
	instruction int
	sentinel    Value
}

func (stream *stringIOValue) executeWriteLines(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	if exception := checkNativeArguments("writelines", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	if stream.closed {
		return raiseOutcome(newException("ValueError", "I/O operation on closed file")), nil
	}
	call := &stringIOWriteLines{stream: stream, instruction: instruction, sentinel: &dictValue{}}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIteratorLookup(caller, instruction, arguments[0])
	}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call.iterator = iterator
		return call.next(current)
	})
}

// next drains native iterators without recursive Go calls and suspends when
// Python iteration needs a frame. Each write finishes before requesting a line.
func (call *stringIOWriteLines) next(caller *frame) (instructionOutcome, error) {
	if iterator, ok := call.iterator.(valueIterator); ok {
		for {
			line, found, exception := iterator.next()
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !found {
				return pushOutcome(caller, call.instruction, None)
			}
			if _, exception := call.stream.call("write", []Value{line}, nil); exception != nil {
				return raiseOutcome(exception), nil
			}
		}
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeBuiltinNext(caller, call.instruction, len(caller.stack), []Value{call.iterator, call.sentinel}, nil)
	}, func(current *frame, line Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if line == call.sentinel {
			return pushOutcome(current, call.instruction, None)
		}
		if _, exception := call.stream.call("write", []Value{line}, nil); exception != nil {
			return raiseOutcome(exception), nil
		}
		return call.next(current)
	})
}
