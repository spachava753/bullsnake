package runtime_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

func TestModuleCacheAPI(t *testing.T) {
	code := compileSource(t, "answer = 42\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("cached", code)
	if err != nil {
		t.Fatal(err)
	}
	if module.Name() != "cached" {
		t.Fatalf("module name = %q, want cached", module.Name())
	}
	cached, ok := runtime.Module("cached")
	if !ok || cached != module {
		t.Fatal("successful module was not cached by identity")
	}
}

func TestFunctionValueMetadata(t *testing.T) {
	code := compileSource(t, "def add(left, right):\n    return left + right\n")
	module, err := bullruntime.New().ExecuteModule("functions", code)
	if err != nil {
		t.Fatal(err)
	}
	function, ok := module.Get("add")
	if !ok {
		t.Fatal("module has no add binding")
	}
	if got := function.TypeName(); got != "function" {
		t.Errorf("add type = %q, want function", got)
	}
}

func TestCachedModuleImports(t *testing.T) {
	libraryCode := compileSource(t, "value = 41\n"+
		"public = 'ready'\n"+
		"_private = 'hidden'\n")
	runtime := bullruntime.New()
	if _, err := runtime.ExecuteModule("library", libraryCode); err != nil {
		t.Fatal(err)
	}
	mainCode := compileSource(t, "import library as first\n"+
		"import library as second\n"+
		"from library import public as selected\n"+
		"from library import *\n"+
		"def read():\n"+
		"    import library\n"+
		"    return library.value\n"+
		"answer = first.value + 1\n"+
		"same = first is second\n"+
		"function_value = read()\n"+
		"module_repr = f'{first!r}'\n")
	module, err := runtime.ExecuteModule("main", mainCode)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"answer":         "42",
		"same":           "True",
		"selected":       "'ready'",
		"public":         "'ready'",
		"function_value": "41",
		"module_repr":    `"<module 'library'>"`,
	}
	for name, expected := range want {
		value, ok := module.Get(name)
		if !ok {
			t.Fatalf("module has no %q binding", name)
		}
		if got := value.Repr(); got != expected {
			t.Errorf("%s = %s, want %s", name, got, expected)
		}
	}
	if _, leaked := module.Get("_private"); leaked {
		t.Fatal("star import copied a private binding")
	}
}

func TestImportedModuleMutation(t *testing.T) {
	runtime := bullruntime.New()
	libraryCode := compileSource(t, "value = 1\nremovable = 2\n")
	library, err := runtime.ExecuteModule("library", libraryCode)
	if err != nil {
		t.Fatal(err)
	}
	mutatorCode := compileSource(t, "import library\n"+
		"library.value = 3\n"+
		"del library.removable\n")
	if _, err := runtime.ExecuteModule("mutator", mutatorCode); err != nil {
		t.Fatal(err)
	}
	value, ok := library.Get("value")
	if !ok || value.Repr() != "3" {
		t.Fatalf("library value = %v, %t, want 3", value, ok)
	}
	if _, found := library.Get("removable"); found {
		t.Fatal("deleted module member remained cached")
	}
	observerCode := compileSource(t, "import library\nobserved = library.value\n")
	observer, err := runtime.ExecuteModule("observer", observerCode)
	if err != nil {
		t.Fatal(err)
	}
	observed, ok := observer.Get("observed")
	if !ok || observed.Repr() != "3" {
		t.Fatalf("observed = %v, %t, want 3", observed, ok)
	}
}

