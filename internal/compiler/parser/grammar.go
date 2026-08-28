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
	body, end, err := parser.parseStatementList(lexer.EndMarker)
	if err != nil {
		return nil, err
	}
	span := end.Span
	if len(body) != 0 {
		span = joinSpans(body[0].Span(), body[len(body)-1].Span())
	}
	return &compilerast.Module{Range: span, Body: body}, nil
}

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

// setStoreContext marks valid assignment targets as Store and recursively updates tuple elements.
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

// parseExpression promotes comma-separated comparisons to a tuple while
// leaving a single expression unchanged.
func (parser *parserState) parseExpression() (compilerast.Expr, error) {
	first, err := parser.parseDisjunction()
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
		element, err := parser.parseDisjunction()
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

func (parser *parserState) parsePower() (compilerast.Expr, error) {
	left, err := parser.parsePrimary()
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
	case lexer.String:
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		return &compilerast.StringLiteral{Range: token.Span, Text: token.Text}, nil
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
		lower, err = parser.parseDisjunction()
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
		upper, err = parser.parseDisjunction()
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
			step, err = parser.parseDisjunction()
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
		argument, err := parser.parseDisjunction()
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

func (parser *parserState) takeKeyword(text string) (lexer.Token, bool, error) {
	token, err := parser.peek(0)
	if err != nil {
		return token, false, err
	}
	if token.Kind != lexer.Name || token.Text != text {
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
