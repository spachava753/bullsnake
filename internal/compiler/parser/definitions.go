package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseFunctionDefinition parses a synchronous or asynchronous definition,
// including parameters, optional return annotation, and body suite.
func (parser *parserState) parseFunctionDefinition(asynchronous bool) (compilerast.Stmt, error) {
	start, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if asynchronous {
		if _, err := parser.expectKeyword("async", "expected 'async'"); err != nil {
			return nil, err
		}
	}
	if _, err := parser.expectKeyword("def", "expected 'def'"); err != nil {
		return nil, err
	}
	name, err := parser.expect(lexer.Name, "expected function name")
	if err != nil {
		return nil, err
	}
	if isHardKeyword(name.Text) {
		return nil, parser.syntaxError(name, "expected function name")
	}
	if _, err := parser.expect(lexer.LParen, "expected '(' after function name"); err != nil {
		return nil, err
	}
	parameters, err := parser.parseParameterList(lexer.RParen, true)
	if err != nil {
		return nil, err
	}
	var returns compilerast.Expr
	if _, matched, err := parser.take(lexer.RightArrow); err != nil {
		return nil, err
	} else if matched {
		returns, err = parser.parseConditionalExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after function signature"); err != nil {
		return nil, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, err
	}
	return &compilerast.FunctionDefStmt{
		Range:      joinSpans(start.Span, body[len(body)-1].Span()),
		Name:       name.Text,
		Parameters: parameters,
		Returns:    returns,
		Body:       body,
		Async:      asynchronous,
	}, nil
}

func (parser *parserState) parseLambdaExpression() (compilerast.Expr, error) {
	keyword, err := parser.expectKeyword("lambda", "expected 'lambda'")
	if err != nil {
		return nil, err
	}
	parameters, err := parser.parseParameterList(lexer.Colon, false)
	if err != nil {
		return nil, err
	}
	body, err := parser.parseConditionalExpression()
	if err != nil {
		return nil, err
	}
	return &compilerast.LambdaExpr{
		Range:      joinSpans(keyword.Span, body.Span()),
		Parameters: parameters,
		Body:       body,
	}, nil
}
