package runtime

import (
	"math/big"
	"strconv"
)

type byteBuffer struct {
	data []byte
}

type bytearrayValue struct {
	buffer *byteBuffer
}

func (*bytearrayValue) TypeName() string { return "bytearray" }
func (value *bytearrayValue) Repr() string {
	return "bytearray(" + (&bytesValue{value: string(value.buffer.data)}).Repr() + ")"
}
func (*bytearrayValue) isValue() {}

// binaryData returns contiguous supported buffer contents. Callers copy before
// retaining bytes across mutation or exposing an immutable Python snapshot.
func binaryData(value Value) ([]byte, *Exception) {
	switch value := value.(type) {
	case *bytesValue:
		return []byte(value.value), nil
	case *bytearrayValue:
		return value.buffer.data, nil
	default:
		return nil, newException("TypeError", "a bytes-like object is required, not '"+value.TypeName()+"'")
	}
}

// newBinaryValue constructs an empty buffer, a zero-filled size, or a copy of
// supported binary data. Immutable bytes never share mutable buffer storage.
func newBinaryValue(arguments []Value, keywords *dictValue, mutable bool) (Value, *Exception) {
	name := "bytes"
	if mutable {
		name = "bytearray"
	}
	if exception := checkNativeArguments(name, arguments, keywords, 0, 1); exception != nil {
		return nil, exception
	}
	var data []byte
	if len(arguments) == 1 {
		if size, ok := integerOperand(arguments[0]); ok {
			if !size.IsInt64() || int64(int(size.Int64())) != size.Int64() {
				return nil, newException("OverflowError", "cannot fit 'int' into an index-sized integer")
			}
			if size.Sign() < 0 {
				return nil, newException("ValueError", "negative count")
			}
			data = make([]byte, int(size.Int64()))
		} else {
			var exception *Exception
			data, exception = binaryData(arguments[0])
			if exception != nil {
				return nil, exception
			}
		}
	}
	if mutable {
		return &bytearrayValue{buffer: &byteBuffer{data: append([]byte(nil), data...)}}, nil
	}
	return &bytesValue{value: string(data)}, nil
}

func (value *bytearrayValue) subscript(key Value) (Value, *Exception) {
	result, exception := subscriptBytes(&bytesValue{value: string(value.buffer.data)}, key)
	if exception != nil {
		return nil, exception
	}
	if bytes, sliced := result.(*bytesValue); sliced {
		return &bytearrayValue{buffer: &byteBuffer{data: []byte(bytes.value)}}, nil
	}
	return result, nil
}

// assign validates the complete replacement before mutating. Contiguous slices
// may resize; extended slices require exactly one byte per selected position.
func (value *bytearrayValue) assign(key, replacement Value, remove bool) *Exception {
	data := value.buffer.data
	if descriptor, sliced := key.(*sliceValue); sliced {
		start, stop, step, exception := normalizeSlice(descriptor, len(data))
		if exception != nil {
			return exception
		}
		var insert []byte
		if !remove {
			insert, exception = binaryData(replacement)
			if exception != nil {
				return exception
			}
			insert = append([]byte(nil), insert...)
		}
		if step.IsInt64() && step.Int64() == 1 {
			if stop < start {
				stop = start
			}
			next := make([]byte, 0, len(data)-(stop-start)+len(insert))
			next = append(next, data[:start]...)
			next = append(next, insert...)
			value.buffer.data = append(next, data[stop:]...)
			return nil
		}
		positions := binarySlicePositions(start, stop, step)
		if !remove && len(insert) != len(positions) {
			return newException("ValueError", "attempt to assign bytes of size "+strconv.Itoa(len(insert))+" to extended slice of size "+strconv.Itoa(len(positions)))
		}
		if remove {
			selected := make([]bool, len(data))
			for _, index := range positions {
				selected[index] = true
			}
			result := make([]byte, 0, len(data)-len(positions))
			for index, item := range data {
				if !selected[index] {
					result = append(result, item)
				}
			}
			value.buffer.data = result
		} else {
			for index, position := range positions {
				data[position] = insert[index]
			}
		}
		return nil
	}
	index, exception := normalizeTextIndex(key, len(data), false)
	if exception != nil {
		return exception
	}
	if remove {
		value.buffer.data = append(data[:index], data[index+1:]...)
		return nil
	}
	integer, ok := integerOperand(replacement)
	if !ok {
		return newException("TypeError", "'"+replacement.TypeName()+"' object cannot be interpreted as an integer")
	}
	if !integer.IsUint64() || integer.Uint64() > 255 {
		return newException("ValueError", "byte must be in range(0, 256)")
	}
	data[index] = byte(integer.Uint64())
	return nil
}

func binarySlicePositions(start, stop int, step big.Int) []int {
	var result []int
	current, boundary := big.NewInt(int64(start)), big.NewInt(int64(stop))
	for (step.Sign() > 0 && current.Cmp(boundary) < 0) || (step.Sign() < 0 && current.Cmp(boundary) > 0) {
		result = append(result, int(current.Int64()))
		current.Add(current, &step)
	}
	return result
}
