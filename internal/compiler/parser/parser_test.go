package parser

import (
	"errors"
	"testing"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// TestSourceValidationPrecedesGrammar uses an invalid UTF-8 Go string that a
// JSON corpus cannot represent and verifies that Parse returns the lexer error.
func TestSourceValidationPrecedesGrammar(t *testing.T) {
	root, err := Parse("bad.py", "\xff")
	if root != nil {
		t.Fatalf("root = %#v, want nil", root)
	}
	var lexErr *lexer.Error
	if !errors.As(err, &lexErr) {
		t.Fatalf("error = %#v, want *lexer.Error", err)
	}
	if lexErr.Kind != lexer.EncodingError {
		t.Fatalf("error kind = %s, want EncodingError", lexErr.Kind)
	}
}

// TestParsePreservesSourceSpans covers successful-node locations, which the
// structural corpus intentionally omits to keep its AST snapshots readable.
func TestParsePreservesSourceSpans(t *testing.T) {
	source := "result = 1 + 2\n"
	root, err := Parse("input.py", source)
	if err != nil {
		t.Fatal(err)
	}
	if root.Source() != source {
		t.Fatalf("module source = %q, want %q", root.Source(), source)
	}
	want := `Module(body=[AssignStmt(targets=[Name(id="result", context=Store)@1:0-1:6], value=BinaryExpr(left=NumberLiteral(text="1")@1:9-1:10, op=Add, right=NumberLiteral(text="2")@1:13-1:14)@1:9-1:14)@1:0-1:14])@1:0-1:14`
	if got := compilerast.Dump(root, compilerast.DumpOptions{IncludeSpans: true}); got != want {
		t.Fatalf("AST with spans =\n%s\nwant:\n%s", got, want)
	}

	root, err = Parse("input.py", "if value:\n    pass\nelse:\n    pass\n")
	if err != nil {
		t.Fatal(err)
	}
	want = `Module(body=[IfStmt(condition=Name(id="value", context=Load)@1:3-1:8, body=[PassStmt()@2:4-2:8], else=[PassStmt()@4:4-4:8])@1:0-4:8])@1:0-4:8`
	if got := compilerast.Dump(root, compilerast.DumpOptions{IncludeSpans: true}); got != want {
		t.Fatalf("conditional AST with spans =\n%s\nwant:\n%s", got, want)
	}

	root, err = Parse("input.py", "not a and b or c")
	if err != nil {
		t.Fatal(err)
	}
	want = `Module(body=[ExprStmt(value=BooleanExpr(op=Or, values=[BooleanExpr(op=And, values=[UnaryExpr(op=Not, operand=Name(id="a", context=Load)@1:4-1:5)@1:0-1:5, Name(id="b", context=Load)@1:10-1:11])@1:0-1:11, Name(id="c", context=Load)@1:15-1:16])@1:0-1:16)@1:0-1:16])@1:0-1:16`
	if got := compilerast.Dump(root, compilerast.DumpOptions{IncludeSpans: true}); got != want {
		t.Fatalf("boolean AST with spans =\n%s\nwant:\n%s", got, want)
	}

	root, err = Parse("input.py", "object.items[1:3]")
	if err != nil {
		t.Fatal(err)
	}
	want = `Module(body=[ExprStmt(value=SubscriptExpr(value=AttributeExpr(value=Name(id="object", context=Load)@1:0-1:6, name="items", context=Load)@1:0-1:12, index=SliceExpr(lower=NumberLiteral(text="1")@1:13-1:14, upper=NumberLiteral(text="3")@1:15-1:16, step=nil)@1:13-1:16, context=Load)@1:0-1:17)@1:0-1:17])@1:0-1:17`
	if got := compilerast.Dump(root, compilerast.DumpOptions{IncludeSpans: true}); got != want {
		t.Fatalf("subscript AST with spans =\n%s\nwant:\n%s", got, want)
	}
}

func TestEmptyFormattedStringSpecIsPresent(t *testing.T) {
	for _, test := range []struct {
		source  string
		present bool
	}{
		{source: `f"{value}"`},
		{source: `f"{value:}"`, present: true},
	} {
		root, err := Parse("input.py", test.source)
		if err != nil {
			t.Fatal(err)
		}
		statement := root.Body[0].(*compilerast.ExprStmt)
		formatted := statement.Value.(*compilerast.FormattedStringExpr)
		value := formatted.Parts[0].(*compilerast.FormattedValueExpr)
		if got := value.Format != nil; got != test.present {
			t.Errorf("Parse(%q) format present = %t, want %t", test.source, got, test.present)
		}
	}
}

// TestErrorFormatting protects the complete human-readable diagnostic,
// including the one-based display column, which the corpus does not compare.
func TestErrorFormatting(t *testing.T) {
	failure := &Error{
		Kind:     SyntaxError,
		Message:  "expected expression",
		Filename: "input.py",
		Span: lexer.Span{
			Start: lexer.Position{Offset: 4, Line: 2, Column: 3},
			End:   lexer.Position{Offset: 4, Line: 2, Column: 3},
		},
	}
	if got, want := failure.Error(), "input.py:2:4: SyntaxError: expected expression"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if got := ErrorKind(255).String(); got != "ErrorKind(255)" {
		t.Fatalf("unknown error kind = %q", got)
	}
}

// FuzzParserNeverPanics explores inputs beyond the fixed corpus and requires
// every completed parse to return a usable AST or a located compiler error.
func FuzzParserNeverPanics(f *testing.F) {
	for _, source := range []string{
		"", "value", "1 + 2 * 3", "if value:\n    pass\n", "(]", `f"{value!r}"`, "\xff",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		root, err := Parse("fuzz.py", source)
		if err != nil {
			assertLocatedCompilerError(t, err, len(source))
			return
		}
		if root == nil {
			t.Fatal("Parse returned a nil root without an error")
		}
		_ = compilerast.Dump(root, compilerast.DumpOptions{IncludeSpans: true})
	})
}

// assertLocatedCompilerError limits parser failures to the two structured
// compiler error types and checks that their source spans are usable.
func assertLocatedCompilerError(t *testing.T, err error, sourceLength int) {
	t.Helper()
	var lexErr *lexer.Error
	if errors.As(err, &lexErr) {
		assertValidSpan(t, lexErr.Span, sourceLength)
		return
	}
	var parseErr *Error
	if errors.As(err, &parseErr) {
		assertValidSpan(t, parseErr.Span, sourceLength)
		return
	}
	t.Fatalf("error = %#v, want a located lexer or parser error", err)
}

// assertValidSpan rejects reversed or out-of-bounds byte offsets.
func assertValidSpan(t *testing.T, span lexer.Span, sourceLength int) {
	t.Helper()
	if span.Start.Offset < 0 || span.End.Offset < span.Start.Offset || span.End.Offset > sourceLength {
		t.Fatalf("invalid span %+v for %d-byte source", span, sourceLength)
	}
}