func TestPreloadedImportFailures(t *testing.T) {
	runtime := bullruntime.New()
	libraryCode := compileSource(t, "present = 1\n")
	if _, err := runtime.ExecuteModule("library", libraryCode); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		source      string
		wantType    string
		wantMessage string
	}{
		{
			name:        "member",
			source:      "from library import missing\n",
			wantType:    "ImportError",
			wantMessage: "cannot import name 'missing' from 'library' (unknown location)",
		},
		{
			name:        "deleted member",
			source:      "import library\ndel library.missing\n",
			wantType:    "AttributeError",
			wantMessage: "module 'library' has no attribute 'missing'",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := compileSource(t, test.source)
			module, err := runtime.ExecuteModule("failure_"+test.name, code)
			if module != nil {
				t.Fatalf("module = %#v, want nil", module)
			}
			var raised *bullruntime.UncaughtException
			if !errors.As(err, &raised) {
				t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
			}
			if got := raised.Exception().TypeName(); got != test.wantType {
				t.Errorf("exception type = %q, want %q", got, test.wantType)
			}
			if got := raised.Exception().Message(); got != test.wantMessage {
				t.Errorf("exception message = %q, want %q", got, test.wantMessage)
			}
		})
	}
}

func TestLoadedModuleIdentity(t *testing.T) {
	loads := make(map[string]int)
	sources := map[string]string{
		"helper":  "value = 40\n",
		"library": "import helper\nvalue = helper.value + 2\n",
	}
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		source, found := sources[name]
		if !found {
			return bullruntime.ModuleSpec{}, false, nil
		}
		loads[name]++
		return bullruntime.ModuleSpec{Code: compileSource(t, source)}, true, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t,
		"import library as first\n"+
			"import library as second\n"+
			"answer = first.value\n"+
			"same = first is second\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, module, "answer", "42")
	assertModuleRepr(t, module, "same", "True")
	if loads["library"] != 1 || loads["helper"] != 1 {
		t.Fatalf("load counts = %v, want library:1 helper:1", loads)
	}
	library, found := runtime.Module("library")
	if !found {
		t.Fatal("loaded library is absent from the runtime cache")
	}
	first, _ := module.Get("first")
	if first != library {
		t.Fatal("import did not retain the cached module identity")
	}
}

