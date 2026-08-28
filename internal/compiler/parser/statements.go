package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseStatementList consumes blank logical lines and statements until its
// caller's terminator while rejecting misplaced indentation tokens.
func (parser *parserState) parseStatementList(terminator lexer.Kind) ([]compilerast.Stmt, lexer.Token, error) {
	var body []compilerast.Stmt
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, token, err
		}
		if token.Kind == terminator {
			return body, token, nil
		}
		switch token.Kind {
		case lexer.EndMarker:
			return nil, token, parser.syntaxError(token, "expected end of indented block")
		case lexer.Newline:
			if _, err := parser.advance(); err != nil {
				return nil, token, err
			}
			continue
		case lexer.Indent, lexer.Dedent:
			return nil, token, parser.syntaxError(token, "unexpected indentation")
		}
		statement, err := parser.parseStatement()
		if err != nil {
			return nil, token, err
		}
		body = append(body, statement)
	}
}

// parseStatement dispatches the supported hard-keyword statements before the
// generic expression-or-assignment path.
func (parser *parserState) parseStatement() (compilerast.Stmt, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Name {
		switch token.Text {
		case "if":
			keyword, err := parser.advance()
			if err != nil {
				return nil, err
			}
			return parser.parseIfClause(keyword)
		case "pass":
			return parser.parsePassStatement()
		}
	}
	return parser.parseSimpleStatement()
}

// parseIfClause parses one conditional clause and recursively folds an elif
// clause into a nested IfStmt alternative.
func (parser *parserState) parseIfClause(keyword lexer.Token) (*compilerast.IfStmt, error) {
	condition, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':'"); err != nil {
		return nil, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, err
	}

	statement := &compilerast.IfStmt{Condition: condition, Body: body}
	end := body[len(body)-1].Span()
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Name && token.Text == "elif" {
		keyword, err := parser.advance()
		if err != nil {
			return nil, err
		}
		alternative, err := parser.parseIfClause(keyword)
		if err != nil {
			return nil, err
		}
		statement.Else = []compilerast.Stmt{alternative}
		end = alternative.Span()
	} else if token.Kind == lexer.Name && token.Text == "else" {
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		if _, err := parser.expect(lexer.Colon, "expected ':'"); err != nil {
			return nil, err
		}
		statement.Else, err = parser.parseSuite()
		if err != nil {
			return nil, err
		}
		end = statement.Else[len(statement.Else)-1].Span()
	}
	statement.Range = joinSpans(keyword.Span, end)
	return statement, nil
}

// parseSuite chooses between one same-line simple statement and a newline,
// indentation pair, nested statement list, and matching dedent.
func (parser *parserState) parseSuite() ([]compilerast.Stmt, error) {
	_, multiline, err := parser.take(lexer.Newline)
	if err != nil {
		return nil, err
	}
	if !multiline {
		statement, err := parser.parseSuiteStatement()
		if err != nil {
			return nil, err
		}
		return []compilerast.Stmt{statement}, nil
	}
	if _, err := parser.expect(lexer.Indent, "expected an indented block"); err != nil {
		return nil, err
	}
	body, _, err := parser.parseStatementList(lexer.Dedent)
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Dedent, "expected end of indented block"); err != nil {
		return nil, err
	}
	return body, nil
}

func (parser *parserState) parseSuiteStatement() (compilerast.Stmt, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Name && token.Text == "pass" {
		return parser.parsePassStatement()
	}
	return parser.parseSimpleStatement()
}

func (parser *parserState) parsePassStatement() (compilerast.Stmt, error) {
	token, err := parser.advance()
	if err != nil {
		return nil, err
	}
	if err := parser.expectStatementEnd(); err != nil {
		return nil, err
	}
	return &compilerast.PassStmt{Range: token.Span}, nil
}

// parseSimpleStatement parses the shared expression prefix once, then turns it
// into an expression statement or one-or-more-target assignment.
func (parser *parserState) parseSimpleStatement() (compilerast.Stmt, error) {
	left, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}

	current := left
	var targets []compilerast.Expr
	for {
		_, matched, err := parser.take(lexer.Equal)
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		if starred, ok := current.(*compilerast.StarredExpr); ok {
			return nil, parser.errorAt(starred.Span(), "starred target must be in a list or tuple", false)
		}
		if err := parser.setStoreContext(current); err != nil {
			return nil, err
		}
		targets = append(targets, current)
		current, err = parser.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	if err := parser.expectStatementEnd(); err != nil {
		return nil, err
	}
	if len(targets) != 0 {
		return &compilerast.AssignStmt{
			Range:   joinSpans(targets[0].Span(), current.Span()),
			Targets: targets,
			Value:   current,
		}, nil
	}
	if starred, ok := left.(*compilerast.StarredExpr); ok {
		return nil, parser.errorAt(starred.Span(), "starred expression must be in an expression list", false)
	}
	return &compilerast.ExprStmt{Range: left.Span(), Value: left}, nil
}

func (parser *parserState) expectStatementEnd() error {
	token, err := parser.peek(0)
	if err != nil {
		return err
	}
	if token.Kind == lexer.EndMarker {
		return nil
	}
	if token.Kind != lexer.Newline {
		return parser.syntaxError(token, "expected end of statement")
	}
	_, err = parser.advance()
	return err
}

// setStoreContext marks valid assignment targets as Store and recursively
// updates tuple elements.
func (parser *parserState) setStoreContext(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		expression.Context = compilerast.Store
		return nil
	case *compilerast.TupleExpr:
		expression.Context = compilerast.Store
		for _, element := range expression.Elements {
			if err := parser.setStoreContext(element); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.ListExpr:
		expression.Context = compilerast.Store
		for _, element := range expression.Elements {
			if err := parser.setStoreContext(element); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.StarredExpr:
		expression.Context = compilerast.Store
		return parser.setStoreContext(expression.Value)
	case *compilerast.AttributeExpr:
		expression.Context = compilerast.Store
		return nil
	case *compilerast.SubscriptExpr:
		expression.Context = compilerast.Store
		return nil
	default:
		return parser.errorAt(expression.Span(), "invalid assignment target", false)
	}
}
