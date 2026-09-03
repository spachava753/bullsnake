package runtime

type superValue struct {
	start  *typeValue
	object Value
}

func (*superValue) TypeName() string { return "super" }
func (*superValue) Repr() string     { return "<super object>" }
func (*superValue) isValue()         {}

// attribute searches the receiver MRO after the recorded starting class.
func (value *superValue) attribute(name string) (Value, bool) {
	class := value.start
	if instance, ok := value.object.(*instanceValue); ok {
		class = instance.class
	}
	order := class.methodResolutionOrder()
	start := -1
	for index, candidate := range order {
		if candidate == value.start {
			start = index
			break
		}
	}
	for _, candidate := range order[start+1:] {
		if attribute, found := candidate.namespace.get(name); found {
			if name == "__new__" {
				return attribute, true
			}
			return bindCallable(attribute, value.object), true
		}
	}
	if builtin := class.inheritedBuiltinBase(); builtin != nil {
		if attribute, found := builtinTypeMethod(builtin.name, name); found {
			if name == "__new__" {
				return attribute, true
			}
			return bindCallable(attribute, value.object), true
		}
	}
	attribute, found := builtinTypeMethod("object", name)
	if !found {
		return nil, false
	}
	if name == "__new__" {
		return attribute, true
	}
	return bindCallable(attribute, value.object), true
}

// builtinSuper builds explicit or zero-argument super proxies from call-frame metadata.
func builtinSuper(caller *frame, arguments []Value) (Value, *Exception, error) {
	if len(arguments) == 2 {
		class, ok := arguments[0].(*typeValue)
		if !ok {
			return nil, newException("TypeError", "super() argument 1 must be a type"), nil
		}
		return &superValue{start: class, object: arguments[1]}, nil, nil
	}
	if len(arguments) != 0 {
		return nil, newException("TypeError", "super() takes 0 or 2 arguments"), nil
	}
	class, found := callerClass(caller)
	if !found {
		return nil, newException("RuntimeError", "super(): no __class__ cell"), nil
	}
	object, found := callerFirstArgument(caller)
	if !found {
		return nil, newException("RuntimeError", "super(): no arguments"), nil
	}
	return &superValue{start: class, object: object}, nil, nil
}

func callerClass(caller *frame) (*typeValue, bool) {
	for index, name := range caller.code.cells {
		if name == "__class__" {
			class, ok := caller.deref[index].value.(*typeValue)
			return class, ok
		}
	}
	for index, name := range caller.code.freeVars {
		if name == "__class__" {
			class, ok := caller.deref[len(caller.code.cells)+index].value.(*typeValue)
			return class, ok
		}
	}
	return nil, false
}

// callerFirstArgument retrieves positional slot zero from fast locals or its cell.
func callerFirstArgument(caller *frame) (Value, bool) {
	if len(caller.code.locals) == 0 {
		return nil, false
	}
	if len(caller.fastLocals) != 0 && caller.fastLocals[0] != nil {
		return caller.fastLocals[0], true
	}
	for index, localIndex := range caller.code.cellLocals {
		if localIndex == 0 && caller.deref[index].value != nil {
			return caller.deref[index].value, true
		}
	}
	return nil, false
}

var _ Value = (*superValue)(nil)
