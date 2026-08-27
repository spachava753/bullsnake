package lexer

// Some semantic cases are adapted from CPython's lexer tests.
// See LICENSES/CPython-3.14.txt.

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

type tokenPair struct {
	kind Kind
	text string
}

func TestBasicTokenStream(t *testing.T) {
	source := "if value >= 0x10:\n\t# retain this\n\tresult = value + 1\n"
	want := []tokenPair{
		{Name, "if"},
		{Name, "value"},
		{GreaterEqual, ">="},
		{Number, "0x10"},
		{Colon, ":"},
		{Newline, "\n"},
		{Comment, "# retain this"},
		{NL, "\n"},
		{Indent, "\t"},
		{Name, "result"},
		{Equal, "="},
		{Name, "value"},
		{Plus, "+"},
		{Number, "1"},
		{Newline, "\n"},
		{Dedent, ""},
		{EndMarker, ""},
	}
	assertTokenPairs(t, source, want)
}

func TestTokenSpansUseUTF8ByteOffsets(t *testing.T) {
	tokens, err := collectTokens("alpha = \u03c0\r\n")
	if err != nil {
		t.Fatal(err)
	}

	pi := tokens[2]
	if pi.Kind != Name || pi.Text != "\u03c0" {
		t.Fatalf("unexpected token: %v", pi)
	}
	want := Span{
		Start: Position{Offset: 8, Line: 1, Column: 8},
		End:   Position{Offset: 10, Line: 1, Column: 10},
	}
	if pi.Span != want {
		t.Fatalf("pi span = %+v, want %+v", pi.Span, want)
	}

	newline := tokens[3]
	if newline.Text != "\r\n" || newline.Span.End != (Position{Offset: 12, Line: 1, Column: 12}) {
		t.Fatalf("newline = %+v", newline)
	}
}

func TestImplicitLineJoining(t *testing.T) {
	source := "if x:\n    # comment\n\n    value = (\n        1 +\n        2\n    )\ndone\n"
	want := []tokenPair{
		{Name, "if"}, {Name, "x"}, {Colon, ":"}, {Newline, "\n"},
		{Comment, "# comment"}, {NL, "\n"}, {NL, "\n"},
		{Indent, "    "}, {Name, "value"}, {Equal, "="}, {LParen, "("}, {NL, "\n"},
		{Number, "1"}, {Plus, "+"}, {NL, "\n"},
		{Number, "2"}, {NL, "\n"},
		{RParen, ")"}, {Newline, "\n"},
		{Dedent, ""}, {Name, "done"}, {Newline, "\n"}, {EndMarker, ""},
	}
	assertTokenPairs(t, source, want)
}

func TestIndentationErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
		kind   ErrorKind
	}{
		{"ambiguous same level", "if x:\n\tpass\n        pass\n", TabError},
		{"ambiguous deeper level", "if x:\n    if y:\n\tpass\n", TabError},
		{"unknown dedent", "if x:\n    pass\n  pass\n", IndentationError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertErrorKind(t, test.source, test.kind)
		})
	}
}

func TestMultipleDedents(t *testing.T) {
	source := "if a:\n  if b:\n    pass\ndone"
	want := []tokenPair{
		{Name, "if"}, {Name, "a"}, {Colon, ":"}, {Newline, "\n"},
		{Indent, "  "}, {Name, "if"}, {Name, "b"}, {Colon, ":"}, {Newline, "\n"},
		{Indent, "    "}, {Name, "pass"}, {Newline, "\n"},
		{Dedent, ""}, {Dedent, ""}, {Name, "done"}, {Newline, ""}, {EndMarker, ""},
	}
	assertTokenPairs(t, source, want)
}

func TestExplicitLineContinuations(t *testing.T) {
	source := "total = 1 + \\\n    2\nif total:\n    \\\npass\n"
	want := []tokenPair{
		{Name, "total"}, {Equal, "="}, {Number, "1"}, {Plus, "+"}, {Number, "2"}, {Newline, "\n"},
		{Name, "if"}, {Name, "total"}, {Colon, ":"}, {Newline, "\n"},
		{Indent, ""}, {Name, "pass"}, {Newline, "\n"}, {Dedent, ""}, {EndMarker, ""},
	}
	assertTokenPairs(t, source, want)
}

