package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseTryStatement parses handlers, an optional normal-completion suite, and
// an optional final suite while enforcing Python's clause ordering.
func (parser *parserState) parseTryStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("try", "expected 'try'")
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after try"); err != nil {
		return nil, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, err
	}

	var handlers []compilerast.ExceptHandler
	starredHandlers := false
	bareHandler := false
	for parser.nextKeywordIs("except") {
		handler, err := parser.parseExceptHandler()
		if err != nil {
			return nil, err
		}
		if len(handlers) != 0 && handler.Star != starredHandlers {
			return nil, parser.errorAt(handler.Range, "cannot mix except and except* handlers", false)
		}
		if bareHandler {
			return nil, parser.errorAt(handler.Range, "bare except must be last", false)
		}
		starredHandlers = handler.Star
		bareHandler = handler.Type == nil
		handlers = append(handlers, handler)
	}

	var alternative []compilerast.Stmt
	if parser.nextKeywordIs("else") {
		if len(handlers) == 0 {
			token, _ := parser.peek(0)
			return nil, parser.syntaxError(token, "try else requires an except handler")
		}
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		if _, err := parser.expect(lexer.Colon, "expected ':' after else"); err != nil {
			return nil, err
		}
		alternative, err = parser.parseSuite()
		if err != nil {
			return nil, err
		}
	}

	var final []compilerast.Stmt
	if parser.nextKeywordIs("finally") {
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		if _, err := parser.expect(lexer.Colon, "expected ':' after finally"); err != nil {
			return nil, err
		}
		final, err = parser.parseSuite()
		if err != nil {
			return nil, err
		}
	}
	if len(handlers) == 0 && len(final) == 0 {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		return nil, parser.syntaxError(token, "try requires except or finally")
	}

	end := body[len(body)-1].Span()
	if len(handlers) != 0 {
		end = handlers[len(handlers)-1].Range
	}
	if len(alternative) != 0 {
		end = alternative[len(alternative)-1].Span()
	}
	if len(final) != 0 {
		end = final[len(final)-1].Span()
	}
	return &compilerast.TryStmt{
		Range:    joinSpans(keyword.Span, end),
		Body:     body,
		Handlers: handlers,
		Else:     alternative,
		Finally:  final,
	}, nil
}

// parseExceptHandler parses an except or except* type, optional name, and body.
func (parser *parserState) parseExceptHandler() (compilerast.ExceptHandler, error) {
	keyword, err := parser.expectKeyword("except", "expected 'except'")
	if err != nil {
		return compilerast.ExceptHandler{}, err
	}
	_, starred, err := parser.take(lexer.Star)
	if err != nil {
		return compilerast.ExceptHandler{}, err
	}
	handler := compilerast.ExceptHandler{Star: starred}
	token, err := parser.peek(0)
	if err != nil {
		return compilerast.ExceptHandler{}, err
	}
	if token.Kind != lexer.Colon {
		handler.Type, err = parser.parseConditionalExpression()
		if err != nil {
			return compilerast.ExceptHandler{}, err
		}
		if _, matched, err := parser.takeKeyword("as"); err != nil {
			return compilerast.ExceptHandler{}, err
		} else if matched {
			name, err := parser.expect(lexer.Name, "expected name after 'as'")
			if err != nil {
				return compilerast.ExceptHandler{}, err
			}
			if isHardKeyword(name.Text) {
				return compilerast.ExceptHandler{}, parser.syntaxError(name, "expected name after 'as'")
			}
			handler.Name = name.Text
		}
	} else if starred {
		return compilerast.ExceptHandler{}, parser.syntaxError(token, "except* requires an exception type")
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after except handler"); err != nil {
		return compilerast.ExceptHandler{}, err
	}
	handler.Body, err = parser.parseSuite()
	if err != nil {
		return compilerast.ExceptHandler{}, err
	}
	handler.Range = joinSpans(keyword.Span, handler.Body[len(handler.Body)-1].Span())
	return handler, nil
}

func (parser *parserState) nextKeywordIs(text string) bool {
	token, err := parser.peek(0)
	return err == nil && token.Kind == lexer.Name && token.Text == text
}
