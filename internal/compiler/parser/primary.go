package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parsePrimary repeatedly attaches call, attribute, and subscript suffixes to
// an atom, which permits chains without left-recursive grammar rules.
func (parser *parserState) parsePrimary() (compilerast.Expr, error) {
	expression, err := parser.parseAtom()
	if err != nil {
		return nil, err
	}
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		switch token.Kind {
		case lexer.LParen:
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			expression, err = parser.finishCall(expression)
		case lexer.Dot:
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			expression, err = parser.finishAttribute(expression)
		case lexer.LSquare:
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			expression, err = parser.finishSubscript(expression)
		default:
			return expression, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// parseAtom constructs terminal expressions and lets parentheses recursively
// enter the full tuple-capable expression grammar.
func (parser *parserState) parseAtom() (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	switch token.Kind {
	case lexer.Name:
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		switch token.Text {
		case "True":
			return &compilerast.BooleanLiteral{Range: token.Span, Value: true}, nil
		case "False":
			return &compilerast.BooleanLiteral{Range: token.Span, Value: false}, nil
		case "None":
			return &compilerast.NoneLiteral{Range: token.Span}, nil
		default:
			if isHardKeyword(token.Text) {
				return nil, parser.syntaxError(token, "expected expression")
			}
			return &compilerast.Name{Range: token.Span, ID: token.Text, Context: compilerast.Load}, nil
		}
	case lexer.Number:
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		return &compilerast.NumberLiteral{Range: token.Span, Text: token.Text}, nil
	case lexer.String, lexer.FStringStart, lexer.TStringStart:
		return parser.parseStrings()
	case lexer.Ellipsis:
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		return &compilerast.EllipsisLiteral{Range: token.Span}, nil
	case lexer.LSquare:
		open, err := parser.advance()
		if err != nil {
			return nil, err
		}
		return parser.parseListDisplay(open)
	case lexer.LBrace:
		open, err := parser.advance()
		if err != nil {
			return nil, err
		}
		return parser.parseBraceDisplay(open)
	case lexer.LParen:
		open, err := parser.advance()
		if err != nil {
			return nil, err
		}
		close, empty, err := parser.take(lexer.RParen)
		if err != nil {
			return nil, err
		}
		if empty {
			return &compilerast.TupleExpr{
				Range:   joinSpans(open.Span, close.Span),
				Context: compilerast.Load,
			}, nil
		}
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		var first compilerast.Expr
		if token.Kind == lexer.Name && token.Text == "yield" {
			first, err = parser.parseYieldExpression()
		} else {
			first, err = parser.parseNamedExpression()
		}
		if err != nil {
			return nil, err
		}
		if parser.comprehensionStarts() {
			clauses, err := parser.parseComprehensionClauses()
			if err != nil {
				return nil, err
			}
			close, err = parser.expect(lexer.RParen, "expected ')' after generator expression")
			if err != nil {
				return nil, err
			}
			return &compilerast.GeneratorExpr{
				Range:   joinSpans(open.Span, close.Span),
				Element: first,
				Clauses: clauses,
			}, nil
		}
		expression, err := parser.finishTupleExpression(first)
		if err != nil {
			return nil, err
		}
		close, err = parser.expect(lexer.RParen, "expected ')'")
		if err != nil {
			return nil, err
		}
		if tuple, ok := expression.(*compilerast.TupleExpr); ok {
			tuple.Range = joinSpans(open.Span, close.Span)
		}
		return expression, nil
	default:
		return nil, parser.syntaxError(token, "expected expression")
	}
}

func (parser *parserState) finishAttribute(value compilerast.Expr) (compilerast.Expr, error) {
	name, err := parser.expect(lexer.Name, "expected attribute name")
	if err != nil {
		return nil, err
	}
	if isHardKeyword(name.Text) {
		return nil, parser.syntaxError(name, "expected attribute name")
	}
	return &compilerast.AttributeExpr{
		Range:   joinSpans(value.Span(), name.Span),
		Value:   value,
		Name:    name.Text,
		Context: compilerast.Load,
	}, nil
}

// finishSubscript parses one or more indexes or slices and keeps commas as a
// tuple in the subscript index.
func (parser *parserState) finishSubscript(value compilerast.Expr) (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.RSquare || token.Kind == lexer.Comma {
		return nil, parser.syntaxError(token, "expected subscript")
	}

	first, err := parser.parseSliceItem()
	if err != nil {
		return nil, err
	}
	items := []compilerast.Expr{first}
	end := first.Span()
	tuple := false
	for {
		comma, matched, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		tuple = true
		end = comma.Span
		token, err = parser.peek(0)
		if err != nil {
			return nil, err
		}
		if token.Kind == lexer.RSquare {
			break
		}
		if token.Kind == lexer.Comma {
			return nil, parser.syntaxError(token, "expected subscript")
		}
		item, err := parser.parseSliceItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		end = item.Span()
	}

	index := first
	if tuple {
		index = &compilerast.TupleExpr{
			Range:    joinSpans(first.Span(), end),
			Elements: items,
			Context:  compilerast.Load,
		}
	}
	close, err := parser.expect(lexer.RSquare, "expected ']'")
	if err != nil {
		return nil, err
	}
	return &compilerast.SubscriptExpr{
		Range:   joinSpans(value.Span(), close.Span),
		Value:   value,
		Index:   index,
		Context: compilerast.Load,
	}, nil
}

// parseSliceItem parses either one index expression or a slice with optional
// lower, upper, and step expressions.
func (parser *parserState) parseSliceItem() (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	var lower compilerast.Expr
	if token.Kind != lexer.Colon {
		lower, err = parser.parseConditionalExpression()
		if err != nil {
			return nil, err
		}
	}

	colon, sliced, err := parser.take(lexer.Colon)
	if err != nil {
		return nil, err
	}
	if !sliced {
		return lower, nil
	}
	start := colon.Span
	if lower != nil {
		start = lower.Span()
	}
	end := colon.Span

	var upper compilerast.Expr
	token, err = parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind != lexer.Colon && token.Kind != lexer.Comma && token.Kind != lexer.RSquare {
		upper, err = parser.parseConditionalExpression()
		if err != nil {
			return nil, err
		}
		end = upper.Span()
	}

	var step compilerast.Expr
	secondColon, stepped, err := parser.take(lexer.Colon)
	if err != nil {
		return nil, err
	}
	if stepped {
		end = secondColon.Span
		token, err = parser.peek(0)
		if err != nil {
			return nil, err
		}
		if token.Kind != lexer.Comma && token.Kind != lexer.RSquare {
			step, err = parser.parseConditionalExpression()
			if err != nil {
				return nil, err
			}
			end = step.Span()
		}
	}
	return &compilerast.SliceExpr{
		Range: joinSpans(start, end),
		Lower: lower,
		Upper: upper,
		Step:  step,
	}, nil
}

func isHardKeyword(text string) bool {
	switch text {
	case "False", "None", "True", "and", "as", "assert", "async", "await",
		"break", "class", "continue", "def", "del", "elif", "else", "except",
		"finally", "for", "from", "global", "if", "import", "in", "is",
		"lambda", "nonlocal", "not", "or", "pass", "raise", "return", "try",
		"while", "with", "yield":
		return true
	default:
		return false
	}
}
