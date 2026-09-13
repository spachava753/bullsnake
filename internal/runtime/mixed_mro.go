package runtime

// compatibleScalarBases permits one integer or string payload with ordinary
// Python mixins. Other native or private layouts retain their rejection guards.
func compatibleScalarBases(bases []Value) bool {
	var layout *nativeTypeValue
	for _, base := range bases {
		var candidate *nativeTypeValue
		switch base := base.(type) {
		case *nativeTypeValue:
			if base != objectNativeType {
				candidate = base
			}
		case *typeValue:
			for _, parent := range base.mro {
				if _, slots := parent.namespace.get("__slots__"); slots {
					return false
				}
			}
			candidate = base.nativeClassBase()
			if candidate == nil && !rootAllocatableClass(base) {
				return false
			}
		default:
			return false
		}
		if candidate == nil {
			continue
		}
		if candidate != intNativeType && candidate != stringNativeType || layout != nil && candidate != layout {
			return false
		}
		layout = candidate
	}
	return layout != nil
}

// needsMixedMRO keeps native positions when combining a scalar layout with
// mixins or inheriting a class that already has an interleaved linearization.
func needsMixedMRO(bases []Value) bool {
	if len(bases) > 1 && compatibleScalarBases(bases) {
		return true
	}
	for _, base := range bases {
		if user, ok := base.(*typeValue); ok && user.mixedMRO != nil {
			return true
		}
	}
	return false
}

// classMROValues exposes the actual class order to the shared C3 merge. Legacy
// pure-Python linearizations retain their native ancestry at the end.
func classMROValues(value Value) []Value {
	if class, ok := value.(*typeValue); ok {
		if class.mixedMRO != nil {
			return class.mixedMRO
		}
		result := typeTuple(class.mro).elements
		if native := class.nativeClassBase(); native != nil {
			result = append(result, classMROValues(native)...)
		} else if class.exceptionBase == nil {
			result = append(result, objectNativeType)
		}
		return result
	}
	class := value.(*nativeTypeValue)
	result := []Value{class}
	if class.base != nil {
		return append(result, classMROValues(class.base)...)
	}
	if class != objectNativeType {
		result = append(result, objectNativeType)
	}
	return result
}

// lookupMixedMRO searches native namespaces at their C3 positions, rather than
// appending every native descriptor after every Python class namespace.
func (class *typeValue) lookupMixedMRO(start int, name string) (Value, bool) {
	for _, entry := range class.mixedMRO[start:] {
		switch entry := entry.(type) {
		case *typeValue:
			if value, found := entry.namespace.get(name); found {
				return value, true
			}
		case *nativeTypeValue:
			if dictionary := class.mixedNativeSlots[entry]; dictionary != nil {
				if value, found, _ := dictionary.get(&stringValue{value: name}); found {
					return value, true
				}
			}
		}
	}
	return nil, false
}

// mixedLayoutBase selects the most derived direct scalar-bearing base. Plain
// Python mixins do not determine where the immutable native payload is allocated.
func (class *typeValue) mixedLayoutBase() Value {
	var best Value
	layout := class.nativeClassBase()
	for _, base := range class.mixedBases {
		scalar := base == layout
		if user, ok := base.(*typeValue); ok {
			scalar = user.nativeClassBase() == layout
		}
		if !scalar {
			continue
		}
		if best == nil {
			best = base
			continue
		}
		if derived, _ := subclassMatchesClass(base, best); derived {
			best = base
		}
	}
	return best
}
