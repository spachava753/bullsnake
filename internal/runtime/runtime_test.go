package runtime_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
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

func TestDefaultArgumentBinding(t *testing.T) {
	code := compileSource(t, "seed = 10\n"+
		"def choose(first=seed, second=seed + 1):\n"+
		"    return first, second\n"+
		"seed = 99\n"+
		"both_defaulted = choose()\n"+
		"second_defaulted = choose(20)\n"+
		"none_defaulted = choose(20, 30)\n"+
		"def combine(required, optional=5):\n"+
		"    return required + optional\n"+
		"combined = combine(7)\n"+
		"def positional(first=1, /, second=2):\n"+
		"    return first, second\n"+
		"positional_defaults = positional()\n"+
		"marker = []\n"+
		"def retain(value=marker):\n"+
		"    return value\n"+
		"default_identity = retain() is marker\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("defaults", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"both_defaulted":      "(10, 11)",
		"second_defaulted":    "(20, 11)",
		"none_defaulted":      "(20, 30)",
		"combined":            "12",
		"positional_defaults": "(1, 2)",
		"default_identity":    "True",
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

func TestVariadicArgumentBinding(t *testing.T) {
	code := compileSource(t, "def collect(first, *items):\n"+
		"    local = first\n"+
		"    return local, items\n"+
		"empty_items = collect(1)\n"+
		"many_items = collect(1, 2, 3, 4)\n"+
		"def only(*items):\n"+
		"    return items\n"+
		"only_empty = only()\n"+
		"only_many = only(5, 6)\n"+
		"def defaulted(first=7, *items):\n"+
		"    return first, items\n"+
		"default_empty = defaulted()\n"+
		"default_many = defaulted(8, 9, 10)\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("varargs", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"empty_items":   "(1, ())",
		"many_items":    "(1, (2, 3, 4))",
		"only_empty":    "()",
		"only_many":     "(5, 6)",
		"default_empty": "(7, ())",
		"default_many":  "(8, (9, 10))",
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

func TestCallArgumentExpansion(t *testing.T) {
	code := compileSource(t, "def add(left, right):\n"+
		"    return left + right\n"+
		"from_tuple = add(*(40, 2))\n"+
		"from_list = add(*[20, 22])\n"+
		"def collect(*items):\n"+
		"    return items\n"+
		"mixed = collect(1, *[2, 3], 4, *(5, 6))\n"+
		"empty = collect(*())\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("unpacked calls", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"from_tuple": "42",
		"from_list":  "42",
		"mixed":      "(1, 2, 3, 4, 5, 6)",
		"empty":      "()",
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

func TestKeywordArgumentCalls(t *testing.T) {
	code := compileSource(t, "def combine(first, second, third=3):\n"+
		"    return first, second, third\n"+
		"all_keywords = combine(first=1, second=2)\n"+
		"mixed = combine(4, third=6, second=5)\n"+
		"options = {'second': 8}\n"+
		"unpacked = combine(7, **options)\n"+
		"expanded = combine(*(9,), **{'second': 10, 'third': 11})\n"+
		"def positional(first, /, second):\n"+
		"    return first, second\n"+
		"positional_ok = positional(12, second=13)\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("keyword calls", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"all_keywords":  "(1, 2, 3)",
		"mixed":         "(4, 5, 6)",
		"unpacked":      "(7, 8, 3)",
		"expanded":      "(9, 10, 11)",
		"positional_ok": "(12, 13)",
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

func TestNamedOnlyParameters(t *testing.T) {
	code := compileSource(t, "def configure(*, required, mode='safe', retries=3, verbose):\n"+
		"    return required, mode, retries, verbose\n"+
		"defaulted = configure(required=1, verbose=True)\n"+
		"overridden = configure(required=2, mode='fast', retries=5, verbose=False)\n"+
		"def mixed(first=10, *items, flag, mode='mixed'):\n"+
		"    return first, items, flag, mode\n"+
		"mixed_default = mixed(flag=True)\n"+
		"mixed_values = mixed(20, 30, 40, flag=False, mode='custom')\n"+
		"marker = []\n"+
		"def retain(*, value=marker):\n"+
		"    return value\n"+
		"default_identity = retain() is marker\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("keyword only", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"defaulted":        "(1, 'safe', 3, True)",
		"overridden":       "(2, 'fast', 5, False)",
		"mixed_default":    "(10, (), True, 'mixed')",
		"mixed_values":     "(20, (30, 40), False, 'custom')",
		"default_identity": "True",
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

func TestCatchAllKeywordBinding(t *testing.T) {
	code := compileSource(t, "def collect(first=1, **options):\n"+
		"    return first, options\n"+
		"empty = collect()\n"+
		"filled = collect(2, mode='fast', retries=3)\n"+
		"def mixed(first, *items, flag='default', **options):\n"+
		"    return first, items, flag, options\n"+
		"combined = mixed(10, 20, flag='set', extra=30)\n"+
		"def preserve(name, /, **options):\n"+
		"    return name, options\n"+
		"preserved = preserve('bound', name='extra')\n"+
		"def fresh(**options):\n"+
		"    return options\n"+
		"distinct = fresh() is not fresh()\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("variadic keywords", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"empty":     "(1, {})",
		"filled":    "(2, {'mode': 'fast', 'retries': 3})",
		"combined":  "(10, (20,), 'set', {'extra': 30})",
		"preserved": "('bound', {'name': 'extra'})",
		"distinct":  "True",
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

func TestLexicalClosures(t *testing.T) {
	code := compileSource(t, "def make(value):\n"+
		"    offset = 2\n"+
		"    def apply(item):\n"+
		"        return value + offset + item\n"+
		"    return apply\n"+
		"captured = make(40)(0)\n"+
		"def counter(start):\n"+
		"    value = start\n"+
		"    def set_value(new):\n"+
		"        nonlocal value\n"+
		"        value = new\n"+
		"    def read():\n"+
		"        return value\n"+
		"    return set_value, read\n"+
		"set_value, read = counter(3)\n"+
		"before = read()\n"+
		"set_result = set_value(9)\n"+
		"after = read()\n"+
		"def outer():\n"+
		"    a = 1\n"+
		"    z = 2\n"+
		"    def middle():\n"+
		"        def inner():\n"+
		"            return z, a\n"+
		"        return inner\n"+
		"    return middle\n"+
		"transitive = outer()()()\n"+
		"def make_lambda(offset, default):\n"+
		"    return lambda value=default: offset + value\n"+
		"lambda_result = make_lambda(10, 32)()\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("closures", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"captured":      "42",
		"before":        "3",
		"set_result":    "None",
		"after":         "9",
		"transitive":    "(2, 1)",
		"lambda_result": "42",
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

func TestDecoratedDefinitions(t *testing.T) {
	code := compileSource(t, "order = 0\n"+
		"def record(value):\n"+
		"    global order\n"+
		"    order = order * 10 + value\n"+
		"    return value\n"+
		"def decorate(label):\n"+
		"    record(label)\n"+
		"    def apply(function):\n"+
		"        record(label + 2)\n"+
		"        def wrapped(value):\n"+
		"            return function(value) + label\n"+
		"        return wrapped\n"+
		"    return apply\n"+
		"@decorate(1)\n"+
		"@decorate(2)\n"+
		"def target(value=record(5)):\n"+
		"    return value\n"+
		"observed_order = order\n"+
		"decorated = target(10)\n"+
		"def replace(function):\n"+
		"    def replacement():\n"+
		"        return 42\n"+
		"    return replacement\n"+
		"@replace\n"+
		"def ignored():\n"+
		"    return 0\n"+
		"replaced = ignored()\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("decorators", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"observed_order": "12543",
		"decorated":      "13",
		"replaced":       "42",
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

func TestDeferredFunctionAnnotations(t *testing.T) {
	code := compileSource(t, "events = 0\n"+
		"def mark():\n"+
		"    global events\n"+
		"    events = events + 1\n"+
		"    return 99\n"+
		"def identity(value: mark()) -> mark():\n"+
		"    return value\n"+
		"before = events\n"+
		"result = identity(42)\n"+
		"after = events\n"+
		"def outer(annotation):\n"+
		"    def nested(value: annotation) -> annotation:\n"+
		"        return value\n"+
		"    return nested\n"+
		"nested_result = outer('kind')(7)\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("annotations", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"before":        "0",
		"result":        "42",
		"after":         "0",
		"nested_result": "7",
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

func TestConstructedClassDefinitions(t *testing.T) {
	code := compileSource(t, "marker = 0\n"+
		"class Empty:\n"+
		"    global marker\n"+
		"    marker = 41\n"+
		"    value = 42\n"+
		"    def method(self):\n"+
		"        return 7\n"+
		"created = Empty\n"+
		"same = created is Empty\n"+
		"observed_marker = marker\n"+
		"def preserve(cls):\n"+
		"    return cls\n"+
		"@preserve\n"+
		"class Decorated:\n"+
		"    pass\n"+
		"decorated = Decorated\n"+
		"def make(value):\n"+
		"    class Inner:\n"+
		"        global marker\n"+
		"        marker = value\n"+
		"    return Inner\n"+
		"nested = make(43)\n"+
		"nested_marker = marker\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("classes", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"created":         "<class 'classes.Empty'>",
		"same":            "True",
		"observed_marker": "41",
		"decorated":       "<class 'classes.Decorated'>",
		"nested":          "<class 'classes.make.<locals>.Inner'>",
		"nested_marker":   "43",
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

func TestUnaryOperators(t *testing.T) {
	code := compileSource(t, "positive_integer = +7\n"+
		"positive_bool = +True\n"+
		"negative_integer = -7\n"+
		"negative_bool = -True\n"+
		"inverted_integer = ~7\n"+
		"inverted_bool = ~False\n"+
		"negative_float = -1.25\n"+
		"positive_imaginary = +2j\n"+
		"negative_imaginary = -2j\n"+
		"not_none = not None\n"+
		"not_false = not False\n"+
		"not_zero = not 0\n"+
		"not_zero_float = not 0.0\n"+
		"not_zero_complex = not 0j\n"+
		"not_empty_text = not ''\n"+
		"not_empty_bytes = not b''\n"+
		"not_ellipsis = not ...\n"+
		"not_nonzero = not 1\n"+
		"not_text = not 'x'\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("unary", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"positive_integer":   "7",
		"positive_bool":      "1",
		"negative_integer":   "-7",
		"negative_bool":      "-1",
		"inverted_integer":   "-8",
		"inverted_bool":      "-1",
		"negative_float":     "-1.25",
		"positive_imaginary": "2j",
		"negative_imaginary": "(-0-2j)",
		"not_none":           "True",
		"not_false":          "True",
		"not_zero":           "True",
		"not_zero_float":     "True",
		"not_zero_complex":   "True",
		"not_empty_text":     "True",
		"not_empty_bytes":    "True",
		"not_ellipsis":       "False",
		"not_nonzero":        "False",
		"not_text":           "False",
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

func TestBasicIntegerOperations(t *testing.T) {
	code := compileSource(t, "addition = 40 + 2\n"+
		"bool_addition = True + 2\n"+
		"subtraction = 1000000000000000000000000000000 - 1\n"+
		"multiplication = -12 * 11\n"+
		"bool_multiplication = False * 99\n"+
		"bitwise_or = 10 | 5\n"+
		"bitwise_xor = 10 ^ 3\n"+
		"bitwise_and = 10 & 6\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("integer binary", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"addition":            "42",
		"bool_addition":       "3",
		"subtraction":         "999999999999999999999999999999",
		"multiplication":      "-132",
		"bool_multiplication": "0",
		"bitwise_or":          "15",
		"bitwise_xor":         "9",
		"bitwise_and":         "2",
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

func TestFloorDivisionAndModulo(t *testing.T) {
	code := compileSource(t, "positive_floor = 5 // 2\n"+
		"left_negative_floor = -5 // 2\n"+
		"right_negative_floor = 5 // -2\n"+
		"both_negative_floor = -5 // -2\n"+
		"left_negative_modulo = -5 % 2\n"+
		"right_negative_modulo = 5 % -2\n"+
		"both_negative_modulo = -5 % -2\n"+
		"large_floor = 1000000000000000000000000000000 // 3\n"+
		"large_modulo = 1000000000000000000000000000000 % 3\n"+
		"bool_floor = True // True\n"+
		"bool_modulo = False % True\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("integer floor division", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"positive_floor":        "2",
		"left_negative_floor":   "-3",
		"right_negative_floor":  "-3",
		"both_negative_floor":   "2",
		"left_negative_modulo":  "1",
		"right_negative_modulo": "-1",
		"both_negative_modulo":  "-1",
		"large_floor":           "333333333333333333333333333333",
		"large_modulo":          "1",
		"bool_floor":            "1",
		"bool_modulo":           "0",
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

func TestBooleanAndConditionalControlFlow(t *testing.T) {
	code := compileSource(t, "false_and = 0 and missing\n"+
		"true_and = 5 and 9\n"+
		"true_or = 5 or missing\n"+
		"false_or = 0 or 9\n"+
		"true_conditional = 10 if 'x' else missing\n"+
		"false_conditional = missing if '' else 20\n"+
		"if None:\n"+
		"    branch = missing\n"+
		"else:\n"+
		"    branch = 30\n"+
		"if 1:\n"+
		"    second_branch = 40\n"+
		"else:\n"+
		"    second_branch = missing\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("control flow", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"false_and":         "0",
		"true_and":          "9",
		"true_or":           "5",
		"false_or":          "9",
		"true_conditional":  "10",
		"false_conditional": "20",
		"branch":            "30",
		"second_branch":     "40",
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

func TestComparisons(t *testing.T) {
	code := compileSource(t, "equal = 2 == 2\n"+
		"not_equal = 2 != 3\n"+
		"less = 2 < 3\n"+
		"less_equal = 2 <= 2\n"+
		"greater = 3 > 2\n"+
		"greater_equal = 3 >= 3\n"+
		"bool_integer_equal = True == 1\n"+
		"mixed_numeric_equal = 1 == 1.0\n"+
		"mixed_numeric_order = 1 < 1.5\n"+
		"complex_zero_equal = 0j == 0\n"+
		"text_order = 'alpha' < 'beta'\n"+
		"bytes_order = b'beta' > b'alpha'\n"+
		"none_equal = None == None\n"+
		"different_types = None != 1\n"+
		"same_identity = None is None\n"+
		"different_identity = None is not ...\n"+
		"true_chain = 1 < 2 < 3\n"+
		"false_chain = 1 < 3 < 2\n"+
		"short_chain = 3 < 2 < missing\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("comparisons", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"equal":               "True",
		"not_equal":           "True",
		"less":                "True",
		"less_equal":          "True",
		"greater":             "True",
		"greater_equal":       "True",
		"bool_integer_equal":  "True",
		"mixed_numeric_equal": "True",
		"mixed_numeric_order": "True",
		"complex_zero_equal":  "True",
		"text_order":          "True",
		"bytes_order":         "True",
		"none_equal":          "True",
		"different_types":     "True",
		"same_identity":       "True",
		"different_identity":  "True",
		"true_chain":          "True",
		"false_chain":         "False",
		"short_chain":         "False",
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

func TestCollectionDisplays(t *testing.T) {
	code := compileSource(t, "empty_tuple = ()\n"+
		"tuple_value = (1, True, 'text')\n"+
		"single_tuple = (1,)\n"+
		"empty_list = []\n"+
		"list_value = [1, None, [2, 3]]\n"+
		"tuple_is_false = not empty_tuple\n"+
		"list_is_false = not empty_list\n"+
		"list_is_true = not list_value\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("sequence displays", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"empty_tuple":    "()",
		"tuple_value":    "(1, True, 'text')",
		"single_tuple":   "(1,)",
		"empty_list":     "[]",
		"list_value":     "[1, None, [2, 3]]",
		"tuple_is_false": "True",
		"list_is_false":  "True",
		"list_is_true":   "False",
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

func TestSequenceIndexing(t *testing.T) {
	code := compileSource(t, "tuple_value = (10, 20, 30)\n"+
		"tuple_first = tuple_value[0]\n"+
		"tuple_last = tuple_value[-1]\n"+
		"tuple_bool = tuple_value[True]\n"+
		"list_value = [40, 50, 60]\n"+
		"list_first = list_value[0]\n"+
		"list_last = list_value[-1]\n"+
		"list_bool = list_value[False]\n"+
		"nested = [(1, 2), [3, 4]][1][0]\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("sequence indexing", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"tuple_first": "10",
		"tuple_last":  "30",
		"tuple_bool":  "20",
		"list_first":  "40",
		"list_last":   "60",
		"list_bool":   "40",
		"nested":      "3",
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

func TestTextAndBytesSubscription(t *testing.T) {
	code := compileSource(t, "text = 'A\\u00e9\\u2603\\ud800Z'\n"+
		"text_first = text[0]\n"+
		"text_accent = text[1]\n"+
		"text_snowman = text[2]\n"+
		"text_surrogate = text[3]\n"+
		"text_last = text[-1]\n"+
		"text_bool = text[True]\n"+
		"text_middle = text[1:4]\n"+
		"text_reverse = text[::-1]\n"+
		"text_stride = text[4:0:-2]\n"+
		"text_empty = text[10:]\n"+
		"text_same = text[:] is text\n"+
		"text_snake = '\\U0001f40d'[0]\n"+
		"data = b'\\x00A\\xff'\n"+
		"byte_first = data[0]\n"+
		"byte_middle = data[1]\n"+
		"byte_last = data[-1]\n"+
		"byte_bool = data[True]\n"+
		"bytes_tail = data[1:]\n"+
		"bytes_reverse = data[::-1]\n"+
		"bytes_stride = data[::2]\n"+
		"bytes_empty = data[3:1]\n"+
		"bytes_same = data[:] is data\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("text subscription", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"text_first":     "'A'",
		"text_accent":    "'é'",
		"text_snowman":   "'☃'",
		"text_surrogate": "'\\ud800'",
		"text_last":      "'Z'",
		"text_bool":      "'é'",
		"text_middle":    "'é☃\\ud800'",
		"text_reverse":   "'Z\\ud800☃éA'",
		"text_stride":    "'Z☃'",
		"text_empty":     "''",
		"text_same":      "True",
		"text_snake":     "'🐍'",
		"byte_first":     "0",
		"byte_middle":    "65",
		"byte_last":      "255",
		"byte_bool":      "65",
		"bytes_tail":     "b'A\\xff'",
		"bytes_reverse":  "b'\\xffA\\x00'",
		"bytes_stride":   "b'\\x00\\xff'",
		"bytes_empty":    "b''",
		"bytes_same":     "True",
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

func TestTupleAndListSlicing(t *testing.T) {
	code := compileSource(t, "tuple_value = (0, 1, 2, 3, 4)\n"+
		"tuple_middle = tuple_value[1:4]\n"+
		"tuple_reverse = tuple_value[::-1]\n"+
		"tuple_stride = tuple_value[4:0:-2]\n"+
		"tuple_clipped = tuple_value[-100:100]\n"+
		"tuple_same = tuple_value[:] is tuple_value\n"+
		"list_value = [0, 1, 2, 3, 4]\n"+
		"list_middle = list_value[-4:-1]\n"+
		"list_reverse = list_value[::-1]\n"+
		"list_stride = list_value[::2]\n"+
		"list_empty = list_value[3:1]\n"+
		"list_bool_bounds = list_value[False:True]\n"+
		"list_none_step = list_value[1:4:None]\n"+
		"list_huge = list_value[-1000000000000000000000000000000:"+
		"1000000000000000000000000000000:"+
		"1000000000000000000000000000000]\n"+
		"list_copy = list_value[:]\n"+
		"list_same = list_copy is list_value\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("sequence slicing", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"tuple_middle":     "(1, 2, 3)",
		"tuple_reverse":    "(4, 3, 2, 1, 0)",
		"tuple_stride":     "(4, 2)",
		"tuple_clipped":    "(0, 1, 2, 3, 4)",
		"tuple_same":       "True",
		"list_middle":      "[1, 2, 3]",
		"list_reverse":     "[4, 3, 2, 1, 0]",
		"list_stride":      "[0, 2, 4]",
		"list_empty":       "[]",
		"list_bool_bounds": "[0]",
		"list_none_step":   "[1, 2, 3]",
		"list_huge":        "[0]",
		"list_copy":        "[0, 1, 2, 3, 4]",
		"list_same":        "False",
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

func TestStarredTupleAndListDisplays(t *testing.T) {
	code := compileSource(t, "source = [1, 2]\n"+
		"list_value = [0, *source, 3, *(4, 5)]\n"+
		"tuple_value = (0, *source, 3, *[4, 5])\n"+
		"nested = [*[(1, 2)], *[[3, 4]]]\n"+
		"empty_list = [*()]\n"+
		"empty_tuple = (*[],)\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("starred displays", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"source":      "[1, 2]",
		"list_value":  "[0, 1, 2, 3, 4, 5]",
		"tuple_value": "(0, 1, 2, 3, 4, 5)",
		"nested":      "[(1, 2), [3, 4]]",
		"empty_list":  "[]",
		"empty_tuple": "()",
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

func TestDictionaryDisplays(t *testing.T) {
	code := compileSource(t, "empty = {}\n"+
		"ordered = {'first': 1, 'second': 2}\n"+
		"duplicate = {'key': 1, 'other': 0, 'key': 2}\n"+
		"numeric = {True: 'bool', 1: 'int', 1.0: 'float'}\n"+
		"tuple_key = {(1, 2): 'pair'}\n"+
		"tuple_duplicate = {(1, 2): 'first', (1, 2): 'second'}\n"+
		"nested = {'list': [1, 2], 'dict': {'x': 3}}\n"+
		"empty_false = not empty\n"+
		"ordered_false = not ordered\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("dictionary displays", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"empty":           "{}",
		"ordered":         "{'first': 1, 'second': 2}",
		"duplicate":       "{'key': 2, 'other': 0}",
		"numeric":         "{True: 'float'}",
		"tuple_key":       "{(1, 2): 'pair'}",
		"tuple_duplicate": "{(1, 2): 'second'}",
		"nested":          "{'list': [1, 2], 'dict': {'x': 3}}",
		"empty_false":     "True",
		"ordered_false":   "False",
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

func TestMappingSubscription(t *testing.T) {
	code := compileSource(t, "mapping = {'name': 'bullsnake', (1, 2): 'pair', True: 'truth'}\n"+
		"by_name = mapping['name']\n"+
		"by_tuple = mapping[(1, 2)]\n"+
		"by_integer = mapping[1]\n"+
		"nested = {'inner': {'value': 7}}['inner']['value']\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("dictionary subscription", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"by_name":    "'bullsnake'",
		"by_tuple":   "'pair'",
		"by_integer": "'truth'",
		"nested":     "7",
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

func TestUnpackedDictionaryDisplays(t *testing.T) {
	code := compileSource(t, "base = {'first': 1, 'second': 2}\n"+
		"copied = {**base}\n"+
		"mixed = {'first': 0, **base, 'third': 3}\n"+
		"replaced = {**{'a': 1, 'b': 2}, **{'b': 20, 'c': 3}, 'a': 10}\n"+
		"empty = {**{}}\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("unpacked dictionaries", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"base":     "{'first': 1, 'second': 2}",
		"copied":   "{'first': 1, 'second': 2}",
		"mixed":    "{'first': 1, 'second': 2, 'third': 3}",
		"replaced": "{'a': 10, 'b': 20, 'c': 3}",
		"empty":    "{}",
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

func TestSetDisplays(t *testing.T) {
	code := compileSource(t, "values = {3, 1, 2, 1}\n"+
		"numeric = {True, 1, 1.0, False, 0}\n"+
		"tuples = {(1, 2), (1, 2), (3, 4)}\n"+
		"starred = {0, *[1, 2], *(2, 3)}\n"+
		"from_set = {*{1, 2}, 3}\n"+
		"empty = {*()}\n"+
		"empty_false = not empty\n"+
		"values_false = not values\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("set displays", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"values":       "{3, 1, 2}",
		"numeric":      "{True, False}",
		"tuples":       "{(1, 2), (3, 4)}",
		"starred":      "{0, 1, 2, 3}",
		"from_set":     "{1, 2, 3}",
		"empty":        "set()",
		"empty_false":  "True",
		"values_false": "False",
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

func TestMembershipOperations(t *testing.T) {
	code := compileSource(t, "tuple_hit = 2 in (1, 2, 3)\n"+
		"tuple_miss = 4 in (1, 2, 3)\n"+
		"list_not_in = 4 not in [1, 2, 3]\n"+
		"dict_key = 'key' in {'key': 1}\n"+
		"dict_value = 1 in {'key': 1}\n"+
		"dict_tuple = (1, 2) in {(1, 2): 'pair'}\n"+
		"set_numeric = 1 in {True}\n"+
		"set_miss = 3 in {1, 2}\n"+
		"set_not_in = 3 not in {1, 2}\n"+
		"text_character = '\\u00e9' in 'caf\\u00e9'\n"+
		"text_substring = 'af\\u00e9' in 'caf\\u00e9'\n"+
		"text_surrogate = '\\ud800' in 'a\\ud800b'\n"+
		"text_empty = '' in 'bullsnake'\n"+
		"text_not_in = 'python' not in 'bullsnake'\n"+
		"bytes_integer = 65 in b'\\x00A\\xff'\n"+
		"bytes_bool = False in b'\\x00A\\xff'\n"+
		"bytes_subsequence = b'A\\xff' in b'\\x00A\\xff'\n"+
		"bytes_empty = b'' in b'bullsnake'\n"+
		"bytes_not_in = b'python' not in b'bullsnake'\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("collection membership", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"tuple_hit":         "True",
		"tuple_miss":        "False",
		"list_not_in":       "True",
		"dict_key":          "True",
		"dict_value":        "False",
		"dict_tuple":        "True",
		"set_numeric":       "True",
		"set_miss":          "False",
		"set_not_in":        "True",
		"text_character":    "True",
		"text_substring":    "True",
		"text_surrogate":    "True",
		"text_empty":        "True",
		"text_not_in":       "True",
		"bytes_integer":     "True",
		"bytes_bool":        "True",
		"bytes_subsequence": "True",
		"bytes_empty":       "True",
		"bytes_not_in":      "True",
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

func TestDestructuringAssignment(t *testing.T) {
	code := compileSource(t, "first, second = (1, 2)\n"+
		"[third, fourth] = [3, 4]\n"+
		"left, (middle, right) = [5, (6, 7)]\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("sequence unpacking", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"first":  "1",
		"second": "2",
		"third":  "3",
		"fourth": "4",
		"left":   "5",
		"middle": "6",
		"right":  "7",
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

func TestExtendedDestructuringAssignment(t *testing.T) {
	code := compileSource(t, "first, *middle, last = (1, 2, 3, 4)\n"+
		"head, *tail = [5, 6, 7]\n"+
		"*prefix, end = (8, 9)\n"+
		"only, *empty = [10]\n"+
		"(left, *center), right = [(11, 12, 13), 14]\n"+
		"[list_left, *list_middle, list_right] = [15, 16, 17, 18]\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("starred unpacking", code)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"first":       "1",
		"middle":      "[2, 3]",
		"last":        "4",
		"head":        "5",
		"tail":        "[6, 7]",
		"prefix":      "[8]",
		"end":         "9",
		"only":        "10",
		"empty":       "[]",
		"left":        "11",
		"center":      "[12, 13]",
		"right":       "14",
		"list_left":   "15",
		"list_middle": "[16, 17]",
		"list_right":  "18",
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

func TestPythonExceptions(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantType    string
		wantMessage string
	}{
		{
			name: "unbound local cell",
			source: "def outer():\n" +
				"    def read():\n" +
				"        return value\n" +
				"    current = value\n" +
				"    value = 1\n" +
				"    return current\n" +
				"answer = outer()\n",
			wantType:    "UnboundLocalError",
			wantMessage: "cannot access local variable 'value' where it is not associated with a value",
		},
		{
			name: "unbound free cell",
			source: "def outer():\n" +
				"    def read():\n" +
				"        return value\n" +
				"    result = read()\n" +
				"    value = 1\n" +
				"    return result\n" +
				"answer = outer()\n",
			wantType:    "NameError",
			wantMessage: "cannot access free variable 'value' where it is not associated with a value in enclosing scope",
		},
		{
			name: "delete empty free cell",
			source: "def outer():\n" +
				"    value = 1\n" +
				"    def clear():\n" +
				"        nonlocal value\n" +
				"        del value\n" +
				"        del value\n" +
				"    clear()\n" +
				"answer = outer()\n",
			wantType:    "NameError",
			wantMessage: "cannot access free variable 'value' where it is not associated with a value in enclosing scope",
		},
		{
			name: "missing keyword-only arguments",
			source: "def configure(*, required, mode='safe', verbose):\n" +
				"    return required, mode, verbose\n" +
				"answer = configure()\n",
			wantType:    "TypeError",
			wantMessage: "configure() missing 2 required keyword-only arguments: 'required' and 'verbose'",
		},
		{
			name: "positional and keyword duplicate",
			source: "def combine(first, second):\n" +
				"    return first + second\n" +
				"answer = combine(1, first=2, second=3)\n",
			wantType:    "TypeError",
			wantMessage: "combine() got multiple values for argument 'first'",
		},
		{
			name: "positional-only keyword",
			source: "def combine(first, /, second):\n" +
				"    return first + second\n" +
				"answer = combine(first=1, second=2)\n",
			wantType:    "TypeError",
			wantMessage: "combine() got some positional-only arguments passed as keyword arguments: 'first'",
		},
		{
			name: "unexpected keyword",
			source: "def identity(value):\n" +
				"    return value\n" +
				"answer = identity(missing=1)\n",
			wantType:    "TypeError",
			wantMessage: "identity() got an unexpected keyword argument 'missing'",
		},
		{
			name: "duplicate expanded keyword",
			source: "def identity(value):\n" +
				"    return value\n" +
				"answer = identity(value=1, **{'value': 2})\n",
			wantType:    "TypeError",
			wantMessage: "identity() got multiple values for keyword argument 'value'",
		},
		{
			name: "non-mapping keyword expansion",
			source: "def identity(value):\n" +
				"    return value\n" +
				"answer = identity(**1)\n",
			wantType:    "TypeError",
			wantMessage: "identity() argument after ** must be a mapping, not int",
		},
		{
			name: "non-string keyword",
			source: "def identity(value):\n" +
				"    return value\n" +
				"answer = identity(**{1: 2})\n",
			wantType:    "TypeError",
			wantMessage: "identity() keywords must be strings",
		},
		{
			name: "missing argument after keywords",
			source: "def combine(first, second):\n" +
				"    return first + second\n" +
				"answer = combine(second=2)\n",
			wantType:    "TypeError",
			wantMessage: "combine() missing 1 required positional argument: 'first'",
		},
		{
			name: "missing positional argument after unpacking",
			source: "def add(left, right):\n" +
				"    return left + right\n" +
				"answer = add(*(1,))\n",
			wantType:    "TypeError",
			wantMessage: "add() missing 1 required positional argument: 'right'",
		},
		{
			name: "missing required argument before varargs",
			source: "def collect(first, *items):\n" +
				"    return first, items\n" +
				"answer = collect()\n",
			wantType:    "TypeError",
			wantMessage: "collect() missing 1 required positional argument: 'first'",
		},
		{
			name: "missing required argument before defaults",
			source: "def choose(required, optional=2):\n" +
				"    return required + optional\n" +
				"answer = choose()\n",
			wantType:    "TypeError",
			wantMessage: "choose() missing 1 required positional argument: 'required'",
		},
		{
			name: "too many arguments with defaults",
			source: "def choose(required, optional=2):\n" +
				"    return required + optional\n" +
				"answer = choose(1, 2, 3)\n",
			wantType:    "TypeError",
			wantMessage: "choose() takes from 1 to 2 positional arguments but 3 were given",
		},
		{
			name:        "non-callable value",
			source:      "answer = 1()\n",
			wantType:    "TypeError",
			wantMessage: "'int' object is not callable",
		},
		{
			name: "missing positional argument",
			source: "def add(left, right):\n" +
				"    return left + right\n" +
				"answer = add(1)\n",
			wantType:    "TypeError",
			wantMessage: "add() missing 1 required positional argument: 'right'",
		},
		{
			name: "too many positional arguments",
			source: "def add(left, right):\n" +
				"    return left + right\n" +
				"answer = add(1, 2, 3)\n",
			wantType:    "TypeError",
			wantMessage: "add() takes 2 positional arguments but 3 were given",
		},
		{
			name: "unbound fast local",
			source: "def read():\n" +
				"    observed = value\n" +
				"    value = 1\n" +
				"    return observed\n" +
				"answer = read()\n",
			wantType:    "UnboundLocalError",
			wantMessage: "cannot access local variable 'value' where it is not associated with a value",
		},
		{
			name:        "missing name",
			source:      "answer = missing\n",
			wantType:    "NameError",
			wantMessage: "name 'missing' is not defined",
		},
		{
			name:        "unsupported addition",
			source:      "answer = None + 1\n",
			wantType:    "TypeError",
			wantMessage: "unsupported operand type(s) for +: 'NoneType' and 'int'",
		},
		{
			name:        "floor division by zero",
			source:      "answer = 1 // 0\n",
			wantType:    "ZeroDivisionError",
			wantMessage: "integer division or modulo by zero",
		},
		{
			name:        "modulo by zero",
			source:      "answer = 1 % 0\n",
			wantType:    "ZeroDivisionError",
			wantMessage: "integer division or modulo by zero",
		},
		{
			name:        "negative left shift",
			source:      "answer = 1 << -1\n",
			wantType:    "ValueError",
			wantMessage: "negative shift count",
		},
		{
			name:        "negative right shift",
			source:      "answer = 1 >> -1\n",
			wantType:    "ValueError",
			wantMessage: "negative shift count",
		},
		{
			name: "oversized left shift",
			source: "answer = 1 << 1" + strings.Repeat("0", 100) +
				"\n",
			wantType:    "OverflowError",
			wantMessage: "too many digits in integer",
		},
		{
			name:        "unsupported shift count",
			source:      "answer = 1 << 1.0\n",
			wantType:    "TypeError",
			wantMessage: "unsupported operand type(s) for <<: 'int' and 'float'",
		},
		{
			name:        "unsupported subtraction",
			source:      "answer = 1 - None\n",
			wantType:    "TypeError",
			wantMessage: "unsupported operand type(s) for -: 'int' and 'NoneType'",
		},
		{
			name:        "unsupported bitwise and",
			source:      "answer = 1 & 1.0\n",
			wantType:    "TypeError",
			wantMessage: "unsupported operand type(s) for &: 'int' and 'float'",
		},
		{
			name:        "non-string text membership",
			source:      "answer = 1 in 'abc'\n",
			wantType:    "TypeError",
			wantMessage: "'in <string>' requires string as left operand, not int",
		},
		{
			name:        "string in bytes membership",
			source:      "answer = 'A' in b'ABC'\n",
			wantType:    "TypeError",
			wantMessage: "a bytes-like object is required, not 'str'",
		},
		{
			name:        "out-of-range byte membership",
			source:      "answer = 256 in b'ABC'\n",
			wantType:    "ValueError",
			wantMessage: "byte must be in range(0, 256)",
		},
		{
			name:        "huge integer byte membership",
			source:      "answer = 1000000000000000000000000000000 in b'ABC'\n",
			wantType:    "TypeError",
			wantMessage: "a bytes-like object is required, not 'int'",
		},
		{
			name:        "unhashable set membership",
			source:      "answer = [] in {1}\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'list' as a set element (unhashable type: 'list')",
		},
		{
			name:        "unhashable dictionary membership",
			source:      "answer = [] in {}\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'list' as a dict key (unhashable type: 'list')",
		},
		{
			name:        "membership in non-container",
			source:      "answer = 1 in 2\n",
			wantType:    "TypeError",
			wantMessage: "argument of type 'int' is not a container or iterable",
		},
		{
			name:        "unhashable set element",
			source:      "answer = {[1]}\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'list' as a set element (unhashable type: 'list')",
		},
		{
			name:        "nested unhashable set element",
			source:      "answer = {([1],)}\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'tuple' as a set element (unhashable type: 'list')",
		},
		{
			name:        "non-iterable starred set",
			source:      "answer = {*1}\n",
			wantType:    "TypeError",
			wantMessage: "'int' object is not iterable",
		},
		{
			name:        "non-mapping dictionary unpack",
			source:      "answer = {**1}\n",
			wantType:    "TypeError",
			wantMessage: "'int' object is not a mapping",
		},
		{
			name:        "delete missing dictionary key",
			source:      "mapping = {}\ndel mapping['missing']\n",
			wantType:    "KeyError",
			wantMessage: "'missing'",
		},
		{
			name:        "unhashable dictionary assignment",
			source:      "mapping = {}\nmapping[[1]] = 2\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'list' as a dict key (unhashable type: 'list')",
		},
		{
			name:        "unhashable dictionary deletion",
			source:      "mapping = {}\ndel mapping[[1]]\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'list' as a dict key (unhashable type: 'list')",
		},
		{
			name:        "missing dictionary key",
			source:      "answer = {'present': 1}['missing']\n",
			wantType:    "KeyError",
			wantMessage: "'missing'",
		},
		{
			name:        "missing tuple dictionary key",
			source:      "answer = {}[(1, 2)]\n",
			wantType:    "KeyError",
			wantMessage: "(1, 2)",
		},
		{
			name:        "unhashable dictionary subscription",
			source:      "answer = {}[[1]]\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'list' as a dict key (unhashable type: 'list')",
		},
		{
			name:        "unhashable dictionary key",
			source:      "answer = {[1]: 2}\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'list' as a dict key (unhashable type: 'list')",
		},
		{
			name:        "nested unhashable dictionary key",
			source:      "answer = {([1],): 2}\n",
			wantType:    "TypeError",
			wantMessage: "cannot use 'tuple' as a dict key (unhashable type: 'list')",
		},
		{
			name:        "non-iterable starred display",
			source:      "answer = [*1]\n",
			wantType:    "TypeError",
			wantMessage: "Value after * must be an iterable, not int",
		},
		{
			name:        "zero slice step",
			source:      "answer = [1, 2][::0]\n",
			wantType:    "ValueError",
			wantMessage: "slice step cannot be zero",
		},
		{
			name:        "non-integer slice bound",
			source:      "answer = (1, 2)[1.5:]\n",
			wantType:    "TypeError",
			wantMessage: "slice indices must be integers or None or have an __index__ method",
		},
		{
			name:        "non-subscriptable slice",
			source:      "answer = None[:]\n",
			wantType:    "TypeError",
			wantMessage: "'NoneType' object is not subscriptable",
		},
		{
			name:        "huge string index",
			source:      "answer = 'x'[1000000000000000000000000000000]\n",
			wantType:    "IndexError",
			wantMessage: "cannot fit 'int' into an index-sized integer",
		},
		{
			name:        "string index out of range",
			source:      "answer = 'x'[1]\n",
			wantType:    "IndexError",
			wantMessage: "string index out of range",
		},
		{
			name:        "bytes index out of range",
			source:      "answer = b'x'[-2]\n",
			wantType:    "IndexError",
			wantMessage: "index out of range",
		},
		{
			name:        "non-integer string index",
			source:      "answer = 'x'[1.5]\n",
			wantType:    "TypeError",
			wantMessage: "string indices must be integers, not 'float'",
		},
		{
			name:        "non-integer bytes index",
			source:      "answer = b'x'[1.5]\n",
			wantType:    "TypeError",
			wantMessage: "byte indices must be integers or slices, not float",
		},
		{
			name:        "tuple index out of range",
			source:      "answer = (1, 2)[2]\n",
			wantType:    "IndexError",
			wantMessage: "tuple index out of range",
		},
		{
			name:        "list index out of range",
			source:      "answer = [1, 2][-3]\n",
			wantType:    "IndexError",
			wantMessage: "list index out of range",
		},
		{
			name:        "huge sequence index",
			source:      "answer = [1][1000000000000000000000000000000]\n",
			wantType:    "IndexError",
			wantMessage: "cannot fit 'int' into an index-sized integer",
		},
		{
			name:        "non-integer sequence index",
			source:      "answer = [1][1.5]\n",
			wantType:    "TypeError",
			wantMessage: "list indices must be integers or slices, not float",
		},
		{
			name:        "non-subscriptable value",
			source:      "answer = None[0]\n",
			wantType:    "TypeError",
			wantMessage: "'NoneType' object is not subscriptable",
		},
		{
			name: "dictionary insertion during iteration",
			source: "mapping = {'first': 1}\n" +
				"for key in mapping:\n" +
				"    mapping['second'] = 2\n",
			wantType:    "RuntimeError",
			wantMessage: "dictionary changed size during iteration",
		},
		{
			name: "dictionary deletion during iteration",
			source: "mapping = {'first': 1, 'second': 2}\n" +
				"for key in mapping:\n" +
				"    del mapping['second']\n",
			wantType:    "RuntimeError",
			wantMessage: "dictionary changed size during iteration",
		},
		{
			name: "dictionary keys replaced during iteration",
			source: "mapping = {'first': 1, 'second': 2}\n" +
				"for key in mapping:\n" +
				"    del mapping['second']\n" +
				"    mapping['third'] = 3\n",
			wantType:    "RuntimeError",
			wantMessage: "dictionary keys changed during iteration",
		},
		{
			name:        "non-iterable for loop",
			source:      "for value in 1:\n    pass\n",
			wantType:    "TypeError",
			wantMessage: "'int' object is not iterable",
		},
		{
			name:        "not enough values for starred unpack",
			source:      "first, *middle, last = [1]\n",
			wantType:    "ValueError",
			wantMessage: "not enough values to unpack (expected at least 2, got 1)",
		},
		{
			name:        "non-iterable starred unpack",
			source:      "first, *middle = 1\n",
			wantType:    "TypeError",
			wantMessage: "cannot unpack non-iterable int object",
		},
		{
			name:        "not enough values to unpack",
			source:      "first, second = (1,)\n",
			wantType:    "ValueError",
			wantMessage: "not enough values to unpack (expected 2, got 1)",
		},
		{
			name:        "too many values to unpack",
			source:      "first, second = (1, 2, 3)\n",
			wantType:    "ValueError",
			wantMessage: "too many values to unpack (expected 2)",
		},
		{
			name:        "non-iterable unpack",
			source:      "first, second = 1\n",
			wantType:    "TypeError",
			wantMessage: "cannot unpack non-iterable int object",
		},
		{
			name:        "unsupported ordering",
			source:      "answer = 1 < 'text'\n",
			wantType:    "TypeError",
			wantMessage: "'<' not supported between instances of 'int' and 'str'",
		},
		{
			name:        "unsupported unary positive",
			source:      "answer = +'text'\n",
			wantType:    "TypeError",
			wantMessage: "bad operand type for unary +: 'str'",
		},
		{
			name:        "unsupported unary invert",
			source:      "answer = ~1.5\n",
			wantType:    "TypeError",
			wantMessage: "bad operand type for unary ~: 'float'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := compileSource(t, test.source)
			runtime := bullruntime.New()
			module, err := runtime.ExecuteModule(test.name, code)
			if module != nil {
				t.Fatalf("module = %#v, want nil after exception", module)
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
			if _, ok := runtime.Module(test.name); ok {
				t.Fatal("failed module remained in the runtime cache")
			}
		})
	}
}

func TestBytecodeValidation(t *testing.T) {
	tests := []struct {
		name         string
		code         *bytecode.Code
		wantFragment string
	}{
		{
			name: "function child index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "child code index 0 out of range",
		},
		{
			name: "fast local index",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadFast, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				Locals: []string{"value"},
			}),
			wantFragment: "local index 1 out of range",
		},
		{
			name: "global name index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadGlobal},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "name index 0 out of range",
		},
		{
			name: "call underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Call, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "invalid nested child",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.StoreName},
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.Integer("1")},
				Names:     []string{"visible"},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.LoadAttr},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						[]string{"attribute"},
					),
				},
			}),
			wantFragment: "unsupported opcode LOAD_ATTR",
		},
		{
			name: "function parameters exceed locals",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants:       []bytecode.Constant{bytecode.None()},
				Flags:           bytecode.Optimized | bytecode.NewLocals,
				PositionalCount: 1,
			}),
			wantFragment: "positional parameter count 1 exceeds local table length 0",
		},
		{
			name: "variadic positional local index",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants:       []bytecode.Constant{bytecode.None()},
				Flags:           bytecode.Optimized | bytecode.NewLocals | bytecode.VarArgs,
				PositionalCount: 1,
				Locals:          []string{"first"},
			}),
			wantFragment: "variadic positional parameter index 1 out of range",
		},
		{
			name: "unsupported unpacked call operand",
			code: testCode(
				0,
				[]bytecode.Instruction{
					{Opcode: bytecode.CallEx, Operand: 2},
				},
				nil,
				nil,
			),
			wantFragment: "unsupported CALL_EX operand 2",
		},
		{
			name: "invalid unpacked call payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CallEx, Operand: bytecode.CallExNoKeywords},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "CALL_EX positional arguments are not a tuple",
		},
		{
			name: "invalid keyword call payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 3,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.BuildTuple},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CallEx, Operand: bytecode.CallExWithKeywords},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "CALL_EX keyword arguments are not a dictionary",
		},
		{
			name: "map merge underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap},
					{Opcode: bytecode.MapMerge},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "variadic keyword local range",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Flags:     bytecode.Optimized | bytecode.NewLocals | bytecode.VarKeywords,
			}),
			wantFragment: "variadic keyword parameter index 0 out of range",
		},
		{
			name: "keyword-only local range",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants:        []bytecode.Constant{bytecode.None()},
				Flags:            bytecode.Optimized | bytecode.NewLocals,
				KeywordOnlyCount: 1,
			}),
			wantFragment: "keyword-only parameter range exceeds local table length",
		},
		{
			name: "invalid keyword defaults payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionKeywordDefaults),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "function keyword defaults payload is not a dictionary",
		},
		{
			name: "deref index",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadDeref, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				Flags:  bytecode.Optimized | bytecode.NewLocals,
				Locals: []string{"value"},
				Cells:  []string{"value"},
			}),
			wantFragment: "deref index 1 out of range",
		},
		{
			name: "invalid closure payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionClosure),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCodeSpec(bytecode.CodeSpec{
						StackSize: 1,
						Instructions: []bytecode.Instruction{
							{Opcode: bytecode.LoadDeref},
							{Opcode: bytecode.ReturnValue},
						},
						Flags:    bytecode.Optimized | bytecode.NewLocals | bytecode.Nested,
						FreeVars: []string{"captured"},
					}),
				},
			}),
			wantFragment: "function closure payload is not a tuple",
		},
		{
			name: "closure cell count",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.BuildTuple},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionClosure),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Children: []*bytecode.Code{
					testCodeSpec(bytecode.CodeSpec{
						StackSize: 1,
						Instructions: []bytecode.Instruction{
							{Opcode: bytecode.LoadDeref},
							{Opcode: bytecode.ReturnValue},
						},
						Flags:    bytecode.Optimized | bytecode.NewLocals | bytecode.Nested,
						FreeVars: []string{"captured"},
					}),
				},
			}),
			wantFragment: "function closure has 0 cells for 1 free variables",
		},
		{
			name: "invalid annotate payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionAnnotate),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "function annotate payload is not a function",
		},
		{
			name: "unsupported function attribute",
			code: testCode(
				0,
				[]bytecode.Instruction{
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: 32,
					},
				},
				nil,
				nil,
			),
			wantFragment: "unsupported SET_FUNCTION_ATTRIBUTE operand 32",
		},
		{
			name: "function defaults underflow",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionDefaults),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "operand stack underflow",
		},
		{
			name: "invalid function defaults payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionDefaults),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "function defaults payload is not a tuple",
		},
		{
			name: "set build underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildSet, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "set element count",
			code: testCode(
				0,
				[]bytecode.Instruction{{Opcode: bytecode.BuildSet, Operand: 1}},
				nil,
				nil,
			),
			wantFragment: "set element count 1 exceeds stack size",
		},
		{
			name: "set add underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildSet},
					{Opcode: bytecode.SetAdd},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "set update underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildSet},
					{Opcode: bytecode.SetUpdate},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map set underflow",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MapSet},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map update underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap},
					{Opcode: bytecode.MapUpdate},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "store subscript underflow",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.StoreSubscript},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "delete subscript underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.DeleteSubscript},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map build underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildMap, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map item count",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "map item count 1 exceeds stack size",
		},
		{
			name: "starred unpack underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.UnpackEx},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "starred unpack count",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.UnpackEx, Operand: 1 | 1<<8},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "unpack count 3 exceeds stack size",
		},
		{
			name: "list append underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildList},
					{Opcode: bytecode.ListAppend},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "list extend underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildList},
					{Opcode: bytecode.ListExtend},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "list to tuple underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.ListToTuple},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "invalid build slice operand",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildSlice, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "unsupported BUILD_SLICE operand 1",
		},
		{
			name: "build slice underflow",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildSlice, Operand: 3},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "binary subscript underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BinarySubscript},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "get iterator underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.GetIter},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "for iterator underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.ForIter, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "for iterator jump target",
			code: testCode(
				1,
				[]bytecode.Instruction{{Opcode: bytecode.ForIter, Operand: 1}},
				nil,
				nil,
			),
			wantFragment: "jump target 1 out of range",
		},
		{
			name: "sequence build underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildTuple, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "copy depth",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Copy},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "COPY depth must be at least 1",
		},
		{
			name: "swap depth",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Swap, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "SWAP depth must be at least 2",
		},
		{
			name: "unsupported comparison",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CompareOp, Operand: 99},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				nil,
			),
			wantFragment: "unsupported COMPARE_OP operand 99",
		},
		{
			name: "jump target",
			code: testCode(
				0,
				[]bytecode.Instruction{{Opcode: bytecode.Jump, Operand: 1}},
				nil,
				nil,
			),
			wantFragment: "jump target 1 out of range",
		},
		{
			name: "conditional stack underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.PopJumpIfFalse, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "stack depth merge",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.PopJumpIfFalse, Operand: 4},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Jump, Operand: 5},
					{Opcode: bytecode.Nop},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "stack depth mismatch at instruction 5",
		},
		{
			name: "no reachable return",
			code: testCode(
				0,
				[]bytecode.Instruction{{Opcode: bytecode.Jump}},
				nil,
				nil,
			),
			wantFragment: "code has no reachable RETURN_VALUE",
		},
		{
			name: "unsupported opcode",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.StoreName},
					{Opcode: bytecode.LoadAttr},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				[]string{"changed"},
			),
			wantFragment: "unsupported opcode LOAD_ATTR",
		},
		{
			name: "unsupported binary operation",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BinaryOp, Operand: bytecode.BinaryPower},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				nil,
			),
			wantFragment: "unsupported BINARY_OP operand 7",
		},
		{
			name: "unsupported unary operation",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.UnaryOp, Operand: 99},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				nil,
			),
			wantFragment: "unsupported UNARY_OP operand 99",
		},
		{
			name: "unsupported constant",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{{Kind: bytecode.ConstantKind(255)}},
				nil,
			),
			wantFragment: "unsupported constant Constant(kind=255)",
		},
		{
			name: "invalid integer constant",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("not-an-integer")},
				nil,
			),
			wantFragment: "invalid integer",
		},
		{
			name: "invalid string encoding",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("\xff")},
				nil,
			),
			wantFragment: "invalid string constant encoding",
		},
		{
			name: "constant index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "constant index 1 out of range",
		},
		{
			name: "name index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadName, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				[]string{"present"},
			),
			wantFragment: "name index 1 out of range",
		},
		{
			name: "stack underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.PopTop},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "declared stack too small",
			code: testCode(
				0,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack exceeds declared size 0",
		},
		{
			name:         "fallthrough",
			code:         testCode(0, []bytecode.Instruction{{Opcode: bytecode.Nop}}, nil, nil),
			wantFragment: "code falls through without RETURN_VALUE",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := bullruntime.New()
			module, err := runtime.ExecuteModule("broken", test.code)
			if module != nil {
				t.Fatalf("module = %#v, want nil", module)
			}
			var invalid *bullruntime.BytecodeError
			if !errors.As(err, &invalid) {
				t.Fatalf("error = %T %v, want *runtime.BytecodeError", err, err)
			}
			if !strings.Contains(invalid.Error(), test.wantFragment) {
				t.Fatalf("error = %q, want fragment %q", invalid, test.wantFragment)
			}
			if _, ok := runtime.Module("broken"); ok {
				t.Fatal("invalid module entered the runtime cache")
			}
		})
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

func testCode(
	stackSize int,
	instructions []bytecode.Instruction,
	constants []bytecode.Constant,
	names []string,
) *bytecode.Code {
	return testCodeSpec(bytecode.CodeSpec{
		StackSize:    stackSize,
		Instructions: instructions,
		Constants:    constants,
		Names:        names,
	})
}

func testCodeSpec(spec bytecode.CodeSpec) *bytecode.Code {
	positions := make([]lexer.Span, len(spec.Instructions))
	for index := range positions {
		positions[index] = lexer.Span{
			Start: lexer.Position{Line: 1, Column: index},
			End:   lexer.Position{Line: 1, Column: index + 1},
		}
	}
	if spec.Filename == "" {
		spec.Filename = "<broken>"
	}
	if spec.Name == "" {
		spec.Name = "<module>"
	}
	if spec.QualifiedName == "" {
		spec.QualifiedName = spec.Name
	}
	if spec.FirstLine == 0 {
		spec.FirstLine = 1
	}
	spec.Positions = positions
	return bytecode.NewCode(spec)
}
