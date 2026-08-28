package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseTypeParameters parses an optional square-bracketed generic parameter
// list after a definition or type-alias name.
func (parser *parserState) parseTypeParameters() ([]compilerast.TypeParameter, error) {
	_, matched, err := parser.take(lexer.LSquare)
	if err != nil || !matched {
		return nil, err
	}
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.RSquare {
		return nil, parser.syntaxError(token, "type parameter list cannot be empty")
	}

	names := make(map[string]struct{})
	var parameters []compilerast.TypeParameter
	for {
		parameter, err := parser.parseTypeParameter()
		if err != nil {
			return nil, err
		}
		if _, duplicate := names[parameter.Name]; duplicate {
			return nil, parser.errorAt(parameter.Range, "duplicate type parameter", false)
		}
		names[parameter.Name] = struct{}{}
		parameters = append(parameters, parameter)

		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
		if _, done, err := parser.take(lexer.RSquare); err != nil {
			return nil, err
		} else if done {
			return parameters, nil
		}
	}
	if _, err := parser.expect(lexer.RSquare, "expected ']' after type parameters"); err != nil {
		return nil, err
	}
	return parameters, nil
}

// parseTypeParameter parses an optional * or ** prefix, name, bound, and
// default value.
func (parser *parserState) parseTypeParameter() (compilerast.TypeParameter, error) {
	start, err := parser.peek(0)
	if err != nil {
		return compilerast.TypeParameter{}, err
	}
	kind := compilerast.TypeVariable
	if _, matched, err := parser.take(lexer.DoubleStar); err != nil {
		return compilerast.TypeParameter{}, err
	} else if matched {
		kind = compilerast.ParameterSpecification
	} else if _, matched, err := parser.take(lexer.Star); err != nil {
		return compilerast.TypeParameter{}, err
	} else if matched {
		kind = compilerast.TypeVariableTuple
	}
	name, err := parser.expect(lexer.Name, "expected type parameter name")
	if err != nil {
		return compilerast.TypeParameter{}, err
	}
	if isHardKeyword(name.Text) {
		return compilerast.TypeParameter{}, parser.syntaxError(name, "expected type parameter name")
	}
	parameter := compilerast.TypeParameter{Range: joinSpans(start.Span, name.Span), Name: name.Text, Kind: kind}
	if _, matched, err := parser.take(lexer.Colon); err != nil {
		return compilerast.TypeParameter{}, err
	} else if matched {
		parameter.Bound, err = parser.parseConditionalExpression()
		if err != nil {
			return compilerast.TypeParameter{}, err
		}
		parameter.Range = joinSpans(parameter.Range, parameter.Bound.Span())
	}
	if _, matched, err := parser.take(lexer.Equal); err != nil {
		return compilerast.TypeParameter{}, err
	} else if matched {
		star, starred, err := parser.take(lexer.Star)
		if err != nil {
			return compilerast.TypeParameter{}, err
		}
		parameter.Default, err = parser.parseConditionalExpression()
		if err != nil {
			return compilerast.TypeParameter{}, err
		}
		if starred {
			parameter.Default = &compilerast.StarredExpr{
				Range:   joinSpans(star.Span, parameter.Default.Span()),
				Value:   parameter.Default,
				Context: compilerast.Load,
			}
		}
		parameter.Range = joinSpans(parameter.Range, parameter.Default.Span())
	}
	return parameter, nil
}
