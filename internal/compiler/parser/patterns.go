package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseMatchStatement parses an indented sequence of case blocks for one
// subject expression.
func (parser *parserState) parseMatchStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("match", "expected 'match'")
	if err != nil {
		return nil, err
	}
	subject, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after match subject"); err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Newline, "expected newline after match subject"); err != nil {
		return nil, err
	}
	if _, err := parser.expect(lexer.Indent, "expected an indented case block"); err != nil {
		return nil, err
	}
	var cases []compilerast.MatchCase
	for parser.nextKeywordIs("case") {
		matchCase, err := parser.parseMatchCase()
		if err != nil {
			return nil, err
		}
		cases = append(cases, matchCase)
	}
	if len(cases) == 0 {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		return nil, parser.syntaxError(token, "match requires at least one case")
	}
	if _, err := parser.expect(lexer.Dedent, "expected end of match statement"); err != nil {
		return nil, err
	}
	return &compilerast.MatchStmt{
		Range:   joinSpans(keyword.Span, cases[len(cases)-1].Range),
		Subject: subject,
		Cases:   cases,
	}, nil
}

// parseMatchCase parses one case pattern, optional guard, and body suite.
func (parser *parserState) parseMatchCase() (compilerast.MatchCase, error) {
	keyword, err := parser.expectKeyword("case", "expected 'case'")
	if err != nil {
		return compilerast.MatchCase{}, err
	}
	pattern, err := parser.parsePattern()
	if err != nil {
		return compilerast.MatchCase{}, err
	}
	var guard compilerast.Expr
	if _, matched, err := parser.takeKeyword("if"); err != nil {
		return compilerast.MatchCase{}, err
	} else if matched {
		guard, err = parser.parseNamedExpression()
		if err != nil {
			return compilerast.MatchCase{}, err
		}
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after case pattern"); err != nil {
		return compilerast.MatchCase{}, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return compilerast.MatchCase{}, err
	}
	return compilerast.MatchCase{
		Range:   joinSpans(keyword.Span, body[len(body)-1].Span()),
		Pattern: pattern,
		Guard:   guard,
		Body:    body,
	}, nil
}

// parsePattern applies the low-precedence as capture after an OR pattern.
func (parser *parserState) parsePattern() (compilerast.Pattern, error) {
	pattern, err := parser.parseOrPattern()
	if err != nil {
		return nil, err
	}
	if _, matched, err := parser.takeKeyword("as"); err != nil {
		return nil, err
	} else if matched {
		name, err := parser.expect(lexer.Name, "expected capture name after 'as'")
		if err != nil {
			return nil, err
		}
		if isHardKeyword(name.Text) || name.Text == "_" {
			return nil, parser.syntaxError(name, "expected capture name after 'as'")
		}
		return &compilerast.AsPattern{
			Range:   joinSpans(pattern.Span(), name.Span),
			Pattern: pattern,
			Name:    name.Text,
		}, nil
	}
	return pattern, nil
}

// parseOrPattern collects one or more closed-pattern alternatives.
func (parser *parserState) parseOrPattern() (compilerast.Pattern, error) {
	first, err := parser.parseClosedPattern()
	if err != nil {
		return nil, err
	}
	patterns := []compilerast.Pattern{first}
	for {
		_, matched, err := parser.take(lexer.VBar)
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		pattern, err := parser.parseClosedPattern()
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}
	if len(patterns) == 1 {
		return first, nil
	}
	return &compilerast.OrPattern{
		Range:    joinSpans(first.Span(), patterns[len(patterns)-1].Span()),
		Patterns: patterns,
	}, nil
}

// parseClosedPattern dispatches literal, capture, value, class, sequence, and
// mapping forms from one token of lookahead.
func (parser *parserState) parseClosedPattern() (compilerast.Pattern, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	switch token.Kind {
	case lexer.Name:
		return parser.parseNamePattern()
	case lexer.Number, lexer.String, lexer.Plus, lexer.Minus:
		value, err := parser.parsePatternValueExpression()
		if err != nil {
			return nil, err
		}
		return &compilerast.ValuePattern{Range: value.Span(), Value: value}, nil
	case lexer.LParen, lexer.LSquare:
		return parser.parseSequencePattern()
	case lexer.LBrace:
		return parser.parseMappingPattern()
	default:
		return nil, parser.syntaxError(token, "expected pattern")
	}
}

