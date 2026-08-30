package runtime_test

import (
	"errors"
	"testing"

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
	loader := bullruntime.ModuleLoader(func(name string) (*bytecode.Code, bool, error) {
		source, found := sources[name]
		if !found {
			return nil, false, nil
		}
		loads[name]++
		return compileSource(t, source), true, nil
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
	loader := bullruntime.ModuleLoader(func(name string) (*bytecode.Code, bool, error) {
		loads[name]++
		if name != "beta" {
			return nil, false, nil
		}
		return compileSource(t, "import alpha\nobserved = alpha.state\n"), true, nil
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
	loader := bullruntime.ModuleLoader(func(name string) (*bytecode.Code, bool, error) {
		source, found := sources[name]
		if !found {
			return nil, false, nil
		}
		loads[name]++
		return compileSource(t, source), true, nil
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
	loader := bullruntime.ModuleLoader(func(name string) (*bytecode.Code, bool, error) {
		switch name {
		case "bridge":
			return compileSource(t, "import unavailable\n"), true, nil
		case "unavailable":
			return nil, false, loadFailure
		default:
			return nil, false, nil
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
