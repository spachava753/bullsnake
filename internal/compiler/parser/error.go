package parser

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// ErrorKind identifies the Python exception family for a parser failure.
type ErrorKind uint8

const (
	SyntaxError ErrorKind = iota
)

// String returns the Python exception family name.
func (kind ErrorKind) String() string {
	if kind == SyntaxError {
		return "SyntaxError"
	}
	return fmt.Sprintf("ErrorKind(%d)", kind)
}

// Error is a source-located parser failure.
type Error struct {
	Kind       ErrorKind
	Message    string
	Filename   string
	Span       lexer.Span
	Incomplete bool
}

// Error returns a filename and one-based display column with the parser diagnostic.
func (failure *Error) Error() string {
	location := fmt.Sprintf("%d:%d", failure.Span.Start.Line, failure.Span.Start.Column+1)
	if failure.Filename != "" {
		location = fmt.Sprintf("%s:%s", failure.Filename, location)
	}
	return fmt.Sprintf("%s: %s: %s", location, failure.Kind, failure.Message)
}