// parseNamePattern distinguishes wildcard and singleton literals, dotted value
// and class patterns, and ordinary capture names.
func (parser *parserState) parseNamePattern() (compilerast.Pattern, error) {
	first, err := parser.advance()
	if err != nil {
		return nil, err
	}
	switch first.Text {
	case "_":
		return &compilerast.WildcardPattern{Range: first.Span}, nil
	case "True":
		value := &compilerast.BooleanLiteral{Range: first.Span, Value: true}
		return &compilerast.ValuePattern{Range: first.Span, Value: value}, nil
	case "False":
		value := &compilerast.BooleanLiteral{Range: first.Span}
		return &compilerast.ValuePattern{Range: first.Span, Value: value}, nil
	case "None":
		value := &compilerast.NoneLiteral{Range: first.Span}
		return &compilerast.ValuePattern{Range: first.Span, Value: value}, nil
	}
	if isHardKeyword(first.Text) {
		return nil, parser.syntaxError(first, "expected pattern")
	}

	value := compilerast.Expr(&compilerast.Name{Range: first.Span, ID: first.Text, Context: compilerast.Load})
	dotted := false
	for {
		_, matched, err := parser.take(lexer.Dot)
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		name, err := parser.expect(lexer.Name, "expected name after '.' in pattern")
		if err != nil {
			return nil, err
		}
		value = &compilerast.AttributeExpr{
			Range:   joinSpans(value.Span(), name.Span),
			Value:   value,
			Name:    name.Text,
			Context: compilerast.Load,
		}
		dotted = true
	}
	if _, class, err := parser.take(lexer.LParen); err != nil {
		return nil, err
	} else if class {
		return parser.finishClassPattern(value)
	}
	if dotted {
		return &compilerast.ValuePattern{Range: value.Span(), Value: value}, nil
	}
	return &compilerast.CapturePattern{Range: first.Span, Name: first.Text}, nil
}

// parseSequencePattern parses list patterns, parenthesized sequence patterns,
// and grouped patterns while enforcing one starred remainder.
func (parser *parserState) parseSequencePattern() (compilerast.Pattern, error) {
	open, err := parser.advance()
	if err != nil {
		return nil, err
	}
	closeKind := lexer.RParen
	if open.Kind == lexer.LSquare {
		closeKind = lexer.RSquare
	}
	if close, empty, err := parser.take(closeKind); err != nil {
		return nil, err
	} else if empty {
		return &compilerast.SequencePattern{Range: joinSpans(open.Span, close.Span)}, nil
	}

	var patterns []compilerast.Pattern
	starred := false
	commaSeen := false
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		var pattern compilerast.Pattern
		if token.Kind == lexer.Star {
			if starred {
				return nil, parser.syntaxError(token, "multiple starred names in sequence pattern")
			}
			pattern, err = parser.parseStarPattern()
			starred = true
		} else {
			pattern, err = parser.parsePattern()
		}
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
		commaSeen = true
		if close, done, err := parser.take(closeKind); err != nil {
			return nil, err
		} else if done {
			return &compilerast.SequencePattern{Range: joinSpans(open.Span, close.Span), Elements: patterns}, nil
		}
	}
	close, err := parser.expect(closeKind, "expected closing delimiter after sequence pattern")
	if err != nil {
		return nil, err
	}
	if open.Kind == lexer.LParen && !commaSeen && len(patterns) == 1 {
		return patterns[0], nil
	}
	return &compilerast.SequencePattern{Range: joinSpans(open.Span, close.Span), Elements: patterns}, nil
}

func (parser *parserState) parseStarPattern() (compilerast.Pattern, error) {
	star, err := parser.advance()
	if err != nil {
		return nil, err
	}
	name, err := parser.expect(lexer.Name, "expected name after '*' in pattern")
	if err != nil {
		return nil, err
	}
	if isHardKeyword(name.Text) {
		return nil, parser.syntaxError(name, "expected name after '*' in pattern")
	}
	capture := name.Text
	if capture == "_" {
		capture = ""
	}
	return &compilerast.StarPattern{Range: joinSpans(star.Span, name.Span), Name: capture}, nil
}

