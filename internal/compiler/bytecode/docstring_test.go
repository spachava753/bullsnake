package bytecode

import "testing"

func TestOptionalDocstringMetadata(t *testing.T) {
	text := "original"
	code := NewCode(CodeSpec{Docstring: &text})
	text = "changed"
	if got, present := code.Docstring(); !present || got != "original" {
		t.Fatalf("docstring = %q, %v", got, present)
	}
	empty := ""
	if got, present := NewCode(CodeSpec{Docstring: &empty}).Docstring(); !present || got != "" {
		t.Fatalf("empty docstring = %q, %v", got, present)
	}
	if _, present := NewCode(CodeSpec{}).Docstring(); present {
		t.Fatal("absent docstring reported present")
	}
}
