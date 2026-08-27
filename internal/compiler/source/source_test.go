package source_test

// Some semantic cases are adapted from CPython's source-encoding tests.
// See LICENSES/CPython-3.14.txt.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/source"
)

func TestCodingCookies(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		wantEncoding string
		wantText     string
	}{
		{
			name:         "empty source",
			data:         nil,
			wantEncoding: "utf-8",
			wantText:     "",
		},
		{
			name:         "default UTF-8",
			data:         []byte("name = \"caf\xc3\xa9\"\n"),
			wantEncoding: "utf-8",
			wantText:     "name = \"caf\u00e9\"\n",
		},
		{
			name:         "first line ISO-8859-15",
			data:         []byte("# coding: iso8859-15\nprice = \"\xa4\"\n"),
			wantEncoding: "iso8859-15",
			wantText:     "# coding: iso8859-15\nprice = \"\u20ac\"\n",
		},
		{
			name:         "second line after shebang",
			data:         []byte("#!/usr/bin/python\n# coding=latin-1\nname = \"caf\xe9\"\n"),
			wantEncoding: "iso-8859-1",
			wantText:     "#!/usr/bin/python\n# coding=latin-1\nname = \"caf\u00e9\"\n",
		},
		{
			name:         "second line after empty line",
			data:         []byte("\n# coding: cp1252\nprice = \"\x80\"\n"),
			wantEncoding: "cp1252",
			wantText:     "\n# coding: cp1252\nprice = \"\u20ac\"\n",
		},
		{
			name:         "second line after malformed first cookie",
			data:         []byte("# coding:\n# coding: latin-1\nname = \"caf\xe9\""),
			wantEncoding: "iso-8859-1",
			wantText:     "# coding:\n# coding: latin-1\nname = \"caf\u00e9\"",
		},
		{
			name:         "case-sensitive cookie marker",
			data:         []byte("# Coding: latin-1\nname = \"caf\xc3\xa9\""),
			wantEncoding: "utf-8",
			wantText:     "# Coding: latin-1\nname = \"caf\u00e9\"",
		},
		{
			name:         "third line ignored",
			data:         []byte("#!/usr/bin/python\n# comment\n# coding: latin-1\nname = \"caf\xc3\xa9\"\n"),
			wantEncoding: "utf-8",
			wantText:     "#!/usr/bin/python\n# comment\n# coding: latin-1\nname = \"caf\u00e9\"\n",
		},
		{
			name:         "second line after code ignored",
			data:         []byte("value = 1\n# coding: latin-1\nname = \"caf\xc3\xa9\"\n"),
			wantEncoding: "utf-8",
			wantText:     "value = 1\n# coding: latin-1\nname = \"caf\u00e9\"\n",
		},
		{
			name:         "first cookie wins",
			data:         []byte("# coding: latin-1\n# coding: utf-8\nname = \"caf\xe9\"\n"),
			wantEncoding: "iso-8859-1",
			wantText:     "# coding: latin-1\n# coding: utf-8\nname = \"caf\u00e9\"\n",
		},
		{
			name:         "Shift-JIS Python alias",
			data:         []byte("# coding: shift_jis\nname = \"\x93\xfa\x96\x7b\"\n"),
			wantEncoding: "shift_jis",
			wantText:     "# coding: shift_jis\nname = \"\u65e5\u672c\"\n",
		},
		{
			name:         "physical endings preserved",
			data:         []byte("# coding: latin-1\rname = \"caf\xe9\"\r\n"),
			wantEncoding: "iso-8859-1",
			wantText:     "# coding: latin-1\rname = \"caf\u00e9\"\r\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unit, err := source.Decode("example.py", test.data)
			if err != nil {
				t.Fatal(err)
			}
			if unit.Filename != "example.py" || unit.Encoding != test.wantEncoding || unit.Text != test.wantText {
				t.Fatalf("unit = %#v, want filename %q, encoding %q, text %q", unit, "example.py", test.wantEncoding, test.wantText)
			}
		})
	}
}

func TestSupportedCodecNames(t *testing.T) {
	names := []string{
		"US-ASCII",
		"ISO-8859-15",
		"windows-1252",
		"Shift_JIS",
		"EUC-JP",
		"EUC-KR",
		"ISO-2022-JP",
		"GBK",
		"GB18030",
		"HZ-GB-2312",
		"Big5",
		"KOI8-R",
		"macintosh",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			text := "# coding: " + name + "\nvalue = 1\n"
			unit, err := source.Decode("codec.py", []byte(text))
			if err != nil {
				t.Fatal(err)
			}
			if unit.Encoding != name || unit.Text != text {
				t.Fatalf("unit = %#v, want encoding %q and unchanged ASCII text", unit, name)
			}
		})
	}
}

