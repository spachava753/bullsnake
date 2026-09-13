package runtime

// nativeNamespace retains the implemented native descriptors per runtime.
// Inherited attributes are resolved separately rather than copied into __dict__.
func (runtime *Runtime) nativeNamespace(class *nativeTypeValue) *dictValue {
	if dictionary, found := runtime.nativeNamespaces[class]; found {
		return dictionary
	}
	dictionary := &dictValue{}
	runtime.nativeNamespaces[class] = dictionary
	if class == typeNativeType {
		for _, name := range []string{"__new__", "__init__", "__prepare__", "__subclasses__", "__instancecheck__", "__subclasscheck__"} {
			method, _ := nativeMetaclassMethod(name)
			dictionary.set(&stringValue{value: name}, method)
		}
	}
	if class == objectNativeType {
		dictionary.set(&stringValue{value: "__str__"}, &nativeDescriptorValue{class: objectNativeType, name: "__str__", call: executeObjectString})
		dictionary.set(&stringValue{value: "__repr__"}, &nativeDescriptorValue{class: objectNativeType, name: "__repr__", call: executeObjectRepresentation})
		dictionary.set(&stringValue{value: "__init__"}, &nativeDescriptorValue{class: objectNativeType, name: "__init__", call: executeObjectInit})
		dictionary.set(&stringValue{value: "__subclasshook__"}, defaultSubclassHook())
	}
	if class == dictNativeType {
		dictionary.set(&stringValue{value: "fromkeys"}, &nativeDescriptorValue{kind: nativeClassMethodDescriptor, class: class, name: "fromkeys", call: executeDictionaryFromkeys})
	}
	if class == stringNativeType {
		dictionary.set(&stringValue{value: "join"}, &nativeDescriptorValue{kind: nativeMethodDescriptor, class: class, name: "join", call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeStringJoinCall(caller, instruction, len(caller.stack), &stringJoinMethod{separator: self.(*stringValue)}, arguments, keywords)
		}})
	}
	if class == setNativeType || class == frozenSetNativeType {
		dictionary.set(&stringValue{value: "__contains__"}, &setContainsDescriptor{frozen: class == frozenSetNativeType})
	}
	if hasNativeClassGetitem(class) {
		dictionary.set(&stringValue{value: "__class_getitem__"}, nativeClassGetitem(class))
	}
	addNativeDataDescriptors(class, dictionary)
	addNativeBufferDescriptors(class, dictionary)
	addNativeHashCallDescriptors(class, dictionary)
	addNativeCollectionDescriptors(class, dictionary)
	return dictionary
}

// nativeClassAttribute walks native ancestry and binds class methods to the
// original receiver while returning ordinary descriptors without binding.
func (runtime *Runtime) nativeClassAttribute(class *nativeTypeValue, name string) (Value, bool) {
	for current := class; current != nil; {
		value, found, _ := runtime.nativeNamespace(current).get(&stringValue{value: name})
		if found {
			if descriptor, ok := value.(*nativeDescriptorValue); ok && descriptor.kind == nativeClassMethodDescriptor {
				return &boundNativeDescriptorValue{descriptor: descriptor, self: class}, true
			}
			if name == "__class_getitem__" || name == "__subclasshook__" {
				return &boundMethodValue{callable: value, self: class}, true
			}
			return value, true
		}
		if current.base != nil {
			current = current.base
		} else if current != objectNativeType {
			if name == "__repr__" || name == "__str__" {
				return nil, false
			}
			if name == "__init__" && nativeHasOwnInitializer(class) {
				return nil, false
			}
			current = objectNativeType
		} else {
			current = nil
		}
	}
	return nil, false
}
