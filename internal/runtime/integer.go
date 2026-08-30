package runtime

import "math/big"

type intValue struct {
	value big.Int
}

func (*intValue) TypeName() string   { return "int" }
func (value *intValue) Repr() string { return value.value.String() }
func (*intValue) isValue()           {}
