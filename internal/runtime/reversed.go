package runtime

import (
	"math/big"
	"strconv"
)

type reverseIterator struct {
	sequence Value
	index    int
	offsets  []int
}

func (iterator *reverseIterator) TypeName() string {
	if _, list := iterator.sequence.(*listValue); list {
		return "list_reverseiterator"
	}
	return "reversed"
}
func (iterator *reverseIterator) Repr() string {
	return "<" + iterator.TypeName() + " object>"
}
func (*reverseIterator) isValue() {}

// next yields the current reverse index and exhausts a list iterator if a
// shrink makes that original index invalid.
func (iterator *reverseIterator) next() (Value, bool, *Exception) {
	if iterator.index < 0 {
		return nil, false, nil
	}
	switch sequence := iterator.sequence.(type) {
	case *listValue:
		if iterator.index >= len(sequence.elements) {
			iterator.index = -1
			return nil, false, nil
		}
		value := sequence.elements[iterator.index]
		iterator.index--
		return value, true, nil
	case *tupleValue:
		value := sequence.elements[iterator.index]
		iterator.index--
		return value, true, nil
	case *stringValue:
		start := iterator.offsets[iterator.index]
		end := iterator.offsets[iterator.index+1]
		iterator.index--
		return &stringValue{value: sequence.value[start:end]}, true, nil
	case *bytesValue:
		value := newByteInteger(sequence.value[iterator.index])
		iterator.index--
		return value, true, nil
	default:
		iterator.index = -1
		return nil, false, nil
	}
}

// executeBuiltinReversed invokes the class reverse slot through the VM and
// otherwise retains the existing native sequence implementation.
func executeBuiltinReversed(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if len(arguments) == 1 && (keywords == nil || len(keywords.entries) == 0) {
		if instance, ok := arguments[0].(*instanceValue); ok {
			if method, found := lookupInstanceSpecial(instance, "__reversed__"); found {
				discardCallSegment(caller, base)
				if method == None {
					return raiseOutcome(newException("TypeError", "'"+instance.TypeName()+"' object is not reversible")), nil
				}
				return executeFunctionCall(caller, instruction, len(caller.stack), method, nil, nil)
			}
		}
	}
	result, exception := builtinReversed(arguments, keywords)
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, result)
}

// builtinReversed creates a lazy native reverse iterator while preserving each
// sequence kind's indexing behavior.
func builtinReversed(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if keywords != nil && len(keywords.entries) != 0 {
		return nil, newException("TypeError", "reversed() takes no keyword arguments")
	}
	if len(arguments) != 1 {
		return nil, newException(
			"TypeError",
			"reversed expected 1 argument, got "+strconv.Itoa(len(arguments)),
		)
	}
	sequence := arguments[0]
	switch sequence := sequence.(type) {
	case *listValue:
		return &reverseIterator{sequence: sequence, index: len(sequence.elements) - 1}, nil
	case *tupleValue:
		return &reverseIterator{sequence: sequence, index: len(sequence.elements) - 1}, nil
	case *stringValue:
		offsets := stringCodepointOffsets(sequence.value)
		return &reverseIterator{
			sequence: sequence,
			index:    len(offsets) - 2,
			offsets:  offsets,
		}, nil
	case *bytesValue:
		return &reverseIterator{sequence: sequence, index: len(sequence.value) - 1}, nil
	case *rangeValue:
		return newReverseRangeIterator(sequence), nil
	case *instanceValue:
		return nil, newException(
			"NotImplementedError",
			"user reversed protocols are not supported",
		)
	default:
		return nil, newException(
			"TypeError",
			"'"+sequence.TypeName()+"' object is not reversible",
		)
	}
}

func newReverseRangeIterator(value *rangeValue) *rangeIterator {
	iterator := &rangeIterator{}
	iterator.remaining.Set(&value.length)
	iterator.step.Neg(&value.step)
	if value.length.Sign() == 0 {
		return iterator
	}
	var last big.Int
	last.Sub(&value.length, big.NewInt(1))
	last.Mul(&last, &value.step)
	iterator.current.Add(&value.start, &last)
	return iterator
}

var _ valueIterator = (*reverseIterator)(nil)
