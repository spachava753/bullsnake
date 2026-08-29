package bytecode

import (
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func TestCodeCopiesMutableInputAndOutput(t *testing.T) {
	span := lexer.Span{
		Start: lexer.Position{Line: 1, Column: 2},
		End:   lexer.Position{Line: 1, Column: 3},
	}
	instructions := []Instruction{{Opcode: LoadConst, Operand: 0}}
	names := []string{"value"}
	code := NewCode(CodeSpec{
		Name:          "f",
		QualifiedName: "f",
		FirstLine:     1,
		StackSize:     1,
		Instructions:  instructions,
		Positions:     []lexer.Span{span},
		Constants:     []Constant{None()},
		Names:         names,
	})

	instructions[0].Opcode = Nop
	names[0] = "changed"
	gotInstructions := code.Instructions()
	gotNames := code.Names()
	gotInstructions[0].Opcode = Nop
	gotNames[0] = "changed again"

	if got := code.Instructions()[0]; got != (Instruction{Opcode: LoadConst}) {
		t.Fatalf("instruction = %+v", got)
	}
	if got := code.Names()[0]; got != "value" {
		t.Fatalf("name = %q", got)
	}
	if got, ok := code.Position(0); !ok || got != span {
		t.Fatalf("position = (%+v, %t)", got, ok)
	}
	if _, ok := code.Position(1); ok {
		t.Fatal("out-of-range position unexpectedly exists")
	}
}

func TestOpcodeFormattingAndStackEffects(t *testing.T) {
	if got := (Instruction{Opcode: LoadName, Operand: 3}).String(); got != "LOAD_NAME 3" {
		t.Fatalf("instruction = %q", got)
	}
	if got := Opcode(255).String(); got != "Opcode(255)" {
		t.Fatalf("unknown opcode = %q", got)
	}
	depth := 0
	maximum := 0
	for _, instruction := range []Instruction{
		{Opcode: LoadConst},
		{Opcode: Copy, Operand: 1},
		{Opcode: StoreName},
		{Opcode: ReturnValue},
	} {
		depth += instruction.Opcode.StackEffect(instruction.Operand)
		if depth > maximum {
			maximum = depth
		}
	}
	if depth != 0 || maximum != 2 {
		t.Fatalf("stack depth = %d, maximum = %d", depth, maximum)
	}
	if got := (Instruction{Opcode: ConvertValue, Operand: ConversionRepr}).String(); got != "CONVERT_VALUE 2" {
		t.Fatalf("format conversion = %q", got)
	}
	if got := BuildString.StackEffect(3); got != -2 {
		t.Fatalf("BUILD_STRING 3 stack effect = %d, want -2", got)
	}
	if got := FormatWithSpec.StackEffect(0); got != -1 {
		t.Fatalf("FORMAT_WITH_SPEC stack effect = %d, want -1", got)
	}
	if got := Dump(nil); got != "nil" {
		t.Fatalf("Dump(nil) = %q", got)
	}
}
