package runtime

type descriptorKind uint8

const (
	classMethodDescriptor descriptorKind = iota
	staticMethodDescriptor
	propertyDescriptor
)

type descriptorValue struct {
	class      *typeValue
	kind       descriptorKind
	callable   Value
	attributes *Namespace
}

func (descriptor *descriptorValue) TypeName() string { return descriptor.class.name }
func (descriptor *descriptorValue) Repr() string {
	return "<" + descriptor.class.name + "(" + descriptor.callable.Repr() + ")>"
}
func (*descriptorValue) isValue() {}

// attribute resolves callable metadata and property cloning helpers.
func (descriptor *descriptorValue) attribute(name string) (Value, bool) {
	if name == "__func__" || (descriptor.kind == propertyDescriptor && name == "fget") {
		return descriptor.callable, true
	}
	if name == "__wrapped__" && descriptor.kind != propertyDescriptor {
		return descriptor.callable, true
	}
	if descriptor.kind == propertyDescriptor {
		var attribute string
		switch name {
		case "getter":
			attribute = "fget"
		case "setter":
			attribute = "fset"
		case "deleter":
			attribute = "fdel"
		}
		if attribute != "" {
			return nativeFunctionNamed("property."+name, 1, 1,
				func(_ *frame, arguments []Value) (Value, *Exception, error) {
					updated := &descriptorValue{
						class:      descriptor.class,
						kind:       descriptor.kind,
						callable:   descriptor.callable,
						attributes: newNamespace(),
					}
					for key, value := range descriptor.attributes.values {
						updated.attributes.values[key] = value
					}
					if attribute == "fget" {
						updated.callable = arguments[0]
					} else {
						updated.attributes.values[attribute] = arguments[0]
					}
					return updated, nil, nil
				}), true
		}
	}
	if descriptor.attributes == nil {
		if descriptor.kind == propertyDescriptor && (name == "fset" || name == "fdel" || name == "__doc__") {
			return None, true
		}
		return nil, false
	}
	value, found := descriptor.attributes.get(name)
	if !found && descriptor.kind == propertyDescriptor && (name == "fset" || name == "fdel" || name == "__doc__") {
		return None, true
	}
	return value, found
}

// newDescriptorType creates a built-in descriptor class with constructor validation.
func newDescriptorType(name string, kind descriptorKind) *typeValue {
	class := &typeValue{
		name:          name,
		qualifiedName: name,
		module:        "builtins",
		namespace:     newNamespace(),
	}
	class.constructor = func(
		actualClass *typeValue,
		_ *frame,
		arguments []Value,
		keywords *dictValue,
	) (Value, *Exception, error) {
		if kind != propertyDescriptor && (len(arguments) != 1 || keywordCount(keywords) != 0) {
			return nil, newException("TypeError", name+"() takes exactly one argument"), nil
		}
		if kind == propertyDescriptor && len(arguments) > 4 {
			return nil, newException("TypeError", "property() takes at most 4 arguments"), nil
		}
		callable := Value(None)
		if len(arguments) != 0 {
			callable = arguments[0]
		}
		attributes := newNamespace()
		if len(arguments) >= 2 {
			attributes.values["fset"] = arguments[1]
		}
		if len(arguments) >= 3 {
			attributes.values["fdel"] = arguments[2]
		}
		if len(arguments) >= 4 {
			attributes.values["__doc__"] = arguments[3]
		}
		if keywords != nil {
			for _, entry := range keywords.entries {
				key, ok := entry.key.(*stringValue)
				if !ok || key.value != "doc" {
					return nil, newException("TypeError", "property() got an unexpected keyword argument"), nil
				}
				attributes.values["__doc__"] = entry.value
			}
		}
		return &descriptorValue{
			class:      actualClass,
			kind:       kind,
			callable:   callable,
			attributes: attributes,
		}, nil, nil
	}
	if kind == propertyDescriptor {
		class.namespace.values["__get__"] = nativeMethodNamed(
			"property.__get__", 2, 3,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				descriptor, ok := arguments[0].(*descriptorValue)
				if !ok {
					return nil, newException("TypeError", "property descriptor required"), nil
				}
				if arguments[1] == None {
					return descriptor, nil, nil
				}
				return callValueSynchronously(caller, descriptor.callable, []Value{arguments[1]})
			},
		)
		class.namespace.values["__set__"] = nativeMethodNamed(
			"property.__set__", 3, 3,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				descriptor, ok := arguments[0].(*descriptorValue)
				if !ok {
					return nil, newException("TypeError", "property descriptor required"), nil
				}
				setter, found := descriptor.attribute("fset")
				if !found || setter == None {
					return nil, newException("AttributeError", "property has no setter"), nil
				}
				return callValueSynchronously(caller, setter, arguments[1:])
			},
		)
	}
	return class
}

func keywordCount(keywords *dictValue) int {
	if keywords == nil {
		return 0
	}
	return len(keywords.entries)
}

var descriptorTypes []*typeValue

func init() {
	descriptorTypes = []*typeValue{
		newDescriptorType("classmethod", classMethodDescriptor),
		newDescriptorType("staticmethod", staticMethodDescriptor),
		newDescriptorType("property", propertyDescriptor),
	}
}

func bindCallable(callable Value, self Value) Value {
	switch callable := callable.(type) {
	case *functionValue:
		return &boundMethodValue{function: callable, self: self}
	case *nativeFunctionValue:
		if callable.bindReceiver {
			return &boundNativeMethodValue{function: callable, self: self}
		}
		return callable
	default:
		return callable
	}
}

// resolvePythonDescriptor invokes a user-defined descriptor's __get__ method.
// The boolean reports whether value implements the descriptor protocol.
func resolvePythonDescriptor(
	caller *frame,
	value Value,
	instance Value,
	owner Value,
) (Value, bool, *Exception, error) {
	descriptor, ok := value.(*instanceValue)
	if !ok {
		return value, false, nil, nil
	}
	getter, found := descriptor.class.lookup("__get__")
	if !found {
		return value, false, nil, nil
	}
	result, exception, err := callValueSynchronously(
		caller,
		bindCallable(getter, descriptor),
		[]Value{instance, owner},
	)
	return result, true, exception, err
}

func lookupBoundSpecialMethod(
	caller *frame,
	instance *instanceValue,
	name string,
) (Value, bool, *Exception, error) {
	method, found := instance.class.lookup(name)
	if !found {
		return nil, false, nil, nil
	}
	if resolved, descriptor, exception, err := resolvePythonDescriptor(
		caller, method, instance, instance.class,
	); descriptor {
		return resolved, true, exception, err
	}
	return bindCallable(method, instance), true, nil, nil
}

func bindDescriptorCallable(callable Value, self Value) Value {
	if native, ok := callable.(*nativeFunctionValue); ok {
		return &boundNativeMethodValue{function: native, self: self}
	}
	if function, ok := callable.(*functionValue); ok {
		return &boundMethodValue{function: function, self: self}
	}
	return &genericBoundMethodValue{callable: callable, self: self}
}

func bindClassAttribute(value Value, class *typeValue) Value {
	descriptor, ok := value.(*descriptorValue)
	if !ok {
		return value
	}
	switch descriptor.kind {
	case classMethodDescriptor:
		return bindDescriptorCallable(descriptor.callable, class)
	case staticMethodDescriptor:
		return descriptor.callable
	default:
		return descriptor
	}
}

var _ Value = (*descriptorValue)(nil)
