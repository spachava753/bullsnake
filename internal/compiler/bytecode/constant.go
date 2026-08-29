package bytecode

import "fmt"

// ConstantKind identifies a compiler-level literal value.
type ConstantKind uint8

const (
	NoneConstant ConstantKind = iota
	BoolConstant
	EllipsisConstant
)

// Constant is an immutable literal descriptor stored in a code object.
type Constant struct {
	Kind ConstantKind
	Bool bool
}

// None returns the Python None constant descriptor.
func None() Constant { return Constant{Kind: NoneConstant} }

// Bool returns a Python boolean constant descriptor.
func Bool(value bool) Constant { return Constant{Kind: BoolConstant, Bool: value} }

// Ellipsis returns the Python ellipsis constant descriptor.
func Ellipsis() Constant { return Constant{Kind: EllipsisConstant} }

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
	default:
		return fmt.Sprintf("Constant(kind=%d)", constant.Kind)
	}
}
