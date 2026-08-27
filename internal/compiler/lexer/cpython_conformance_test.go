package lexer

// The table data in cpython_cases_test.go is adapted from CPython's
// Lib/test/test_tokenize.py. See LICENSES/CPython-3.14.txt.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	cpythonValidCaseCount     = 83
	cpythonMalformedCaseCount = 32
	cpythonExpectedTokenCount = 787
)

func TestConformance(t *testing.T) {
	if len(tokenizeCases) != cpythonValidCaseCount || len(cpythonMalformedCases) != cpythonMalformedCaseCount {
		t.Fatalf(
			"table has %d valid and %d malformed cases, want %d and %d",
			len(tokenizeCases),
			len(cpythonMalformedCases),
			cpythonValidCaseCount,
			cpythonMalformedCaseCount,
		)
	}

	expectedTokens := 0
	for _, test := range tokenizeCases {
		expectedTokens += len(test.tokens)
		t.Run(test.name, func(t *testing.T) {
			assertTokenStream(t, test)
		})
	}
	if expectedTokens != cpythonExpectedTokenCount {
		t.Fatalf("table has %d expected tokens, want %d", expectedTokens, cpythonExpectedTokenCount)
	}

	for _, test := range cpythonMalformedCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := collectTokens(test.source)
			if _, ok := errors.AsType[*Error](err); !ok {
				t.Fatalf("error = %#v, want lexical error", err)
			}
		})
	}
}

func TestLongCommentLines(t *testing.T) {
	source := fmt.Sprintf(
		"#coding: latin-1\n#%s\n#%s",
		strings.Repeat("a", 10000),
		strings.Repeat("a", 10002),
	)
	tokens, err := collectTokens(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 7 {
		t.Fatalf("token count = %d, want 7", len(tokens))
	}
	if tokens[4].Kind != Comment || len(tokens[4].Text) != 10003 {
		t.Fatalf("last comment = %s with %d bytes", tokens[4].Kind, len(tokens[4].Text))
	}
	if tokens[5].Kind != NL || tokens[6].Kind != EndMarker {
		t.Fatalf("final tokens = %s, %s", tokens[5].Kind, tokens[6].Kind)
	}
}

func TestIndentStackLimit(t *testing.T) {
	valid := nestedIndentationSource(maxIndent - 1)
	if _, err := collectTokens(valid); err != nil {
		t.Fatalf("valid maximum indentation: %v", err)
	}
	assertErrorKind(t, nestedIndentationSource(maxIndent), IndentationError)
}

func nestedIndentationSource(indents int) string {
	var source strings.Builder
	for level := range indents {
		fmt.Fprintf(&source, "%sif True:\n", strings.Repeat("  ", level))
	}
	fmt.Fprintf(&source, "%spass\n", strings.Repeat("  ", indents))
	return source.String()
}

func assertTokenStream(t *testing.T, test cpythonTokenizeCase) {
	t.Helper()
	tokens, err := collectTokens(test.source)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != len(test.tokens) {
		t.Fatalf("token count = %d, want %d\n%s", len(tokens), len(test.tokens), formatTokens(tokens))
	}
	for index, token := range tokens {
		want := test.tokens[index]
		if token.Kind != want.kind || token.Text != want.text {
			t.Fatalf(
				"token %d = %s %q, want %s %q",
				index,
				token.Kind,
				token.Text,
				want.kind,
				want.text,
			)
		}
		wantSpan := Span{Start: pos(want.start), End: pos(want.end)}
		if token.Span != wantSpan {
			t.Fatalf("token %d %s span = %+v, want %+v", index, token.Kind, token.Span, wantSpan)
		}
	}
}

func pos(value [3]int) Position {
	return Position{Offset: value[0], Line: value[1], Column: value[2]}
}

func formatTokens(tokens []Token) string {
	var result strings.Builder
	for index, token := range tokens {
		if index != 0 {
			result.WriteByte(' ')
		}
		fmt.Fprintf(&result, "%s(%q)", token.Kind, token.Text)
	}
	return result.String()
}
