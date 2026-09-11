package runtime

// addNativeBufferDescriptors installs receiver-checked byte exporters and the
// release wrapper only for native types that own a release-buffer operation.
func addNativeBufferDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	if class != bytesNativeType && class != bytearrayNativeType {
		return
	}
	for _, name := range []string{"__buffer__", "__release_buffer__"} {
		if name == "__release_buffer__" && class == bytesNativeType {
			continue
		}
		dictionary.set(&stringValue{value: name}, &builtinFunctionValue{name: name, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			if exception := checkNativeArguments(name, arguments, keywords, 2, 2); exception != nil {
				discardCallSegment(caller, base)
				return raiseOutcome(exception), nil
			}
			self, argument := arguments[0], arguments[1]
			discardCallSegment(caller, base)
			if !nativeReceiverMatches(self, class) {
				return raiseOutcome(newException("TypeError", "descriptor '"+name+"' requires a '"+class.name+"' object")), nil
			}
			return executeBufferMethod(caller, instruction, self, name, []Value{argument}, nil)
		}})
	}
}

// executeBufferMethod validates release ownership or converts flags through the
// index protocol before acquiring a real, explicitly releasable export.
func executeBufferMethod(caller *frame, instruction int, self Value, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	if name == "__release_buffer__" {
		view := viewOf(arguments[0])
		if view == nil {
			return raiseOutcome(newException("TypeError", "expected a memoryview object")), nil
		}
		if view.released {
			return pushOutcome(caller, instruction, None)
		}
		if view.owner != self {
			return raiseOutcome(newException("ValueError", "memoryview's buffer is not this object")), nil
		}
		return executeViewMethod(caller, instruction, arguments[0].(*instanceValue), "release", nil, nil)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, arguments[0])
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		flags := result.(*intValue).value.Int64()
		if flags < -2147483648 || flags > 2147483647 {
			return raiseOutcome(newException("OverflowError", "buffer flags out of range")), nil
		}
		view, exception := newProtocolBuffer(current.runtime.memoryViewClass, self, int(flags))
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(current, instruction, view)
	})
}

// newProtocolBuffer keeps the exporting object as .obj, unlike ordinary
// memoryview copying, and enforces the requested writable/contiguity flags.
func newProtocolBuffer(class *typeValue, source Value, flags int) (*instanceValue, *Exception) {
	var view memoryView
	switch source := source.(type) {
	case *bytesValue:
		if flags&1 != 0 {
			return nil, newException("BufferError", "Object is not writable.")
		}
		view = memoryView{buffer: &byteBuffer{data: []byte(source.value)}, owner: source, length: len(source.value), stride: 1, readonly: true}
	case *bytearrayValue:
		view = memoryView{buffer: source.buffer, owner: source, length: len(source.buffer.data), stride: 1}
	default:
		original := viewOf(source)
		if original == nil {
			return nil, newException("TypeError", "object does not provide a native buffer")
		}
		if exception := checkViewBufferFlags(original, flags); exception != nil {
			return nil, exception
		}
		view = *original
		view.owner = source
	}
	return newViewInstance(class, view), nil
}

// checkViewBufferFlags applies the supported one-dimensional unsigned-byte
// buffer requirements before any new export is published.
func checkViewBufferFlags(view *memoryView, flags int) *Exception {
	if view.released {
		return releasedViewError()
	}
	if flags&1 != 0 && view.readonly {
		return newException("BufferError", "memoryview: underlying buffer is not writable")
	}
	if view.length > 1 && view.stride != 1 {
		switch {
		case flags&0x38 == 0x38:
			return newException("BufferError", "memoryview: underlying buffer is not C-contiguous")
		case flags&0x58 == 0x58:
			return newException("BufferError", "memoryview: underlying buffer is not Fortran contiguous")
		case flags&0x98 == 0x98:
			return newException("BufferError", "memoryview: underlying buffer is not contiguous")
		case flags&0x18 != 0x18:
			return newException("BufferError", "memoryview: underlying buffer is not C-contiguous")
		}
	}
	if flags&8 == 0 && flags&4 != 0 {
		return newException("BufferError", "memoryview: cannot cast to unsigned bytes if the format flag is present")
	}
	return nil
}

// hasProtocolExports checks weak export records, so dead buffers stop pinning a
// source view without invoking Python or cleanup callbacks from Go GC.
func (instance *instanceValue) hasProtocolExports() bool {
	view := instance.io.view
	if view.released || view.buffer == nil {
		return false
	}
	view.buffer.hasViews()
	for _, reference := range view.buffer.views {
		if exported := reference.Value(); exported != nil && !exported.io.view.released && exported.io.view.owner == instance {
			return true
		}
	}
	return false
}
