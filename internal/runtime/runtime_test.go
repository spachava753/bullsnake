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