func TestCircularModuleInitialization(t *testing.T) {
	loads := make(map[string]int)
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		loads[name]++
		if name != "beta" {
			return bullruntime.ModuleSpec{}, false, nil
		}
		return bullruntime.ModuleSpec{
			Code: compileSource(t, "import alpha\nobserved = alpha.state\n"),
		}, true, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	alpha, err := runtime.ExecuteModule("alpha", compileSource(t,
		"state = 'starting'\n"+
			"import beta\n"+
			"observed = beta.observed\n"+
			"state = 'finished'\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, alpha, "observed", "'starting'")
	assertModuleRepr(t, alpha, "state", "'finished'")
	if loads["alpha"] != 0 || loads["beta"] != 1 {
		t.Fatalf("load counts = %v, want alpha:0 beta:1", loads)
	}
}

func TestFailedModuleInitialization(t *testing.T) {
	loads := make(map[string]int)
	sources := map[string]string{
		"side":   "value = 1\n",
		"broken": "import side\nraise ValueError('boom')\n",
	}
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		source, found := sources[name]
		if !found {
			return bullruntime.ModuleSpec{}, false, nil
		}
		loads[name]++
		return bullruntime.ModuleSpec{Code: compileSource(t, source)}, true, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t,
		"attempts = 0\n"+
			"try:\n"+
			"    import broken\n"+
			"except ValueError:\n"+
			"    attempts = attempts + 1\n"+
			"try:\n"+
			"    import broken\n"+
			"except ValueError:\n"+
			"    attempts = attempts + 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, module, "attempts", "2")
	if loads["broken"] != 2 || loads["side"] != 1 {
		t.Fatalf("load counts = %v, want broken:2 side:1", loads)
	}
	if _, found := runtime.Module("broken"); found {
		t.Fatal("failed module remained in the runtime cache")
	}
	if _, found := runtime.Module("side"); !found {
		t.Fatal("successful side import was removed after its importer failed")
	}
}

func TestLoaderHostFailure(t *testing.T) {
	loadFailure := errors.New("module storage unavailable")
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		switch name {
		case "bridge":
			return bullruntime.ModuleSpec{
				Code: compileSource(t, "import unavailable\n"),
			}, true, nil
		case "unavailable":
			return bullruntime.ModuleSpec{}, false, loadFailure
		default:
			return bullruntime.ModuleSpec{}, false, nil
		}
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t, "import bridge\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	if !errors.Is(err, loadFailure) {
		t.Fatalf("error = %v, want loader failure", err)
	}
	for _, name := range []string{"main", "bridge", "unavailable"} {
		if _, found := runtime.Module(name); found {
			t.Fatalf("failed module %q remained in the runtime cache", name)
		}
	}
}

func TestDottedImports(t *testing.T) {
	loads := make(map[string]int)
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		loads[name]++
		switch name {
		case "package":
			if request.SearchLocations != nil {
				t.Fatalf("top-level search locations = %v, want nil", request.SearchLocations)
			}
			return bullruntime.ModuleSpec{
				Code:            compileSource(t, "marker = 'root'\n"),
				IsPackage:       true,
				Origin:          "/modules/package/__init__.py",
				SearchLocations: []string{"/modules/package"},
			}, true, nil
		case "package.child":
			if len(request.SearchLocations) != 1 || request.SearchLocations[0] != "/modules/package" {
				t.Fatalf("child search locations = %v, want package path", request.SearchLocations)
			}
			return bullruntime.ModuleSpec{
				Code:   compileSource(t, "value = 42\n"),
				Origin: "/modules/package/child.py",
			}, true, nil
		default:
			return bullruntime.ModuleSpec{}, false, nil
		}
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t,
		"import package.child\n"+
			"import package.child as alias\n"+
			"from package.child import value as selected\n"+
			"answer = package.child.value\n"+
			"same = alias is package.child\n"+
			"root_package = package.__package__\n"+
			"child_package = alias.__package__\n"+
			"root_path = package.__path__\n"+
			"child_file = alias.__file__\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"answer":        "42",
		"same":          "True",
		"selected":      "42",
		"root_package":  "'package'",
		"child_package": "'package'",
		"root_path":     "['/modules/package']",
		"child_file":    "'/modules/package/child.py'",
	}
	for name, expected := range want {
		assertModuleRepr(t, module, name, expected)
	}
	if loads["package"] != 1 || loads["package.child"] != 1 {
		t.Fatalf("load counts = %v, want each package component once", loads)
	}
	parent, _ := runtime.Module("package")
	child, _ := runtime.Module("package.child")
	published, found := parent.Get("child")
	if !found || published != child {
		t.Fatal("child module was not published on its parent by identity")
	}
}

func TestNonPackageImportParent(t *testing.T) {
	loads := make(map[string]int)
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		loads[name]++
		if name == "plain" {
			return bullruntime.ModuleSpec{Code: compileSource(t, "value = 1\n")}, true, nil
		}
		return bullruntime.ModuleSpec{}, false, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t, "import plain.child\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "ModuleNotFoundError" {
		t.Fatalf("exception type = %q, want ModuleNotFoundError", got)
	}
	want := "No module named 'plain.child'; 'plain' is not a package"
	if got := raised.Exception().Message(); got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if loads["plain"] != 1 || loads["plain.child"] != 0 {
		t.Fatalf("load counts = %v, want plain:1 plain.child:0", loads)
	}
	if _, found := runtime.Module("plain"); !found {
		t.Fatal("successfully loaded parent was removed after child lookup failed")
	}
}

func assertModuleRepr(t *testing.T, module *bullruntime.Module, name, want string) {
	t.Helper()
	value, found := module.Get(name)
	if !found {
		t.Fatalf("module has no %q binding", name)
	}
	if got := value.Repr(); got != want {
		t.Fatalf("%s = %s, want %s", name, got, want)
	}
}