func TestNumbers(t *testing.T) {
	source := "0 00 0_0 123 1_000 0xff 0x_FF 0o7 0b_10 3.14 10. .5 1e10 1.e-2 3j 01j 1else"
	wantText := []string{
		"0", "00", "0_0", "123", "1_000", "0xff", "0x_FF", "0o7", "0b_10",
		"3.14", "10.", ".5", "1e10", "1.e-2", "3j", "01j", "1", "else",
	}
	tokens, err := collectTokens(source)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, token := range tokens {
		if token.Kind == Number || token.Kind == Name {
			got = append(got, token.Text)
		}
	}
	if !slices.Equal(got, wantText) {
		t.Fatalf("number/name texts = %q, want %q", got, wantText)
	}
}

func TestNumericLiteralErrors(t *testing.T) {
	for _, source := range []string{
		"012", "1_", "1.2_", "1e2_", "1e+", "0b12", "0b1_2", "0b", "0o8", "0x1_", "0x", "2sin(x)", "0_x",
	} {
		t.Run(source, func(t *testing.T) {
			assertErrorKind(t, source, SyntaxError)
		})
	}
}

func TestOrdinaryStringsAndPrefixes(t *testing.T) {
	source := "'' r\"x\" R'''x\n''' b\"b\" br\"x\" rb\"x\" u\"x\" rr\"x\""
	want := []tokenPair{
		{String, "''"},
		{String, "r\"x\""},
		{String, "R'''x\n'''"},
		{String, "b\"b\""},
		{String, "br\"x\""},
		{String, "rb\"x\""},
		{String, "u\"x\""},
		{Name, "rr"},
		{String, "\"x\""},
		{Newline, ""},
		{EndMarker, ""},
	}
	assertTokenPairs(t, source, want)
}

func TestIncompatibleStringPrefixes(t *testing.T) {
	for _, source := range []string{`bf"x"`, `ur"x"`, `ut"x"`, `ft"x"`} {
		t.Run(source, func(t *testing.T) {
			assertErrorKind(t, source, SyntaxError)
		})
	}
}

func TestFStringTokens(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []tokenPair
	}{
		{
			name:   "replacement field",
			source: `f"hello {name!r:>{width}}!"`,
			want: []tokenPair{
				{FStringStart, `f"`},
				{FStringMiddle, "hello "},
				{LBrace, "{"},
				{Name, "name"},
				{Exclamation, "!"},
				{Name, "r"},
				{Colon, ":"},
				{FStringMiddle, ">"},
				{LBrace, "{"},
				{Name, "width"},
				{RBrace, "}"},
				{FStringMiddle, ""},
				{RBrace, "}"},
				{FStringMiddle, "!"},
				{FStringEnd, `"`},
				{Newline, ""},
				{EndMarker, ""},
			},
		},
		{
			name:   "escapes",
			source: `f"\N{LEFT CURLY BRACKET}x" f"a\{x}" rf"\N{X}{x}"`,
			want: []tokenPair{
				{FStringStart, `f"`},
				{FStringMiddle, `\N{LEFT CURLY BRACKET}`},
				{FStringMiddle, "x"},
				{FStringEnd, `"`},
				{FStringStart, `f"`},
				{FStringMiddle, `a\`},
				{LBrace, "{"},
				{Name, "x"},
				{RBrace, "}"},
				{FStringEnd, `"`},
				{FStringStart, `rf"`},
				{FStringMiddle, `\N`},
				{LBrace, "{"},
				{Name, "X"},
				{RBrace, "}"},
				{LBrace, "{"},
				{Name, "x"},
				{RBrace, "}"},
				{FStringEnd, `"`},
				{Newline, ""},
				{EndMarker, ""},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertTokenPairs(t, test.source, test.want)
		})
	}
}

func TestTemplateAndNestedFStrings(t *testing.T) {
	source := `t"{{value}} {f'{x}'}"`
	want := []tokenPair{
		{TStringStart, `t"`},
		{TStringMiddle, "{"},
		{TStringMiddle, "value}"},
		{TStringMiddle, " "},
		{LBrace, "{"},
		{FStringStart, `f'`},
		{LBrace, "{"},
		{Name, "x"},
		{RBrace, "}"},
		{FStringEnd, `'`},
		{RBrace, "}"},
		{TStringEnd, `"`},
		{Newline, ""},
		{EndMarker, ""},
	}
	assertTokenPairs(t, source, want)
}

func TestUnicodeIdentifiers(t *testing.T) {
	source := "\u03c0 = a\u00b7b + \u2118\n"
	tokens, err := collectTokens(source)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, token := range tokens {
		if token.Kind == Name {
			names = append(names, token.Text)
		}
	}
	if want := []string{"\u03c0", "a\u00b7b", "\u2118"}; !slices.Equal(names, want) {
		t.Fatalf("names = %q, want %q", names, want)
	}

	assertErrorKind(t, "\u037a = 1", SyntaxError)
	assertErrorKind(t, "\u088f = 1", SyntaxError) // Assigned as XID_Start only after Unicode 16.
	assertErrorKind(t, "\U0001f40d = 1", SyntaxError)
}

