package compiler

import (
	"errors"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

func TestParseStringLiteral(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bytecode.Constant
	}{
		{name: "plain text", text: `'plain'`, want: bytecode.TextString("plain")},
		{
			name: "unicode escapes",
			text: `"line\n\t\x41\u2603\U0001F40D"`,
			want: bytecode.TextString("line\n\tA\u2603\U0001f40d"),
		},
		{name: "unicode name", text: `"\N{snowman}"`, want: bytecode.TextString("\u2603")},
		{name: "raw text", text: `r'\n\x41'`, want: bytecode.TextString(`\n\x41`)},
		{name: "triple newline normalization", text: "'''a\r\nb\rc'''", want: bytecode.TextString("a\nb\nc")},
		{name: "line continuation", text: "'left\\\r\nright'", want: bytecode.TextString("leftright")},
		{name: "unicode octal", text: `'\777'`, want: bytecode.TextString("\u01ff")},
		{name: "lone surrogate", text: `'\ud800'`, want: bytecode.TextString("\xed\xa0\x80")},
		{name: "bytes escapes", text: `b'\101\xFF\n'`, want: bytecode.Bytes("A\xff\n")},
		{name: "bytes octal truncation", text: `b'\777'`, want: bytecode.Bytes("\xff")},
		{name: "raw bytes", text: `br'\x41'`, want: bytecode.Bytes(`\x41`)},
		{name: "unknown escape", text: `'\q'`, want: bytecode.TextString(`\q`)},
		{name: "bytes unicode escape", text: `b'\u2603'`, want: bytecode.Bytes(`\u2603`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseStringLiteral(test.text)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("parseStringLiteral(%q) = %#v, want %#v", test.text, got, test.want)
			}
		})
	}
}

func TestRejectMalformedStringLiterals(t *testing.T) {
	tests := []struct {
		text    string
		message string
	}{
		{text: "b'caf\xc3\xa9'", message: "ASCII"},
		{text: `'\x0'`, message: "truncated \\x"},
		{text: `'\u12xz'`, message: "invalid \\u"},
		{text: `'\U00110000'`, message: "outside the Unicode range"},
		{text: `'\N'`, message: "malformed \\N"},
		{text: `'\N{NOT A UNICODE NAME}'`, message: "unknown Unicode character name"},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			_, err := parseStringLiteral(test.text)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("parseStringLiteral(%q) error = %v, want fragment %q", test.text, err, test.message)
			}
		})
	}
}

func TestCompileStringErrors(t *testing.T) {
	tests := []struct {
		name, source, message string
	}{
		{name: "non-ASCII bytes", source: "value = b'caf\xc3\xa9'\n", message: "ASCII"},
		{name: "mixed adjacent literals", source: "value = b'a' 'b'\n", message: "cannot mix bytes and nonbytes"},
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
				t.Fatalf("Compile() = (%#v, %v), want error", code, err)
			}
			var compileErr *Error
			if !errors.As(err, &compileErr) || compileErr.Filename != "input.py" || compileErr.Span.Start.Line != 1 {
				t.Fatalf("error = %#v, want located *compiler.Error", err)
			}
			if !strings.Contains(compileErr.Message, test.message) {
				t.Fatalf("error = %q, want fragment %q", compileErr.Message, test.message)
			}
		})
	}
}