func TestReraiseKeepsOriginalLocation(t *testing.T) {
	code := testCodeSpec(bytecode.CodeSpec{
		StackSize: 1,
		Instructions: []bytecode.Instruction{
			{Opcode: bytecode.LoadAssertionError},
			{Opcode: bytecode.RaiseVarargs, Operand: 1},
			{Opcode: bytecode.Reraise},
		},
		ExceptionHandlers: []bytecode.ExceptionHandler{
			{Start: 0, End: 2, Target: 2},
		},
	})
	_, err := bullruntime.New().ExecuteModule("reraised", code)
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Error(); got != "<broken>:1:2: AssertionError: " {
		t.Fatalf("error = %q, want original raise location", got)
	}
}

func TestExplicitRaiseMovesOrigin(t *testing.T) {
	code := compileSource(t, "error = ValueError('stored')\n"+
		"try:\n"+
		"    raise error\n"+
		"except:\n"+
		"    pass\n"+
		"raise error\n")
	_, err := bullruntime.New().ExecuteModule("explicit", code)
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Error(); got != "<test>:6:1: ValueError: stored" {
		t.Fatalf("error = %q, want second explicit raise location", got)
	}
}

func TestUncaughtTracebackAPI(t *testing.T) {
	type wantFrame struct {
		name string
		line int
	}
	tests := []struct {
		name          string
		source        string
		wantFrames    []wantFrame
		wantBacktrace string
	}{
		{
			name: "nested calls",
			source: "def inner():\n" +
				"    missing_name\n" +
				"def middle():\n" +
				"    inner()\n" +
				"def outer():\n" +
				"    middle()\n" +
				"outer()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 7},
				{name: "outer", line: 6},
				{name: "middle", line: 4},
				{name: "inner", line: 2},
			},
			wantBacktrace: "Traceback (most recent call last):\n" +
				"  File \"<test>\", line 7, in <module>\n" +
				"  File \"<test>\", line 6, in outer\n" +
				"  File \"<test>\", line 4, in middle\n" +
				"  File \"<test>\", line 2, in inner\n" +
				"NameError: name 'missing_name' is not defined",
		},
		{
			name: "bare reraise",
			source: "def inner():\n" +
				"    try:\n" +
				"        missing_name\n" +
				"    except NameError:\n" +
				"        raise\n" +
				"inner()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 6},
				{name: "inner", line: 3},
			},
		},
		{
			name: "explicit reraise",
			source: "def inner():\n" +
				"    try:\n" +
				"        missing_name\n" +
				"    except NameError as error:\n" +
				"        raise error\n" +
				"inner()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 6},
				{name: "inner", line: 5},
				{name: "inner", line: 3},
			},
		},
		{
			name: "cleared explicit raise",
			source: "saved = None\n" +
				"def inner():\n" +
				"    missing_name\n" +
				"try:\n" +
				"    inner()\n" +
				"except NameError as error:\n" +
				"    saved = error.with_traceback(None)\n" +
				"raise saved\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 8},
			},
		},
		{
			name: "called bare reraise",
			source: "def reraiser():\n" +
				"    raise\n" +
				"def outer():\n" +
				"    try:\n" +
				"        missing_name\n" +
				"    except NameError:\n" +
				"        reraiser()\n" +
				"outer()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 8},
				{name: "outer", line: 7},
				{name: "outer", line: 5},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := bullruntime.New().ExecuteModule("traceback", compileSource(t, test.source))
			var raised *bullruntime.UncaughtException
			if !errors.As(err, &raised) {
				t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
			}
			frames := raised.Traceback()
			if len(frames) != len(test.wantFrames) {
				t.Fatalf("traceback has %d frames, want %d: %#v", len(frames), len(test.wantFrames), frames)
			}
			for index, want := range test.wantFrames {
				frame := frames[index]
				if frame.Filename != "<test>" || frame.Name != want.name || frame.Span.Start.Line != want.line {
					t.Errorf("frame %d = %#v, want <test>:%d in %s", index, frame, want.line, want.name)
				}
			}
			if test.wantBacktrace != "" && raised.Backtrace() != test.wantBacktrace {
				t.Errorf("backtrace = %q, want %q", raised.Backtrace(), test.wantBacktrace)
			}
			frames[0].Name = "changed"
			if raised.Traceback()[0].Name == "changed" {
				t.Fatal("Traceback returned mutable internal storage")
			}
		})
	}
}

