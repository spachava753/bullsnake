package runtime

import (
	"reflect"
	"sort"
)

// builtinTypeMethod selects the Go-backed descriptor for one built-in type slot.
func builtinTypeMethod(typeName, name string) (Value, bool) {
	key := typeName + "." + name
	if method, found := builtinTypeMethodCache[key]; found {
		return method, true
	}
	method, found := buildBuiltinTypeMethod(typeName, name)
	if found {
		builtinTypeMethodCache[key] = method
	}
	return method, found
}

var builtinTypeMethodCache = make(map[string]Value)

// buildBuiltinTypeMethod constructs the native descriptor for one supported
// built-in type slot, including collection and object protocol operations.
func buildBuiltinTypeMethod(typeName, name string) (Value, bool) {
	var minimum, maximum int
	var function nativeFunction
	switch typeName + "." + name {
	case "dict.__new__":
		minimum, maximum, function = 1, 1, builtinDictNew
	case "dict.__init__":
		minimum, maximum, function = 1, 2, builtinDictInit
	case "dict.__setitem__":
		minimum, maximum, function = 3, 3, builtinDictSetItem
	case "dict.__delitem__":
		minimum, maximum, function = 2, 2, builtinDictDeleteItem
	case "dict.pop":
		minimum, maximum, function = 2, 3, builtinDictPop
	case "dict.clear":
		minimum, maximum, function = 1, 1, func(_ *frame, arguments []Value) (Value, *Exception, error) {
			dictionary, ok := dictionaryOperand(arguments[0])
			if !ok {
				return nil, newException("TypeError", "descriptor 'clear' requires a 'dict' object"), nil
			}
			dictionary.entries = nil
			dictionary.version++
			return None, nil, nil
		}
	case "dict.__eq__":
		minimum, maximum, function = 2, 2, builtinObjectEqual
	case "dict.get":
		minimum, maximum, function = 2, 3, func(_ *frame, arguments []Value) (Value, *Exception, error) {
			dictionary, ok := dictionaryOperand(arguments[0])
			if namespace, namespaceOK := arguments[0].(*namespaceValue); namespaceOK {
				dictionary, ok = namespace.dictionary(), true
			}
			if !ok {
				return nil, newException("TypeError", "descriptor 'get' requires a 'dict' object"), nil
			}
			return dictionary.getMethod(nil, arguments[1:])
		}
	case "dict.fromkeys":
		minimum, maximum, function = 1, 2, builtinDictFromKeys
	case "tuple.__new__":
		minimum, maximum, function = 1, 2, builtinTupleNew
	case "list.__contains__":
		minimum, maximum, function = 2, 2, builtinContainsMethod
	case "str.maketrans":
		minimum, maximum, function = 1, 3, builtinStringMakeTrans
	case "float.__getformat__":
		minimum, maximum, function = 1, 1, func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &stringValue{value: "IEEE, little-endian"}, nil, nil
		}
	case "object.__repr__":
		minimum, maximum, function = 1, 1, builtinObjectRepr
	case "object.__str__":
		minimum, maximum, function = 1, 1, builtinObjectStr
	case "object.__getattribute__":
		minimum, maximum, function = 2, 2, builtinGetAttr
	case "object.__setattr__":
		minimum, maximum, function = 3, 3, builtinObjectSetAttr
	case "object.__delattr__":
		minimum, maximum, function = 2, 2, builtinObjectDelAttr
	case "object.__init__":
		minimum, maximum, function = 1, 1, func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		}
	case "object.__init_subclass__":
		minimum, maximum, function = 1, 1, func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		}
	case "object.__hash__":
		minimum, maximum, function = 1, 1, builtinObjectHash
	case "object.__sizeof__":
		minimum, maximum, function = 1, 1, builtinObjectInteger
	case "object.__dir__":
		minimum, maximum, function = 1, 1, builtinObjectDir
	case "object.__ne__":
		minimum, maximum, function = 2, 2, builtinObjectNotEqual
	case "member_descriptor.__get__":
		minimum, maximum, function = 2, 3, func(_ *frame, arguments []Value) (Value, *Exception, error) {
			descriptor, descriptorOK := arguments[0].(*memberDescriptorValue)
			instance, instanceOK := arguments[1].(*instanceValue)
			if !descriptorOK {
				return nil, newException("TypeError", "member descriptor required"), nil
			}
			if arguments[1] == None {
				return descriptor, nil, nil
			}
			if !instanceOK {
				return nil, newException("TypeError", "descriptor does not apply to object"), nil
			}
			value, found := instance.attributes.get(descriptor.name)
			if !found {
				return nil, newException("AttributeError", "object has no attribute '"+descriptor.name+"'"), nil
			}
			return value, nil, nil
		}
	case "member_descriptor.__set__":
		minimum, maximum, function = 3, 3, func(_ *frame, arguments []Value) (Value, *Exception, error) {
			descriptor, descriptorOK := arguments[0].(*memberDescriptorValue)
			instance, instanceOK := arguments[1].(*instanceValue)
			if !descriptorOK || !instanceOK {
				return nil, newException("TypeError", "descriptor does not apply to object"), nil
			}
			instance.attributes.values[descriptor.name] = arguments[2]
			return None, nil, nil
		}
	case "member_descriptor.__delete__":
		minimum, maximum, function = 2, 2, func(_ *frame, arguments []Value) (Value, *Exception, error) {
			descriptor, descriptorOK := arguments[0].(*memberDescriptorValue)
			instance, instanceOK := arguments[1].(*instanceValue)
			if !descriptorOK || !instanceOK {
				return nil, newException("TypeError", "descriptor does not apply to object"), nil
			}
			if _, found := instance.attributes.values[descriptor.name]; !found {
				return nil, newException("AttributeError", "object has no attribute '"+descriptor.name+"'"), nil
			}
			delete(instance.attributes.values, descriptor.name)
			return None, nil, nil
		}
	default:
		if name == "__repr__" {
			return nativeMethodNamed(typeName+"."+name, 1, 1, builtinObjectRepr), true
		}
		if name == "__str__" {
			return nativeMethodNamed(typeName+"."+name, 1, 1, builtinObjectStr), true
		}
		if name == "__hash__" {
			return nativeMethodNamed(typeName+".__hash__", 1, 1, builtinObjectInteger), true
		}
		if name != "__new__" {
			return nil, false
		}
		return nativeMethodNamed(typeName+".__new__", 1, -1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				if typeName == "object" {
					if class, ok := arguments[0].(*typeValue); ok {
						return &instanceValue{class: class, attributes: newNamespace()}, nil, nil
					}
					return &objectValue{}, nil, nil
				}
				value, exception, err := constructBuiltinValue(caller.runtime, typeName, arguments[1:])
				if err != nil {
					return nil, nil, err
				}
				if value == nil && exception == nil {
					exception = newException("TypeError", typeName+".__new__ is not implemented")
				}
				return value, exception, nil
			}), true
	}
	return nativeMethodNamed(typeName+"."+name, minimum, maximum, function), true
}

