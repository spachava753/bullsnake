package runtime

// cellValue keeps one captured binding alive independently of the frame that
// created it. Closure tuples carry cells, while LOAD_DEREF exposes their value.
type cellValue struct {
	value Value
}

func (*cellValue) TypeName() string { return "cell" }
func (*cellValue) Repr() string     { return "<cell>" }
func (*cellValue) isValue()         {}
func (cell *cellValue) attribute(name string) (Value, bool) {
	if name != "cell_contents" || cell.value == nil {
		return nil, false
	}
	return cell.value, true
}

var cellType = &builtinTypeValue{
	name:    "cell",
	matches: func(value Value) bool { _, ok := value.(*cellValue); return ok },
}

func initializeDeref(
	code *preparedCode,
	fastLocals []Value,
	closure []*cellValue,
) ([]*cellValue, bool) {
	if len(closure) != len(code.freeVars) {
		return nil, false
	}
	deref := make([]*cellValue, len(code.cells)+len(code.freeVars))
	for index, localIndex := range code.cellLocals {
		var value Value
		if localIndex >= 0 {
			value = fastLocals[localIndex]
			fastLocals[localIndex] = nil
		}
		deref[index] = &cellValue{value: value}
	}
	copy(deref[len(code.cells):], closure)
	return deref, true
}

func unboundDerefException(code *preparedCode, index int) *Exception {
	if index < len(code.cells) {
		return newException(
			"UnboundLocalError",
			"cannot access local variable '"+code.cells[index]+
				"' where it is not associated with a value",
		)
	}
	name := code.freeVars[index-len(code.cells)]
	return newException(
		"NameError",
		"cannot access free variable '"+name+
			"' where it is not associated with a value in enclosing scope",
	)
}