func TestScalarConstants(t *testing.T) {
	code := compileSource(t, "none_value = None\n"+
		"false_value = False\n"+
		"true_value = True\n"+
		"ellipsis_value = ...\n"+
		"float_value = 1.25\n"+
		"imaginary_value = 2j\n"+
		"text_value = 'line\\nsnowman: \\N{SNOWMAN}'\n"+
		"surrogate_value = '\\ud800'\n"+
		"bytes_value = b'\\x00A\\xff'\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("scalars", code)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]struct {
		typeName string
		repr     string
	}{
		"none_value":      {typeName: "NoneType", repr: "None"},
		"false_value":     {typeName: "bool", repr: "False"},
		"true_value":      {typeName: "bool", repr: "True"},
		"ellipsis_value":  {typeName: "ellipsis", repr: "Ellipsis"},
		"float_value":     {typeName: "float", repr: "1.25"},
		"imaginary_value": {typeName: "complex", repr: "2j"},
		"text_value":      {typeName: "str", repr: "'line\\nsnowman: \u2603'"},
		"surrogate_value": {typeName: "str", repr: "'\\ud800'"},
		"bytes_value":     {typeName: "bytes", repr: "b'\\x00A\\xff'"},
	}

	for name, expected := range want {
		value, ok := module.Get(name)
		if !ok {
			t.Fatalf("module has no %q binding", name)
		}
		if got := value.TypeName(); got != expected.typeName {
			t.Errorf("%s type = %q, want %q", name, got, expected.typeName)
		}
		if got := value.Repr(); got != expected.repr {
			t.Errorf("%s repr = %q, want %q", name, got, expected.repr)
		}
	}

	first, _ := module.Get("true_value")
	secondCode := compileSource(t, "other = True\n")
	secondModule, err := runtime.ExecuteModule("other", secondCode)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := secondModule.Get("other")
	if first != second {
		t.Fatal("True did not retain singleton identity across code objects")
	}
}

func compileSource(t *testing.T, source string) *bytecode.Code {
	t.Helper()
	module, err := parser.Parse("<test>", source)
	if err != nil {
		t.Fatal(err)
	}
	table, err := resolver.Resolve("<test>", module)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile("<test>", module, table)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestConfiguredArgumentsIsolation(t *testing.T) {
	args := []string{"tests", "case"}
	first, err := bullruntime.NewWithConfig(bullruntime.Config{Args: args})
	if err != nil {
		t.Fatal(err)
	}
	second, err := bullruntime.NewWithConfig(bullruntime.Config{Args: args})
	if err != nil {
		t.Fatal(err)
	}
	args[0] = "changed"
	code := compileSource(t, "import sys\nassert sys.argv == ['tests', 'case']\nsys.argv.append('local')\n")
	for _, runtime := range []*bullruntime.Runtime{first, second} {
		if _, err := runtime.ExecuteModule("check", code); err != nil {
			t.Fatal(err)
		}
	}
	left, _ := first.Module("sys")
	right, _ := second.Module("sys")
	if left == right {
		t.Fatal("runtimes share sys")
	}
}

func TestInvalidArgumentEncoding(t *testing.T) {
	runtime, err := bullruntime.NewWithConfig(bullruntime.Config{Args: []string{string([]byte{0xff})}})
	if runtime != nil || err == nil {
		t.Fatalf("construction = %v, %v", runtime, err)
	}
}

func TestNativeModulePrecedesSource(t *testing.T) {
	calls := 0
	runtime := bullruntime.NewWithLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		calls++
		return bullruntime.ModuleSpec{}, false, nil
	})
	code := compileSource(t, "import sys\nimport sys as again\nassert sys is again\nsys.marker = 42\n")
	if _, err := runtime.ExecuteModule("check", code); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("source loader called %d times", calls)
	}
}

