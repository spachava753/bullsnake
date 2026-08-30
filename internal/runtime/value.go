// Package runtime executes Bullsnake bytecode in isolated runtime instances.
package runtime

// Value is a Python value held by the runtime. The sealed method keeps every
// implementation on the runtime's GC-visible object path.
type Value interface {
	TypeName() string
	Repr() string
	isValue()
}

type noneValue struct{}

// None is the immutable Python None singleton.
var None Value = &noneValue{}

func (*noneValue) TypeName() string { return "NoneType" }
func (*noneValue) Repr() string     { return "None" }
func (*noneValue) isValue()         {}
