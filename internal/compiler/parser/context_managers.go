package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseWithStatement parses parenthesized or ordinary context-manager items
// and a synchronous or asynchronous body suite.
func (parser *parserState) parseWithStatement(asynchronous bool) (compilerast.Stmt, error) {
	start, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if asynchronous {
		if _, err := parser.expectKeyword("async", "expected 'async'"); err != nil {
			return nil, err
		}
	}
	if _, err := parser.expectKeyword("with", "expected 'with'"); err != nil {
		return nil, err
	}
	_, parenthesized, err := parser.take(lexer.LParen)
	if err != nil {
		return nil, err
	}
	wrapped := parenthesized
	var items []compilerast.WithItem
	for {
		item, err := parser.parseWithItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
		if parenthesized {
			if _, done, err := parser.take(lexer.RParen); err != nil {
				return nil, err
			} else if done {
				parenthesized = false
				break
			}
		}
	}
	if parenthesized {
		if _, err := parser.expect(lexer.RParen, "expected ')' after with items"); err != nil {
			return nil, err
		}
	}
	if wrapped && parser.nextKeywordIs("as") {
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
		target, err := parser.parsePrimary()
		if err != nil {
			return nil, err
		}
		if err := parser.setStoreContext(target); err != nil {
			return nil, err
		}
		context := items[0].Context
		if len(items) > 1 {
			elements := make([]compilerast.Expr, 0, len(items))
			for _, item := range items {
				if item.Target != nil {
					return nil, parser.errorAt(item.Range, "invalid parenthesized with item", false)
				}
				elements = append(elements, item.Context)
			}
			context = &compilerast.TupleExpr{
				Range:    joinSpans(elements[0].Span(), elements[len(elements)-1].Span()),
				Elements: elements,
				Context:  compilerast.Load,
			}
		}
		items = []compilerast.WithItem{{
			Range:   joinSpans(context.Span(), target.Span()),
			Context: context,
			Target:  target,
		}}
	}
	if _, err := parser.expect(lexer.Colon, "expected ':' after with items"); err != nil {
		return nil, err
	}
	body, err := parser.parseSuite()
	if err != nil {
		return nil, err
	}
	return &compilerast.WithStmt{
		Range: joinSpans(start.Span, body[len(body)-1].Span()),
		Items: items,
		Body:  body,
		Async: asynchronous,
	}, nil
}

// parseWithItem parses one context expression and its optional as target.
func (parser *parserState) parseWithItem() (compilerast.WithItem, error) {
	context, err := parser.parseConditionalExpression()
	if err != nil {
		return compilerast.WithItem{}, err
	}
	item := compilerast.WithItem{Range: context.Span(), Context: context}
	if _, matched, err := parser.takeKeyword("as"); err != nil {
		return compilerast.WithItem{}, err
	} else if matched {
		item.Target, err = parser.parsePrimary()
		if err != nil {
			return compilerast.WithItem{}, err
		}
		if err := parser.setStoreContext(item.Target); err != nil {
			return compilerast.WithItem{}, err
		}
		item.Range = joinSpans(item.Range, item.Target.Span())
	}
	return item, nil
}
