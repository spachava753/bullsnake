package runtime

import "weak"

// weakClass targets the class allocation itself, never an interface box with a
// separate lifetime. Native and exception classes use the same weak contract.
type weakClass struct {
	user      weak.Pointer[typeValue]
	native    weak.Pointer[nativeTypeValue]
	exception weak.Pointer[exceptionTypeValue]
}

func makeWeakClass(value Value) weakClass {
	switch class := value.(type) {
	case *typeValue:
		return weakClass{user: weak.Make(class)}
	case *nativeTypeValue:
		return weakClass{native: weak.Make(class)}
	case *exceptionTypeValue:
		return weakClass{exception: weak.Make(class)}
	default:
		return weakClass{}
	}
}

// value promotes one weak pointer to a strong reference for the caller's use.
// A cleared reference returns nil, not an interface containing a nil pointer.
func (reference weakClass) value() Value {
	if class := reference.user.Value(); class != nil {
		return class
	}
	if class := reference.native.Value(); class != nil {
		return class
	}
	if class := reference.exception.Value(); class != nil {
		return class
	}
	return nil
}

// weakClassSet retains insertion order but never owns its member classes.
// Dead entries are pruned synchronously on access; no cleanup callback runs.
type weakClassSet struct {
	entries    []weakClass
	references map[weakClass]*classWeakReference
}

func (set *weakClassSet) prune() {
	live := set.entries[:0]
	for _, entry := range set.entries {
		if entry.value() != nil {
			live = append(live, entry)
		} else {
			delete(set.references, entry)
		}
	}
	clear(set.entries[len(live):])
	set.entries = live
}

func (set *weakClassSet) contains(class Value) bool {
	set.prune()
	key := makeWeakClass(class)
	for _, entry := range set.entries {
		if entry == key {
			return true
		}
	}
	return false
}

// userSubclasses returns a strong snapshot of immediate user subclasses.
// Native base enumeration remains outside this initial class-tree slice.
func userSubclasses(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if len(arguments) != 1 || (keywords != nil && len(keywords.entries) != 0) {
		return nil, newException("TypeError", "__subclasses__() requires one positional receiver")
	}
	class, ok := arguments[0].(*typeValue)
	if !ok {
		return nil, newException("TypeError", "__subclasses__() requires a user class receiver")
	}
	result := &listValue{}
	for _, reference := range class.subclasses.snapshot() {
		if child := reference.value(); child != nil {
			result.elements = append(result.elements, child)
		}
	}
	return result, nil
}

func (set *weakClassSet) add(class Value) {
	if !set.contains(class) {
		set.entries = append(set.entries, makeWeakClass(class))
	}
}

func (set *weakClassSet) snapshot() []weakClass {
	set.prune()
	return append([]weakClass(nil), set.entries...)
}

func (set *weakClassSet) reset() {
	set.entries = nil
	set.references = nil
}
