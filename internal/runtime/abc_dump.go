package runtime

import "math/big"

// classWeakReference is the callback-free class reference returned by ABC dumps.
// It owns a cached hash but no strong path to its target class.
type classWeakReference struct {
	target weakClass
	hash   int64
}

func (*classWeakReference) TypeName() string { return "weakref.ReferenceType" }
func (*classWeakReference) isValue()         {}
func (reference *classWeakReference) Repr() string {
	if class := reference.target.value(); class != nil {
		return "<weakref; to " + class.Repr() + ">"
	}
	return "<weakref; dead>"
}

// executeClassWeakReference promotes the target once and returns None after
// collection. These private diagnostic references never accept callbacks.
func executeClassWeakReference(caller *frame, instruction, base int, reference *classWeakReference, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	valid := len(arguments) == 0 && (keywords == nil || len(keywords.entries) == 0)
	discardCallSegment(caller, base)
	if !valid {
		return raiseOutcome(newException("TypeError", "weakref expected 0 arguments")), nil
	}
	value := reference.target.value()
	if value == nil {
		value = None
	}
	return pushOutcome(caller, instruction, value)
}

// dump supplies independent sets that share their reference objects, as CPython's
// shallow diagnostic copies do. Materializing a dump does not retain classes.
func (set *weakClassSet) dump() *setValue {
	set.prune()
	if set.references == nil {
		set.references = make(map[weakClass]*classWeakReference)
	}
	result := &setValue{}
	for _, key := range set.entries {
		class := key.value()
		if class == nil {
			continue
		}
		reference := set.references[key]
		if reference == nil {
			hash, _, _ := fixedValueHash(class)
			reference = &classWeakReference{target: key, hash: hash}
			set.references[key] = reference
		}
		result.entries = append(result.entries, reference)
	}
	return result
}

func dumpABC(caller *frame, instruction int, arguments []Value) (instructionOutcome, error) {
	return withABCData(caller, instruction, arguments[0], func(current *frame, data *abcData) (instructionOutcome, error) {
		return pushOutcome(current, instruction, &tupleValue{elements: []Value{
			data.registry.dump(), data.positive.dump(), data.negative.dump(),
			&intValue{value: *new(big.Int).SetUint64(data.version)},
		}})
	})
}
