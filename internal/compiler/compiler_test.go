package compiler

import (
	"errors"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

func TestInstructionPositions(t *testing.T) {
	module, err := parser.Parse("input.py", "value = None\nvalue\n")
	if err != nil {
		t.Fatal(err)
	}
	table, err := resolver.Resolve("input.py", module)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile("input.py", module, table)
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		opcode              bytecode.Opcode
		line, start, finish int
	}{
		{opcode: bytecode.LoadConst, line: 1, start: 8, finish: 12},
		{opcode: bytecode.StoreName, line: 1, start: 0, finish: 5},
		{opcode: bytecode.LoadName, line: 2, start: 0, finish: 5},
		{opcode: bytecode.PopTop, line: 2, start: 0, finish: 5},
	}
	instructions := code.Instructions()
	for index, expected := range want {
		if instructions[index].Opcode != expected.opcode {
			t.Errorf("instruction %d = %s, want %s", index, instructions[index].Opcode, expected.opcode)
		}
		span, ok := code.Position(index)
		if !ok {
			t.Fatalf("instruction %d has no position", index)
		}
		if span.Start.Line != expected.line || span.Start.Column != expected.start || span.End.Column != expected.finish {
			t.Errorf("instruction %d span = %+v", index, span)
		}
	}
	if code.Filename() != "input.py" || code.Name() != "<module>" || code.QualifiedName() != "<module>" {
		t.Fatalf("code metadata = %q, %q, %q", code.Filename(), code.Name(), code.QualifiedName())
	}
}

func TestMissingResolverTableError(t *testing.T) {
	module, err := parser.Parse("input.py", "pass\n")
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile("input.py", module, nil)
	if code != nil || err == nil {
		t.Fatalf("Compile() = (%#v, %v)", code, err)
	}
	var compileErr *Error
	if !errors.As(err, &compileErr) {
		t.Fatalf("error = %#v, want *compiler.Error", err)
	}
}

func TestGenericCompilerBoundaries(t *testing.T) {
	tests := []struct {
		name, source, message string
	}{
		{
			name:    "async function",
			source:  "async def generic[T]():\n    return T\n",
			message: "async functions are not compiled",
		},
		{
			name:    "class parameter metadata",
			source:  "class Generic[T = Default]:\n    pass\n",
			message: "generic class TypeVar defaults and variadic type parameters are not compiled",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module, err := parser.Parse("input.py", test.source)
			if err != nil {
				t.Fatal(err)
			}
			table, err := resolver.Resolve("input.py", module)
			if err != nil {
				t.Fatal(err)
			}
			code, err := Compile("input.py", module, table)
			if code != nil || err == nil {
				t.Fatalf("Compile() = (%#v, %v)", code, err)
			}
			var compileErr *Error
			if !errors.As(err, &compileErr) {
				t.Fatalf("error = %#v, want *compiler.Error", err)
			}
			if compileErr.Message != test.message {
				t.Fatalf("message = %q", compileErr.Message)
			}
		})
	}
}
