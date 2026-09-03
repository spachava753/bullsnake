package runtime

import (
	"errors"
	"io/fs"
	"math/big"

	"github.com/spachava753/bullsnake/host"
)

var systemModuleNames = []string{
	"__future__", "_abc", "_ast", "_codecs", "_collections", "_contextvars", "_functools", "_imp", "_io", "_opcode", "_pickle", "_random", "_signal", "_string", "_struct", "_thread", "_tokenize", "_types", "_weakref", "builtins",
	"asyncio", "asyncio.coroutines", "asyncio.events", "atexit", "copy", "enum", "errno", "gc", "importlib", "importlib._bootstrap_external", "importlib.machinery", "io", "itertools", "math", "operator", "os", "os.path", "pkgutil", "posix", "re", "reprlib", "subprocess", "sys", "sysconfig", "time", "typing", "warnings",
}

func (runtime *Runtime) initializeSystemModules() {
	sys := runtime.newSysModule()
	runtime.cacheModule("sys", sys)
	sys.globals.values["modules"] = runtime.moduleMap

	builtins := &Module{name: "builtins", globals: runtime.builtins}
	builtins.globals.values["__name__"] = &stringValue{value: "builtins"}
	builtins.globals.values["__package__"] = &stringValue{value: ""}
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
	case "atexit":
		module = newSystemModule("atexit", "")
		setNativeFunction(module, "register", 1, -1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				return arguments[0], nil, nil
			})
	case "copy":
		module = newSystemModule("copy", "")
		setNativeFunction(module, "copy", 1, 1, identityFirstArgument)
		setNativeFunction(module, "deepcopy", 1, 2, identityFirstArgument)
		module.globals.values["Error"] = exceptionType
	case "sysconfig":
		module = newSystemModule("sysconfig", "")
		setNativeFunction(module, "get_config_var", 1, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			})
		setNativeFunction(module, "get_config_vars", 0, -1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &dictValue{}, nil, nil
			})
		setNativeFunction(module, "is_python_build", 0, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return falseSingleton, nil, nil
			})
	case "gc":
		module = newSystemModule("gc", "")
		setNativeFunction(module, "collect", 0, 1,
			func(caller *frame, _ []Value) (Value, *Exception, error) {
				for _, reference := range runtime.weakRefs {
					reference.referent = None
				}
				collected := int64(0)
				for _, coroutine := range runtime.coroutines {
					if coroutine == nil || coroutine.done || coroutine.warned {
						continue
					}
					coroutine.warned = true
					collected++
					if runtime.warningState != nil {
						_, exception, err := runtime.warningState.warn(caller, []Value{
							&stringValue{value: "coroutine '" + coroutine.name + "' was never awaited"},
							runtimeWarningType,
						}, nil)
						if err != nil || exception != nil {
							return nil, exception, err
						}
					}
				}
				retained := runtime.coroutines[:0]
				for _, coroutine := range runtime.coroutines {
					if coroutine != nil && !coroutine.done && !coroutine.warned {
						retained = append(retained, coroutine)
					}
				}
				runtime.coroutines = retained
				return newInt64(collected), nil, nil
			})
		setNativeFunction(module, "isenabled", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return trueSingleton, nil, nil
			})
		module.globals.values["garbage"] = &listValue{}
	case "__future__":
		module = newFutureModule()
	case "_abc":
		module = runtime.newABCModule()
	case "_ast":
		module = newASTBootstrapModule()
	case "_codecs":
		module = newCodecsModule()
	case "_collections":
		module = newCollectionsBootstrapModule()
	case "_contextvars":
		module = newSystemModule("_contextvars", "")
		context := newBootstrapClass("Context", nil)
		context.module = "_contextvars"
		context.constructor = func(
			_ *typeValue, _ *frame, _ []Value, _ *dictValue,
		) (Value, *Exception, error) {
			return &contextVarsValue{}, nil, nil
		}
		contextVar := newBootstrapClass("ContextVar", nil)
		contextVar.module = "_contextvars"
		contextVar.constructor = constructContextVar
		token := newBootstrapClass("Token", nil)
		token.module = "_contextvars"
		module.globals.values["Context"] = context
		module.globals.values["ContextVar"] = contextVar
		module.globals.values["Token"] = token
		module.globals.values["copy_context"] = nativeFunctionNamed(
			"copy_context", 0, 0,
			func(caller *frame, _ []Value) (Value, *Exception, error) {
				return caller.runtime.context().clone(), nil, nil
			},
		)
	case "asyncio":
		module = runtime.newAsyncioModule()
	case "asyncio.events":
		module = runtime.newAsyncioEventsModule()
	case "asyncio.coroutines":
		asyncio := runtime.newAsyncioModule()
		module, _ = asyncio.globals.values["coroutines"].(*Module)
	case "_opcode":
		module = newOpcodeBootstrapModule()
	case "_pickle":
		module = newPickleModule()
	case "_random":
		module = runtime.newRandomBootstrapModule()
	case "_signal":
		module = runtime.newSignalBootstrapModule()
	case "_functools":
		module = newFunctoolsModule()
	case "_imp":
		module = newSystemModule("_imp", "")
		setNativeFunction(module, "_override_frozen_modules_for_tests", 1, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) { return None, nil, nil })
		setNativeFunction(module, "_override_multi_interp_extensions_check", 1, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) { return newInt64(0), nil, nil })
	case "_string":
		module = newSystemModule("_string", "")
		setNativeFunction(module, "formatter_parser", 1, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &sequenceIterator{sequence: &listValue{}}, nil, nil
			})
		setNativeFunction(module, "formatter_field_name_split", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				return &tupleValue{elements: []Value{
					arguments[0], &sequenceIterator{sequence: &listValue{}},
				}}, nil, nil
			})
	case "_struct":
		module = newStructModule()
	case "_tokenize":
		module = newSystemModule("_tokenize", "")
		setNativeKeywordFunction(module, "TokenizerIter", 1, -1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &sequenceIterator{sequence: &listValue{}}, nil, nil
			})
	case "itertools":
		module = newItertoolsModule()
	case "math":
		module = newMathModule()
	case "operator":
		module = newOperatorModule()
	case "pkgutil":
		module = newSystemModule("pkgutil", "")
		setNativeFunction(module, "resolve_name", 1, 1, runtime.pkgutilResolveName)
	case "typing":
		module = newSystemModule("typing", "")
		module.globals.values["ClassVar"] = &builtinTypeValue{
			name: "ClassVar", matches: func(Value) bool { return false },
		}
		setNativeFunction(module, "get_origin", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				if alias, ok := arguments[0].(*genericAliasValue); ok {
					return alias.origin, nil, nil
				}
				return None, nil, nil
			})
	case "posix":
		module = newSystemModule("posix", "")
	case "reprlib":
		module = newSystemModule("reprlib", "")
		setNativeFunction(module, "recursive_repr", 0, 1, reprlibRecursiveRepr)
	case "_weakref":
		module = newWeakrefModule()
	case "_thread":
		module = newThreadModule()
	case "_types":
		module = newTypesBootstrapModule()
	case "enum":
		module = newEnumModule()
	case "errno":
		module = newErrnoBootstrapModule()
	case "importlib":
		module = runtime.newImportlibBootstrapModule()
	case "importlib._bootstrap_external":
		module = newSystemModule("importlib._bootstrap_external", "importlib")
		namespaceLoader := newBootstrapClass("NamespaceLoader", nil)
		namespaceLoader.constructor = func(
			class *typeValue,
			_ *frame,
			_ []Value,
			_ *dictValue,
		) (Value, *Exception, error) {
			return &instanceValue{class: class, attributes: newNamespace()}, nil, nil
		}
		module.globals.values["NamespaceLoader"] = namespaceLoader
	case "importlib.machinery":
		module = newSystemModule("importlib.machinery", "importlib")
		module.globals.values["SOURCE_SUFFIXES"] = stringList([]string{".py"})
		module.globals.values["BYTECODE_SUFFIXES"] = stringList([]string{".pyc"})
		module.globals.values["EXTENSION_SUFFIXES"] = &listValue{}
		setNativeFunction(module, "all_suffixes", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return stringList([]string{".py", ".pyc"}), nil, nil
			})
	case "test.support.import_helper":
		module = newSystemModule("test.support.import_helper", "test.support")
		setNativeFunction(module, "DirsOnSysPath", 0, -1, runtime.newDirsOnSysPath)
	case "test.test_importlib.util":
		module = newSystemModule("test.test_importlib.util", "test.test_importlib")
		setNativeFunction(module, "uncache", 0, -1, runtime.newModuleUncacheContext)
	case "re":
		module = newRegexModule()
	case "subprocess":
		module = runtime.newSubprocessModule()
	case "warnings":
		module = runtime.newWarningsModule()
	case "time":
		module = runtime.newTimeModule()
	case "os":
		module = runtime.newOSModule()
	case "os.path":
		module = runtime.newOSPathModule()
	case "io", "_io":
		module = newSystemModule(name, "")
		if open, found := runtime.builtins.get("open"); found {
			module.globals.values["open"] = open
		}
		module.globals.values["DEFAULT_BUFFER_SIZE"] = newInt64(8192)
		module.globals.values["SEEK_SET"] = newInt64(0)
		module.globals.values["SEEK_CUR"] = newInt64(1)
		module.globals.values["SEEK_END"] = newInt64(2)
		module.globals.values["UnsupportedOperation"] = osErrorType
		for _, className := range []string{"IOBase", "RawIOBase", "BufferedIOBase", "TextIOBase"} {
			module.globals.values[className] = &builtinTypeValue{
				name: className, matches: func(Value) bool { return false },
			}
		}
		module.globals.values["StringIO"] = &builtinTypeValue{
			name: "StringIO",
			matches: func(value Value) bool {
				_, ok := value.(*stringIOValue)
				return ok
			},
		}
		module.globals.values["BytesIO"] = &builtinTypeValue{
			name: "BytesIO",
			matches: func(value Value) bool {
				_, ok := value.(*bytesIOValue)
				return ok
			},
		}
		setNativeKeywordFunction(module, "TextIOWrapper", 1, -1, newTextIOWrapper)
	default:
		return nil, false
	}
	runtime.cacheModule(name, module)
	return module, true
}

func newFutureModule() *Module {
	module := newSystemModule("__future__", "")
	for _, name := range []string{
		"nested_scopes",
		"generators",
		"division",
		"absolute_import",
		"with_statement",
		"print_function",
		"unicode_literals",
		"generator_stop",
	} {
		module.globals.values[name] = &tupleValue{elements: []Value{
			newInt64(2),
			newInt64(1),
			newInt64(0),
		}}
	}
	module.globals.values["all_feature_names"] = stringList([]string{
		"nested_scopes",
		"generators",
		"division",
		"absolute_import",
		"with_statement",
		"print_function",
		"unicode_literals",
		"generator_stop",
	})
	return module
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

func setNativeKeywordFunction(
	module *Module,
	name string,
	minimum int,
	maximum int,
	function nativeFunction,
) {
	module.globals.values[name] = nativeKeywordFunctionNamed(
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
