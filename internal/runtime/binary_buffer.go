package runtime

// binaryBufferAdd concatenates contiguous bytes-like storage. Bytearray +=
// retains identity and checks exports before resizing; immutable results copy.
func binaryBufferAdd(left, right Value, inPlace bool) (Value, *Exception, bool) {
	array, mutable := left.(*bytearrayValue)
	_, immutable := left.(*bytesValue)
	if !mutable && !immutable {
		return nil, nil, false
	}
	rightData, exception := binaryData(right)
	if exception != nil {
		return nil, nil, false
	}
	leftData, _ := binaryData(left)
	if len(leftData) > int(^uint(0)>>1)-len(rightData) {
		return nil, newException("OverflowError", "concatenated bytes are too long"), true
	}
	if mutable && inPlace {
		if len(rightData) != 0 && (array.buffer.hasViews() || left == right) {
			return nil, bufferExportError(), true
		}
		array.buffer.data = append(array.buffer.data, rightData...)
		return array, nil, true
	}
	if immutable {
		return &bytesValue{value: string(leftData) + string(rightData)}, nil, true
	}
	data := append(append([]byte(nil), leftData...), rightData...)
	return &bytearrayValue{buffer: &byteBuffer{data: data}}, nil, true
}
