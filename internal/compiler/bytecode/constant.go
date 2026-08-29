package bytecode

import (
	"fmt"
	"math"
	"strconv"
)

// ConstantKind identifies a compiler-level literal value.
type ConstantKind uint8

const (
	NoneConstant ConstantKind = iota
	BoolConstant
	EllipsisConstant
	IntegerConstant
	FloatConstant
	ImaginaryConstant
)

// Constant is an immutable literal descriptor stored in a code object.
type Constant struct {
	Kind ConstantKind
	Bool bool
	Text string
	Bits uint64
}

// None returns the Python None constant descriptor.
func None() Constant { return Constant{Kind: NoneConstant} }

// Bool returns a Python boolean constant descriptor.
func Bool(value bool) Constant { return Constant{Kind: BoolConstant, Bool: value} }

// Ellipsis returns the Python ellipsis constant descriptor.
func Ellipsis() Constant { return Constant{Kind: EllipsisConstant} }

// Integer returns an arbitrary-precision integer in canonical decimal form.
func Integer(decimal string) Constant {
	return Constant{Kind: IntegerConstant, Text: decimal}
}

// Float returns a binary64 floating-point constant.
func Float(value float64) Constant {
	return Constant{Kind: FloatConstant, Bits: math.Float64bits(value)}
}

// Imaginary returns a pure-imaginary binary64 constant.
func Imaginary(value float64) Constant {
	return Constant{Kind: ImaginaryConstant, Bits: math.Float64bits(value)}
}

// String returns the stable dump spelling of a constant.
func (constant Constant) String() string {
	switch constant.Kind {
	case NoneConstant:
		return "None"
	case BoolConstant:
		if constant.Bool {
			return "True"
		}
		return "False"
	case EllipsisConstant:
		return "Ellipsis"
	case IntegerConstant:
		return "Int(" + constant.Text + ")"
	case FloatConstant:
		return "Float(" + formatFloat(constant.Bits) + ")"
	case ImaginaryConstant:
		return "Imag(" + formatFloat(constant.Bits) + ")"
	default:
		return fmt.Sprintf("Constant(kind=%d)", constant.Kind)
	}
}

func formatFloat(bits uint64) string {
	return strconv.FormatFloat(math.Float64frombits(bits), 'g', -1, 64)
}
