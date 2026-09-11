package runtime

const (
	protocolLength uint8 = 1 << iota
	protocolIter
	protocolNext
	protocolContains
)

// nativeCollectionProtocols lists only protocols backed by executable native
// operations. It is immutable; each runtime allocates its own descriptors.
var nativeCollectionProtocols = map[string]uint8{
	"list":                 protocolLength | protocolIter | protocolContains,
	"tuple":                protocolLength | protocolIter | protocolContains,
	"dict":                 protocolLength | protocolIter | protocolContains,
	"str":                  protocolLength | protocolIter | protocolContains,
	"bytes":                protocolLength | protocolIter | protocolContains,
	"set":                  protocolLength | protocolIter | protocolContains,
	"frozenset":            protocolLength | protocolIter | protocolContains,
	"bytearray":            protocolLength | protocolIter,
	"range":                protocolLength | protocolIter,
	"dict_keys":            protocolLength | protocolIter,
	"dict_items":           protocolLength | protocolIter,
	"dict_values":          protocolLength | protocolIter,
	"mappingproxy":         protocolLength | protocolIter | protocolContains,
	"FrameLocalsProxy":     protocolLength | protocolIter | protocolContains,
	"list_iterator":        protocolIter | protocolNext,
	"tuple_iterator":       protocolIter | protocolNext,
	"str_iterator":         protocolIter | protocolNext,
	"bytes_iterator":       protocolIter | protocolNext,
	"bytearray_iterator":   protocolIter | protocolNext,
	"range_iterator":       protocolIter | protocolNext,
	"dict_keyiterator":     protocolIter | protocolNext,
	"dict_valueiterator":   protocolIter | protocolNext,
	"dict_itemiterator":    protocolIter | protocolNext,
	"set_iterator":         protocolIter | protocolNext,
	"list_reverseiterator": protocolIter | protocolNext,
	"reversed":             protocolIter | protocolNext,
	"enumerate":            protocolIter | protocolNext,
	"map":                  protocolIter | protocolNext,
	"filter":               protocolIter | protocolNext,
	"zip":                  protocolIter | protocolNext,
	"generator":            protocolIter | protocolNext,
	"memory_iterator":      protocolIter | protocolNext,
}

func nativeProtocolFlag(name string) uint8 {
	switch name {
	case "__len__":
		return protocolLength
	case "__iter__":
		return protocolIter
	case "__next__":
		return protocolNext
	case "__contains__":
		return protocolContains
	}
	return 0
}

func addNativeCollectionDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	for _, name := range []string{"__len__", "__iter__", "__next__", "__contains__"} {
		if nativeCollectionProtocols[class.name]&nativeProtocolFlag(name) == 0 {
			continue
		}
		key := &stringValue{value: name}
		if _, found, _ := dictionary.get(key); !found {
			dictionary.set(key, nativeCollectionDescriptor(class, name))
		}
	}
}

// nativeCollectionDescriptor validates an explicit native receiver before
// delegating to the same length, iteration, next, or containment operation.
func nativeCollectionDescriptor(class *nativeTypeValue, name string) Value {
	return &builtinFunctionValue{name: name, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		count := 1
		if name == "__contains__" {
			count++
		}
		if exception := checkNativeArguments(name, arguments, keywords, count, count); exception != nil {
			discardCallSegment(caller, base)
			return raiseOutcome(exception), nil
		}
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		actual, exception := typeOf(arguments[0])
		if exception != nil || actual != class {
			return raiseOutcome(newException("TypeError", "descriptor '"+name+"' requires a '"+class.name+"' object")), nil
		}
		switch name {
		case "__len__":
			return executeBuiltinLen(caller, instruction, len(caller.stack), arguments, nil)
		case "__iter__":
			return executeIteratorLookup(caller, instruction, arguments[0])
		case "__next__":
			return executeBuiltinNext(caller, instruction, len(caller.stack), arguments, nil)
		default:
			found, exception := containsValue(arguments[0], arguments[1])
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			result := falseSingleton
			if found {
				result = trueSingleton
			}
			return pushOutcome(caller, instruction, result)
		}
	}}
}

// boundNativeCollectionMethod preserves the existing set method representation
// and binds other implemented native protocols to their concrete receiver.
func (runtime *Runtime) boundNativeCollectionMethod(owner Value, name string) (Value, bool) {
	if name == "__contains__" {
		if target, ok := owner.(setContainsTarget); ok {
			return &setContainsMethod{target: target}, true
		}
	}
	flag := nativeProtocolFlag(name)
	if flag == 0 {
		return nil, false
	}
	actual, exception := typeOf(owner)
	class, ok := actual.(*nativeTypeValue)
	if exception != nil || !ok || nativeCollectionProtocols[class.name]&flag == 0 {
		return nil, false
	}
	method, found, _ := runtime.nativeNamespace(class).get(&stringValue{value: name})
	return &boundMethodValue{callable: method, self: owner}, found
}
