package runtime

import (
	"errors"
	"io/fs"
	"math/big"

	"github.com/spachava753/bullsnake/host"
)

var systemModuleNames = []string{"_io", "builtins", "io", "os", "os.path", "sys", "time"}

func (runtime *Runtime) initializeSystemModules() {
	sys := runtime.newSysModule()
	runtime.cacheModule("sys", sys)
	sys.globals.values["modules"] = runtime.moduleMap

	builtins := newSystemModule("builtins", "")
	for name, value := range runtime.builtins.values {
		builtins.globals.values[name] = value
	}
	runtime.cacheModule("builtins", builtins)
}

// loadSystemModule creates lazy Go-backed modules through the same cache path
// as source modules. Eager sys and builtins modules are already cached.
func (runtime *Runtime) loadSystemModule(name string) (*Module, bool) {
	if module, found := runtime.modules[name]; found {
		return module, true
	}
	var module *Module
	switch name {
	case "time":
		module = runtime.newTimeModule()
	case "os":
		module = runtime.newOSModule()
	case "os.path":
		module = runtime.newOSPathModule()
	case "io", "_io":
		module = newSystemModule(name, "")
		setNativeFunction(module, "StringIO", 0, 1, newStringIO)
	default:
		return nil, false
	}
	runtime.cacheModule(name, module)
	return module, true
}

func newSystemModule(name, packageName string) *Module {
	globals := newNamespace()
	globals.values["__name__"] = &stringValue{value: name}
	globals.values["__package__"] = &stringValue{value: packageName}
	return &Module{name: name, globals: globals}
}

func setNativeFunction(
	module *Module,
	name string,
	minimum int,
	maximum int,
	function nativeFunction,
) {
	module.globals.values[name] = nativeFunctionNamed(
		module.name+"."+name,
		minimum,
		maximum,
		function,
	)
}

func hostFailure(operation string, err error) *Exception {
	typeName := "OSError"
	switch {
	case errors.Is(err, host.ErrDenied), errors.Is(err, fs.ErrPermission):
		typeName = "PermissionError"
	case errors.Is(err, fs.ErrNotExist):
		typeName = "FileNotFoundError"
	}
	message := operation
	if err != nil {
		message += ": " + err.Error()
	}
	return newException(typeName, message)
}

func newInt64(value int64) *intValue {
	var integer big.Int
	integer.SetInt64(value)
	return &intValue{value: integer}
}

func stringList(values []string) *listValue {
	elements := make([]Value, len(values))
	for index, value := range values {
		elements[index] = &stringValue{value: value}
	}
	return &listValue{elements: elements}
}
