package compiler

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

const compilerCasesPath = "testdata/compiler_cases.json"

type compilerCase struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	WantCode string `json:"want_code"`
}

type compilerCases struct {
	Cases []compilerCase `json:"cases"`
}

// TestCompilerCases compiles every checked-in source case through the complete
// parser, resolver, and compiler front end.
func TestCompilerCases(t *testing.T) {
	data, err := os.ReadFile(compilerCasesPath)
	if err != nil {
		t.Fatal(err)
	}
	var cases compilerCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, test := range cases.Cases {
		t.Run(test.Name, func(t *testing.T) {
			module, err := parser.Parse("<test>", test.Source)
			if err != nil {
				t.Fatal(err)
			}
			table, err := resolver.Resolve("<test>", module)
			if err != nil {
				t.Fatal(err)
			}
			code, err := Compile("<test>", module, table)
			if err != nil {
				t.Fatal(err)
			}
			if got := bytecode.Dump(code); got != test.WantCode {
				t.Fatalf("code =\n%s\nwant:\n%s", got, test.WantCode)
			}
		})
	}
}
