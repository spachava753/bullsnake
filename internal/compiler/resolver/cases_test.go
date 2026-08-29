package resolver

// The cases record normalized behavior derived from CPython 3.14.7.
// See LICENSES/CPython-3.14.txt.

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/parser"
)

const resolverCasesPath = "testdata/resolver_cases.json"

type tableCase struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	WantTable string `json:"want_table"`
}

type expectedPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type expectedSpan struct {
	Start expectedPosition `json:"start"`
	End   expectedPosition `json:"end"`
}

type resolverErrorCase struct {
	Name            string        `json:"name"`
	Source          string        `json:"source"`
	Kind            string        `json:"kind"`
	MessageContains string        `json:"message_contains"`
	Span            *expectedSpan `json:"span,omitempty"`
}

type resolverCases struct {
	Cases  []tableCase         `json:"cases"`
	Errors []resolverErrorCase `json:"errors"`
}

// TestResolverCasesMatchReference runs every successful and failing resolver case.
func TestResolverCasesMatchReference(t *testing.T) {
	cases := loadResolverCases(t)
	for _, test := range cases.Cases {
		t.Run(test.Name, func(t *testing.T) {
			module, err := parser.Parse("<test>", test.Source)
			if err != nil {
				t.Fatalf("parser rejected resolver success case: %v", err)
			}
			table, err := Resolve("<test>", module)
			if err != nil {
				t.Fatal(err)
			}
			if got := Dump(table); got != test.WantTable {
				t.Fatalf("table =\n%s\nwant:\n%s", got, test.WantTable)
			}
		})
	}

	for _, test := range cases.Errors {
		t.Run(test.Name, func(t *testing.T) {
			module, err := parser.Parse("<test>", test.Source)
			if err != nil {
				t.Fatalf("parser owns resolver error case: %v", err)
			}
			table, err := Resolve("<test>", module)
			if table != nil || err == nil {
				t.Fatalf("Resolve() = (%#v, %v), want a failure", table, err)
			}
			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("error = %#v, want *resolver.Error", err)
			}
			if got := failure.Kind.String(); got != test.Kind {
				t.Errorf("kind = %s, want %s", got, test.Kind)
			}
			if !strings.Contains(failure.Message, test.MessageContains) {
				t.Errorf("message = %q, want substring %q", failure.Message, test.MessageContains)
			}
			if test.Span != nil {
				got := expectedSpan{
					Start: expectedPosition{Line: failure.Span.Start.Line, Column: failure.Span.Start.Column},
					End:   expectedPosition{Line: failure.Span.End.Line, Column: failure.Span.End.Column},
				}
				if got != *test.Span {
					t.Errorf("span = %+v, want %+v", got, *test.Span)
				}
			}
		})
	}
}

func loadResolverCases(t *testing.T) resolverCases {
	t.Helper()
	data, err := os.ReadFile(resolverCasesPath)
	if err != nil {
		t.Fatal(err)
	}
	var cases resolverCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}
