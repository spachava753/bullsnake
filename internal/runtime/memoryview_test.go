package runtime

import (
	goruntime "runtime"
	"testing"
	"weak"
)

// TestMemoryViewOwnership verifies that exports neither retain unreachable
// views nor lose live exporters, using actual Python view allocations.
func TestMemoryViewOwnership(t *testing.T) {
	namespace := newNamespace()
	initializeMemoryViewClass(namespace)
	class := namespace.values["memoryview"].(*typeValue)
	buffer := &byteBuffer{data: []byte("kept")}
	dead := temporaryBufferView(class, buffer)
	other := temporaryBufferView(class, buffer)
	for range 10 {
		goruntime.GC()
		if dead.Value() == nil && other.Value() == nil {
			break
		}
	}
	if dead.Value() != nil || other.Value() != nil || buffer.hasViews() {
		t.Fatal("export table retained an unreachable view cycle")
	}
	owner := &bytearrayValue{buffer: buffer}
	view := newViewInstance(class, memoryView{buffer: buffer, owner: owner, length: 4, stride: 1})
	ownerReference := weak.Make(owner)
	owner = nil
	goruntime.GC()
	if ownerReference.Value() == nil || !buffer.hasViews() {
		t.Fatal("live view lost its exporter or export restriction")
	}
	view.io.view.released, view.io.view.owner, view.io.view.buffer = true, nil, nil
	if buffer.hasViews() {
		t.Fatal("explicit release retained the export restriction")
	}
	goruntime.KeepAlive(view)
}

func temporaryBufferView(class *typeValue, buffer *byteBuffer) weak.Pointer[instanceValue] {
	view := newViewInstance(class, memoryView{buffer: buffer, length: len(buffer.data), stride: 1})
	view.attributes.values["cycle"] = view
	return weak.Make(view)
}