// builtinDictFromKeys builds a dictionary from every key in an iterable.
func builtinDictFromKeys(_ *frame, arguments []Value) (Value, *Exception, error) {
	iterator, ok := newIterator(arguments[0])
	if !ok {
		return nil, newException("TypeError", "fromkeys argument is not iterable"), nil
	}
	value := Value(None)
	if len(arguments) == 2 {
		value = arguments[1]
	}
	dictionary := &dictValue{}
	for {
		key, available, exception := iterator.next()
		if exception != nil {
			return nil, exception, nil
		}
		if !available {
			return dictionary, nil, nil
		}
		if exception := dictionary.set(key, value); exception != nil {
			return nil, exception, nil
		}
	}
}

func builtinStringMakeTrans(_ *frame, _ []Value) (Value, *Exception, error) {
	return &dictValue{}, nil, nil
}

func builtinDictNew(_ *frame, arguments []Value) (Value, *Exception, error) {
	if len(arguments) != 0 {
		if class, ok := arguments[0].(*typeValue); ok {
			return &instanceValue{
				class: class, attributes: newNamespace(), mapping: &dictValue{},
			}, nil, nil
		}
	}
	return &dictValue{}, nil, nil
}

// builtinDictInit resets a concrete or subclass-backed dictionary and copies
// entries from the optional mapping argument.
func builtinDictInit(_ *frame, arguments []Value) (Value, *Exception, error) {
	dictionary, ok := dictionaryOperand(arguments[0])
	if !ok {
		return nil, newException("TypeError", "dict.__init__ requires a dict"), nil
	}
	dictionary.entries = nil
	dictionary.version++
	if len(arguments) == 1 {
		return None, nil, nil
	}
	source, ok := dictionaryOperand(arguments[1])
	if namespace, namespaceOK := arguments[1].(*namespaceValue); namespaceOK {
		source, ok = namespace.dictionary(), true
	}
	if !ok {
		return nil, newException("TypeError", "dict update argument is not a mapping"), nil
	}
	for _, entry := range source.entries {
		if exception := dictionary.set(entry.key, entry.value); exception != nil {
			return nil, exception, nil
		}
	}
	return None, nil, nil
}

