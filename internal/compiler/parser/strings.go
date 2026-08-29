package parser

import (
	"strings"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseStrings parses one or more adjacent plain, formatted, or template
// strings while rejecting template/non-template concatenation.
func (parser *parserState) parseStrings() (compilerast.Expr, error) {
	var parts []compilerast.Expr
	templateSeen := false
	regularSeen := false
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		var part compilerast.Expr
		switch token.Kind {
		case lexer.String:
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			part = &compilerast.StringLiteral{Range: token.Span, Text: token.Text}
			regularSeen = true
		case lexer.FStringStart:
			part, err = parser.parseFormattedString(false)
			regularSeen = true
		case lexer.TStringStart:
			part, err = parser.parseFormattedString(true)
			templateSeen = true
		default:
			if len(parts) == 1 {
				return parts[0], nil
			}
			return &compilerast.StringConcatExpr{
				Range: joinSpans(parts[0].Span(), parts[len(parts)-1].Span()),
				Parts: parts,
			}, nil
		}
		if err != nil {
			return nil, err
		}
		if templateSeen && regularSeen {
			return nil, parser.errorAt(part.Span(), "cannot concatenate template and non-template strings", false)
		}
		parts = append(parts, part)
	}
}

// parseFormattedString consumes one complete f-string or template string and
// parses each replacement field recursively.
func (parser *parserState) parseFormattedString(template bool) (compilerast.Expr, error) {
	start, err := parser.advance()
	if err != nil {
		return nil, err
	}
	middleKind := lexer.FStringMiddle
	endKind := lexer.FStringEnd
	if template {
		middleKind = lexer.TStringMiddle
		endKind = lexer.TStringEnd
	}
	var parts []compilerast.Expr
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		switch {
		case token.Kind == middleKind:
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			parts = append(parts, &compilerast.StringLiteral{Range: token.Span, Text: token.Text})
		case token.Kind == lexer.LBrace:
			open, err := parser.advance()
			if err != nil {
				return nil, err
			}
			value, err := parser.parseFormattedValue(open, middleKind)
			if err != nil {
				return nil, err
			}
			parts = append(parts, value)
		case token.Kind == endKind:
			end, err := parser.advance()
			if err != nil {
				return nil, err
			}
			quote := strings.IndexAny(start.Text, "'\"")
			raw := quote >= 0 && strings.Contains(strings.ToLower(start.Text[:quote]), "r")
			return &compilerast.FormattedStringExpr{
				Range:    joinSpans(start.Span, end.Span),
				Parts:    parts,
				Template: template,
				Raw:      raw,
			}, nil
		default:
			return nil, parser.syntaxError(token, "expected formatted string part")
		}
	}
}

// parseFormattedValue parses an expression, optional debug marker and
// conversion, optional nested format specifier, and closing brace.
func (parser *parserState) parseFormattedValue(open lexer.Token, middleKind lexer.Kind) (compilerast.Expr, error) {
	value, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	_, debug, err := parser.take(lexer.Equal)
	if err != nil {
		return nil, err
	}
	debugText := ""
	if debug {
		next, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		debugText = parser.source[open.Span.End.Offset:next.Span.Start.Offset]
	}
	conversion := ""
	if _, matched, err := parser.take(lexer.Exclamation); err != nil {
		return nil, err
	} else if matched {
		token, err := parser.expect(lexer.Name, "expected conversion after '!'")
		if err != nil {
			return nil, err
		}
		if token.Text != "s" && token.Text != "r" && token.Text != "a" {
			return nil, parser.syntaxError(token, "invalid formatted string conversion")
		}
		conversion = token.Text
	}
	var format []compilerast.Expr
	if _, matched, err := parser.take(lexer.Colon); err != nil {
		return nil, err
	} else if matched {
		format, err = parser.parseFormatSpec(middleKind)
		if err != nil {
			return nil, err
		}
	}
	close, err := parser.expect(lexer.RBrace, "expected '}' after replacement field")
	if err != nil {
		return nil, err
	}
	return &compilerast.FormattedValueExpr{
		Range:      joinSpans(open.Span, close.Span),
		Value:      value,
		Conversion: conversion,
		Format:     format,
		Debug:      debug,
		DebugText:  debugText,
	}, nil
}

// parseFormatSpec collects literal text and nested replacement fields until
// the enclosing replacement's closing brace.
func (parser *parserState) parseFormatSpec(middleKind lexer.Kind) ([]compilerast.Expr, error) {
	var parts []compilerast.Expr
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		switch {
		case token.Kind == lexer.RBrace:
			return parts, nil
		case token.Kind == middleKind:
			if _, err := parser.advance(); err != nil {
				return nil, err
			}
			parts = append(parts, &compilerast.StringLiteral{Range: token.Span, Text: token.Text})
		case token.Kind == lexer.LBrace:
			open, err := parser.advance()
			if err != nil {
				return nil, err
			}
			value, err := parser.parseFormattedValue(open, middleKind)
			if err != nil {
				return nil, err
			}
			parts = append(parts, value)
		default:
			return nil, parser.syntaxError(token, "expected format specifier")
		}
	}
}
