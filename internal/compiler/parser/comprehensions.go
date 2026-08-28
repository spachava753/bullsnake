package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseComprehensionClauses parses one or more async or synchronous for
// clauses and attaches each clause's following if filters.
func (parser *parserState) parseComprehensionClauses() ([]compilerast.Comprehension, error) {
	var clauses []compilerast.Comprehension
	for parser.comprehensionStarts() {
		start, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		_, asynchronous, err := parser.takeKeyword("async")
		if err != nil {
			return nil, err
		}
		if _, err := parser.expectKeyword("for", "expected 'for' in comprehension"); err != nil {
			return nil, err
		}
		target, err := parser.parseComprehensionTarget()
		if err != nil {
			return nil, err
		}
		if _, err := parser.expectKeyword("in", "expected 'in' in comprehension"); err != nil {
			return nil, err
		}
		iterable, err := parser.parseDisjunction()
		if err != nil {
			return nil, err
		}

		end := iterable.Span()
		var conditions []compilerast.Expr
		for {
			_, matched, err := parser.takeKeyword("if")
			if err != nil {
				return nil, err
			}
			if !matched {
				break
			}
			condition, err := parser.parseDisjunction()
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, condition)
			end = condition.Span()
		}
		clauses = append(clauses, compilerast.Comprehension{
			Range:      joinSpans(start.Span, end),
			Target:     target,
			Iterable:   iterable,
			Conditions: conditions,
			Async:      asynchronous,
		})
	}
	if len(clauses) == 0 {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		return nil, parser.syntaxError(token, "expected comprehension clause")
	}
	return clauses, nil
}

// parseComprehensionTarget parses a primary or comma-separated target without
// consuming the clause's comparison-like in separator.
func (parser *parserState) parseComprehensionTarget() (compilerast.Expr, error) {
	first, err := parser.parseComprehensionTargetElement()
	if err != nil {
		return nil, err
	}
	elements := []compilerast.Expr{first}
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
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		if token.Kind == lexer.Name && token.Text == "in" {
			break
		}
		element, err := parser.parseComprehensionTargetElement()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
		end = element.Span()
	}

	target := first
	if tuple {
		target = &compilerast.TupleExpr{
			Range:    joinSpans(first.Span(), end),
			Elements: elements,
			Context:  compilerast.Load,
		}
	} else if _, starred := first.(*compilerast.StarredExpr); starred {
		return nil, parser.errorAt(first.Span(), "starred target must be in a list or tuple", false)
	}
	if err := parser.setStoreContext(target); err != nil {
		return nil, err
	}
	return target, nil
}

func (parser *parserState) parseComprehensionTargetElement() (compilerast.Expr, error) {
	star, matched, err := parser.take(lexer.Star)
	if err != nil {
		return nil, err
	}
	value, err := parser.parsePrimary()
	if err != nil {
		return nil, err
	}
	if !matched {
		return value, nil
	}
	return &compilerast.StarredExpr{
		Range:   joinSpans(star.Span, value.Span()),
		Value:   value,
		Context: compilerast.Load,
	}, nil
}

// comprehensionStarts reports whether lookahead begins a for or async for
// clause without consuming either keyword.
func (parser *parserState) comprehensionStarts() bool {
	token, err := parser.peek(0)
	if err != nil || token.Kind != lexer.Name {
		return false
	}
	if token.Text == "for" {
		return true
	}
	if token.Text != "async" {
		return false
	}
	next, err := parser.peek(1)
	return err == nil && next.Kind == lexer.Name && next.Text == "for"
}