func builtinDictSetItem(_ *frame, arguments []Value) (Value, *Exception, error) {
	dictionary, ok := dictionaryOperand(arguments[0])
	if !ok {
		return nil, newException("TypeError", "dict.__setitem__ requires a dict"), nil
	}
	if exception := dictionary.set(arguments[1], arguments[2]); exception != nil {
		return nil, exception, nil
	}
	return None, nil, nil
}

// builtinDictPop removes a matching key while honoring the optional default.
func builtinDictPop(_ *frame, arguments []Value) (Value, *Exception, error) {
	dictionary, ok := dictionaryOperand(arguments[0])
	if !ok {
		return nil, newException("TypeError", "dict.pop requires a dict"), nil
	}
	for index, entry := range dictionary.entries {
		if entry.key == arguments[1] || valuesEqual(entry.key, arguments[1]) {
			dictionary.entries = append(dictionary.entries[:index], dictionary.entries[index+1:]...)
			dictionary.version++
			return entry.value, nil, nil
		}
	}
	if len(arguments) == 3 {
		return arguments[2], nil, nil
	}
	return nil, newException("KeyError", arguments[1].Repr()), nil
}

func dictionaryOperand(value Value) (*dictValue, bool) {
	switch value := value.(type) {
	case *dictValue:
		return value, true
	case *instanceValue:
		return value.mapping, value.mapping != nil
	default:
		return nil, false
	}
}

func builtinDictDeleteItem(_ *frame, arguments []Value) (Value, *Exception, error) {
	dictionary, ok := dictionaryOperand(arguments[0])
	if !ok {
		return nil, newException("TypeError", "dict.__delitem__ requires a dict"), nil
	}
	deleted, exception := dictionary.delete(arguments[1])
	if exception != nil {
		return nil, exception, nil
	}
	if !deleted {
		return nil, newException("KeyError", arguments[1].Repr()), nil
	}
	return None, nil, nil
}

func builtinTupleNew(caller *frame, arguments []Value) (Value, *Exception, error) {
	if class, ok := arguments[0].(*typeValue); ok {
		value, exception, err := constructBuiltinCollection(caller.runtime, "tuple", arguments[1:])
		if err != nil || exception != nil {
			return nil, exception, err
		}
		return &instanceValue{
			class: class, attributes: newNamespace(), tuple: value.(*tupleValue),
		}, nil, nil
	}
	if len(arguments) == 1 {
		return &tupleValue{}, nil, nil
	}
	return constructBuiltinCollection(caller.runtime, "tuple", arguments[1:])
}

// builtinContainsMethod checks list and list-subclass elements through Python's
// rich equality protocol.
func builtinContainsMethod(caller *frame, arguments []Value) (Value, *Exception, error) {
	container, ok := arguments[0].(*listValue)
	if instance, instanceOK := arguments[0].(*instanceValue); instanceOK && instance.sequence != nil {
		container, ok = instance.sequence, true
	}
	if !ok {
		return nil, newException("TypeError", "descriptor requires a list"), nil
	}
	for _, value := range container.elements {
		equal, exception, err := valuesEqualForFrame(caller, value, arguments[1])
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if equal {
			return trueSingleton, nil, nil
		}
	}
	return falseSingleton, nil, nil
}

