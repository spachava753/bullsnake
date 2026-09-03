package runtime

import (
	"slices"
	"strings"

	"github.com/spachava753/bullsnake/host"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// ModuleSpec describes immutable code and import metadata supplied by a loader.
type ModuleSpec struct {
	Code            *bytecode.Code
	IsPackage       bool
	Origin          string
	SearchLocations []string
}

// ModuleRequest names a module and the parent package locations that may
// contain it. SearchLocations is nil for a top-level import.
type ModuleRequest struct {
	Name            string
	SearchLocations []string
}

// ModuleLoader finds one absolute module description.
type ModuleLoader func(request ModuleRequest) (spec ModuleSpec, found bool, err error)

// Config supplies module discovery, import paths, and explicit host
// capabilities to one runtime. A zero Host grants no ambient capabilities.
type Config struct {
	Loader ModuleLoader
	Path   []string
	Host   host.Services
}

// Runtime owns mutable interpreter state shared by executions in one isolated
// Python runtime instance.
type Runtime struct {
	builtins  *Namespace
	modules   map[string]*Module
	moduleMap *dictValue
	prepared  map[*bytecode.Code]*preparedCode
	loader    ModuleLoader
	path      []string
	host      host.Services
}

// New constructs an empty runtime instance without a module loader.
func New() *Runtime {
	return newRuntime(Config{Host: host.Default()})
}

// NewWithLoader constructs an empty runtime that can load module code on cache
// misses.
func NewWithLoader(loader ModuleLoader) *Runtime {
	return newRuntime(Config{Loader: loader, Host: host.Default()})
}

// NewWithConfig constructs a runtime with explicitly selected host authority.
// Unlike New and NewWithLoader, it does not fill missing capabilities.
func NewWithConfig(config Config) *Runtime {
	return newRuntime(config)
}

func newRuntime(config Config) *Runtime {
	builtins := newNamespace()
	for _, exceptionType := range builtinExceptionTypes {
		builtins.values[exceptionType.name] = exceptionType
	}
	for _, builtinType := range builtinTypes {
		builtins.values[builtinType.name] = builtinType
	}
	runtime := &Runtime{
		builtins:  builtins,
		modules:   make(map[string]*Module),
		moduleMap: &dictValue{},
		prepared:  make(map[*bytecode.Code]*preparedCode),
		loader:    config.Loader,
		path:      slices.Clone(config.Path),
		host:      config.Host,
	}
	runtime.initializeSystemModules()
	return runtime
}

// ExecuteModule validates and executes one code object as an ordinary module.
func (runtime *Runtime) ExecuteModule(name string, code *bytecode.Code) (*Module, error) {
	return runtime.ExecuteModuleSpec(name, ModuleSpec{Code: code})
}

// ExecuteModuleSpec validates and executes a module description while
// preserving its package and origin metadata. The module enters the cache
// before its body runs so imports can observe partial state.
func (runtime *Runtime) ExecuteModuleSpec(name string, spec ModuleSpec) (*Module, error) {
	module, moduleFrame, err := runtime.newModuleFrame(name, spec, nil)
	if err != nil {
		return nil, err
	}
	previous, replaced := runtime.modules[name]
	runtime.cacheModule(name, module)
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

// newModuleFrame validates one module description, initializes its import
// metadata and frame storage, and leaves cache insertion to the caller.
func (runtime *Runtime) newModuleFrame(
	name string,
	spec ModuleSpec,
	previous *frame,
) (*Module, *frame, error) {
	if spec.Code == nil {
		return nil, nil, &BytecodeError{
			Instruction: -1,
			Message:     "module loader returned nil code for " + name,
		}
	}
	prepared, err := runtime.prepare(spec.Code)
	if err != nil {
		return nil, nil, err
	}
	globals := newNamespace()
	globals.values["__name__"] = &stringValue{value: name}
	packageName := name
	if !spec.IsPackage {
		packageName = ""
		if separator := strings.LastIndexByte(name, '.'); separator >= 0 {
			packageName = name[:separator]
		}
	}
	globals.values["__package__"] = &stringValue{value: packageName}
	if spec.Origin != "" {
		globals.values["__file__"] = &stringValue{value: spec.Origin}
	}
	if spec.IsPackage {
		locations := make([]Value, len(spec.SearchLocations))
		for index, location := range spec.SearchLocations {
			locations[index] = &stringValue{value: location}
		}
		globals.values["__path__"] = &listValue{elements: locations}
	}
	searchLocations := slices.Clone(spec.SearchLocations)
	if spec.IsPackage && searchLocations == nil {
		searchLocations = make([]string, 0)
	}
	module := &Module{
		name:            name,
		globals:         globals,
		isPackage:       spec.IsPackage,
		searchLocations: searchLocations,
	}
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
		runtime.cacheModule(name, previous)
	} else {
		runtime.deleteModule(name)
	}
}

// Module returns a cached module by name. A loader callback may observe a
// module whose body is still initializing.
func (runtime *Runtime) Module(name string) (*Module, bool) {
	module, ok := runtime.modules[name]
	return module, ok
}

func (runtime *Runtime) cacheModule(name string, module *Module) {
	runtime.modules[name] = module
	if exception := runtime.moduleMap.set(&stringValue{value: name}, module); exception != nil {
		panic("runtime: module name is not hashable")
	}
}

func (runtime *Runtime) deleteModule(name string) {
	delete(runtime.modules, name)
	deleted, exception := runtime.moduleMap.delete(&stringValue{value: name})
	if exception != nil {
		panic("runtime: module name is not hashable")
	}
	_ = deleted
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
