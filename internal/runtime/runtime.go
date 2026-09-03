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
	IsNamespace     bool
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

// SourceCompiler compiles dynamically supplied Python source without granting
// the runtime any filesystem or process authority.
type SourceCompiler func(filename, text string) (*bytecode.Code, error)

// Config supplies module discovery, import paths, and explicit host
// capabilities to one runtime. A zero Host grants no ambient capabilities.
type Config struct {
	Loader   ModuleLoader
	Path     []string
	Host     host.Services
	Compiler SourceCompiler
}

// Runtime owns mutable interpreter state shared by executions in one isolated
// Python runtime instance.
type Runtime struct {
	builtins       *Namespace
	modules        map[string]*Module
	moduleMap      *dictValue
	prepared       map[*bytecode.Code]*preparedCode
	loader         ModuleLoader
	path           []string
	host           host.Services
	compiler       SourceCompiler
	abcToken       int64
	abcTypes       map[*typeValue][]Value
	weakRefs       []*weakReferenceValue
	coroutines     []*coroutineValue
	warningState   *warningsState
	signalHandlers map[int64]Value
	cancellingTask bool
	currentContext *contextVarsValue
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

// newRuntime initializes isolated builtins, module caches, exception state, and
// the exact host capabilities selected by the embedder.
func newRuntime(config Config) *Runtime {
	builtins := newNamespace()
	for _, exceptionType := range builtinExceptionTypes {
		builtins.values[exceptionType.name] = exceptionType
	}
	for _, builtinType := range builtinTypes {
		builtins.values[builtinType.name] = builtinType
	}
	for _, descriptorType := range descriptorTypes {
		builtins.values[descriptorType.name] = descriptorType
	}
	builtins.values["NotImplemented"] = notImplementedSingleton
	builtins.values["__debug__"] = trueSingleton
	runtime := &Runtime{
		builtins:       builtins,
		modules:        make(map[string]*Module),
		moduleMap:      &dictValue{},
		prepared:       make(map[*bytecode.Code]*preparedCode),
		loader:         config.Loader,
		path:           slices.Clone(config.Path),
		host:           config.Host,
		compiler:       config.Compiler,
		abcTypes:       make(map[*typeValue][]Value),
		signalHandlers: make(map[int64]Value),
	}
	for _, function := range []*nativeFunctionValue{
		nativeFunctionNamed("iter", 1, 1, builtinIter),
		nativeFunctionNamed("reversed", 1, 1, builtinReversed),
		nativeFunctionNamed("globals", 0, 0, builtinGlobals),
		nativeFunctionNamed("locals", 0, 0, builtinLocals),
		nativeFunctionNamed("getattr", 2, 3, builtinGetAttr),
		nativeFunctionNamed("hasattr", 2, 2, builtinHasAttr),
		nativeFunctionNamed("setattr", 3, 3, builtinSetAttr),
		nativeFunctionNamed("delattr", 2, 2, builtinDelAttr),
		nativeFunctionNamed("dir", 0, 1, builtinDir),
		nativeFunctionNamed("len", 1, 1, builtinLen),
		nativeFunctionNamed("isinstance", 2, 2, builtinIsInstance),
		nativeFunctionNamed("issubclass", 2, 2, builtinIsSubclass),
		nativeFunctionNamed("callable", 1, 1, builtinCallable),
		nativeFunctionNamed("abs", 1, 1, builtinAbs),
		nativeFunctionNamed("round", 1, 2, builtinRound),
		nativeFunctionNamed("bin", 1, 1, builtinBaseRepresentation(2, "0b")),
		nativeFunctionNamed("oct", 1, 1, builtinBaseRepresentation(8, "0o")),
		nativeFunctionNamed("hex", 1, 1, builtinBaseRepresentation(16, "0x")),
		nativeFunctionNamed("all", 1, 1, builtinAll),
		nativeFunctionNamed("any", 1, 1, builtinAny),
		nativeFunctionNamed("next", 1, 2, builtinNext),
		nativeFunctionNamed("enumerate", 1, 2, builtinEnumerate),
		nativeFunctionNamed("map", 2, -1, builtinMap),
		nativeFunctionNamed("filter", 2, 2, builtinFilter),
		nativeFunctionNamed("sum", 1, 2, builtinSum),
		nativeFunctionNamed("divmod", 2, 2, builtinDivmod),
		nativeFunctionNamed("ord", 1, 1, builtinOrd),
		nativeFunctionNamed("chr", 1, 1, builtinChr),
		nativeFunctionNamed("id", 1, 1, builtinID),
		nativeFunctionNamed("repr", 1, 1, builtinRepr),
		nativeFunctionNamed("ascii", 1, 1, builtinRepr),
		nativeFunctionNamed("hash", 1, 1, builtinHash),
		nativeFunctionNamed("super", 0, 2, builtinSuper),
		nativeFunctionNamed("eval", 1, 3, runtime.builtinEval),
		nativeFunctionNamed("exec", 1, 3, runtime.builtinExec),
		nativeFunctionNamed("__import__", 1, 5, runtime.builtinImport),
	} {
		runtime.builtins.values[function.name] = function
	}
	runtime.builtins.values["sorted"] = nativeKeywordAwareFunctionNamed(
		"sorted", 1, 1, builtinSorted,
	)
	runtime.builtins.values["zip"] = nativeKeywordAwareFunctionNamed(
		"zip", 0, -1,
		func(caller *frame, arguments []Value, keywords *dictValue) (Value, *Exception, error) {
			if keywords != nil {
				for _, entry := range keywords.entries {
					name, ok := entry.key.(*stringValue)
					if !ok || name.value != "strict" {
						return nil, newException("TypeError", "zip() got an unexpected keyword argument"), nil
					}
				}
			}
			return builtinZip(caller, arguments)
		},
	)
	runtime.builtins.values["max"] = nativeKeywordAwareFunctionNamed(
		"max", 1, -1, builtinMax,
	)
	runtime.builtins.values["min"] = nativeKeywordAwareFunctionNamed(
		"min", 1, -1, builtinMin,
	)
	runtime.builtins.values["print"] = nativeKeywordAwareFunctionNamed(
		"print", 0, -1, runtime.builtinPrint,
	)
	runtime.builtins.values["__bullsnake_template__"] = nativeFunctionNamed(
		"__bullsnake_template__", 1, 1, builtinTemplate,
	)
	runtime.builtins.values["open"] = nativeKeywordFunctionNamed(
		"open", 1, 8, runtime.builtinOpen,
	)
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
	_, mainExists := runtime.modules["__main__"]
	mainAlias := name != "__main__" && !mainExists
	if mainAlias {
		runtime.cacheModule("__main__", module)
	}
	thread := &threadState{current: moduleFrame}

	_, raised, err := execute(thread)
	if err != nil {
		runtime.restoreModule(name, module, previous, replaced)
		if mainAlias {
			runtime.deleteModule("__main__")
		}
		return nil, err
	}
	if raised != nil {
		runtime.restoreModule(name, module, previous, replaced)
		if mainAlias {
			runtime.deleteModule("__main__")
		}
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
	globals.values["__spec__"] = &moduleSpecValue{
		name: name, origin: spec.Origin,
		namespace: spec.IsNamespace, searchLocations: slices.Clone(spec.SearchLocations),
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

type moduleSpecValue struct {
	name            string
	origin          string
	namespace       bool
	searchLocations []string
}

func (*moduleSpecValue) TypeName() string { return "ModuleSpec" }
func (*moduleSpecValue) Repr() string     { return "ModuleSpec(...)" }
func (*moduleSpecValue) isValue()         {}

// attribute exposes importlib-compatible module-spec metadata to Python code.
func (spec *moduleSpecValue) attribute(name string) (Value, bool) {
	switch name {
	case "name":
		return &stringValue{value: spec.name}, true
	case "loader":
		return None, true
	case "origin":
		if spec.origin == "" {
			return None, true
		}
		return &stringValue{value: spec.origin}, true
	case "submodule_search_locations":
		if !spec.namespace {
			return None, true
		}
		return stringList(spec.searchLocations), true
	default:
		return nil, false
	}
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
