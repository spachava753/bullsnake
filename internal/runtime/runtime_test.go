package runtime_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

func TestModuleExecution(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   map[string]string
	}{
		{
			name: "integer addition",
			source: "left = 40\n" +
				"right = 2\n" +
				"answer = left + right\n",
			want: map[string]string{"left": "40", "right": "2", "answer": "42"},
		},
		{
			name: "arbitrary precision",
			source: "left = 99999999999999999999999999999999999999\n" +
				"right = 1\n" +
				"answer = left + right\n",
			want: map[string]string{
				"left":   "99999999999999999999999999999999999999",
				"right":  "1",
				"answer": "100000000000000000000000000000000000000",
			},
		},
		{
			name:   "pass and discarded expression",
			source: "pass\n40\nanswer = 2\n",
			want:   map[string]string{"answer": "2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := compileSource(t, test.source)
			runtime := bullruntime.New()
			module, err := runtime.ExecuteModule(test.name, code)
			if err != nil {
				t.Fatal(err)
			}
			if module.Name() != test.name {
				t.Fatalf("module name = %q, want %q", module.Name(), test.name)
			}
			for name, want := range test.want {
				value, ok := module.Get(name)
				if !ok {
					t.Fatalf("module has no %q binding", name)
				}
				if got := value.Repr(); got != want {
					t.Errorf("%s = %s, want %s", name, got, want)
				}
			}
			cached, ok := runtime.Module(test.name)
			if !ok || cached != module {
				t.Fatal("successful module was not cached by identity")
			}
		})
	}
}

func TestFunctionFrames(t *testing.T) {
	code := compileSource(t, "module_value = 10\n"+
		"def add(left, right):\n"+
		"    total = left + right\n"+
		"    return total\n"+
		"first = add(40, 2)\n"+
		"second = add(1, 2)\n"+
		"def add_module(value):\n"+
		"    return value + module_value\n"+
		"with_global = add_module(5)\n"+
		"def set_shared(value):\n"+
		"    global shared\n"+
		"    shared = value\n"+
		"implicit = set_shared(7)\n"+
		"def outer():\n"+
		"    def inner(value):\n"+
		"        local = value + 1\n"+
		"        return local\n"+
		"    return inner\n"+
		"first_inner = outer()\n"+
		"second_inner = outer()\n"+
		"fresh_inner = first_inner is not second_inner\n"+
		"nested = first_inner(8)\n"+
		"def countdown(value):\n"+
		"    if value:\n"+
		"        return countdown(value - 1)\n"+
		"    return value\n"+
		"recursive = countdown(50)\n"+
		"def identity(value, /):\n"+
		"    return value\n"+
		"positional_only = identity(9)\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("functions", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"first":           "42",
		"second":          "3",
		"with_global":     "15",
		"shared":          "7",
		"implicit":        "None",
		"fresh_inner":     "True",
		"nested":          "9",
		"recursive":       "0",
		"positional_only": "9",
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
	function, ok := module.Get("add")
	if !ok {
		t.Fatal("module has no add binding")
	}
	if got := function.TypeName(); got != "function" {
		t.Errorf("add type = %q, want function", got)
	}
	if got := function.Repr(); got != "<function add>" {
		t.Errorf("add repr = %q, want <function add>", got)
	}
}

func TestAnnotationFormatRejection(t *testing.T) {
	code := testCode(
		1,
		[]bytecode.Instruction{
			{Opcode: bytecode.LoadNotImplementedError},
			{Opcode: bytecode.RaiseVarargs, Operand: 1},
		},
		nil,
		nil,
	)
	_, err := bullruntime.New().ExecuteModule("annotation format", code)
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "NotImplementedError" {
		t.Errorf("exception type = %q, want NotImplementedError", got)
	}
	if got := raised.Exception().Message(); got != "" {
		t.Errorf("exception message = %q, want empty", got)
	}
}

func TestClassBuilderArguments(t *testing.T) {
	body := testCode(
		1,
		[]bytecode.Instruction{
			{Opcode: bytecode.LoadConst},
			{Opcode: bytecode.ReturnValue},
		},
		[]bytecode.Constant{bytecode.None()},
		nil,
	)
	tests := []struct {
		name        string
		code        *bytecode.Code
		wantMessage string
	}{
		{
			name: "too few",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadBuildClass},
					{Opcode: bytecode.Call},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantMessage: "__build_class__: not enough arguments",
		},
		{
			name: "body is not function",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadBuildClass},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.Call, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None(), bytecode.TextString("Broken")},
				nil,
			),
			wantMessage: "__build_class__: func must be a function",
		},
		{
			name: "name is not string",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 3,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadBuildClass},
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Call, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children:  []*bytecode.Code{body},
			}),
			wantMessage: "__build_class__: name is not a string",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := bullruntime.New().ExecuteModule("class error", test.code)
			var raised *bullruntime.UncaughtException
			if !errors.As(err, &raised) {
				t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
			}
			if got := raised.Exception().TypeName(); got != "TypeError" {
				t.Errorf("exception type = %q, want TypeError", got)
			}
			if got := raised.Exception().Message(); got != test.wantMessage {
				t.Errorf("exception message = %q, want %q", got, test.wantMessage)
			}
		})
	}
}

