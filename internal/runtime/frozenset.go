package runtime

type frozenSetValue struct {
	entries []Value
}

func (*frozenSetValue) TypeName() string { return "frozenset" }
func (set *frozenSetValue) Repr() string {
	if len(set.entries) == 0 {
		return "frozenset()"
	}
	return "frozenset(" + (&setValue{entries: set.entries}).Repr() + ")"
}
func (*frozenSetValue) isValue() {}

func (set *frozenSetValue) attribute(name string) (Value, bool) {
	if name == "difference" {
		return (&setValue{entries: set.entries}).attribute(name)
	}
	if name == "__contains__" {
		return nativeFunctionNamed("frozenset.__contains__", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				for _, entry := range set.entries {
					if valuesEqual(entry, arguments[0]) {
						return trueSingleton, nil, nil
					}
				}
				return falseSingleton, nil, nil
			}), true
	}
	return nil, false
}

var _ Value = (*frozenSetValue)(nil)
