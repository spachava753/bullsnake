package runtime

import (
	"math/big"
	"strconv"
)

type rangeValue struct {
	start  big.Int
	stop   big.Int
	step   big.Int
	length big.Int
}

func (*rangeValue) TypeName() string { return "range" }
func (value *rangeValue) Repr() string {
	if value.step.Cmp(big.NewInt(1)) == 0 {
		return "range(" + value.start.String() + ", " + value.stop.String() + ")"
	}
	return "range(" + value.start.String() + ", " + value.stop.String() +
		", " + value.step.String() + ")"
}
func (*rangeValue) isValue() {}

type rangeIterator struct {
	current   big.Int
	step      big.Int
	remaining big.Int
}

func (*rangeIterator) TypeName() string { return "range_iterator" }
func (*rangeIterator) Repr() string     { return "<range_iterator object>" }
func (*rangeIterator) isValue()         {}

func (iterator *rangeIterator) next() (Value, bool, *Exception) {
	if iterator.remaining.Sign() == 0 {
		return nil, false, nil
	}
	var current big.Int
	current.Set(&iterator.current)
	iterator.current.Add(&iterator.current, &iterator.step)
	iterator.remaining.Sub(&iterator.remaining, big.NewInt(1))
	return &intValue{value: current}, true, nil
}

// executeRangeTypeCall validates positional integer bounds, rejects a zero step,
// computes the exact logical length, and returns a lazy immutable range value.
func executeRangeTypeCall(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"range() takes no keyword arguments",
		)), nil
	}
	if len(arguments) == 0 || len(arguments) > 3 {
		message := "range expected at least 1 argument, got 0"
		if len(arguments) > 3 {
			message = "range expected at most 3 arguments, got " +
				strconv.Itoa(len(arguments))
		}
		discardCallSegment(caller, base)
		return raiseOutcome(newException("TypeError", message)), nil
	}

	converted := make([]big.Int, len(arguments))
	for index, argument := range arguments {
		integer, ok := integerOperand(argument)
		if !ok {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"TypeError",
				"'"+argument.TypeName()+"' object cannot be interpreted as an integer",
			)), nil
		}
		converted[index].Set(&integer)
	}

	var start, stop, step big.Int
	step.SetInt64(1)
	if len(converted) == 1 {
		stop.Set(&converted[0])
	} else {
		start.Set(&converted[0])
		stop.Set(&converted[1])
		if len(converted) == 3 {
			step.Set(&converted[2])
		}
	}
	if step.Sign() == 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"ValueError",
			"range() arg 3 must not be zero",
		)), nil
	}

	result := &rangeValue{start: start, stop: stop, step: step}
	result.length.Set(rangeLength(&result.start, &result.stop, &result.step))
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, result)
}

func rangeLength(start, stop, step *big.Int) *big.Int {
	length := new(big.Int)
	one := big.NewInt(1)
	if step.Sign() > 0 {
		if start.Cmp(stop) >= 0 {
			return length
		}
		length.Sub(stop, start)
		length.Sub(length, one)
		length.Quo(length, step)
		return length.Add(length, one)
	}
	if start.Cmp(stop) <= 0 {
		return length
	}
	negativeStep := new(big.Int).Neg(step)
	length.Sub(start, stop)
	length.Sub(length, one)
	length.Quo(length, negativeStep)
	return length.Add(length, one)
}

func newRangeIterator(value *rangeValue) *rangeIterator {
	iterator := &rangeIterator{}
	iterator.current.Set(&value.start)
	iterator.step.Set(&value.step)
	iterator.remaining.Set(&value.length)
	return iterator
}

func executeRangeAttributeLoad(
	frame *frame,
	instruction int,
	owner *rangeValue,
	name string,
) (instructionOutcome, error) {
	var integer big.Int
	switch name {
	case "start":
		integer.Set(&owner.start)
	case "stop":
		integer.Set(&owner.stop)
	case "step":
		integer.Set(&owner.step)
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'range' object has no attribute '"+name+"'",
		)), nil
	}
	return pushOutcome(frame, instruction, &intValue{value: integer})
}

var (
	_ Value         = (*rangeValue)(nil)
	_ valueIterator = (*rangeIterator)(nil)
)
