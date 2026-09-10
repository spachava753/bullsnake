package runtime

import "math/big"

// executeViewSubscription applies byte indexing and one-dimensional slices.
// Slice writes copy their input first, preserving overlapping assignments.
func executeViewSubscription(caller *frame, instruction int, self *instanceValue, arguments []Value, store bool) (instructionOutcome, error) {
	view := self.io.view
	if store && view.readonly {
		return raiseOutcome(newException("TypeError", "cannot modify read-only memory")), nil
	}
	if descriptor, ok := arguments[0].(*sliceValue); ok {
		start, stop, step, exception := normalizeSlice(descriptor, view.length)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		positions := binarySlicePositions(start, stop, step)
		if store {
			data, exception := binaryCopyData(arguments[1])
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if len(data) != len(positions) {
				return raiseOutcome(newException("ValueError", "memoryview assignment: lvalue and rvalue have different structures")), nil
			}
			data = append([]byte(nil), data...)
			for index, position := range positions {
				view.buffer.data[view.offset+position*view.stride] = data[index]
			}
			return pushOutcome(caller, instruction, None)
		}
		copy := *view
		copy.length = len(positions)
		if len(positions) != 0 {
			copy.offset += positions[0] * view.stride
		}
		stride := new(big.Int).Mul(&step, big.NewInt(int64(view.stride)))
		if !stride.IsInt64() || int64(int(stride.Int64())) != stride.Int64() {
			return raiseOutcome(newException("OverflowError", "memoryview stride is too large")), nil
		}
		copy.stride = int(stride.Int64())
		return pushOutcome(caller, instruction, newViewInstance(self.class, copy))
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, arguments[0])
	}, func(current *frame, key Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if view.released {
			return raiseOutcome(releasedViewError()), nil
		}
		index, exception := normalizeTextIndex(key, view.length, false)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if !store {
			return pushOutcome(current, instruction, newByteInteger(view.buffer.data[view.offset+index*view.stride]))
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeIOIndex(current, instruction, arguments[1])
		}, func(resumed *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if view.released {
				return raiseOutcome(releasedViewError()), nil
			}
			integer := result.(*intValue).value.Int64()
			if integer < 0 || integer > 255 {
				return raiseOutcome(newException("ValueError", "memoryview: invalid value for format 'B'")), nil
			}
			view.buffer.data[view.offset+index*view.stride] = byte(integer)
			return pushOutcome(resumed, instruction, None)
		})
	})
}
