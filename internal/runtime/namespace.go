package runtime

// Namespace stores the string-keyed bindings used by module execution. It is
// narrower than a Python dict until general mapping protocols are implemented.
type Namespace struct {
	values map[string]Value
}

func newNamespace() *Namespace {
	return &Namespace{values: make(map[string]Value)}
}

func (namespace *Namespace) get(name string) (Value, bool) {
	value, ok := namespace.values[name]
	return value, ok
}

// Module is one successfully executed module and its global namespace.
type Module struct {
	name    string
	globals *Namespace
}

// Name returns the runtime cache name of the module.
func (module *Module) Name() string { return module.name }

// Get returns one module global binding.
func (module *Module) Get(name string) (Value, bool) {
	return module.globals.get(name)
}
