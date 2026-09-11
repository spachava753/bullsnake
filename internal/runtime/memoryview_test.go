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

// TestProtocolExportOwnership checks that explicit buffer exports retain their
// source view, while the source's weak export records cannot retain children.
func TestProtocolExportOwnership(t *testing.T) {
	namespace := newNamespace()
	class := initializeMemoryViewClass(namespace)
	source := newViewInstance(class, memoryView{buffer: &byteBuffer{data: []byte("ab")}, length: 2, stride: 1})
	dead := temporaryProtocolExport(t, class, source)
	for range 10 {
		goruntime.GC()
		if dead.Value() == nil {
			break
		}
	}
	if dead.Value() != nil || source.hasProtocolExports() {
		t.Fatal("source retained an unreachable exported buffer cycle")
	}
	exported, exception := newProtocolBuffer(class, source, 0)
	if exception != nil {
		t.Fatal(exception)
	}
	if !source.hasProtocolExports() {
		t.Fatal("live protocol export did not pin its source")
	}
	reference := weak.Make(source)
	source = nil
	goruntime.GC()
	if reference.Value() == nil {
		t.Fatal("live protocol export lost its source view")
	}
	exported.io.view.released, exported.io.view.owner, exported.io.view.buffer = true, nil, nil
	for range 10 {
		goruntime.GC()
		if reference.Value() == nil {
			break
		}
	}
	if reference.Value() != nil {
		t.Fatal("released protocol export retained its source view")
	}
	goruntime.KeepAlive(exported)
}

func temporaryProtocolExport(t *testing.T, class *typeValue, source *instanceValue) weak.Pointer[instanceValue] {
	t.Helper()
	exported, exception := newProtocolBuffer(class, source, 0)
	if exception != nil {
		t.Fatal(exception)
	}
	exported.attributes.values["cycle"] = exported
	return weak.Make(exported)
}

func temporaryBufferView(class *typeValue, buffer *byteBuffer) weak.Pointer[instanceValue] {
	view := newViewInstance(class, memoryView{buffer: buffer, length: len(buffer.data), stride: 1})
	view.attributes.values["cycle"] = view
	return weak.Make(view)
}
