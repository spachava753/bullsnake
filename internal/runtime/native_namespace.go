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
		dictionary.set(&stringValue{value: "__subclasshook__"}, defaultSubclassHook())
	}
	if class == setNativeType || class == frozenSetNativeType {
		dictionary.set(&stringValue{value: "__contains__"}, &setContainsDescriptor{frozen: class == frozenSetNativeType})
	}
	if hasNativeClassGetitem(class) {
		dictionary.set(&stringValue{value: "__class_getitem__"}, nativeClassGetitem(class))
	}
	return dictionary
}

// nativeClassAttribute walks native ancestry and binds class methods to the
// original receiver while returning ordinary descriptors without binding.
func (runtime *Runtime) nativeClassAttribute(class *nativeTypeValue, name string) (Value, bool) {
	for current := class; current != nil; {
		value, found, _ := runtime.nativeNamespace(current).get(&stringValue{value: name})
		if found {
			if name == "__class_getitem__" || name == "__subclasshook__" {
				return &boundMethodValue{callable: value, self: class}, true
			}
			return value, true
		}
		if current.base != nil {
			current = current.base
		} else if current != objectNativeType {
			current = objectNativeType
		} else {
			current = nil
		}
	}
	return nil, false
}
