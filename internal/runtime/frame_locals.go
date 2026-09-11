package runtime

// frameLocalsProxy reads live fast locals and closure cells. Extra dictionary
// keys are shared by all proxies for the frame but never become lexical slots.
type frameLocalsProxy struct {
	frame *frame
}

func (*frameLocalsProxy) TypeName() string { return "FrameLocalsProxy" }
func (*frameLocalsProxy) isValue()         {}
func (proxy *frameLocalsProxy) Repr() string {
	if proxy.frame.localsRepr {
		return "{...}"
	}
	proxy.frame.localsRepr = true
	defer func() { proxy.frame.localsRepr = false }()
	return proxy.snapshot().Repr()
}

// slot prefers cells over their original fast-local argument storage so reads
// and writes agree with LOAD_DEREF after a captured argument has changed.
func (proxy *frameLocalsProxy) slot(name string) *Value {
	target := proxy.frame
	for index, cell := range target.code.cells {
		if cell == name {
			return &target.deref[index].value
		}
	}
	for index, free := range target.code.freeVars {
		if free == name {
			return &target.deref[len(target.code.cells)+index].value
		}
	}
	for index, local := range target.code.locals {
		if local == name {
			return &target.fastLocals[index]
		}
	}
	return nil
}

func (proxy *frameLocalsProxy) get(key Value) (Value, bool, *Exception) {
	if name, ok := key.(*stringValue); ok {
		if slot := proxy.slot(name.value); slot != nil && *slot != nil {
			return *slot, true, nil
		}
	}
	if proxy.frame.extraLocals != nil {
		return proxy.frame.extraLocals.get(key)
	}
	return nil, false, validateDictKey(key)
}

// assign writes lexical slots directly but only permits deleting extra keys.
// A nil value represents item deletion, not Python None.
func (proxy *frameLocalsProxy) assign(key, value Value) *Exception {
	if name, ok := key.(*stringValue); ok {
		if slot := proxy.slot(name.value); slot != nil {
			if value == nil {
				return newException("ValueError", "cannot remove local variables from FrameLocalsProxy")
			}
			*slot = value
			return nil
		}
	}
	if proxy.frame.extraLocals == nil {
		proxy.frame.extraLocals = &dictValue{}
	}
	if value != nil {
		return proxy.frame.extraLocals.set(key, value)
	}
	found, exception := proxy.frame.extraLocals.delete(key)
	if exception != nil {
		return exception
	}
	if !found {
		return newException("KeyError", key.Repr())
	}
	return nil
}

// snapshot follows lexical slot order, omits unbound names, and appends extras.
// Unlike dictionary views, frame-local keys/items/values are snapshot lists.
func (proxy *frameLocalsProxy) snapshot() *dictValue {
	result := &dictValue{}
	for _, names := range [][]string{proxy.frame.code.locals, proxy.frame.code.cells, proxy.frame.code.freeVars} {
		for _, name := range names {
			if slot := proxy.slot(name); slot != nil && *slot != nil {
				result.set(&stringValue{value: name}, *slot)
			}
		}
	}
	if extra := proxy.frame.extraLocals; extra != nil {
		for _, entry := range extra.entries {
			result.set(entry.key, entry.value)
		}
	}
	return result
}

func executeFrameLocalsAttributeLoad(caller *frame, instruction int, proxy *frameLocalsProxy, name string) (instructionOutcome, error) {
	switch name {
	case "get", "pop", "setdefault", "update", "copy", "keys", "values", "items":
		method := &builtinFunctionValue{name: name, call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
			return proxy.call(name, arguments, keywords)
		}}
		return pushOutcome(caller, instruction, method)
	default:
		return raiseOutcome(newException("AttributeError", "'FrameLocalsProxy' object has no attribute '"+name+"'")), nil
	}
}

// call dispatches snapshot methods separately from operations on live bindings.
func (proxy *frameLocalsProxy) call(name string, arguments []Value, keywords *dictValue) (Value, *Exception) {
	minimum, maximum := 0, 0
	switch name {
	case "update":
		minimum, maximum = 1, 1
	case "get", "pop", "setdefault":
		minimum, maximum = 1, 2
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return nil, exception
	}
	if name == "update" {
		return proxy.update(arguments[0])
	}
	if maximum != 0 {
		return proxy.keyMethod(name, arguments)
	}
	snapshot := proxy.snapshot()
	if name == "copy" {
		return snapshot, nil
	}
	result := &listValue{}
	for _, entry := range snapshot.entries {
		value := entry.key
		if name == "values" {
			value = entry.value
		} else if name == "items" {
			value = &tupleValue{elements: []Value{entry.key, entry.value}}
		}
		result.elements = append(result.elements, value)
	}
	return result, nil
}

// keyMethod keeps lookup, default insertion, and restricted deletion consistent
// across get, setdefault, and pop, including missing-key errors.
func (proxy *frameLocalsProxy) keyMethod(name string, arguments []Value) (Value, *Exception) {
	key := arguments[0]
	value, found, exception := proxy.get(key)
	if exception != nil {
		return nil, exception
	}
	if found {
		if name == "pop" {
			return value, proxy.assign(key, nil)
		}
		return value, nil
	}
	value = None
	if len(arguments) == 2 {
		value = arguments[1]
	} else if name == "pop" {
		return nil, newException("KeyError", key.Repr())
	}
	if name == "setdefault" {
		return value, proxy.assign(key, value)
	}
	return value, nil
}

func (proxy *frameLocalsProxy) update(source Value) (Value, *Exception) {
	if other, ok := source.(*frameLocalsProxy); ok {
		source = other.snapshot()
	}
	dictionary, ok := source.(*dictValue)
	if !ok {
		return nil, newException("TypeError", "FrameLocalsProxy.update currently requires a dict or frame locals proxy")
	}
	for _, entry := range dictionary.entries {
		if exception := proxy.assign(entry.key, entry.value); exception != nil {
			return nil, exception
		}
	}
	return None, nil
}
