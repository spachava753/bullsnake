package lexer

import "fmt"

// ErrorKind identifies the Python exception family a later compiler layer
// should expose for a lexical failure.
type ErrorKind uint8

const (
	SyntaxError ErrorKind = iota
	IndentationError
	TabError
	EncodingError
)

func (kind ErrorKind) String() string {
	switch kind {
	case SyntaxError:
		return "SyntaxError"
	case IndentationError:
		return "IndentationError"
	case TabError:
		return "TabError"
	case EncodingError:
		return "EncodingError"
	default:
		return fmt.Sprintf("ErrorKind(%d)", kind)
	}
}

// Error is a source-located lexer failure. Incomplete tells an interactive
// compiler that more input could make the token valid.
type Error struct {
	Kind       ErrorKind
	Message    string
	Filename   string
	Span       Span
	Incomplete bool
}

func (err *Error) Error() string {
	location := fmt.Sprintf("%d:%d", err.Span.Start.Line, err.Span.Start.Column+1)
	if err.Filename != "" {
		location = fmt.Sprintf("%s:%s", err.Filename, location)
	}
	return fmt.Sprintf("%s: %s: %s", location, err.Kind, err.Message)
}
