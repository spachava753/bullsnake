package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseExpression parses yield or a possibly starred, comma-separated
// expression list.
func (parser *parserState) parseExpression() (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Name && token.Text == "yield" {
		return parser.parseYieldExpression()
	}
	first, err := parser.parseStarExpression()
	if err != nil {
		return nil, err
	}
	return parser.finishTupleExpression(first)
}

func (parser *parserState) parseStarExpression() (compilerast.Expr, error) {
	star, matched, err := parser.take(lexer.Star)
	if err != nil {
		return nil, err
	}
	if !matched {
		return parser.parseConditionalExpression()
	}
	value, err := parser.parseDisjunction()
	if err != nil {
		return nil, err
	}
	return &compilerast.StarredExpr{
		Range:   joinSpans(star.Span, value.Span()),
		Value:   value,
		Context: compilerast.Load,
	}, nil
}

// parseNamedExpression recognizes NAME := value with two-token lookahead and
// otherwise leaves the input to ordinary conditional-expression parsing.
func (parser *parserState) parseNamedExpression() (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind != lexer.Name || isHardKeyword(token.Text) {
		return parser.parseConditionalExpression()
	}
	next, err := parser.peek(1)
	if err != nil {
		return nil, err
	}
	if next.Kind != lexer.ColonEqual {
		return parser.parseConditionalExpression()
	}
	if _, err := parser.advance(); err != nil {
		return nil, err
	}
	if _, err := parser.advance(); err != nil {
		return nil, err
	}
	value, err := parser.parseConditionalExpression()
	if err != nil {
		return nil, err
	}
	return &compilerast.NamedExpr{
		Range:  joinSpans(token.Span, value.Span()),
		Target: &compilerast.Name{Range: token.Span, ID: token.Text, Context: compilerast.Store},
		Value:  value,
	}, nil
}

// parseConditionalExpression parses Python's right-associative value if
// condition else alternative form above boolean precedence.
func (parser *parserState) parseConditionalExpression() (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Name && token.Text == "lambda" {
		return parser.parseLambdaExpression()
	}
	thenValue, err := parser.parseDisjunction()
	if err != nil {
		return nil, err
	}
	_, matched, err := parser.takeKeyword("if")
	if err != nil {
		return nil, err
	}
	if !matched {
		return thenValue, nil
	}
	condition, err := parser.parseDisjunction()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expectKeyword("else", "expected 'else' in conditional expression"); err != nil {
		return nil, err
	}
	alternative, err := parser.parseConditionalExpression()
	if err != nil {
		return nil, err
	}
	return &compilerast.ConditionalExpr{
		Range:     joinSpans(thenValue.Span(), alternative.Span()),
		Condition: condition,
		Then:      thenValue,
		Else:      alternative,
	}, nil
}

// parseYieldExpression parses an empty yield, a yielded expression list, or
// delegation through yield from.
func (parser *parserState) parseYieldExpression() (compilerast.Expr, error) {
	keyword, err := parser.expectKeyword("yield", "expected 'yield'")
	if err != nil {
		return nil, err
	}
	_, from, err := parser.takeKeyword("from")
	if err != nil {
		return nil, err
	}
	if from {
		value, err := parser.parseConditionalExpression()
		if err != nil {
			return nil, err
		}
		return &compilerast.YieldExpr{Range: joinSpans(keyword.Span, value.Span()), Value: value, From: true}, nil
	}

	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	switch token.Kind {
	case lexer.Newline, lexer.EndMarker, lexer.RParen, lexer.RSquare, lexer.RBrace, lexer.Semicolon:
		return &compilerast.YieldExpr{Range: keyword.Span}, nil
	}
	value, err := parser.parseStarExpression()
	if err != nil {
		return nil, err
	}
	value, err = parser.finishTupleExpression(value)
	if err != nil {
		return nil, err
	}
	return &compilerast.YieldExpr{Range: joinSpans(keyword.Span, value.Span()), Value: value}, nil
}