func TestPassingAssertion(t *testing.T) {
	code := compileSource(t, "assert True\nanswer = 42\n")
	module, err := bullruntime.New().ExecuteModule("assertions", code)
	if err != nil {
		t.Fatal(err)
	}
	answer, ok := module.Get("answer")
	if !ok {
		t.Fatal("module has no answer binding")
	}
	if got := answer.Repr(); got != "42" {
		t.Fatalf("answer = %s, want 42", got)
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

func TestImportFailures(t *testing.T) {
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
			name:        "module",
			source:      "import absent\n",
			wantType:    "ModuleNotFoundError",
			wantMessage: "No module named 'absent'",
		},
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

func TestIntegerShifts(t *testing.T) {
	hugeShift := "1" + strings.Repeat("0", 100)
	code := compileSource(t, "left_shift = 5 << 3\n"+
		"right_shift = 40 >> 3\n"+
		"negative_left = -5 << 2\n"+
		"negative_right = -5 >> 1\n"+
		"bool_left = True << 4\n"+
		"bool_right = 8 >> True\n"+
		"huge_right = 1 >> "+hugeShift+"\n"+
		"huge_negative_right = -1 >> "+hugeShift+"\n"+
		"huge_zero_left = 0 << "+hugeShift+"\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("integer shifts", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"left_shift":          "40",
		"right_shift":         "5",
		"negative_left":       "-20",
		"negative_right":      "-3",
		"bool_left":           "16",
		"bool_right":          "4",
		"huge_right":          "0",
		"huge_negative_right": "-1",
		"huge_zero_left":      "0",
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
}

func TestWhileLoops(t *testing.T) {
	code := compileSource(t, "count = 5\n"+
		"total = 0\n"+
		"while count:\n"+
		"    total = total + count\n"+
		"    count = count - 1\n"+
		"else:\n"+
		"    completed = 1\n"+
		"break_count = 3\n"+
		"while break_count:\n"+
		"    break_count = break_count - 1\n"+
		"    if break_count:\n"+
		"        continue\n"+
		"    break\n"+
		"else:\n"+
		"    skipped_else = missing\n"+
		"after_break = break_count\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("while loops", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"count":       "0",
		"total":       "15",
		"completed":   "1",
		"break_count": "0",
		"after_break": "0",
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
	if _, ok := module.Get("skipped_else"); ok {
		t.Fatal("break executed the while else suite")
	}
}

func TestForLoops(t *testing.T) {
	code := compileSource(t, "total = 0\n"+
		"for value in (1, 2, 3):\n"+
		"    total = total + value\n"+
		"else:\n"+
		"    completed = total\n"+
		"continued_total = 0\n"+
		"for value in [4, 5, 6]:\n"+
		"    if value == 5:\n"+
		"        continue\n"+
		"    continued_total = continued_total + value\n"+
		"else:\n"+
		"    continued = 1\n"+
		"for stopped in (7, 8, 9):\n"+
		"    if stopped == 8:\n"+
		"        break\n"+
		"else:\n"+
		"    skipped_else = missing\n"+
		"after_break = stopped\n"+
		"for absent in ():\n"+
		"    missing\n"+
		"else:\n"+
		"    empty_else = 11\n"+
		"pair_total = 0\n"+
		"for left, right in [(1, 2), (3, 4)]:\n"+
		"    pair_total = pair_total + left + right\n"+
		"nested_total = 0\n"+
		"for outer in (1, 2):\n"+
		"    for inner in [10, 20]:\n"+
		"        nested_total = nested_total + outer + inner\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("for loops", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"total":           "6",
		"completed":       "6",
		"continued_total": "10",
		"continued":       "1",
		"after_break":     "8",
		"empty_else":      "11",
		"pair_total":      "10",
		"nested_total":    "66",
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
	if _, ok := module.Get("skipped_else"); ok {
		t.Fatal("break executed the for else suite")
	}
	if _, ok := module.Get("absent"); ok {
		t.Fatal("empty iteration assigned its target")
	}
}

func TestHashContainerLoops(t *testing.T) {
	code := compileSource(t, "mapping = {'first': 1, 'second': 2}\n"+
		"mapping_total = 0\n"+
		"mapping_position = 1\n"+
		"mapping_order = 0\n"+
		"for key in mapping:\n"+
		"    mapping_total = mapping_total + mapping[key]\n"+
		"    if key == 'first':\n"+
		"        mapping_order = mapping_order + mapping_position\n"+
		"    else:\n"+
		"        mapping_order = mapping_order + mapping_position * 10\n"+
		"    mapping_position = mapping_position + 1\n"+
		"else:\n"+
		"    mapping_complete = True\n"+
		"set_total = 0\n"+
		"set_count = 0\n"+
		"for value in {3, 1, 2, 1}:\n"+
		"    set_total = set_total + value\n"+
		"    set_count = set_count + 1\n"+
		"for absent in {*()}:\n"+
		"    missing\n"+
		"else:\n"+
		"    empty_complete = True\n"+
		"pairs = {(4, 5): 1, (6, 7): 2}\n"+
		"pair_total = 0\n"+
		"for left, right in pairs:\n"+
		"    pair_total = pair_total + left + right\n"+
		"for key in mapping:\n"+
		"    mapping[key] = mapping[key] + 10\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("collection iteration", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"mapping":          "{'first': 11, 'second': 12}",
		"mapping_total":    "3",
		"mapping_order":    "21",
		"mapping_complete": "True",
		"set_total":        "6",
		"set_count":        "3",
		"empty_complete":   "True",
		"pair_total":       "22",
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
	if _, ok := module.Get("absent"); ok {
		t.Fatal("empty set iteration assigned its target")
	}
}

func TestUnicodeAndByteLoops(t *testing.T) {
	code := compileSource(t, "text_seen = {}\n"+
		"text_count = 0\n"+
		"for character in 'A\\u00e9\\U0001f40d\\ud800':\n"+
		"    text_seen[text_count] = character\n"+
		"    text_count = text_count + 1\n"+
		"byte_seen = {}\n"+
		"byte_count = 0\n"+
		"byte_total = 0\n"+
		"for octet in b'\\x00A\\xff':\n"+
		"    byte_seen[byte_count] = octet\n"+
		"    byte_count = byte_count + 1\n"+
		"    byte_total = byte_total + octet\n"+
		"for absent_text in '':\n"+
		"    missing\n"+
		"else:\n"+
		"    empty_text_complete = True\n"+
		"for absent_byte in b'':\n"+
		"    missing\n"+
		"else:\n"+
		"    empty_bytes_complete = True\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("text iteration", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"text_seen":            "{0: 'A', 1: 'é', 2: '🐍', 3: '\\ud800'}",
		"text_count":           "4",
		"byte_seen":            "{0: 0, 1: 65, 2: 255}",
		"byte_count":           "3",
		"byte_total":           "320",
		"empty_text_complete":  "True",
		"empty_bytes_complete": "True",
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
	if _, ok := module.Get("absent_text"); ok {
		t.Fatal("empty string iteration assigned its target")
	}
	if _, ok := module.Get("absent_byte"); ok {
		t.Fatal("empty bytes iteration assigned its target")
	}
}

func TestItemAssignmentAndDeletion(t *testing.T) {
	code := compileSource(t, "mapping = {'first': 1, 'second': 2}\n"+
		"mapping['first'] = 10\n"+
		"mapping['third'] = 3\n"+
		"mapping[True] = 'bool'\n"+
		"mapping[1] = 'integer'\n"+
		"mapping[(1, 2)] = 'pair'\n"+
		"del mapping['second']\n"+
		"mapping['second'] = 20\n"+
		"nested = {'inner': {}}\n"+
		"nested['inner']['value'] = 7\n"+
		"del nested['inner']['value']\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("dictionary mutation", code)
	if err != nil {
		t.Fatal(err)
	}
	mapping, ok := module.Get("mapping")
	if !ok {
		t.Fatal("module has no mapping binding")
	}
	wantMapping := "{'first': 10, 'third': 3, True: 'integer', " +
		"(1, 2): 'pair', 'second': 20}"
	if got := mapping.Repr(); got != wantMapping {
		t.Errorf("mapping = %s, want %s", got, wantMapping)
	}
	nested, ok := module.Get("nested")
	if !ok {
		t.Fatal("module has no nested binding")
	}
	if got := nested.Repr(); got != "{'inner': {}}" {
		t.Errorf("nested = %s, want {'inner': {}}", got)
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
