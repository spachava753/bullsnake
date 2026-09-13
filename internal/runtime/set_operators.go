package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

// nativeSetBinary handles only native sets and frozen sets. It preserves the
// left result kind and mutable in-place identity; other operands are declined.
func nativeSetBinary(left, right Value, operand uint32, inPlace bool) Value {
	first, leftSet := setLikeEntries(left)
	second, rightSet := setLikeEntries(right)
	if !leftSet || !rightSet {
		return notImplementedSingleton
	}
	var entries []Value
	switch operand {
	case bytecode.BinaryOr:
		entries = append(entries, first...)
		for _, value := range second {
			if !setEntriesContain(first, value) {
				entries = append(entries, value)
			}
		}
	case bytecode.BinaryAnd:
		// CPython retains representatives from the smaller operand, choosing
		// the right operand when sizes tie.
		if len(second) > len(first) {
			first, second = second, first
		}
		for _, value := range second {
			if setEntriesContain(first, value) {
				entries = append(entries, value)
			}
		}
	case bytecode.BinarySubtract, bytecode.BinaryXor:
		for _, value := range first {
			if !setEntriesContain(second, value) {
				entries = append(entries, value)
			}
		}
		if operand == bytecode.BinaryXor {
			for _, value := range second {
				if !setEntriesContain(first, value) {
					entries = append(entries, value)
				}
			}
		}
	default:
		return notImplementedSingleton
	}
	if mutable, ok := left.(*setValue); ok {
		if inPlace {
			mutable.entries = entries
			return mutable
		}
		return &setValue{entries: entries}
	}
	return &frozenSetValue{entries: entries}
}

func setEntriesContain(entries []Value, value Value) bool {
	for _, entry := range entries {
		if entry == value || valuesEqual(entry, value) {
			return true
		}
	}
	return false
}

// addNativeSetOperators exposes native operator wrappers, including only the
// mutable set's in-place slots. Reflected wrappers preserve operand order.
func addNativeSetOperators(class *nativeTypeValue, dictionary *dictValue) {
	if class != setNativeType && class != frozenSetNativeType {
		return
	}
	for _, operand := range []uint32{bytecode.BinaryOr, bytecode.BinaryAnd, bytecode.BinarySubtract, bytecode.BinaryXor} {
		normal, reflected, inplace := binaryMethodNames(operand)
		names := []string{normal, reflected}
		if class == setNativeType {
			names = append(names, inplace)
		}
		for _, name := range names {
			dictionary.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
				if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
					return raiseOutcome(exception), nil
				}
				left, right := self, arguments[0]
				if name == reflected {
					left, right = right, left
				}
				return pushOutcome(caller, instruction, nativeSetBinary(left, right, operand, name == inplace))
			}})
		}
	}
}