func TestInMemoryIOModuleIsolation(t *testing.T) {
	data, err := os.ReadFile("testdata/host/io_isolation.py")
	if err != nil {
		t.Fatal(err)
	}
	code := compileSource(t, string(data))
	var previous *bullruntime.Module
	for range 2 {
		runtime := bullruntime.NewWithLoader(func(bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
			t.Fatal("native I/O import called source loader")
			return bullruntime.ModuleSpec{}, false, nil
		})
		if _, err := runtime.ExecuteModule("check", code); err != nil {
			t.Fatal(err)
		}
		module, found := runtime.Module("_io")
		if !found || module == previous {
			t.Fatal("I/O module is absent or shared across runtimes")
		}
		previous = module
	}
}

func TestBufferedReadIntoHostFailure(t *testing.T) {
	failure := errors.New("source provider failed")
	runtime := bullruntime.NewWithLoader(func(bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		return bullruntime.ModuleSpec{}, false, failure
	})
	for index, name := range []string{"io_lease_failure.py", "io_lease_recovery.py"} {
		data, err := os.ReadFile(filepath.Join("testdata/host", name))
		if err != nil {
			t.Fatal(err)
		}
		_, err = runtime.ExecuteModule("check", compileSource(t, string(data)))
		if index == 0 {
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v, want original source-provider failure", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSystemExitHostBoundary(t *testing.T) {
	code := compileSource(t, "import sys\nsys.exit(7)\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("exit", code)
	var raised *bullruntime.UncaughtException
	if module != nil || !errors.As(err, &raised) {
		t.Fatalf("execution = %v, %v", module, err)
	}
	if raised.Exception().TypeName() != "SystemExit" || raised.Exception().Message() != "7" {
		t.Fatalf("exception = %v", raised)
	}
	if _, found := runtime.Module("exit"); found {
		t.Fatal("failed module remained cached")
	}
	if _, err := runtime.ExecuteModule("after_exit", compileSource(t, "answer = 42\n")); err != nil {
		t.Fatal(err)
	}
}

type sequenceCounter struct {
	values []time.Duration
	calls  int
}

func (counter *sequenceCounter) PerfCounter() time.Duration {
	value := counter.values[counter.calls]
	counter.calls++
	return value
}

func TestPerformanceCounterProvider(t *testing.T) {
	counter := &sequenceCounter{values: []time.Duration{1500 * time.Millisecond, 1750 * time.Millisecond}}
	runHostFixture(t, bullruntime.Config{Counter: counter}, "perf_counter")
	if counter.calls != 2 {
		t.Fatalf("counter calls = %d", counter.calls)
	}
	var absent *sequenceCounter
	if runtime, err := bullruntime.NewWithConfig(bullruntime.Config{Counter: absent}); runtime != nil || err == nil {
		t.Fatal("typed nil counter accepted")
	}
}

// runHostFixture executes Python behavior against explicitly supplied Go providers.
func runHostFixture(t *testing.T, config bullruntime.Config, name string) *bullruntime.Module {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("testdata", "host", name+".py"))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := bullruntime.NewWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	module, err := runtime.ExecuteModule(name, compileSource(t, string(source)))
	if err != nil {
		t.Fatal(err)
	}
	return module
}

type borrowedWriter struct {
	bytes.Buffer
	flushes    int
	closes     int
	seeks      int
	flushError error
}

func (writer *borrowedWriter) Flush() error                   { writer.flushes++; return writer.flushError }
func (writer *borrowedWriter) Close() error                   { writer.closes++; return nil }
func (writer *borrowedWriter) IsTerminal() bool               { return true }
func (writer *borrowedWriter) Seek(int64, int) (int64, error) { writer.seeks++; return 0, nil }

type borrowedReader struct {
	*strings.Reader
	closes  int
	flushes int
}

func (reader *borrowedReader) Close() error { reader.closes++; return nil }
func (reader *borrowedReader) Flush() error { reader.flushes++; return nil }

type resultReader struct {
	data  []byte
	err   error
	reads int
}

func (reader *resultReader) Read(buffer []byte) (int, error) {
	reader.reads++
	n := copy(buffer, reader.data)
	reader.data = reader.data[n:]
	return n, reader.err
}

type resultWriter struct {
	count int
	err   error
	calls int
}

func (writer *resultWriter) Write(buffer []byte) (int, error) {
	writer.calls++
	return writer.count, writer.err
}

func TestHostTextStreams(t *testing.T) {
	t.Run("borrowed input recognizes no Flush or Close", func(t *testing.T) {
		reader := &borrowedReader{Reader: strings.NewReader("hé🙂\r\nnext")}
		runHostFixture(t, bullruntime.Config{Stdin: reader}, "input")
		if reader.closes != 0 || reader.flushes != 0 {
			t.Fatal("input used an unrelated provider interface")
		}
	})
	t.Run("invalid UTF-8", func(t *testing.T) {
		writer := &resultWriter{}
		runHostFixture(t, bullruntime.Config{Stdout: writer, Stdin: bytes.NewReader([]byte{0xff})}, "unicode_streams")
		if writer.calls != 0 {
			t.Fatal("surrogates reached writer")
		}
	})
	t.Run("blocking write character count", func(t *testing.T) {
		writer := &resultWriter{count: 4, err: syscall.EAGAIN}
		runHostFixture(t, bullruntime.Config{Stdout: writer}, "blocking_write")
	})
	t.Run("split UTF-8 reads", func(t *testing.T) {
		reader := iotest.OneByteReader(strings.NewReader("hé🙂\r\nnext"))
		runHostFixture(t, bullruntime.Config{Stdin: reader}, "input")
	})
	t.Run("typed nil streams", func(t *testing.T) {
		var writer *bytes.Buffer
		for _, config := range []bullruntime.Config{{Stdout: writer}, {Stderr: writer}, {Stdin: writer}} {
			if runtime, err := bullruntime.NewWithConfig(config); runtime != nil || err == nil {
				t.Fatal("typed nil stream accepted")
			}
		}
	})
	t.Run("wrappers are isolated and import does not call providers", func(t *testing.T) {
		writer := &borrowedWriter{}
		config := bullruntime.Config{Stdout: writer, Stderr: writer}
		first, err := bullruntime.NewWithConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		second, err := bullruntime.NewWithConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		source := compileSource(t, "import sys\nassert sys.stdout is not sys.stderr\nassert not sys.stdout.closed\n")
		for _, runtime := range []*bullruntime.Runtime{first, second} {
			if _, err := runtime.ExecuteModule("check", source); err != nil {
				t.Fatal(err)
			}
		}
		if writer.flushes != 0 || writer.closes != 0 || writer.seeks != 0 || writer.Len() != 0 {
			t.Fatal("import invoked a provider")
		}
		if _, err := first.ExecuteModule("close", compileSource(t, "import sys\nsys.stdout.close()\nassert not sys.stderr.closed\n")); err != nil {
			t.Fatal(err)
		}
		if _, err := second.ExecuteModule("check", source); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("minimal output and replacement", func(t *testing.T) {
		var buffer bytes.Buffer
		runHostFixture(t, bullruntime.Config{Stdout: &buffer}, "output")
		if buffer.String() != "hé🙂\n" {
			t.Fatalf("output = %q", buffer.String())
		}
	})
	t.Run("Unicode input and EOF", func(t *testing.T) {
		reader := &resultReader{data: []byte("hé🙂\r\nnext"), err: io.EOF}
		runHostFixture(t, bullruntime.Config{Stdin: reader}, "input")
	})
	t.Run("data before read error", func(t *testing.T) {
		reader := &resultReader{data: []byte("hé🙂"), err: fs.ErrPermission}
		runHostFixture(t, bullruntime.Config{Stdin: reader}, "read_failure")
		if reader.reads != 1 {
			t.Fatalf("read calls = %d", reader.reads)
		}
	})
	t.Run("borrowed ownership and optional interfaces", func(t *testing.T) {
		writer := &borrowedWriter{}
		runHostFixture(t, bullruntime.Config{Stdout: writer}, "optional_streams")
		if writer.String() != "borrowed" || writer.closes != 0 || writer.seeks != 0 || writer.flushes != 2 {
			t.Fatalf("writer = %#v", writer)
		}
	})
	t.Run("flush failure closes wrapper", func(t *testing.T) {
		writer := &borrowedWriter{flushError: fs.ErrPermission}
		runHostFixture(t, bullruntime.Config{Stdout: writer}, "flush_failure")
		if writer.closes != 0 || writer.flushes != 2 {
			t.Fatalf("closes=%d flushes=%d", writer.closes, writer.flushes)
		}
	})
	t.Run("arguments checked before provider access", func(t *testing.T) {
		writer := &resultWriter{}
		reader := &resultReader{err: io.EOF}
		runHostFixture(t, bullruntime.Config{Stdout: writer, Stdin: reader}, "stream_arguments")
		if writer.calls != 0 || reader.reads != 0 {
			t.Fatal("invalid calls reached providers")
		}
	})
	for _, test := range []struct {
		name   string
		count  int
		err    error
		kind   string
		number string
	}{
		{"short write", 2, nil, "OSError", "5"},
		{"permission", 0, fs.ErrPermission, "PermissionError", "13"},
		{"partial error", 3, io.ErrClosedPipe, "BrokenPipeError", "32"},
		{"path privacy", 0, &fs.PathError{Op: "write", Path: "/private/host/path", Err: fs.ErrNotExist}, "FileNotFoundError", "2"},
		{"invalid count", 100, nil, "OSError", "5"},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := &resultWriter{count: test.count, err: test.err}
			module := runHostFixture(t, bullruntime.Config{Stdout: writer}, "write_failure")
			kind, _ := module.Get("error_type")
			number, _ := module.Get("error_errno")
			filename, _ := module.Get("error_filename")
			message, _ := module.Get("error_text")
			if kind.Repr() != "'"+test.kind+"'" || number.Repr() != test.number || filename.Repr() != "None" || strings.Contains(message.Repr(), "/private") {
				t.Fatalf("error = %v %v %v %v", kind, number, filename, message)
			}
			if writer.calls != 1 {
				t.Fatalf("write calls = %d", writer.calls)
			}
		})
	}
}

func TestPrintHost(t *testing.T) {
	t.Run("borrowed output", func(t *testing.T) {
		writer := &borrowedWriter{}
		runHostFixture(t, bullruntime.Config{Stdout: writer}, "print_output")
		if writer.String() != "hé🙂|7!" || writer.flushes != 2 || writer.closes != 0 {
			t.Fatalf("output=%q flushes=%d closes=%d", writer.String(), writer.flushes, writer.closes)
		}
	})
	t.Run("short write stops output", func(t *testing.T) {
		writer := &resultWriter{count: 1}
		runHostFixture(t, bullruntime.Config{Stdout: writer}, "print_failure")
		if writer.calls != 1 {
			t.Fatalf("write calls=%d, want 1", writer.calls)
		}
	})
	t.Run("flush failure", func(t *testing.T) {
		writer := &borrowedWriter{flushError: io.ErrClosedPipe}
		runHostFixture(t, bullruntime.Config{Stdout: writer}, "print_failure")
		if writer.String() != "abc later\n" || writer.flushes != 1 || writer.closes != 0 {
			t.Fatalf("output=%q flushes=%d closes=%d", writer.String(), writer.flushes, writer.closes)
		}
	})
}