func TestBOMHandling(t *testing.T) {
	const bom = "\xef\xbb\xbf"
	tests := []struct {
		name     string
		data     string
		wantText string
	}{
		{name: "BOM only", data: bom, wantText: ""},
		{name: "implicit UTF-8", data: bom + "value\n", wantText: "value\n"},
		{name: "explicit UTF-8", data: bom + "# coding: utf_8\nvalue\n", wantText: "# coding: utf_8\nvalue\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unit, err := source.Decode("bom.py", []byte(test.data))
			if err != nil {
				t.Fatal(err)
			}
			if unit.Encoding != "utf-8" || unit.Text != test.wantText {
				t.Fatalf("unit = %#v, want UTF-8 text %q", unit, test.wantText)
			}
		})
	}

	conflicts := []struct {
		name     string
		data     string
		wantLine int
	}{
		{name: "first line", data: bom + "# coding: latin-1\n", wantLine: 1},
		{name: "second line", data: bom + "# comment\n# coding: cp1252\n", wantLine: 2},
		{name: "non-normalized UTF-8 alias", data: bom + "# coding: utf8\n", wantLine: 1},
	}
	for _, test := range conflicts {
		t.Run("conflict "+test.name, func(t *testing.T) {
			_, err := source.Decode("bom.py", []byte(test.data))
			assertSourceError(t, err, test.wantLine)
		})
	}
}

func TestEncodingErrors(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantLine int
	}{
		{name: "partial UTF-8 BOM", data: []byte("\xef\xbbvalue\n"), wantLine: 1},
		{name: "invalid default UTF-8", data: []byte("# comment\n\xff\n"), wantLine: 2},
		{name: "invalid declared ASCII", data: []byte("# coding: ascii\nvalue = '\x80'\n"), wantLine: 2},
		{name: "truncated UTF-8", data: []byte("value = '\xe2\x82"), wantLine: 1},
		{name: "unknown encoding", data: []byte("# coding: c1252\nvalue\n"), wantLine: 1},
		{name: "unsupported IANA encoding", data: []byte("# coding: ISO-2022-JP-2\nvalue\n"), wantLine: 1},
		{name: "non-ASCII-compatible encoding", data: []byte("# coding: utf-16\nvalue\n"), wantLine: 1},
		{name: "invalid Shift-JIS", data: []byte("# coding: shift_jis\nvalue = '\x85'\n"), wantLine: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := source.Decode("broken.py", test.data)
			assertSourceError(t, err, test.wantLine)
		})
	}
}

func TestReaderLoading(t *testing.T) {
	unit, err := source.Read("reader.py", strings.NewReader("value = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if unit.Filename != "reader.py" || unit.Text != "value = 1\n" {
		t.Fatalf("unit = %#v", unit)
	}

	wantErr := errors.New("reader failed")
	reader := io.MultiReader(strings.NewReader("value"), errorReader{err: wantErr})
	if _, err := source.Read("reader.py", reader); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped reader error", err)
	}
}

func TestFileLoading(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "latin.py")
	if err := os.WriteFile(filename, []byte("# coding: latin-1\nname = 'caf\xe9'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unit, err := source.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if unit.Filename != filename || unit.Text != "# coding: latin-1\nname = 'caf\u00e9'\n" {
		t.Fatalf("unit = %#v", unit)
	}

	missing := filepath.Join(t.TempDir(), "missing.py")
	if _, err := source.ReadFile(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want os.ErrNotExist", err)
	}
}

func TestLegacyEncodingLexerSpans(t *testing.T) {
	unit, err := source.Decode("latin.py", []byte("# coding: latin-1\ncaf\xe9 = 1\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	lex, err := lexer.NewFile(unit.Filename, unit.Text)
	if err != nil {
		t.Fatal(err)
	}

	var name lexer.Token
	for {
		token, nextErr := lex.Next()
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		if token.Kind == lexer.Name {
			name = token
			break
		}
	}
	wantSpan := lexer.Span{
		Start: lexer.Position{Offset: 18, Line: 2, Column: 0},
		End:   lexer.Position{Offset: 23, Line: 2, Column: 5},
	}
	if name.Text != "caf\u00e9" || name.Span != wantSpan {
		t.Fatalf("name = %+v, want text %q and span %+v", name, "caf\u00e9", wantSpan)
	}
}

func TestLoaderStripsBOMForLexer(t *testing.T) {
	unit, err := source.Decode("bom.py", []byte("\xef\xbb\xbfvalue\n"))
	if err != nil {
		t.Fatal(err)
	}
	lex, err := lexer.NewFile(unit.Filename, unit.Text)
	if err != nil {
		t.Fatal(err)
	}
	token, err := lex.Next()
	if err != nil {
		t.Fatal(err)
	}
	if token.Text != "value" || token.Span.Start != (lexer.Position{Offset: 0, Line: 1, Column: 0}) {
		t.Fatalf("first token = %+v", token)
	}
}

func TestNullByteLexerError(t *testing.T) {
	unit, err := source.Decode("null.py", []byte("x\x00y"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = lexer.NewFile(unit.Filename, unit.Text)
	var lexErr *lexer.Error
	if !errors.As(err, &lexErr) || lexErr.Kind != lexer.SyntaxError {
		t.Fatalf("error = %#v, want lexer.SyntaxError", err)
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

func assertSourceError(t *testing.T, err error, wantLine int) {
	t.Helper()
	var sourceErr *source.Error
	if !errors.As(err, &sourceErr) {
		t.Fatalf("error = %#v, want *source.Error", err)
	}
	if sourceErr.Filename != "broken.py" && sourceErr.Filename != "bom.py" {
		t.Fatalf("filename = %q, want source filename", sourceErr.Filename)
	}
	if sourceErr.Line != wantLine {
		t.Fatalf("line = %d, want %d: %v", sourceErr.Line, wantLine, sourceErr)
	}
}