func TestDelimiterErrors(t *testing.T) {
	for _, source := range []string{"(1+2]", "(1+2}", "{1+2]", "]", `f"}"`} {
		t.Run(source, func(t *testing.T) {
			assertErrorKind(t, source, SyntaxError)
		})
	}

	_, err := collectTokens("value = (1 + 2")
	var lexErr *Error
	if !errors.As(err, &lexErr) || !lexErr.Incomplete {
		t.Fatalf("error = %#v, want incomplete lexical error", err)
	}
}

func TestSourceValidation(t *testing.T) {
	t.Run("UTF-8 BOM", func(t *testing.T) {
		tokens, err := collectTokens("\xef\xbb\xbfvalue\n")
		if err != nil {
			t.Fatal(err)
		}
		if tokens[0].Text != "value" || tokens[0].Span.Start != (Position{Offset: 3, Line: 1, Column: 0}) {
			t.Fatalf("first token = %+v", tokens[0])
		}
	})

	tests := []struct {
		name   string
		source string
		kind   ErrorKind
	}{
		{name: "null byte", source: "x\x00y", kind: SyntaxError},
		{name: "malformed UTF-8", source: "\xff", kind: EncodingError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertErrorKind(t, test.source, test.kind)
		})
	}
}

func TestMaximumFStringNesting(t *testing.T) {
	if _, err := collectTokens(nestedFString(maxStringMode - 1)); err != nil {
		t.Fatalf("valid maximum f-string nesting: %v", err)
	}
	assertErrorKind(t, nestedFString(maxStringMode), SyntaxError)
}

func nestedFString(depth int) string {
	source := "value"
	for range depth {
		source = `f"{` + source + `}"`
	}
	return source
}

func TestUnterminatedStringsReportCompleteness(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		incomplete bool
	}{
		{name: "single quoted", source: `"value`},
		{name: "triple quoted", source: `'''value`, incomplete: true},
		{name: "f-string", source: `f"value`},
		{name: "triple quoted f-string", source: `f'''value`, incomplete: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := collectTokens(test.source)
			var lexErr *Error
			if !errors.As(err, &lexErr) {
				t.Fatalf("error = %#v, want *lexer.Error", err)
			}
			if lexErr.Incomplete != test.incomplete {
				t.Fatalf("incomplete = %t, want %t", lexErr.Incomplete, test.incomplete)
			}
		})
	}
}

func FuzzLexerNeverPanics(f *testing.F) {
	for _, source := range []string{
		"", "x = 1", "if x:\n  pass\n", `f"{value!r:>{width}}"`, "(]", "\xff", "'''",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		lexer, err := New(source)
		if err != nil {
			var lexErr *Error
			if !errors.As(err, &lexErr) {
				t.Fatalf("constructor error = %#v, want *lexer.Error", err)
			}
			if lexErr.Span.Start.Offset < 0 || lexErr.Span.End.Offset < lexErr.Span.Start.Offset || lexErr.Span.End.Offset > len(source) {
				t.Fatalf("invalid error span: %+v for %d-byte source", lexErr.Span, len(source))
			}
			return
		}
		limit := utf8.RuneCountInString(source)*4 + maxIndent + maxStringMode + 16
		for count := 0; count < limit; count++ {
			token, err := lexer.Next()
			if token.Span.Start.Offset < 0 || token.Span.End.Offset < token.Span.Start.Offset || token.Span.End.Offset > len(source) {
				t.Fatalf("invalid token span: %+v for %d-byte source", token, len(source))
			}
			if err != nil || token.Kind == EndMarker {
				return
			}
		}
		t.Fatalf("lexer did not terminate after %d tokens", limit)
	})
}

func assertTokenPairs(t *testing.T, source string, want []tokenPair) {
	t.Helper()
	tokens, err := collectTokens(source)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]tokenPair, len(tokens))
	for index, token := range tokens {
		got[index] = tokenPair{kind: token.Kind, text: token.Text}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tokens:\n got: %s\nwant: %s", formatPairs(got), formatPairs(want))
	}
}

func formatPairs(pairs []tokenPair) string {
	parts := make([]string, len(pairs))
	for index, pair := range pairs {
		parts[index] = fmt.Sprintf("%s(%q)", pair.kind, pair.text)
	}
	return strings.Join(parts, " ")
}
