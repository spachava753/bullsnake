package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

// Runtime owns mutable interpreter state shared by executions in one isolated
// Python runtime instance.
type Runtime struct {
	builtins *Namespace
	modules  map[string]*Module
	prepared map[*bytecode.Code]*preparedCode
}

// New constructs an empty runtime instance.
func New() *Runtime {
	builtins := newNamespace()
	for _, exceptionType := range builtinExceptionTypes {
		builtins.values[exceptionType.name] = exceptionType
	}
	return &Runtime{
		builtins: builtins,
		modules:  make(map[string]*Module),
		prepared: make(map[*bytecode.Code]*preparedCode),
	}
}

// ExecuteModule validates and executes one module code object. A module enters
// the runtime cache only after its body returns normally.
func (runtime *Runtime) ExecuteModule(name string, code *bytecode.Code) (*Module, error) {
	prepared, err := runtime.prepare(code)
	if err != nil {
		return nil, err
	}
	globals := newNamespace()
	globals.values["__name__"] = &stringValue{value: name}
	module := &Module{name: name, globals: globals}
	fastLocals := make([]Value, len(prepared.locals))
	deref, ok := initializeDeref(prepared, fastLocals, nil)
	if !ok {
		return nil, prepared.failure(
			-1,
			"module closure has 0 cells for %d free variables",
			len(prepared.freeVars),
		)
	}
	frame := &frame{
		runtime:    runtime,
		code:       prepared,
		stack:      make([]Value, 0, prepared.stackSize),
		fastLocals: fastLocals,
		deref:      deref,
		locals:     globals,
		globals:    globals,
		builtins:   runtime.builtins,
	}
	thread := &threadState{current: frame}

	_, raised, err := execute(thread)
	if err != nil {
		return nil, err
	}
	if raised != nil {
		return nil, &UncaughtException{
			exception: raised.exception,
			filename:  raised.frame.code.code.Filename(),
			span:      raised.frame.position(raised.instruction),
		}
	}
	runtime.modules[name] = module
	return module, nil
}

// Module returns a successfully executed module by cache name.
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
