package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseClassDefinition parses a class name, optional generic parameters and
// bases, and its body suite.
func (parser *parserState) parseClassDefinition() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("class", "expected 'class'")
	if err != nil {
		return nil, err
	}
	name, err := parser.expect(lexer.Name, "expected class name")
	if err != nil {
		return nil, err
	}
	if isHardKeyword(name.Text) {
		return nil, parser.syntaxError(name, "expected class name")
	}
	typeParameters, err := parser.parseTypeParameters()
	if err != nil {
		return nil, err
	}

	var bases []compilerast.Expr
	var keywords []compilerast.KeywordArgument
	if _, matched, err := parser.take(lexer.LParen); err != nil {
		return nil, err
	} else if matched {
		dummy := &compilerast.Name{Range: name.Span, ID: name.Text, Context: compilerast.Load}
		expression, err := parser.finishCall(dummy)
		if err != nil {
			return nil, err
		}
		call := expression.(*compilerast.CallExpr)
		bases = call.Arguments
		keywords = call.Keywords
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after class definition"); err != nil {
		return nil, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, err
	}
	return &compilerast.ClassDefStmt{
		Range:          joinSpans(keyword.Span, body[len(body)-1].Span()),
		Name:           name.Text,
		TypeParameters: typeParameters,
		Bases:          bases,
		Keywords:       keywords,
		Body:           body,
	}, nil
}

// parseTypeAliasStatement parses the contextual type keyword, optional generic
// parameters, and alias value.
func (parser *parserState) parseTypeAliasStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("type", "expected 'type'")
	if err != nil {
		return nil, err
	}
	name, err := parser.expect(lexer.Name, "expected type alias name")
	if err != nil {
		return nil, err
	}
	if isHardKeyword(name.Text) {
		return nil, parser.syntaxError(name, "expected type alias name")
	}
	typeParameters, err := parser.parseTypeParameters()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Equal, "expected '=' in type alias"); err != nil {
		return nil, err
	}
	value, err := parser.parseConditionalExpression()
	if err != nil {
		return nil, err
	}
	return &compilerast.TypeAliasStmt{
		Range:          joinSpans(keyword.Span, value.Span()),
		Name:           name.Text,
		TypeParameters: typeParameters,
		Value:          value,
	}, nil
}

// parseDecoratedStatement collects decorator lines, parses the following
// function or class, and attaches the decorators in source order.
func (parser *parserState) parseDecoratedStatement() (compilerast.Stmt, error) {
	start, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	var decorators []compilerast.Expr
	for {
		if _, matched, err := parser.take(lexer.At); err != nil {
			return nil, err
		} else if !matched {
			break
		}
		decorator, err := parser.parseNamedExpression()
		if err != nil {
			return nil, err
		}
		decorators = append(decorators, decorator)
		if _, err := parser.expect(lexer.Newline, "expected newline after decorator"); err != nil {
			return nil, err
		}
	}

	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	var statement compilerast.Stmt
	switch {
	case token.Kind == lexer.Name && token.Text == "def":
		statement, err = parser.parseFunctionDefinition(false)
	case token.Kind == lexer.Name && token.Text == "class":
		statement, err = parser.parseClassDefinition()
	case token.Kind == lexer.Name && token.Text == "async":
		statement, err = parser.parseFunctionDefinition(true)
	default:
		return nil, parser.syntaxError(token, "decorator must precede a function or class")
	}
	if err != nil {
		return nil, err
	}
	switch statement := statement.(type) {
	case *compilerast.FunctionDefStmt:
		statement.Decorators = decorators
		statement.Range = joinSpans(start.Span, statement.Range)
	case *compilerast.ClassDefStmt:
		statement.Decorators = decorators
		statement.Range = joinSpans(start.Span, statement.Range)
	}
	return statement, nil
}