// finishTupleExpression parses the comma-separated tail after an already
// parsed first element and preserves a trailing comma in the tuple span.
func (parser *parserState) finishTupleExpression(first compilerast.Expr) (compilerast.Expr, error) {
	elements := []compilerast.Expr{first}
	end := first.Span()
	for {
		comma, matched, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		end = comma.Span
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		if tupleTerminator(token.Kind) {
			break
		}
		element, err := parser.parseStarExpression()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
		end = element.Span()
	}
	if len(elements) == 1 && end == first.Span() {
		return first, nil
	}
	return &compilerast.TupleExpr{
		Range:    joinSpans(first.Span(), end),
		Elements: elements,
		Context:  compilerast.Load,
	}, nil
}

func (parser *parserState) parseDisjunction() (compilerast.Expr, error) {
	return parser.parseBooleanChain("or", compilerast.Or, parser.parseConjunction)
}

func (parser *parserState) parseConjunction() (compilerast.Expr, error) {
	return parser.parseBooleanChain("and", compilerast.And, parser.parseInversion)
}

// parseBooleanChain collects one short-circuiting and/or chain into a single
// node while its operand function handles the next tighter precedence level.
func (parser *parserState) parseBooleanChain(
	keyword string,
	operator compilerast.BooleanOperator,
	parseOperand func() (compilerast.Expr, error),
) (compilerast.Expr, error) {
	first, err := parseOperand()
	if err != nil {
		return nil, err
	}
	values := []compilerast.Expr{first}
	for {
		_, matched, err := parser.takeKeyword(keyword)
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		value, err := parseOperand()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if len(values) == 1 {
		return first, nil
	}
	return &compilerast.BooleanExpr{
		Range:  joinSpans(first.Span(), values[len(values)-1].Span()),
		Op:     operator,
		Values: values,
	}, nil
}

func (parser *parserState) parseInversion() (compilerast.Expr, error) {
	keyword, matched, err := parser.takeKeyword("not")
	if err != nil {
		return nil, err
	}
	if !matched {
		return parser.parseComparison()
	}
	operand, err := parser.parseInversion()
	if err != nil {
		return nil, err
	}
	return &compilerast.UnaryExpr{
		Range:   joinSpans(keyword.Span, operand.Span()),
		Op:      compilerast.Not,
		Operand: operand,
	}, nil
}

// parseComparison retains every operator and right operand so chained
// comparisons are not reduced to ordinary left-associated binary nodes.
func (parser *parserState) parseComparison() (compilerast.Expr, error) {
	left, err := parser.parseBinary(1)
	if err != nil {
		return nil, err
	}

	var operators []compilerast.ComparisonOperator
	var comparators []compilerast.Expr
	for {
		operator, width, matched, err := parser.comparisonOperator()
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		for range width {
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
		}
		right, err := parser.parseBinary(1)
		if err != nil {
			return nil, err
		}
		operators = append(operators, operator)
		comparators = append(comparators, right)
	}
	if len(operators) == 0 {
		return left, nil
	}
	return &compilerast.CompareExpr{
		Range:       joinSpans(left.Span(), comparators[len(comparators)-1].Span()),
		Left:        left,
		Operators:   operators,
		Comparators: comparators,
	}, nil
}

// parseBinary uses precedence climbing for the ordinary left-associative
// operators currently supported by the AST.
func (parser *parserState) parseBinary(minimumPrecedence int) (compilerast.Expr, error) {
	left, err := parser.parseFactor()
	if err != nil {
		return nil, err
	}
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		definition, matched := ordinaryBinaryOperators[token.Kind]
		if !matched || definition.precedence < minimumPrecedence {
			return left, nil
		}
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		right, err := parser.parseBinary(definition.precedence + 1)
		if err != nil {
			return nil, err
		}
		left = &compilerast.BinaryExpr{
			Range: joinSpans(left.Span(), right.Span()),
			Left:  left,
			Op:    definition.operator,
			Right: right,
		}
	}
}

func (parser *parserState) parseFactor() (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	operator, matched := factorOperators[token.Kind]
	if !matched {
		return parser.parsePower()
	}
	if _, err := parser.advance(); err != nil {
		return nil, err
	}
	operand, err := parser.parseFactor()
	if err != nil {
		return nil, err
	}
	return &compilerast.UnaryExpr{
		Range:   joinSpans(token.Span, operand.Span()),
		Op:      operator,
		Operand: operand,
	}, nil
}

