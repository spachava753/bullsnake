package parser

import (
	"errors"
	"testing"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// TestOutOfRangeParseModeRejected covers an API input the corpus cannot
// express because fixture mode names are validated before Parse is called.
func TestOutOfRangeParseModeRejected(t *testing.T) {
	root, err := Parse("input.py", "value\n", Mode(255))
	if root != nil {
		t.Fatalf("root = %#v, want nil", root)
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("error = %v, want ErrInvalidMode", err)
	}
}

// TestSourceValidationPrecedesGrammar uses an invalid UTF-8 Go string that a
// JSON corpus cannot represent and verifies that Parse returns the lexer error.
func TestSourceValidationPrecedesGrammar(t *testing.T) {
	root, err := Parse("bad.py", "\xff", FileMode)
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
		root, err := Parse("fuzz.py", source, FileMode)
		if errors.Is(err, ErrNotImplemented) {
			return
		}
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
