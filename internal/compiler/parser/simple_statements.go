package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseSimpleStatementLine parses one or more small statements separated by
// semicolons and consumes their shared line ending.
func (parser *parserState) parseSimpleStatementLine() ([]compilerast.Stmt, error) {
	var statements []compilerast.Stmt
	for {
		statement, err := parser.parseSmallStatement()
		if err != nil {
			return nil, err
		}
		statements = append(statements, statement)
		_, semicolon, err := parser.take(lexer.Semicolon)
		if err != nil {
			return nil, err
		}
		if !semicolon {
			break
		}
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		if token.Kind == lexer.Newline || token.Kind == lexer.EndMarker {
			break
		}
	}
	if err := parser.expectStatementEnd(); err != nil {
		return nil, err
	}
	return statements, nil
}

// parseSmallStatement dispatches hard-keyword simple statements before the
// expression-or-assignment path.
func (parser *parserState) parseSmallStatement() (compilerast.Stmt, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind != lexer.Name {
		return parser.parseAssignmentOrExpression()
	}
	switch token.Text {
	case "return":
		return parser.parseReturnStatement()
	case "raise":
		return parser.parseRaiseStatement()
	case "pass":
		return parser.parseKeywordOnlyStatement("pass")
	case "break":
		return parser.parseKeywordOnlyStatement("break")
	case "continue":
		return parser.parseKeywordOnlyStatement("continue")
	case "global":
		return parser.parseNameDeclaration(true)
	case "nonlocal":
		return parser.parseNameDeclaration(false)
	case "del":
		return parser.parseDeleteStatement()
	case "assert":
		return parser.parseAssertStatement()
	case "import":
		return parser.parseImportStatement()
	case "from":
		return parser.parseFromImportStatement()
	default:
		return parser.parseAssignmentOrExpression()
	}
}

func (parser *parserState) parseKeywordOnlyStatement(keyword string) (compilerast.Stmt, error) {
	token, err := parser.expectKeyword(keyword, "expected '"+keyword+"'")
	if err != nil {
		return nil, err
	}
	switch keyword {
	case "pass":
		return &compilerast.PassStmt{Range: token.Span}, nil
	case "break":
		return &compilerast.BreakStmt{Range: token.Span}, nil
	default:
		return &compilerast.ContinueStmt{Range: token.Span}, nil
	}
}

func (parser *parserState) parseReturnStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("return", "expected 'return'")
	if err != nil {
		return nil, err
	}
	if parser.simpleStatementEnds() {
		return &compilerast.ReturnStmt{Range: keyword.Span}, nil
	}
	value, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	return &compilerast.ReturnStmt{Range: joinSpans(keyword.Span, value.Span()), Value: value}, nil
}

// parseRaiseStatement parses bare re-raise syntax or an exception with an
// optional explicit cause.
func (parser *parserState) parseRaiseStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("raise", "expected 'raise'")
	if err != nil {
		return nil, err
	}
	if parser.simpleStatementEnds() {
		return &compilerast.RaiseStmt{Range: keyword.Span}, nil
	}
	exception, err := parser.parseConditionalExpression()
	if err != nil {
		return nil, err
	}
	end := exception.Span()
	var cause compilerast.Expr
	if _, matched, err := parser.takeKeyword("from"); err != nil {
		return nil, err
	} else if matched {
		cause, err = parser.parseConditionalExpression()
		if err != nil {
			return nil, err
		}
		end = cause.Span()
	}
	return &compilerast.RaiseStmt{
		Range:     joinSpans(keyword.Span, end),
		Exception: exception,
		Cause:     cause,
	}, nil
}

func (parser *parserState) parseDeleteStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("del", "expected 'del'")
	if err != nil {
		return nil, err
	}
	target, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if err := parser.setDeleteContext(target); err != nil {
		return nil, err
	}
	return &compilerast.DeleteStmt{
		Range:   joinSpans(keyword.Span, target.Span()),
		Targets: []compilerast.Expr{target},
	}, nil
}

// parseAssertStatement parses the required condition and optional comma-led
// failure message.
func (parser *parserState) parseAssertStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("assert", "expected 'assert'")
	if err != nil {
		return nil, err
	}
	condition, err := parser.parseConditionalExpression()
	if err != nil {
		return nil, err
	}
	end := condition.Span()
	var message compilerast.Expr
	if _, matched, err := parser.take(lexer.Comma); err != nil {
		return nil, err
	} else if matched {
		message, err = parser.parseConditionalExpression()
		if err != nil {
			return nil, err
		}
		end = message.Span()
	}
	return &compilerast.AssertStmt{
		Range:     joinSpans(keyword.Span, end),
		Condition: condition,
		Message:   message,
	}, nil
}

// parseNameDeclaration parses a non-empty comma-separated global or nonlocal
// name list and returns the matching statement kind.
func (parser *parserState) parseNameDeclaration(global bool) (compilerast.Stmt, error) {
	keyword, err := parser.advance()
	if err != nil {
		return nil, err
	}
	var names []string
	end := keyword.Span
	for {
		name, err := parser.expect(lexer.Name, "expected name in declaration")
		if err != nil {
			return nil, err
		}
		if isHardKeyword(name.Text) {
			return nil, parser.syntaxError(name, "expected name in declaration")
		}
		names = append(names, name.Text)
		end = name.Span
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
	}
	span := joinSpans(keyword.Span, end)
	if global {
		return &compilerast.GlobalStmt{Range: span, Names: names}, nil
	}
	return &compilerast.NonlocalStmt{Range: span, Names: names}, nil
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

func (parser *parserState) simpleStatementEnds() bool {
	token, err := parser.peek(0)
	if err != nil {
		return false
	}
	return token.Kind == lexer.Semicolon || token.Kind == lexer.Newline || token.Kind == lexer.EndMarker
}
