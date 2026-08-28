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
		statements, err := parser.parseStatement()
		if err != nil {
			return nil, token, err
		}
		body = append(body, statements...)
	}
}

// parseStatement dispatches compound statements before the shared
// semicolon-capable simple statement line.
func (parser *parserState) parseStatement() ([]compilerast.Stmt, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.At {
		statement, err := parser.parseDecoratedStatement()
		if err != nil {
			return nil, err
		}
		return []compilerast.Stmt{statement}, nil
	}
	if token.Kind != lexer.Name {
		return parser.parseSimpleStatementLine()
	}

	var statement compilerast.Stmt
	switch token.Text {
	case "if":
		keyword, advanceErr := parser.advance()
		if advanceErr != nil {
			return nil, advanceErr
		}
		statement, err = parser.parseIfClause(keyword)
	case "while":
		statement, err = parser.parseWhileStatement()
	case "for":
		statement, err = parser.parseForStatement(false)
	case "def":
		statement, err = parser.parseFunctionDefinition(false)
	case "class":
		statement, err = parser.parseClassDefinition()
	case "with":
		statement, err = parser.parseWithStatement(false)
	case "try":
		statement, err = parser.parseTryStatement()
	case "match":
		next, peekErr := parser.peek(1)
		if peekErr != nil {
			return nil, peekErr
		}
		_, augmented := augmentedOperators[next.Kind]
		if next.Kind == lexer.Equal || next.Kind == lexer.ColonEqual || augmented {
			return parser.parseSimpleStatementLine()
		}
		statement, err = parser.parseMatchStatement()
	case "async":
		next, peekErr := parser.peek(1)
		if peekErr != nil {
			return nil, peekErr
		}
		if next.Kind != lexer.Name {
			return parser.parseSimpleStatementLine()
		}
		switch next.Text {
		case "for":
			statement, err = parser.parseForStatement(true)
		case "def":
			statement, err = parser.parseFunctionDefinition(true)
		case "with":
			statement, err = parser.parseWithStatement(true)
		default:
			return parser.parseSimpleStatementLine()
		}
	default:
		return parser.parseSimpleStatementLine()
	}
	if err != nil {
		return nil, err
	}
	return []compilerast.Stmt{statement}, nil
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
		return parser.parseSimpleStatementLine()
	}
	if _, err := parser.expect(lexer.Indent, "expected an indented block"); err != nil {
		return nil, err
	}
	body, end, err := parser.parseStatementList(lexer.Dedent)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, parser.syntaxError(end, "expected statement in indented block")
	}
	if _, err := parser.expect(lexer.Dedent, "expected end of indented block"); err != nil {
		return nil, err
	}
	return body, nil
}
