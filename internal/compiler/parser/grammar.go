package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type parserState struct {
	filename string
	cursor   tokenCursor
}

// parseModule parses the shared statement grammar and covers the root span
// from the first through the final statement.
func (parser *parserState) parseModule() (*compilerast.Module, error) {
	body, end, err := parser.parseStatementList()
	if err != nil {
		return nil, err
	}
	span := end.Span
	if len(body) != 0 {
		span = joinSpans(body[0].Span(), body[len(body)-1].Span())
	}
	return &compilerast.Module{Range: span, Body: body}, nil
}

// parseStatementList consumes blank logical lines and statements until the
// mode's end marker while rejecting indentation outside a suite.
func (parser *parserState) parseStatementList() ([]compilerast.Stmt, lexer.Token, error) {
	var body []compilerast.Stmt
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, token, err
		}
		switch token.Kind {
		case lexer.EndMarker:
			return body, token, nil
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

func (parser *parserState) parseStatement() (compilerast.Stmt, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Name && token.Text == "if" {
		return parser.parseIfPrefix()
	}
	return parser.parseSimpleStatement()
}

// parseIfPrefix recognizes enough suite structure to report indentation and
// interactive-incompleteness errors before full compound statements are added.
func (parser *parserState) parseIfPrefix() (compilerast.Stmt, error) {
	start, err := parser.advance()
	if err != nil {
		return nil, err
	}
	if _, err := parser.parseExpression(); err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':'"); err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Newline, "expected a newline after the if condition"); err != nil {
		return nil, err
	}
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind != lexer.Indent {
		return nil, parser.syntaxError(token, "expected an indented block")
	}
	return nil, parser.errorAt(start.Span, "if statements are not supported", false)
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
	default:
		return parser.errorAt(expression.Span(), "invalid assignment target", false)
	}
}

// parseExpression promotes comma-separated comparisons to a tuple while
// leaving a single expression unchanged.
func (parser *parserState) parseExpression() (compilerast.Expr, error) {
	first, err := parser.parseComparison()
	if err != nil {
		return nil, err
	}

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
		element, err := parser.parseComparison()
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
	left, err := parser.parsePrimary()
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

// parsePrimary repeatedly attaches call suffixes to an atom, which permits
// call chains without left-recursive grammar rules.
func (parser *parserState) parsePrimary() (compilerast.Expr, error) {
	expression, err := parser.parseAtom()
	if err != nil {
		return nil, err
	}
	for {
		_, matched, err := parser.take(lexer.LParen)
		if err != nil {
			return nil, err
		}
		if !matched {
			return expression, nil
		}
		expression, err = parser.finishCall(expression)
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
		if isHardKeyword(token.Text) {
			return nil, parser.syntaxError(token, "expected expression")
		}
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		return &compilerast.Name{Range: token.Span, ID: token.Text, Context: compilerast.Load}, nil
	case lexer.Number:
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		return &compilerast.NumberLiteral{Range: token.Span, Text: token.Text}, nil
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
		expression, err := parser.parseExpression()
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

// finishCall parses positional arguments after an opening parenthesis and
// permits one trailing comma before the closing parenthesis.
func (parser *parserState) finishCall(function compilerast.Expr) (compilerast.Expr, error) {
	close, matched, err := parser.take(lexer.RParen)
	if err != nil {
		return nil, err
	}
	if matched {
		return &compilerast.CallExpr{
			Range:    joinSpans(function.Span(), close.Span),
			Function: function,
		}, nil
	}

	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Comma {
		return nil, parser.syntaxError(token, "expected argument")
	}

	var arguments []compilerast.Expr
	for {
		argument, err := parser.parseComparison()
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)

		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
		token, err = parser.peek(0)
		if err != nil {
			return nil, err
		}
		if token.Kind == lexer.RParen {
			break
		}
		if token.Kind == lexer.Comma {
			return nil, parser.syntaxError(token, "expected argument")
		}
	}

	close, err = parser.expect(lexer.RParen, "expected ')' after arguments")
	if err != nil {
		return nil, err
	}
	return &compilerast.CallExpr{
		Range:     joinSpans(function.Span(), close.Span),
		Function:  function,
		Arguments: arguments,
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

func joinSpans(start, end lexer.Span) lexer.Span {
	return lexer.Span{Start: start.Start, End: end.End}
}
