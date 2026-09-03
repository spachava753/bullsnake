// Package bullsnake embeds the Bullsnake Python interpreter in Go programs.
package bullsnake

import (
	"fmt"
	"slices"

	"github.com/spachava753/bullsnake/host"
	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
	"github.com/spachava753/bullsnake/internal/compiler/source"
	"github.com/spachava753/bullsnake/internal/importer"
	internalruntime "github.com/spachava753/bullsnake/internal/runtime"
)

// Config grants host capabilities and selects the source roots exposed through
// Python imports. A zero Config grants no ambient host access.
type Config struct {
	Host             host.Services
	ModuleSearchPath []string
}

// Interpreter owns one isolated module cache and set of host capabilities.
// It is not safe for concurrent execution.
type Interpreter struct {
	runtime *internalruntime.Runtime
	host    host.Services
}

// Module is a read-only Go view of one executed Python module.
type Module struct {
	module *internalruntime.Module
}

// Value is the stable read-only surface currently exposed for Python values.
type Value interface {
	TypeName() string
	Repr() string
}

// New constructs an interpreter from explicit capabilities. Module source
// imports use Config.Host.Files and cannot bypass that filesystem boundary.
func New(config Config) *Interpreter {
	path := slices.Clone(config.ModuleSearchPath)
	var loader internalruntime.ModuleLoader
	if len(path) != 0 {
		loader = importer.NewFileSystemWithHost(config.Host.Files, path...).Load
	}
	return &Interpreter{
		runtime: internalruntime.NewWithConfig(internalruntime.Config{
			Loader: loader,
			Path:   path,
			Host:   config.Host,
		}),
		host: config.Host,
	}
}

// NewDefault constructs an interpreter with current-process capabilities and
// the supplied module search roots.
func NewDefault(moduleSearchPath ...string) *Interpreter {
	return New(Config{
		Host:             host.Default(),
		ModuleSearchPath: moduleSearchPath,
	})
}

// ExecuteModule compiles and executes source as one named Python module.
func (interpreter *Interpreter) ExecuteModule(
	name string,
	filename string,
	text string,
) (*Module, error) {
	code, err := compile(filename, text)
	if err != nil {
		return nil, err
	}
	module, err := interpreter.runtime.ExecuteModuleSpec(name, internalruntime.ModuleSpec{
		Code:   code,
		Origin: filename,
	})
	if err != nil {
		return nil, err
	}
	return &Module{module: module}, nil
}

// ExecuteFile reads source through the configured filesystem capability before
// compiling and executing it. A nil filesystem returns host.ErrDenied.
func (interpreter *Interpreter) ExecuteFile(name, filename string) (*Module, error) {
	if interpreter.host.Files == nil {
		return nil, host.ErrDenied
	}
	data, err := interpreter.host.Files.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read module %q: %w", name, err)
	}
	unit, err := source.Decode(filename, data)
	if err != nil {
		return nil, err
	}
	return interpreter.ExecuteModule(name, unit.Filename, unit.Text)
}

// Module returns a previously initialized module by cache identity.
func (interpreter *Interpreter) Module(name string) (*Module, bool) {
	module, found := interpreter.runtime.Module(name)
	if !found {
		return nil, false
	}
	return &Module{module: module}, true
}

// Name returns the module's cache name.
func (module *Module) Name() string {
	return module.module.Name()
}

// Get returns a read-only Python value from the module namespace.
func (module *Module) Get(name string) (Value, bool) {
	return module.module.Get(name)
}

func compile(filename, text string) (*bytecode.Code, error) {
	module, err := parser.Parse(filename, text)
	if err != nil {
		return nil, err
	}
	table, err := resolver.Resolve(filename, module)
	if err != nil {
		return nil, err
	}
	return compiler.Compile(filename, module, table)
}
