package runtime

import (
	"math/big"
	"strings"
)

func executeTextSubscript(container, index Value) (Value, *Exception) {
	switch container := container.(type) {
	case *stringValue:
		return subscriptString(container, index)
	case *bytesValue:
		return subscriptBytes(container, index)
	default:
		panic("executeTextSubscript called with non-text container")
	}
}

func subscriptString(value *stringValue, index Value) (Value, *Exception) {
	offsets := stringCodepointOffsets(value.value)
	if descriptor, ok := index.(*sliceValue); ok {
		selected, complete, exception := sliceString(value.value, offsets, descriptor)
		if exception != nil {
			return nil, exception
		}
		if complete {
			return value, nil
		}
		return &stringValue{value: selected}, nil
	}
	position, exception := normalizeTextIndex(index, len(offsets)-1, true)
	if exception != nil {
		return nil, exception
	}
	return &stringValue{value: value.value[offsets[position]:offsets[position+1]]}, nil
}

func subscriptBytes(value *bytesValue, index Value) (Value, *Exception) {
	if descriptor, ok := index.(*sliceValue); ok {
		selected, complete, exception := sliceBytes(value.value, descriptor)
		if exception != nil {
			return nil, exception
		}
		if complete {
			return value, nil
		}
		return &bytesValue{value: selected}, nil
	}
	position, exception := normalizeTextIndex(index, len(value.value), false)
	if exception != nil {
		return nil, exception
	}
	return newByteInteger(value.value[position]), nil
}

func newByteInteger(value byte) *intValue {
	var integer big.Int
	integer.SetUint64(uint64(value))
	return &intValue{value: integer}
}

// normalizeTextIndex accepts bool or integer indexes, applies negative indexing,
// and selects the exact Python error family and wording for str versus bytes.
func normalizeTextIndex(index Value, length int, text bool) (int, *Exception) {
	integer, ok := integerOperand(index)
	if !ok {
		if text {
			return 0, newException(
				"TypeError",
				"string indices must be integers, not '"+index.TypeName()+"'",
			)
		}
		return 0, newException(
			"TypeError",
			"byte indices must be integers or slices, not "+index.TypeName(),
		)
	}
	if !integer.IsInt64() {
		return 0, textIndexOverflow(index)
	}
	raw := integer.Int64()
	position := int(raw)
	if int64(position) != raw {
		return 0, textIndexOverflow(index)
	}
	if position < 0 {
		position += length
	}
	if position < 0 || position >= length {
		message := "index out of range"
		if text {
			message = "string index out of range"
		}
		return 0, newException("IndexError", message)
	}
	return position, nil
}

func textIndexOverflow(index Value) *Exception {
	return newException(
		"IndexError",
		"cannot fit '"+index.TypeName()+"' into an index-sized integer",
	)
}

func stringCodepointOffsets(value string) []int {
	offsets := make([]int, 1, len(value)+1)
	for current := 0; current < len(value); {
		_, size, _ := decodeStringRune(value[current:])
		current += size
		offsets = append(offsets, current)
	}
	return offsets
}

// sliceString applies shared Python slice normalization to decoded code-point
// boundaries and preserves each selected code point's UTF-8 or WTF-8 spelling.
func sliceString(
	value string,
	offsets []int,
	descriptor *sliceValue,
) (selected string, complete bool, exception *Exception) {
	length := len(offsets) - 1
	start, stop, step, exception := normalizeSlice(descriptor, length)
	if exception != nil {
		return "", false, exception
	}
	if step.IsInt64() && step.Int64() == 1 {
		if start == 0 && stop == length {
			return value, true, nil
		}
		if start >= stop {
			return "", false, nil
		}
		return value[offsets[start]:offsets[stop]], false, nil
	}
	var builder strings.Builder
	var current big.Int
	current.SetInt64(int64(start))
	var boundary big.Int
	boundary.SetInt64(int64(stop))
	negative := step.Sign() < 0
	for (negative && current.Cmp(&boundary) > 0) ||
		(!negative && current.Cmp(&boundary) < 0) {
		position := int(current.Int64())
		builder.WriteString(value[offsets[position]:offsets[position+1]])
		current.Add(&current, &step)
	}
	return builder.String(), false, nil
}

// sliceBytes applies shared Python slice normalization to raw byte offsets and
// emits stepped results without interpreting arbitrary byte payloads as text.
func sliceBytes(
	value string,
	descriptor *sliceValue,
) (selected string, complete bool, exception *Exception) {
	start, stop, step, exception := normalizeSlice(descriptor, len(value))
	if exception != nil {
		return "", false, exception
	}
	if step.IsInt64() && step.Int64() == 1 {
		if start == 0 && stop == len(value) {
			return value, true, nil
		}
		if start >= stop {
			return "", false, nil
		}
		return value[start:stop], false, nil
	}
	var builder strings.Builder
	var current big.Int
	current.SetInt64(int64(start))
	var boundary big.Int
	boundary.SetInt64(int64(stop))
	negative := step.Sign() < 0
	for (negative && current.Cmp(&boundary) > 0) ||
		(!negative && current.Cmp(&boundary) < 0) {
		builder.WriteByte(value[int(current.Int64())])
		current.Add(&current, &step)
	}
	return builder.String(), false, nil
}