func builtinObjectEqual(_ *frame, arguments []Value) (Value, *Exception, error) {
	return pythonBool(valuesEqual(arguments[0], arguments[1])), nil, nil
}

func builtinObjectNotEqual(_ *frame, arguments []Value) (Value, *Exception, error) {
	return pythonBool(!valuesEqual(arguments[0], arguments[1])), nil, nil
}

func builtinObjectRepr(_ *frame, arguments []Value) (Value, *Exception, error) {
	return &stringValue{value: arguments[0].Repr()}, nil, nil
}

func builtinObjectStr(caller *frame, arguments []Value) (Value, *Exception, error) {
	return builtinRepr(caller, arguments)
}

func builtinObjectHash(_ *frame, arguments []Value) (Value, *Exception, error) {
	return newInt64(int64(reflect.ValueOf(arguments[0]).Pointer())), nil, nil
}

func builtinObjectInteger(_ *frame, _ []Value) (Value, *Exception, error) {
	return newInt64(1), nil, nil
}

// builtinObjectDir merges instance, class, and intrinsic object names into a
// deterministic directory listing.
func builtinObjectDir(_ *frame, arguments []Value) (Value, *Exception, error) {
	instance, ok := arguments[0].(*instanceValue)
	if !ok {
		return &listValue{}, nil, nil
	}
	names := make(map[string]struct{})
	for name := range instance.attributes.values {
		names[name] = struct{}{}
	}
	for _, class := range instance.class.methodResolutionOrder() {
		for name := range class.namespace.values {
			names[name] = struct{}{}
		}
	}
	for _, name := range []string{
		"__bases__", "__class__", "__delattr__", "__dict__", "__dir__", "__doc__", "__flags__", "__mro__", "__name__",
		"__getattribute__", "__init__", "__new__", "__repr__", "__setattr__", "__str__",
	} {
		names[name] = struct{}{}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	values := make([]Value, len(ordered))
	for index, name := range ordered {
		values[index] = &stringValue{value: name}
	}
	return &listValue{elements: values}, nil, nil
}

// builtinObjectSetAttr applies data-descriptor setters before writing directly
// to a user instance namespace.
func builtinObjectSetAttr(caller *frame, arguments []Value) (Value, *Exception, error) {
	instance, ok := arguments[0].(*instanceValue)
	name, nameOK := arguments[1].(*stringValue)
	if !ok || !nameOK {
		return nil, newException("TypeError", "attribute name must be string"), nil
	}
	if attribute, found := instance.class.lookup(name.value); found {
		if descriptor, ok := attribute.(*descriptorValue); ok && descriptor.kind == propertyDescriptor {
			setter, setterFound := descriptor.attribute("fset")
			if !setterFound || setter == None {
				return nil, newException(
					"AttributeError", "property '"+name.value+"' of '"+instance.TypeName()+"' object has no setter",
				), nil
			}
			return callValueSynchronously(caller, setter, []Value{instance, arguments[2]})
		}
		if descriptor, ok := attribute.(*instanceValue); ok {
			setter, found, exception, err := lookupBoundSpecialMethod(caller, descriptor, "__set__")
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if found {
				return callValueSynchronously(caller, setter, []Value{instance, arguments[2]})
			}
		}
	}
	instance.attributes.values[name.value] = arguments[2]
	return None, nil, nil
}

func builtinObjectDelAttr(_ *frame, arguments []Value) (Value, *Exception, error) {
	instance, ok := arguments[0].(*instanceValue)
	name, nameOK := arguments[1].(*stringValue)
	if !ok || !nameOK {
		return nil, newException("TypeError", "attribute name must be string"), nil
	}
	if _, found := instance.attributes.values[name.value]; !found {
		return nil, newException("AttributeError", "object has no attribute '"+name.value+"'"), nil
	}
	delete(instance.attributes.values, name.value)
	return None, nil, nil
}
