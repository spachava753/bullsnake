package runtime

import "sort"

// Namespace stores string-keyed bindings. Its optional dictionary is the live
// Python mapping once exposed or supplied by a class namespace preparer.
type Namespace struct {
	values     map[string]Value
	dictionary *dictValue
}

func newNamespace() *Namespace {
	return &Namespace{values: make(map[string]Value)}
}

func (namespace *Namespace) get(name string) (Value, bool) {
	if namespace.dictionary != nil {
		value, found, _ := namespace.dictionary.get(&stringValue{value: name})
		return value, found
	}
	value, ok := namespace.values[name]
	return value, ok
}

// asDictionary publishes one live dictionary and keeps map-backed consumers in
// sync through the dictionary's namespace link. Initial host names are sorted.
func (namespace *Namespace) asDictionary() *dictValue {
	if namespace.dictionary == nil {
		namespace.dictionary = &dictValue{namespace: namespace}
		names := make([]string, 0, len(namespace.values))
		for name := range namespace.values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			namespace.dictionary.set(&stringValue{value: name}, namespace.values[name])
		}
	}
	return namespace.dictionary
}

func (namespace *Namespace) store(name string, value Value) {
	namespace.asDictionary().set(&stringValue{value: name}, value)
	namespace.values[name] = value
}

func (namespace *Namespace) delete(name string) {
	if namespace.dictionary != nil {
		namespace.dictionary.delete(&stringValue{value: name})
	}
	delete(namespace.values, name)
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
