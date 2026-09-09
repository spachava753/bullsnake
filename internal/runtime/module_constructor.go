package runtime

import "fmt"

// moduleConstructor initializes a cached module without a Python code object.
// Constructors are private, synchronous, and may return Python or Go failures.
type moduleConstructor struct {
	isPackage  bool
	initialize func(*Runtime, *Module) (*Exception, error)
}

// registerModule rejects duplicate names, including existing bootstrap modules.
func (runtime *Runtime) registerModule(name string, constructor moduleConstructor) error {
	if name == "" || constructor.initialize == nil {
		return fmt.Errorf("invalid module constructor for %q", name)
	}
	if _, exists := runtime.constructors[name]; exists {
		return fmt.Errorf("duplicate module constructor for %q", name)
	}
	if _, exists := runtime.modules[name]; exists {
		return fmt.Errorf("module %q is already cached", name)
	}
	runtime.constructors[name] = constructor
	return nil
}

// initializeModule publishes identity before initialization and removes only
// that identity on failure, preserving successfully initialized dependencies.
func (runtime *Runtime) initializeModule(name string, constructor moduleConstructor) (*Module, *Exception, error) {
	if module, found := runtime.modules[name]; found {
		return module, nil, nil
	}
	module := newModule(name, ModuleSpec{IsPackage: constructor.isPackage})
	runtime.modules[name] = module
	exception, err := constructor.initialize(runtime, module)
	if exception != nil || err != nil {
		runtime.restoreModule(name, module, nil, false)
		return nil, exception, err
	}
	return module, nil, nil
}
