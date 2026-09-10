package runtime

import "weak"

// memoryView retains its exporter and describes a one-dimensional byte view.
// Export tables hold weak references to actual Python view allocations.
type memoryView struct {
	pins     int
	buffer   *byteBuffer
	owner    Value
	offset   int
	length   int
	stride   int
	readonly bool
	released bool
	hash     *intValue
}

func newViewInstance(class *typeValue, view memoryView) *instanceValue {
	view.pins = 0
	view.hash = nil
	instance := &instanceValue{class: class, attributes: newNamespace(), io: &ioState{view: &view}}
	view.buffer.views = append(view.buffer.views, weak.Make(instance))
	return instance
}

// hasViews prunes dead or explicitly released exports on the VM goroutine.
// The exporter never keeps its view objects alive or runs Python from GC.
func (buffer *byteBuffer) hasViews() bool {
	if buffer.pins != 0 {
		return true
	}
	live := buffer.views[:0]
	for _, reference := range buffer.views {
		if value := reference.Value(); value != nil && value.io != nil && !value.io.view.released {
			live = append(live, reference)
		}
	}
	clear(buffer.views[len(live):])
	buffer.views = live
	return len(live) != 0
}

func releasedViewError() *Exception {
	return newException("ValueError", "operation forbidden on released memoryview object")
}

func bufferExportError() *Exception {
	return newException("BufferError", "Existing exports of data: object cannot be re-sized")
}

func viewOf(value Value) *memoryView {
	if instance, ok := value.(*instanceValue); ok && instance.io != nil {
		return instance.io.view
	}
	return nil
}

func (view *memoryView) copyBytes() ([]byte, *Exception) {
	if view.released {
		return nil, releasedViewError()
	}
	data := make([]byte, view.length)
	for index := range data {
		data[index] = view.buffer.data[view.offset+index*view.stride]
	}
	return data, nil
}

func binaryCopyData(value Value) ([]byte, *Exception) {
	if view := viewOf(value); view != nil {
		return view.copyBytes()
	}
	return binaryData(value)
}

// executeMemoryViewTypeCall retains an immutable or mutable exporter, or copies
// an existing view's shape into a separately releasable export.
func executeMemoryViewTypeCall(caller *frame, instruction, base int, class *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	if exception := checkNativeArguments("memoryview", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	var view memoryView
	switch value := arguments[0].(type) {
	case *bytesValue:
		view = memoryView{buffer: &byteBuffer{data: []byte(value.value)}, owner: value, length: len(value.value), stride: 1, readonly: true}
	case *bytearrayValue:
		view = memoryView{buffer: value.buffer, owner: value, length: len(value.buffer.data), stride: 1}
	default:
		original := viewOf(value)
		if original == nil {
			return raiseOutcome(newException("TypeError", "memoryview: a bytes-like object is required, not '"+value.TypeName()+"'")), nil
		}
		if original.released {
			return raiseOutcome(releasedViewError()), nil
		}
		view = *original
	}
	return pushOutcome(caller, instruction, newViewInstance(class, view))
}

type memoryViewIterator struct {
	view  *instanceValue
	index int
}

func (*memoryViewIterator) TypeName() string { return "memory_iterator" }
func (*memoryViewIterator) Repr() string     { return "<memory_iterator object>" }
func (*memoryViewIterator) isValue()         {}
func (iterator *memoryViewIterator) next() (Value, bool, *Exception) {
	view := iterator.view.io.view
	if view.released {
		return nil, false, releasedViewError()
	}
	if iterator.index >= view.length {
		return nil, false, nil
	}
	value := view.buffer.data[view.offset+iterator.index*view.stride]
	iterator.index++
	return newByteInteger(value), true, nil
}
