package runtime

import (
	"strconv"
	"strings"
)

type abstractMethodsStore struct {
	instruction int
	class       *typeValue
	value       Value
	returnNone  bool
}

// finishAbstractMethodsStore snapshots abstractness without propagating it to
// subclasses; ABCMeta is responsible for computing each subclass's own value.
func finishAbstractMethodsStore(frame *frame, call *abstractMethodsStore, abstract bool) (instructionOutcome, error) {
	call.class.namespace.values["__abstractmethods__"] = call.value
	call.class.abstract = abstract
	if call.returnNone {
		return pushOutcome(frame, call.instruction, None)
	}
	return instructionOutcome{kind: advance}, nil
}

// abstractAllocationError validates sorted names before producing CPython's
// singular or plural abstract-allocation diagnostic.
func abstractAllocationError(class *typeValue, items []sortItem) *Exception {
	names := make([]string, len(items))
	for index, item := range items {
		name, ok := item.value.(*stringValue)
		if !ok {
			return newException("TypeError", "sequence item "+strconv.Itoa(index)+": expected str instance, "+item.value.TypeName()+" found")
		}
		names[index] = name.value
	}
	plural := ""
	if len(names) > 1 {
		plural = "s"
	}
	return newException("TypeError", "Can't instantiate abstract class "+class.name+" without an implementation for abstract method"+plural+" '"+strings.Join(names, "', '")+"'")
}