// parseMappingPattern parses literal keys, subpatterns, and an optional **rest
// capture.
func (parser *parserState) parseMappingPattern() (compilerast.Pattern, error) {
	open, err := parser.advance()
	if err != nil {
		return nil, err
	}
	if close, empty, err := parser.take(lexer.RBrace); err != nil {
		return nil, err
	} else if empty {
		return &compilerast.MappingPattern{Range: joinSpans(open.Span, close.Span)}, nil
	}
	var keys []compilerast.Expr
	var patterns []compilerast.Pattern
	rest := ""
	for {
		if token, _ := parser.peek(0); token.Kind == lexer.DoubleStar {
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			name, err := parser.expect(lexer.Name, "expected name after '**' in mapping pattern")
			if err != nil {
				return nil, err
			}
			rest = name.Text
			_, _, err = parser.take(lexer.Comma)
			if err != nil {
				return nil, err
			}
			break
		}
		key, err := parser.parsePatternValueExpression()
		if err != nil {
			return nil, err
		}
		if _, err := parser.expect(lexer.Colon, "expected ':' in mapping pattern"); err != nil {
			return nil, err
		}
		pattern, err := parser.parsePattern()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		patterns = append(patterns, pattern)
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
		if close, done, err := parser.take(lexer.RBrace); err != nil {
			return nil, err
		} else if done {
			return &compilerast.MappingPattern{Range: joinSpans(open.Span, close.Span), Keys: keys, Patterns: patterns, Rest: rest}, nil
		}
	}
	close, err := parser.expect(lexer.RBrace, "expected '}' after mapping pattern")
	if err != nil {
		return nil, err
	}
	return &compilerast.MappingPattern{Range: joinSpans(open.Span, close.Span), Keys: keys, Patterns: patterns, Rest: rest}, nil
}

// finishClassPattern parses positional and keyword subpatterns after the class
// expression and opening parenthesis.
func (parser *parserState) finishClassPattern(class compilerast.Expr) (compilerast.Pattern, error) {
	if close, empty, err := parser.take(lexer.RParen); err != nil {
		return nil, err
	} else if empty {
		return &compilerast.ClassPattern{Range: joinSpans(class.Span(), close.Span), Class: class}, nil
	}
	var positional []compilerast.Pattern
	var keywords []compilerast.PatternKeyword
	keywordSeen := false
	names := make(map[string]struct{})
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		next, err := parser.peek(1)
		if err != nil {
			return nil, err
		}
		if token.Kind == lexer.Name && next.Kind == lexer.Equal {
			keywordSeen = true
			if _, duplicate := names[token.Text]; duplicate {
				return nil, parser.syntaxError(token, "duplicate class pattern keyword")
			}
			names[token.Text] = struct{}{}
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			pattern, err := parser.parsePattern()
			if err != nil {
				return nil, err
			}
			keywords = append(keywords, compilerast.PatternKeyword{Range: joinSpans(token.Span, pattern.Span()), Name: token.Text, Pattern: pattern})
		} else {
			if keywordSeen {
				return nil, parser.syntaxError(token, "positional pattern follows keyword pattern")
			}
			pattern, err := parser.parsePattern()
			if err != nil {
				return nil, err
			}
			positional = append(positional, pattern)
		}
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
		if close, done, err := parser.take(lexer.RParen); err != nil {
			return nil, err
		} else if done {
			return &compilerast.ClassPattern{Range: joinSpans(class.Span(), close.Span), Class: class, Positional: positional, Keywords: keywords}, nil
		}
	}
	close, err := parser.expect(lexer.RParen, "expected ')' after class pattern")
	if err != nil {
		return nil, err
	}
	return &compilerast.ClassPattern{Range: joinSpans(class.Span(), close.Span), Class: class, Positional: positional, Keywords: keywords}, nil
}

// parsePatternValueExpression parses the restricted literal or dotted-name
// expressions accepted by value patterns and mapping keys.
func (parser *parserState) parsePatternValueExpression() (compilerast.Expr, error) {
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.String {
		return parser.parseStrings()
	}
	if token.Kind == lexer.Plus || token.Kind == lexer.Minus {
		return parser.parseFactor()
	}
	if token.Kind == lexer.Number {
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		return &compilerast.NumberLiteral{Range: token.Span, Text: token.Text}, nil
	}
	if token.Kind != lexer.Name {
		return nil, parser.syntaxError(token, "expected literal or value pattern")
	}
	pattern, err := parser.parseNamePattern()
	if err != nil {
		return nil, err
	}
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern:
		return pattern.Value, nil
	default:
		return nil, parser.errorAt(pattern.Span(), "mapping pattern key must be a literal or dotted value", false)
	}
}
