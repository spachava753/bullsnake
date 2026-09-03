package runtime

// Namespace stores the string-keyed bindings used by module execution. It is
// narrower than a Python dict until general mapping protocols are implemented.
type Namespace struct {
	values map[string]Value
	order  []string
}

type namespaceValue struct {
	namespace *Namespace
}

func (*namespaceValue) TypeName() string { return "dict" }
func (*namespaceValue) Repr() string     { return "<class namespace>" }
func (*namespaceValue) isValue()         {}

// attribute exposes the mapping operations supported by live namespaces.
func (mapping *namespaceValue) attribute(name string) (Value, bool) {
	switch name {
	case "keys", "values", "items":
		kind := dictKeysView
		if name == "values" {
			kind = dictValuesView
		} else if name == "items" {
			kind = dictItemsView
		}
		return nativeFunctionNamed("dict."+name, 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &dictViewValue{dictionary: mapping.dictionary(), kind: kind}, nil, nil
			}), true
	case "get":
		return nativeFunctionNamed("dict.get", 1, 2, mapping.get), true
	case "update":
		return nativeFunctionNamed("dict.update", 1, 1, mapping.update), true
	default:
		return nil, false
	}
}

func (mapping *namespaceValue) dictionary() *dictValue {
	dictionary := &dictValue{}
	for name, value := range mapping.namespace.values {
		dictionary.entries = append(dictionary.entries, dictEntry{
			key:   &stringValue{value: name},
			value: value,
		})
	}
	return dictionary
}

func (mapping *namespaceValue) get(_ *frame, arguments []Value) (Value, *Exception, error) {
	name, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "namespace keys must be strings"), nil
	}
	if value, found := mapping.namespace.get(name.value); found {
		return value, nil, nil
	}
	if len(arguments) == 2 {
		return arguments[1], nil, nil
	}
	return None, nil, nil
}

// update copies string-keyed values from a namespace or dictionary.
func (mapping *namespaceValue) update(_ *frame, arguments []Value) (Value, *Exception, error) {
	if source, ok := arguments[0].(*namespaceValue); ok {
		for name, value := range source.namespace.values {
			mapping.namespace.values[name] = value
		}
		return None, nil, nil
	}
	dictionary, ok := arguments[0].(*dictValue)
	if !ok {
		return nil, newException("TypeError", "namespace update requires a dict"), nil
	}
	for _, entry := range dictionary.entries {
		name, stringKey := entry.key.(*stringValue)
		if !stringKey {
			return nil, newException("TypeError", "namespace keys must be strings"), nil
		}
		mapping.namespace.values[name.value] = entry.value
	}
	return None, nil, nil
}

func newNamespace() *Namespace {
	return &Namespace{values: make(map[string]Value)}
}

func (namespace *Namespace) get(name string) (Value, bool) {
	value, ok := namespace.values[name]
	return value, ok
}

// Module is one module object and its global namespace. The runtime may cache
// it while its body is still initializing.
type Module struct {
	name            string
	globals         *Namespace
	isPackage       bool
	searchLocations []string
}

// Name returns the runtime cache name of the module.
func (module *Module) Name() string { return module.name }

func (*Module) TypeName() string { return "module" }
func (module *Module) Repr() string {
	return "<module '" + module.name + "'>"
}
func (*Module) isValue() {}

// Get returns one module global binding.
func (module *Module) Get(name string) (Value, bool) {
	return module.globals.get(name)
}
