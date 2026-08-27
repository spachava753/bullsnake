package lexer

// Adapted from CPython's Lib/test/test_unicode_identifiers.py.
// See LICENSES/CPython-3.14.txt.

import (
	"slices"
	"testing"
)

func TestPEP3131Identifiers(t *testing.T) {
	source := "\u00e4 \u00b5 \u87d2 x\U000e0100 \U0001d518\U0001d52b\U0001d526\U0001d520\U0001d52c\U0001d521\U0001d522"
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
	want := []string{
		"\u00e4",
		"\u00b5",
		"\u87d2",
		"x\U000e0100",
		"\U0001d518\U0001d52b\U0001d526\U0001d520\U0001d52c\U0001d521\U0001d522",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("names = %q, want %q", names, want)
	}
}
