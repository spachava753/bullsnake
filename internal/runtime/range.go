package runtime

import "math/big"

type rangeValue struct {
	start big.Int
	stop  big.Int
	step  big.Int
}

func (*rangeValue) TypeName() string { return "range" }
func (*rangeValue) Repr() string     { return "range(...)" }
func (*rangeValue) isValue()         {}

type rangeIterator struct {
	current big.Int
	stop    big.Int
	step    big.Int
}

func (*rangeIterator) TypeName() string { return "range_iterator" }
func (*rangeIterator) Repr() string     { return "<range_iterator object>" }
func (*rangeIterator) isValue()         {}
func (iterator *rangeIterator) next() (Value, bool, *Exception) {
	comparison := iterator.current.Cmp(&iterator.stop)
	if (iterator.step.Sign() > 0 && comparison >= 0) ||
		(iterator.step.Sign() < 0 && comparison <= 0) {
		return nil, false, nil
	}
	value := &intValue{}
	value.value.Set(&iterator.current)
	iterator.current.Add(&iterator.current, &iterator.step)
	return value, true, nil
}

func newRangeIterator(sequence *rangeValue) *rangeIterator {
	iterator := &rangeIterator{}
	iterator.current.Set(&sequence.start)
	iterator.stop.Set(&sequence.stop)
	iterator.step.Set(&sequence.step)
	return iterator
}

var _ Value = (*rangeValue)(nil)
var _ valueIterator = (*rangeIterator)(nil)
