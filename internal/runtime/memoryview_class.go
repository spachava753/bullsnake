package runtime

// initializeMemoryViewClass installs an immutable non-subclassable byte-view
// class using ordinary descriptors and per-instance private storage.
func initializeMemoryViewClass(namespace *Namespace) {
	class := newIOClass("memoryview", nil)
	class.ioClass, class.bufferViewClass, class.module = false, true, "builtins"
	namespace.values["memoryview"] = class
	for _, name := range []string{"release", "__enter__", "__exit__", "__len__", "__iter__", "__getitem__", "__setitem__", "__eq__", "__ne__", "__hash__", "tobytes", "tolist", "toreadonly"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeViewMethod(caller, instruction, self, name, arguments, keywords)
		}))
	}
	for _, name := range []string{"obj", "nbytes", "readonly", "format", "itemsize", "ndim", "shape", "strides", "suboffsets", "c_contiguous", "f_contiguous", "contiguous"} {
		class.setAttribute(name, &propertyValue{doc: None, getter: ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
			view := self.io.view
			if view.released {
				return raiseOutcome(releasedViewError()), nil
			}
			var value Value
			switch name {
			case "obj":
				value = view.owner
			case "nbytes":
				value = integerFromInt64(int64(view.length))
			case "readonly":
				value = booleanValue(view.readonly)
			case "format":
				value = &stringValue{value: "B"}
			case "itemsize", "ndim":
				value = integerFromInt64(1)
			case "shape":
				value = &tupleValue{elements: []Value{integerFromInt64(int64(view.length))}}
			case "strides":
				value = &tupleValue{elements: []Value{integerFromInt64(int64(view.stride))}}
			case "suboffsets":
				value = &tupleValue{}
			default:
				value = booleanValue(view.length <= 1 || view.stride == 1)
			}
			return pushOutcome(caller, instruction, value)
		})})
	}
}

// executeViewMethod checks release and mutability before touching storage.
// Child and readonly views receive their own export and release state.
func executeViewMethod(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	order := "C"
	if name == "tobytes" {
		var exception *Exception
		order, exception = bindViewOrder(arguments, keywords)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		arguments, keywords = nil, nil
	}
	minimum, maximum := 0, 0
	switch name {
	case "__getitem__", "__eq__", "__ne__":
		minimum, maximum = 1, 1
	case "__setitem__":
		minimum, maximum = 2, 2
	case "__exit__":
		minimum, maximum = 3, 3
	}
	if exception := checkNativeArguments(name, arguments, keywords, minimum, maximum); exception != nil {
		return raiseOutcome(exception), nil
	}
	view := self.io.view
	if name == "release" || name == "__exit__" {
		view.released, view.buffer, view.owner = true, nil, nil
		return pushOutcome(caller, instruction, None)
	}
	if name == "__eq__" || name == "__ne__" {
		equal := self == arguments[0]
		if !equal && !view.released {
			left, _ := view.copyBytes()
			right, exception := binaryCopyData(arguments[0])
			equal = exception == nil && string(left) == string(right)
		}
		return pushOutcome(caller, instruction, booleanValue(equal == (name == "__eq__")))
	}
	if name == "__hash__" && view.hash != nil {
		return pushOutcome(caller, instruction, view.hash)
	}
	if view.released {
		return raiseOutcome(releasedViewError()), nil
	}
	switch name {
	case "__enter__":
		return pushOutcome(caller, instruction, self)
	case "__len__":
		return pushOutcome(caller, instruction, integerFromInt64(int64(view.length)))
	case "__iter__":
		return pushOutcome(caller, instruction, &memoryViewIterator{view: self})
	case "__getitem__", "__setitem__":
		return executeViewSubscription(caller, instruction, self, arguments, name == "__setitem__")
	case "toreadonly":
		copy := *view
		copy.readonly = true
		return pushOutcome(caller, instruction, newViewInstance(self.class, copy))
	case "__hash__":
		if !view.readonly {
			return raiseOutcome(newException("ValueError", "cannot hash writable memoryview object")), nil
		}
		if _, ok := view.owner.(*bytearrayValue); ok {
			return raiseOutcome(unhashableTypeError("bytearray")), nil
		}
		data, _ := view.copyBytes()
		view.hash = integerFromInt64(stableTextHash("bytes", string(data)))
		return pushOutcome(caller, instruction, view.hash)
	case "tolist":
		data, _ := view.copyBytes()
		result := &listValue{}
		for _, item := range data {
			result.elements = append(result.elements, newByteInteger(item))
		}
		return pushOutcome(caller, instruction, result)
	default:
		if order != "C" && order != "F" && order != "A" {
			return raiseOutcome(newException("ValueError", "order must be 'C', 'F' or 'A'")), nil
		}
		data, _ := view.copyBytes()
		return pushOutcome(caller, instruction, &bytesValue{value: string(data)})
	}
}

// bindViewOrder accepts tobytes' positional-or-keyword order and validates its
// type before released-state checks, leaving value validation to the method.
func bindViewOrder(arguments []Value, keywords *dictValue) (string, *Exception) {
	if len(arguments) > 1 {
		return "", newException("TypeError", "tobytes() takes at most 1 argument")
	}
	var value Value = None
	if len(arguments) == 1 {
		value = arguments[0]
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			if entry.key.(*stringValue).value != "order" {
				return "", newException("TypeError", "invalid keyword argument for tobytes()")
			}
			if len(arguments) != 0 {
				return "", newException("TypeError", "tobytes() got multiple values for argument 'order'")
			}
			value = entry.value
		}
	}
	if value == None {
		return "C", nil
	}
	if text, ok := value.(*stringValue); ok {
		return text.value, nil
	}
	return "", newException("TypeError", "order must be str or None")
}
