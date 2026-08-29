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
	if got := BuildMap.StackEffect(2); got != -3 {
		t.Fatalf("BUILD_MAP 2 stack effect = %d, want -3", got)
	}
	if got := MapSet.StackEffect(0); got != -2 {
		t.Fatalf("MAP_SET stack effect = %d, want -2", got)
	}
	binary := Instruction{Opcode: BinaryOp, Operand: BinaryPower}
	if got := binary.String(); got != "BINARY_OP 7" {
		t.Fatalf("binary instruction = %q", got)
	}
	if got := BinaryOp.StackEffect(BinaryPower); got != -1 {
		t.Fatalf("BINARY_OP stack effect = %d, want -1", got)
	}
	inplace := Instruction{Opcode: InplaceOp, Operand: BinaryOr}
	if got := inplace.String(); got != "INPLACE_OP 10" {
		t.Fatalf("in-place instruction = %q", got)
	}
	if got := InplaceOp.StackEffect(BinaryOr); got != -1 {
		t.Fatalf("INPLACE_OP stack effect = %d, want -1", got)
	}
	attribute := Instruction{Opcode: LoadAttr, Operand: 4}
	if got := attribute.String(); got != "LOAD_ATTR 4" {
		t.Fatalf("attribute instruction = %q", got)
	}
	if got := BinarySubscript.StackEffect(0); got != -1 {
		t.Fatalf("BINARY_SUBSCR stack effect = %d, want -1", got)
	}
	if got := BuildSlice.StackEffect(3); got != -2 {
		t.Fatalf("BUILD_SLICE 3 stack effect = %d, want -2", got)
	}
	call := Instruction{Opcode: Call, Operand: 3}
	if got := call.String(); got != "CALL 3" {
		t.Fatalf("call instruction = %q", got)
	}
	if got := Call.StackEffect(3); got != -3 {
		t.Fatalf("CALL 3 stack effect = %d, want -3", got)
	}
	if got := CallEx.StackEffect(CallExWithKeywords); got != -2 {
		t.Fatalf("CALL_EX with keywords stack effect = %d, want -2", got)
	}
	if got := MapMerge.StackEffect(0); got != -1 {
		t.Fatalf("MAP_MERGE stack effect = %d, want -1", got)
	}
	iterator := Instruction{Opcode: ForIter, Operand: 12}
	if got := iterator.String(); got != "FOR_ITER 12" {
		t.Fatalf("iterator instruction = %q", got)
	}
	if got := ForIter.StackEffect(12); got != 1 {
		t.Fatalf("FOR_ITER fallthrough stack effect = %d, want 1", got)
	}
	if got := GetIter.StackEffect(0); got != 0 {
		t.Fatalf("GET_ITER stack effect = %d, want 0", got)
	}
	store := Instruction{Opcode: StoreAttr, Operand: 5}
	if got := store.String(); got != "STORE_ATTR 5" {
		t.Fatalf("attribute store instruction = %q", got)
	}
	if got := StoreAttr.StackEffect(5); got != -2 {
		t.Fatalf("STORE_ATTR stack effect = %d, want -2", got)
	}
	if got := StoreSubscript.StackEffect(0); got != -3 {
		t.Fatalf("STORE_SUBSCR stack effect = %d, want -3", got)
	}
	deletion := Instruction{Opcode: DeleteAttr, Operand: 6}
	if got := deletion.String(); got != "DELETE_ATTR 6" {
		t.Fatalf("attribute delete instruction = %q", got)
	}
	if got := DeleteAttr.StackEffect(6); got != -1 {
		t.Fatalf("DELETE_ATTR stack effect = %d, want -1", got)
	}
	if got := DeleteSubscript.StackEffect(0); got != -2 {
		t.Fatalf("DELETE_SUBSCR stack effect = %d, want -2", got)
	}
	if got := DeleteName.StackEffect(6); got != 0 {
		t.Fatalf("DELETE_NAME stack effect = %d, want 0", got)
	}
	if got := UnpackSequence.StackEffect(3); got != 2 {
		t.Fatalf("UNPACK_SEQUENCE 3 stack effect = %d, want 2", got)
	}
	unpackOperand, ok := PackUnpackEx(1, 2)
	if !ok {
		t.Fatal("PackUnpackEx(1, 2) rejected valid counts")
	}
	unpack := Instruction{Opcode: UnpackEx, Operand: unpackOperand}
	if got := unpack.String(); got != "UNPACK_EX 1 2" {
		t.Fatalf("starred unpack instruction = %q", got)
	}
	if got := UnpackEx.StackEffect(unpackOperand); got != 3 {
		t.Fatalf("UNPACK_EX 1 2 stack effect = %d, want 3", got)
	}
	before, after := UnpackExCounts(unpackOperand)
	if before != 1 || after != 2 {
		t.Fatalf("UNPACK_EX counts = %d, %d", before, after)
	}
	if _, ok := PackUnpackEx(256, 0); ok {
		t.Fatal("PackUnpackEx accepted 256 leading targets")
	}
	if _, ok := PackUnpackEx(0, 1<<24); ok {
		t.Fatal("PackUnpackEx accepted overflowing trailing targets")
	}
	jump := Instruction{Opcode: JumpIfFalseOrPop, Operand: 9}
	if got := jump.String(); got != "JUMP_IF_FALSE_OR_POP 9" {
		t.Fatalf("jump instruction = %q", got)
	}
	if got := JumpIfFalseOrPop.StackEffect(9); got != -1 {
		t.Fatalf("conditional jump fallthrough effect = %d, want -1", got)
	}
	if got := Dump(nil); got != "nil" {
		t.Fatalf("Dump(nil) = %q", got)
	}
}
