// Package parser parses Bullsnake lexer tokens into the internal AST.
package parser

import (
	"errors"
	"fmt"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// ErrNotImplemented marks the temporary boundary before grammar rules are added.
var ErrNotImplemented = errors.New("parser grammar is not implemented")

// ErrInvalidMode reports a parse mode outside the defined set.
var ErrInvalidMode = errors.New("invalid parser mode")

// Mode selects the grammar entry point.
type Mode uint8

const (
	FileMode Mode = iota
	EvalMode
	InteractiveMode

	modeCount
)

// Parse parses one decoded UTF-8 source input according to mode.
func Parse(filename, source string, mode Mode) (compilerast.Root, error) {
	if mode >= modeCount {
		return nil, fmt.Errorf("%w: %d", ErrInvalidMode, mode)
	}
	tokenizer, err := lexer.NewFile(filename, source)
	if err != nil {
		return nil, err
	}
	_ = tokenizer
	return nil, ErrNotImplemented
}
