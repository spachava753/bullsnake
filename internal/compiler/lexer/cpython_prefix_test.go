package lexer

// Adapted from StringPrefixTest in CPython's Lib/test/test_tokenize.py.
// See LICENSES/CPython-3.14.txt.

import (
	"slices"
	"testing"
)

func TestStringPrefixCombinations(t *testing.T) {
	want := validCPython314Prefixes()
	var got []string
	for _, prefix := range distinctPrefixCandidates(3) {
		source := prefix + `""`
		tokens, err := collectTokens(source)
		if err != nil {
			continue
		}
		valid := false
		switch tokens[0].Kind {
		case String:
			valid = tokens[0].Text == source
		case FStringStart, TStringStart:
			valid = tokens[0].Text == prefix+`"`
		}
		if valid {
			got = append(got, prefix)
		}
	}

	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("valid prefixes = %q, want %q", got, want)
	}
}

func validCPython314Prefixes() []string {
	bases := []string{"", "b", "r", "u", "f", "t", "br", "rb", "fr", "rf", "tr", "rt"}
	seen := make(map[string]struct{})
	for _, base := range bases {
		caseVariants(base, 0, []byte(base), seen)
	}
	result := make([]string, 0, len(seen))
	for prefix := range seen {
		result = append(result, prefix)
	}
	slices.Sort(result)
	return result
}

func caseVariants(base string, index int, current []byte, result map[string]struct{}) {
	if index == len(base) {
		result[string(current)] = struct{}{}
		return
	}
	current[index] = base[index]
	caseVariants(base, index+1, current, result)
	current[index] = base[index] - 'a' + 'A'
	caseVariants(base, index+1, current, result)
}

func distinctPrefixCandidates(maxLength int) []string {
	letters := []byte{'b', 'r', 'u', 'f', 't'}
	var result []string
	var build func([]byte, map[byte]bool)
	build = func(prefix []byte, used map[byte]bool) {
		result = append(result, string(prefix))
		if len(prefix) == maxLength {
			return
		}
		for _, letter := range letters {
			if used[letter] {
				continue
			}
			used[letter] = true
			for _, candidate := range []byte{letter, letter - 'a' + 'A'} {
				build(append(prefix, candidate), used)
			}
			delete(used, letter)
		}
	}
	build(nil, make(map[byte]bool))
	return result
}
