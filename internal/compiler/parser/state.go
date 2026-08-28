package parser

import "github.com/spachava753/bullsnake/internal/compiler/lexer"

type parserState struct {
	filename string
	cursor   tokenCursor
}

func (parser *parserState) peek(distance int) (lexer.Token, error) {
	return parser.cursor.peek(distance)
}

func (parser *parserState) advance() (lexer.Token, error) {
	return parser.cursor.next()
}

func (parser *parserState) take(kind lexer.Kind) (lexer.Token, bool, error) {
	token, err := parser.peek(0)
	if err != nil {
		return token, false, err
	}
	if token.Kind != kind {
		return token, false, nil
	}
	token, err = parser.cursor.next()
	return token, true, err
}

func (parser *parserState) takeKeyword(text string) (lexer.Token, bool, error) {
	token, err := parser.peek(0)
	if err != nil {
		return token, false, err
	}
	if token.Kind != lexer.Name || token.Text != text {
		return token, false, nil
	}
	token, err = parser.cursor.next()
	return token, true, err
}

func (parser *parserState) expectKeyword(text, message string) (lexer.Token, error) {
	token, matched, err := parser.takeKeyword(text)
	if err != nil {
		return token, err
	}
	if !matched {
		return token, parser.syntaxError(token, message)
	}
	return token, nil
}

func (parser *parserState) expect(kind lexer.Kind, message string) (lexer.Token, error) {
	token, matched, err := parser.take(kind)
	if err != nil {
		return token, err
	}
	if !matched {
		return token, parser.syntaxError(token, message)
	}
	return token, nil
}

func (parser *parserState) syntaxError(token lexer.Token, message string) error {
	return parser.errorAt(token.Span, message, token.Kind == lexer.EndMarker)
}

func (parser *parserState) errorAt(span lexer.Span, message string, incomplete bool) error {
	return &Error{
		Kind:       SyntaxError,
		Message:    message,
		Filename:   parser.filename,
		Span:       span,
		Incomplete: incomplete,
	}
}

func joinSpans(start, end lexer.Span) lexer.Span {
	return lexer.Span{Start: start.Start, End: end.End}
}
