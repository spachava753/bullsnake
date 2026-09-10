package runtime

import "strconv"

// executeBytesIOPosition preserves each method's conversion order: readlines
// accepts native ints only, and truncate checks exports before index conversion.
func executeBytesIOPosition(caller *frame, instruction int, self *instanceValue, name string, arguments []Value) (instructionOutcome, error) {
	stream := self.io.binary
	size := -1
	if name == "truncate" || name == "readlines" {
		if stream.buffer == nil {
			return raiseOutcome(bytesIOClosedError()), nil
		}
		if name == "truncate" {
			if stream.buffer.hasViews() {
				return raiseOutcome(bufferExportError()), nil
			}
			size = stream.position
		} else if len(arguments) == 1 && arguments[0] != None {
			if _, ok := integerOperand(arguments[0]); !ok {
				return raiseOutcome(newException("TypeError", "integer argument expected, got '"+arguments[0].TypeName()+"'")), nil
			}
		}
	}
	if len(arguments) == 0 || (name != "seek" && arguments[0] == None) {
		return stream.finishPosition(caller, instruction, name, size, 0)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, arguments[0])
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		size := int(result.(*intValue).value.Int64())
		if len(arguments) < 2 {
			return stream.finishPosition(current, instruction, name, size, 0)
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeIOIndex(current, instruction, arguments[1])
		}, func(resumed *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			whence := result.(*intValue).value.Int64()
			if whence < -1<<31 || whence > 1<<31-1 {
				return raiseOutcome(newException("OverflowError", "Python int too large to convert to C int")), nil
			}
			return stream.finishPosition(resumed, instruction, name, size, int(whence))
		})
	})
}

// finishPosition checks state after index callbacks and applies byte offsets.
// Relative seeks clamp at zero; truncation never extends the logical contents.
func (stream *bytesIOState) finishPosition(caller *frame, instruction int, name string, size, whence int) (instructionOutcome, error) {
	if stream.buffer == nil {
		return raiseOutcome(bytesIOClosedError()), nil
	}
	switch name {
	case "read", "read1", "readline":
		return pushOutcome(caller, instruction, stream.read(size, name == "readline"))
	case "readlines":
		return pushOutcome(caller, instruction, stream.readlines(size))
	case "truncate":
		if stream.buffer.hasViews() {
			return raiseOutcome(bufferExportError()), nil
		}
		if size < 0 {
			return raiseOutcome(newException("ValueError", "negative size value "+strconv.Itoa(size))), nil
		}
		if size < stream.size {
			stream.size = size
			stream.buffer.data = stream.buffer.data[:size]
		}
	case "seek":
		if whence == 0 && size < 0 {
			return raiseOutcome(newException("ValueError", "negative seek value "+strconv.Itoa(size))), nil
		}
		relative := 0
		switch whence {
		case 0:
		case 1:
			relative = stream.position
		case 2:
			relative = stream.size
		default:
			return raiseOutcome(newException("ValueError", "invalid whence ("+strconv.Itoa(whence)+", should be 0, 1 or 2)")), nil
		}
		if size > int(^uint(0)>>1)-relative {
			return raiseOutcome(newException("OverflowError", "new position too large")), nil
		}
		size = max(0, size+relative)
		stream.position = size
	}
	return pushOutcome(caller, instruction, integerFromInt64(int64(size)))
}
