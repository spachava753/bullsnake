package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseListDisplay parses a possibly empty list display or a list
// comprehension after its opening bracket.
func (parser *parserState) parseListDisplay(open lexer.Token) (compilerast.Expr, error) {
	close, empty, err := parser.take(lexer.RSquare)
	if err != nil {
		return nil, err
	}
	if empty {
		return &compilerast.ListExpr{Range: joinSpans(open.Span, close.Span), Context: compilerast.Load}, nil
	}

	first, err := parser.parseStarredDisplayElement()
	if err != nil {
		return nil, err
	}
	if parser.comprehensionStarts() {
		if _, starred := first.(*compilerast.StarredExpr); starred {
			return nil, parser.errorAt(first.Span(), "iterable unpacking cannot be used in comprehension", false)
		}
		clauses, err := parser.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		close, err = parser.expect(lexer.RSquare, "expected ']' after list comprehension")
		if err != nil {
			return nil, err
		}
		return &compilerast.ListComprehensionExpr{
			Range:   joinSpans(open.Span, close.Span),
			Element: first,
			Clauses: clauses,
		}, nil
	}

	elements := []compilerast.Expr{first}
	for {
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			close, err = parser.expect(lexer.RSquare, "expected ']' after list display")
			if err != nil {
				return nil, err
			}
			break
		}
		close, empty, err = parser.take(lexer.RSquare)
		if err != nil {
			return nil, err
		}
		if empty {
			break
		}
		element, err := parser.parseStarredDisplayElement()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	return &compilerast.ListExpr{
		Range:    joinSpans(open.Span, close.Span),
		Elements: elements,
		Context:  compilerast.Load,
	}, nil
}

// parseBraceDisplay distinguishes empty dictionaries, dictionary entries, and
// set elements from the first token and expression.
func (parser *parserState) parseBraceDisplay(open lexer.Token) (compilerast.Expr, error) {
	close, empty, err := parser.take(lexer.RBrace)
	if err != nil {
		return nil, err
	}
	if empty {
		return &compilerast.DictExpr{Range: joinSpans(open.Span, close.Span)}, nil
	}

	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.DoubleStar {
		key, value, err := parser.parseDictionaryEntry()
		if err != nil {
			return nil, err
		}
		return parser.finishDictionaryDisplay(open, key, value)
	}
	if token.Kind == lexer.Star {
		first, err := parser.parseStarredDisplayElement()
		if err != nil {
			return nil, err
		}
		return parser.finishSetDisplay(open, first)
	}

	first, err := parser.parseDisjunction()
	if err != nil {
		return nil, err
	}
	if parser.comprehensionStarts() {
		clauses, err := parser.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		close, err := parser.expect(lexer.RBrace, "expected '}' after set comprehension")
		if err != nil {
			return nil, err
		}
		return &compilerast.SetComprehensionExpr{
			Range:   joinSpans(open.Span, close.Span),
			Element: first,
			Clauses: clauses,
		}, nil
	}
	_, dictionary, err := parser.take(lexer.Colon)
	if err != nil {
		return nil, err
	}
	if !dictionary {
		return parser.finishSetDisplay(open, first)
	}
	value, err := parser.parseDisjunction()
	if err != nil {
		return nil, err
	}
	if parser.comprehensionStarts() {
		clauses, err := parser.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		close, err := parser.expect(lexer.RBrace, "expected '}' after dictionary comprehension")
		if err != nil {
			return nil, err
		}
		return &compilerast.DictComprehensionExpr{
			Range:   joinSpans(open.Span, close.Span),
			Key:     first,
			Value:   value,
			Clauses: clauses,
		}, nil
	}
	return parser.finishDictionaryDisplay(open, first, value)
}

// finishSetDisplay parses the remaining comma-separated set elements and the
// closing brace.
func (parser *parserState) finishSetDisplay(open lexer.Token, first compilerast.Expr) (compilerast.Expr, error) {
	elements := []compilerast.Expr{first}
	for {
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			close, err := parser.expect(lexer.RBrace, "expected '}' after set display")
			if err != nil {
				return nil, err
			}
			return &compilerast.SetExpr{Range: joinSpans(open.Span, close.Span), Elements: elements}, nil
		}
		close, done, err := parser.take(lexer.RBrace)
		if err != nil {
			return nil, err
		}
		if done {
			return &compilerast.SetExpr{Range: joinSpans(open.Span, close.Span), Elements: elements}, nil
		}
		element, err := parser.parseStarredDisplayElement()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
}

// finishDictionaryDisplay parses the remaining key/value or dictionary-unpack
// entries and the closing brace.
func (parser *parserState) finishDictionaryDisplay(
	open lexer.Token,
	firstKey compilerast.Expr,
	firstValue compilerast.Expr,
) (compilerast.Expr, error) {
	keys := []compilerast.Expr{firstKey}
	values := []compilerast.Expr{firstValue}
	for {
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			close, err := parser.expect(lexer.RBrace, "expected '}' after dictionary display")
			if err != nil {
				return nil, err
			}
			return &compilerast.DictExpr{Range: joinSpans(open.Span, close.Span), Keys: keys, Values: values}, nil
		}
		close, done, err := parser.take(lexer.RBrace)
		if err != nil {
			return nil, err
		}
		if done {
			return &compilerast.DictExpr{Range: joinSpans(open.Span, close.Span), Keys: keys, Values: values}, nil
		}
		key, value, err := parser.parseDictionaryEntry()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		values = append(values, value)
	}
}

// parseDictionaryEntry parses one key/value pair or a **value unpacking entry.
func (parser *parserState) parseDictionaryEntry() (compilerast.Expr, compilerast.Expr, error) {
	_, matched, err := parser.take(lexer.DoubleStar)
	if err != nil {
		return nil, nil, err
	}
	if matched {
		value, err := parser.parseDisjunction()
		return nil, value, err
	}
	key, err := parser.parseDisjunction()
	if err != nil {
		return nil, nil, err
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' in dictionary entry"); err != nil {
		return nil, nil, err
	}
	value, err := parser.parseDisjunction()
	if err != nil {
		return nil, nil, err
	}
	return key, value, nil
}

func (parser *parserState) parseStarredDisplayElement() (compilerast.Expr, error) {
	star, matched, err := parser.take(lexer.Star)
	if err != nil {
		return nil, err
	}
	if !matched {
		return parser.parseDisjunction()
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
