package runtime

import "math/big"

type sliceValue struct {
	start Value
	stop  Value
	step  Value
}

func (*sliceValue) TypeName() string { return "slice" }
func (value *sliceValue) Repr() string {
	return "slice(" + value.start.Repr() + ", " + value.stop.Repr() + ", " +
		value.step.Repr() + ")"
}
func (*sliceValue) isValue() {}

func executeBuildSlice(
	frame *frame,
	instruction int,
	count int,
) (instructionOutcome, error) {
	step := None
	if count == 3 {
		var ok bool
		step, ok = frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
		}
	}
	stop, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	start, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	return pushOutcome(frame, instruction, &sliceValue{start: start, stop: stop, step: step})
}

// sliceSequence applies one slice descriptor to a tuple or list after
// normalizing arbitrary-size bounds according to the step direction.
func sliceSequence(
	container Value,
	elements []Value,
	descriptor *sliceValue,
) (Value, *Exception) {
	start, stop, step, exception := normalizeSlice(descriptor, len(elements))
	if exception != nil {
		return nil, exception
	}

	if _, tuple := container.(*tupleValue); tuple &&
		step.IsInt64() && step.Int64() == 1 && start == 0 && stop == len(elements) {
		return container, nil
	}

	selected := make([]Value, 0)
	var current big.Int
	current.SetInt64(int64(start))
	var boundary big.Int
	boundary.SetInt64(int64(stop))
	negative := step.Sign() < 0
	for (negative && current.Cmp(&boundary) > 0) ||
		(!negative && current.Cmp(&boundary) < 0) {
		selected = append(selected, elements[int(current.Int64())])
		current.Add(&current, &step)
	}
	if _, tuple := container.(*tupleValue); tuple {
		return &tupleValue{elements: selected}, nil
	}
	return &listValue{elements: selected}, nil
}

func normalizeSlice(
	descriptor *sliceValue,
	length int,
) (start int, stop int, step big.Int, exception *Exception) {
	step, exception = evaluateSliceStep(descriptor.step)
	if exception != nil {
		return 0, 0, step, exception
	}
	negative := step.Sign() < 0
	startDefault := 0
	stopDefault := length
	if negative {
		startDefault = length - 1
		stopDefault = -1
	}
	start, exception = normalizeSliceBound(
		descriptor.start,
		length,
		negative,
		startDefault,
	)
	if exception != nil {
		return 0, 0, step, exception
	}
	stop, exception = normalizeSliceBound(
		descriptor.stop,
		length,
		negative,
		stopDefault,
	)
	return start, stop, step, exception
}

func evaluateSliceStep(value Value) (big.Int, *Exception) {
	var step big.Int
	if _, none := value.(*noneValue); none {
		step.SetInt64(1)
		return step, nil
	}
	step, ok := integerOperand(value)
	if !ok {
		return step, sliceIndexTypeError()
	}
	if step.Sign() == 0 {
		return step, newException("ValueError", "slice step cannot be zero")
	}
	return step, nil
}

// normalizeSliceBound maps None or an arbitrary-size integer into the valid
// start or stop range selected by the step direction.
func normalizeSliceBound(
	value Value,
	length int,
	negative bool,
	defaultIndex int,
) (int, *Exception) {
	if _, none := value.(*noneValue); none {
		return defaultIndex, nil
	}
	index, ok := integerOperand(value)
	if !ok {
		return 0, sliceIndexTypeError()
	}
	var limit big.Int
	limit.SetInt64(int64(length))
	if index.Sign() < 0 {
		index.Add(&index, &limit)
	}
	if index.Sign() < 0 {
		if negative {
			return -1, nil
		}
		return 0, nil
	}
	if index.Cmp(&limit) >= 0 {
		if negative {
			return length - 1, nil
		}
		return length, nil
	}
	return int(index.Int64()), nil
}

func sliceIndexTypeError() *Exception {
	return newException(
		"TypeError",
		"slice indices must be integers or None or have an __index__ method",
	)
}