func (parser *parserState) parseAwaitPrimary() (compilerast.Expr, error) {
	keyword, matched, err := parser.takeKeyword("await")
	if err != nil {
		return nil, err
	}
	if !matched {
		return parser.parsePrimary()
	}
	value, err := parser.parsePrimary()
	if err != nil {
		return nil, err
	}
	return &compilerast.AwaitExpr{Range: joinSpans(keyword.Span, value.Span()), Value: value}, nil
}

func (parser *parserState) parsePower() (compilerast.Expr, error) {
	left, err := parser.parseAwaitPrimary()
	if err != nil {
		return nil, err
	}
	_, matched, err := parser.take(lexer.DoubleStar)
	if err != nil {
		return nil, err
	}
	if !matched {
		return left, nil
	}
	right, err := parser.parseFactor()
	if err != nil {
		return nil, err
	}
	return &compilerast.BinaryExpr{
		Range: joinSpans(left.Span(), right.Span()),
		Left:  left,
		Op:    compilerast.Power,
		Right: right,
	}, nil
}

// comparisonOperator recognizes symbolic comparisons and the one- or two-word
// keyword forms without consuming them.
func (parser *parserState) comparisonOperator() (compilerast.ComparisonOperator, int, bool, error) {
	token, err := parser.peek(0)
	if err != nil {
		return 0, 0, false, err
	}
	switch token.Kind {
	case lexer.EqualEqual:
		return compilerast.Equal, 1, true, nil
	case lexer.NotEqual:
		return compilerast.NotEqual, 1, true, nil
	case lexer.Less:
		return compilerast.Less, 1, true, nil
	case lexer.LessEqual:
		return compilerast.LessEqual, 1, true, nil
	case lexer.Greater:
		return compilerast.Greater, 1, true, nil
	case lexer.GreaterEqual:
		return compilerast.GreaterEqual, 1, true, nil
	}
	if token.Kind != lexer.Name {
		return 0, 0, false, nil
	}
	switch token.Text {
	case "in":
		return compilerast.In, 1, true, nil
	case "is":
		next, err := parser.peek(1)
		if err != nil {
			return 0, 0, false, err
		}
		if next.Kind == lexer.Name && next.Text == "not" {
			return compilerast.IsNot, 2, true, nil
		}
		return compilerast.Is, 1, true, nil
	case "not":
		next, err := parser.peek(1)
		if err != nil {
			return 0, 0, false, err
		}
		if next.Kind == lexer.Name && next.Text == "in" {
			return compilerast.NotIn, 2, true, nil
		}
	}
	return 0, 0, false, nil
}

var factorOperators = map[lexer.Kind]compilerast.UnaryOperator{
	lexer.Plus:  compilerast.Positive,
	lexer.Minus: compilerast.Negative,
	lexer.Tilde: compilerast.Invert,
}

type binaryOperatorDefinition struct {
	operator   compilerast.BinaryOperator
	precedence int
}

var ordinaryBinaryOperators = map[lexer.Kind]binaryOperatorDefinition{
	lexer.VBar:        {operator: compilerast.BitOr, precedence: 1},
	lexer.Circumflex:  {operator: compilerast.BitXor, precedence: 2},
	lexer.Ampersand:   {operator: compilerast.BitAnd, precedence: 3},
	lexer.LeftShift:   {operator: compilerast.LeftShift, precedence: 4},
	lexer.RightShift:  {operator: compilerast.RightShift, precedence: 4},
	lexer.Plus:        {operator: compilerast.Add, precedence: 5},
	lexer.Minus:       {operator: compilerast.Subtract, precedence: 5},
	lexer.Star:        {operator: compilerast.Multiply, precedence: 6},
	lexer.At:          {operator: compilerast.MatrixMultiply, precedence: 6},
	lexer.Slash:       {operator: compilerast.Divide, precedence: 6},
	lexer.DoubleSlash: {operator: compilerast.FloorDivide, precedence: 6},
	lexer.Percent:     {operator: compilerast.Modulo, precedence: 6},
}

func tupleTerminator(kind lexer.Kind) bool {
	switch kind {
	case lexer.RParen, lexer.Newline, lexer.EndMarker, lexer.Equal, lexer.Colon:
		return true
	default:
		return false
	}
}
