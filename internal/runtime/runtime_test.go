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

func TestIntegerBinaryOperators(t *testing.T) {
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

func TestPythonExceptions(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantType    string
		wantMessage string
	}{
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
	positions := make([]lexer.Span, len(instructions))
	for index := range positions {
		positions[index] = lexer.Span{
			Start: lexer.Position{Line: 1, Column: index},
			End:   lexer.Position{Line: 1, Column: index + 1},
		}
	}
	return bytecode.NewCode(bytecode.CodeSpec{
		Filename:      "<broken>",
		Name:          "<module>",
		QualifiedName: "<module>",
		FirstLine:     1,
		StackSize:     stackSize,
		Instructions:  instructions,
		Positions:     positions,
		Constants:     constants,
		Names:         names,
	})
}
