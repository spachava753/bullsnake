package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseWhileStatement parses the condition, body, and optional normal-exit
// else suite of a while loop.
func (parser *parserState) parseWhileStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("while", "expected 'while'")
	if err != nil {
		return nil, err
	}
	condition, err := parser.parseNamedExpression()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after while condition"); err != nil {
		return nil, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, err
	}
	alternative, end, err := parser.parseLoopElse(body[len(body)-1].Span())
	if err != nil {
		return nil, err
	}
	return &compilerast.WhileStmt{
		Range:     joinSpans(keyword.Span, end),
		Condition: condition,
		Body:      body,
		Else:      alternative,
	}, nil
}

// parseForStatement parses a synchronous or asynchronous loop, including its
// assignment target, iterable, body, and optional else suite.
func (parser *parserState) parseForStatement(asynchronous bool) (compilerast.Stmt, error) {
	start, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if asynchronous {
		if _, err := parser.expectKeyword("async", "expected 'async'"); err != nil {
			return nil, err
		}
	}
	if _, err := parser.expectKeyword("for", "expected 'for'"); err != nil {
		return nil, err
	}
	target, err := parser.parseComprehensionTarget()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expectKeyword("in", "expected 'in' in for statement"); err != nil {
		return nil, err
	}
	iterable, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after for iterable"); err != nil {
		return nil, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, err
	}
	alternative, end, err := parser.parseLoopElse(body[len(body)-1].Span())
	if err != nil {
		return nil, err
	}
	return &compilerast.ForStmt{
		Range:    joinSpans(start.Span, end),
		Target:   target,
		Iterable: iterable,
		Body:     body,
		Else:     alternative,
		Async:    asynchronous,
	}, nil
}

// parseLoopElse consumes an else suite when present and otherwise preserves
// the loop body's ending span.
func (parser *parserState) parseLoopElse(end lexer.Span) ([]compilerast.Stmt, lexer.Span, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, end, err
	}
	if token.Kind != lexer.Name || token.Text != "else" {
		return nil, end, nil
	}
	if _, err := parser.advance(); err != nil {
		return nil, end, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after else"); err != nil {
		return nil, end, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, end, err
	}
	return body, body[len(body)-1].Span(), nil
}
