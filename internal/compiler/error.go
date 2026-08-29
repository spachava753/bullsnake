package compiler

import (
	"fmt"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// Error is a source-located bytecode compilation failure.
type Error struct {
	Message  string
	Filename string
	Span     lexer.Span
}

// Error returns a filename and one-based display column with the diagnostic.
func (failure *Error) Error() string {
	location := fmt.Sprintf("%d:%d", failure.Span.Start.Line, failure.Span.Start.Column+1)
	if failure.Filename != "" {
		location = fmt.Sprintf("%s:%s", failure.Filename, location)
	}
	return fmt.Sprintf("%s: compiler error: %s", location, failure.Message)
}

func (compiler *compilerState) error(span lexer.Span, format string, arguments ...any) error {
	return &Error{
		Message:  fmt.Sprintf(format, arguments...),
		Filename: compiler.filename,
		Span:     span,
	}
}

func (compiler *compilerState) unsupported(node compilerast.Node) error {
	return compiler.error(node.Span(), "unsupported syntax node %T", node)
}
