// Package parser parses Bullsnake lexer tokens into the internal AST.
package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// Parse parses one decoded UTF-8 source input as a module.
func Parse(filename, source string) (*compilerast.Module, error) {
	tokenizer, err := lexer.NewFile(filename, source)
	if err != nil {
		return nil, err
	}
	state := parserState{
		filename: filename,
		cursor:   tokenCursor{source: tokenizer},
	}
	return state.parseModule()
}
