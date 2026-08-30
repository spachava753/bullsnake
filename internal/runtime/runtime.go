package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

// ModuleLoader finds immutable code for one absolute module name. A false found
// result means that the configured source does not contain the module.
type ModuleLoader func(name string) (code *bytecode.Code, found bool, err error)

// Runtime owns mutable interpreter state shared by executions in one isolated
// Python runtime instance.
type Runtime struct {
	builtins *Namespace
	modules  map[string]*Module
	prepared map[*bytecode.Code]*preparedCode
	loader   ModuleLoader
}

// New constructs an empty runtime instance without a module loader.
func New() *Runtime {
	return newRuntime(nil)
}

// NewWithLoader constructs an empty runtime that can load module code on cache
// misses.
func NewWithLoader(loader ModuleLoader) *Runtime {
	return newRuntime(loader)
}

func newRuntime(loader ModuleLoader) *Runtime {
	builtins := newNamespace()
	for _, exceptionType := range builtinExceptionTypes {
		builtins.values[exceptionType.name] = exceptionType
	}
	return &Runtime{
		builtins: builtins,
		modules:  make(map[string]*Module),
		prepared: make(map[*bytecode.Code]*preparedCode),
		loader:   loader,
	}
}

// ExecuteModule validates and executes one module code object. The module
// enters the cache before its body runs so imports can observe partial state.
func (runtime *Runtime) ExecuteModule(name string, code *bytecode.Code) (*Module, error) {
	module, moduleFrame, err := runtime.newModuleFrame(name, code, nil)
	if err != nil {
		return nil, err
	}
	previous, replaced := runtime.modules[name]
	runtime.modules[name] = module
	thread := &threadState{current: moduleFrame}

	_, raised, err := execute(thread)
	if err != nil {
		runtime.restoreModule(name, module, previous, replaced)
		return nil, err
	}
	if raised != nil {
		runtime.restoreModule(name, module, previous, replaced)
		return nil, &UncaughtException{
			exception: raised.exception,
			filename:  raised.frame.code.code.Filename(),
			span:      raised.frame.position(raised.instruction),
			traceback: raised.exception.tracebackFrames(),
		}
	}
	return module, nil
}

func (runtime *Runtime) newModuleFrame(
	name string,
	code *bytecode.Code,
	previous *frame,
) (*Module, *frame, error) {
	if code == nil {
		return nil, nil, &BytecodeError{
			Instruction: -1,
			Message:     "module loader returned nil code for " + name,
		}
	}
	prepared, err := runtime.prepare(code)
	if err != nil {
		return nil, nil, err
	}
	globals := newNamespace()
	globals.values["__name__"] = &stringValue{value: name}
	module := &Module{name: name, globals: globals}
	fastLocals := make([]Value, len(prepared.locals))
	deref, ok := initializeDeref(prepared, fastLocals, nil)
	if !ok {
		return nil, nil, prepared.failure(
			-1,
			"module closure has 0 cells for %d free variables",
			len(prepared.freeVars),
		)
	}
	return module, &frame{
		runtime:    runtime,
		code:       prepared,
		stack:      make([]Value, 0, prepared.stackSize),
		fastLocals: fastLocals,
		deref:      deref,
		locals:     globals,
		globals:    globals,
		builtins:   runtime.builtins,
		previous:   previous,
	}, nil
}

func (runtime *Runtime) restoreModule(
	name string,
	current *Module,
	previous *Module,
	replaced bool,
) {
	if runtime.modules[name] != current {
		return
	}
	if replaced {
		runtime.modules[name] = previous
	} else {
		delete(runtime.modules, name)
	}
}

// Module returns a cached module by name. A loader callback may observe a
// module whose body is still initializing.
func (runtime *Runtime) Module(name string) (*Module, bool) {
	module, ok := runtime.modules[name]
	return module, ok
}

func (runtime *Runtime) prepare(code *bytecode.Code) (*preparedCode, error) {
	if prepared, ok := runtime.prepared[code]; ok {
		return prepared, nil
	}
	prepared, err := prepareCode(code)
	if err != nil {
		return nil, err
	}
	prepared.children = make([]*preparedCode, len(prepared.childCodes))
	for index, child := range prepared.childCodes {
		preparedChild, childErr := runtime.prepare(child)
		if childErr != nil {
			return nil, childErr
		}
		prepared.children[index] = preparedChild
	}
	runtime.prepared[code] = prepared
	return prepared, nil
}
