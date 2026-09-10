package runtime

import (
	goruntime "runtime"
	"testing"
)

// TestWeakClassOwnership exercises real class allocations with self-referencing
// MROs. A live registry or parent must not retain unreachable class cycles.
func testWeakClassOwnership(t *testing.T) {
	parent := &typeValue{namespace: newNamespace()}
	parent.mro = []*typeValue{parent}
	set := &weakClassSet{}
	dead := addTemporaryWeakClass(t, parent, set)
	for range 10 {
		goruntime.GC()
		if dead.value() == nil {
			break
		}
	}
	if dead.value() != nil {
		t.Fatal("registry or parent retained unreachable class cycle")
	}
	set.prune()
	parent.subclasses.prune()
	if len(set.entries) != 0 {
		t.Fatal("dead registry entry was not pruned")
	}
	if len(parent.subclasses.entries) != 0 {
		t.Fatal("parent retained dead subclass entry")
	}
	goruntime.KeepAlive(parent)
}

func addTemporaryWeakClass(t *testing.T, parent *typeValue, set *weakClassSet) weakClass {
	t.Helper()
	build := &classBuild{name: "Temporary", namespace: newNamespace(), bases: []*typeValue{parent}}
	value, exception := build.finish(None)
	if exception != nil {
		t.Fatal(exception)
	}
	set.entries = append(set.entries, makeWeakClass(value))
	return makeWeakClass(value)
}

func testWeakClassPromotion(t *testing.T) {
	class := &typeValue{namespace: newNamespace()}
	class.mro = []*typeValue{class}
	set := &weakClassSet{}
	set.entries = append(set.entries, makeWeakClass(class))
	if len(set.entries) != 1 || !set.contains(class) {
		t.Fatal("weak membership lost class identity")
	}
	instance := &instanceValue{class: class, attributes: newNamespace()}
	reference := makeWeakClass(class)
	class = nil
	goruntime.GC()
	if reference.value() != instance.class {
		t.Fatal("instance did not retain its class")
	}
	promoted := reference.value()
	instance = nil
	goruntime.GC()
	if reference.value() != promoted {
		t.Fatal("promoted reference did not retain class")
	}
	goruntime.KeepAlive(promoted)
}

func TestWeakClass(t *testing.T) {
	t.Run("ownership", testWeakClassOwnership)
	t.Run("promotion", testWeakClassPromotion)
}
