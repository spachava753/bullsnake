package parser

// The corpus records normalized cases derived from CPython 3.14.7.
// See LICENSES/CPython-3.14.txt.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

const corpusPath = "testdata/parser_cases.json"

type astCase struct {
	Name    string `json:"name"`
	Mode    string `json:"mode"`
	Source  string `json:"source"`
	WantAST string `json:"want_ast"`
}

type expectedPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type expectedSpan struct {
	Start expectedPosition `json:"start"`
	End   expectedPosition `json:"end"`
}

type errorCase struct {
	Name            string        `json:"name"`
	Mode            string        `json:"mode"`
	Source          string        `json:"source"`
	Phase           string        `json:"phase"`
	Kind            string        `json:"kind"`
	MessageContains string        `json:"message_contains"`
	Incomplete      bool          `json:"incomplete"`
	Span            *expectedSpan `json:"span,omitempty"`
}

type parserCorpus struct {
	Cases  []astCase   `json:"cases"`
	Errors []errorCase `json:"errors"`
}

// TestParserMatchesReference runs every valid and invalid corpus case against
// the parser. It is skipped as a whole only while Parse reports that the
// grammar is not implemented.
func TestParserMatchesReference(t *testing.T) {
	corpus := loadCorpusFile(t)
	if len(corpus.Cases) == 0 || len(corpus.Errors) == 0 {
		t.Fatal("parser corpus must contain successful and failing cases")
	}

	// Probe one case before creating subtests so an absent grammar produces one
	// intentional skip instead of a separate skip for every corpus entry.
	first := corpus.Cases[0]
	mode, err := parseMode(first.Mode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse("<test>", first.Source, mode); errors.Is(err, ErrNotImplemented) {
		t.Skip("parser grammar is not implemented")
	}

	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			mode, err := parseMode(test.Mode)
			if err != nil {
				t.Fatal(err)
			}
			root, err := Parse("<test>", test.Source, mode)
			if err != nil {
				t.Fatal(err)
			}
			if got := compilerast.Dump(root, compilerast.DumpOptions{}); got != test.WantAST {
				t.Fatalf("AST =\n%s\nwant:\n%s", got, test.WantAST)
			}
		})
	}

	for _, test := range corpus.Errors {
		t.Run(test.Name, func(t *testing.T) {
			mode, err := parseMode(test.Mode)
			if err != nil {
				t.Fatal(err)
			}
			root, err := Parse("<test>", test.Source, mode)
			if root != nil || err == nil {
				t.Fatalf("Parse() = (%#v, %v), want a failure", root, err)
			}
			phase, kind, message, incomplete, span := compilerErrorDetails(t, err)
			if phase != test.Phase || kind != test.Kind {
				t.Errorf("error = %s %s, want %s %s", phase, kind, test.Phase, test.Kind)
			}
			if !strings.Contains(message, test.MessageContains) {
				t.Errorf("message = %q, want substring %q", message, test.MessageContains)
			}
			if incomplete != test.Incomplete {
				t.Errorf("incomplete = %t, want %t", incomplete, test.Incomplete)
			}
			if test.Span != nil {
				got := expectedSpan{
					Start: expectedPosition{Line: span.Start.Line, Column: span.Start.Column},
					End:   expectedPosition{Line: span.End.Line, Column: span.End.Column},
				}
				if got != *test.Span {
					t.Errorf("span = %+v, want %+v", got, *test.Span)
				}
			}
		})
	}
}

// compilerErrorDetails normalizes lexer and parser errors into the fields used
// by error fixtures and fails the current test for any other error type.
func compilerErrorDetails(t *testing.T, err error) (string, string, string, bool, lexer.Span) {
	t.Helper()
	var lexErr *lexer.Error
	if errors.As(err, &lexErr) {
		return "lexer", lexErr.Kind.String(), lexErr.Message, lexErr.Incomplete, lexErr.Span
	}
	var parseErr *Error
	if errors.As(err, &parseErr) {
		return "parser", parseErr.Kind.String(), parseErr.Message, parseErr.Incomplete, parseErr.Span
	}
	t.Fatalf("error = %#v, want a lexer or parser error", err)
	return "", "", "", false, lexer.Span{}
}

// loadCorpusFile reads the checked-in parser fixture.
func loadCorpusFile(t *testing.T) parserCorpus {
	t.Helper()
	data, err := os.ReadFile(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	var corpus parserCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus
}

// parseMode converts the fixture's stable mode names to the parser API values.
func parseMode(value string) (Mode, error) {
	switch value {
	case "file":
		return FileMode, nil
	case "eval":
		return EvalMode, nil
	case "interactive":
		return InteractiveMode, nil
	default:
		return 0, fmt.Errorf("unknown parse mode %q", value)
	}
}
