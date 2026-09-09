package runtime

// assignIndex replaces one list element after validating a native integer or
// boolean index. Negative indexes are relative to the current list length.
func (list *listValue) assignIndex(key, value Value) *Exception {
	if _, slice := key.(*sliceValue); slice {
		return newException("TypeError", "list slice assignment is not supported")
	}
	index, ok := integerOperand(key)
	if !ok {
		return newException("TypeError", "list indices must be integers or slices, not "+key.TypeName())
	}
	if !index.IsInt64() {
		return textIndexOverflow(key)
	}
	raw := index.Int64()
	position := int(raw)
	if int64(position) != raw {
		return textIndexOverflow(key)
	}
	if position < 0 {
		position += len(list.elements)
	}
	if position < 0 || position >= len(list.elements) {
		return newException("IndexError", "list assignment index out of range")
	}
	list.elements[position] = value
	return nil
}
