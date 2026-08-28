package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type parameterListState struct {
	parameters  compilerast.Parameters
	positional  []compilerast.Parameter
	names       map[string]struct{}
	seenSlash   bool
	seenDefault bool
	keywordOnly bool
	bareStar    bool
}

// parseParameterList parses a function or lambda parameter list through and
// including its closing token.
func (parser *parserState) parseParameterList(terminator lexer.Kind, annotations bool) (compilerast.Parameters, error) {
	state := parameterListState{names: make(map[string]struct{})}
	if _, empty, err := parser.take(terminator); err != nil {
		return compilerast.Parameters{}, err
	} else if empty {
		return state.parameters, nil
	}

	for {
		token, err := parser.peek(0)
		if err != nil {
			return compilerast.Parameters{}, err
		}
		switch token.Kind {
		case lexer.Slash:
			if err := parser.parsePositionalOnlyMarker(&state); err != nil {
				return compilerast.Parameters{}, err
			}
		case lexer.Star:
			if err := parser.parseVarArgMarker(&state, annotations); err != nil {
				return compilerast.Parameters{}, err
			}
		case lexer.DoubleStar:
			if err := parser.parseKeywordVarArg(&state, annotations); err != nil {
				return compilerast.Parameters{}, err
			}
			done, err := parser.finishParameterItem(terminator)
			if err != nil {
				return compilerast.Parameters{}, err
			}
			if !done {
				token, err := parser.peek(0)
				if err != nil {
					return compilerast.Parameters{}, err
				}
				return compilerast.Parameters{}, parser.syntaxError(token, "arguments cannot follow ** parameter")
			}
			return parser.finishParameterList(&state)
		default:
			if err := parser.parseOrdinaryParameter(&state, annotations); err != nil {
				return compilerast.Parameters{}, err
			}
		}

		done, err := parser.finishParameterItem(terminator)
		if err != nil {
			return compilerast.Parameters{}, err
		}
		if done {
			return parser.finishParameterList(&state)
		}
	}
}

func (parser *parserState) parsePositionalOnlyMarker(state *parameterListState) error {
	token, err := parser.advance()
	if err != nil {
		return err
	}
	if state.seenSlash || state.keywordOnly || len(state.positional) == 0 {
		return parser.syntaxError(token, "invalid positional-only parameter marker")
	}
	state.parameters.PositionalOnly = state.positional
	state.positional = nil
	state.seenSlash = true
	return nil
}

// parseVarArgMarker distinguishes a bare keyword-only separator from a named
// variadic parameter and records its optional annotation.
func (parser *parserState) parseVarArgMarker(state *parameterListState, annotations bool) error {
	star, err := parser.advance()
	if err != nil {
		return err
	}
	if state.keywordOnly {
		return parser.syntaxError(star, "multiple * parameters")
	}
	state.keywordOnly = true
	token, err := parser.peek(0)
	if err != nil {
		return err
	}
	if token.Kind != lexer.Name || isHardKeyword(token.Text) {
		state.bareStar = true
		return nil
	}
	parameter, err := parser.parseParameter(annotations, false)
	if err != nil {
		return err
	}
	if err := parser.recordParameterName(state, parameter); err != nil {
		return err
	}
	state.parameters.VarArg = &parameter
	return nil
}

func (parser *parserState) parseKeywordVarArg(state *parameterListState, annotations bool) error {
	token, err := parser.advance()
	if err != nil {
		return err
	}
	if state.parameters.KeywordVarArg != nil {
		return parser.syntaxError(token, "multiple ** parameters")
	}
	parameter, err := parser.parseParameter(annotations, false)
	if err != nil {
		return err
	}
	if err := parser.recordParameterName(state, parameter); err != nil {
		return err
	}
	state.parameters.KeywordVarArg = &parameter
	state.keywordOnly = true
	return nil
}

// parseOrdinaryParameter records a positional or keyword-only parameter and
// enforces positional default ordering.
func (parser *parserState) parseOrdinaryParameter(state *parameterListState, annotations bool) error {
	parameter, err := parser.parseParameter(annotations, true)
	if err != nil {
		return err
	}
	if err := parser.recordParameterName(state, parameter); err != nil {
		return err
	}
	if state.keywordOnly {
		state.parameters.KeywordOnly = append(state.parameters.KeywordOnly, parameter)
		return nil
	}
	if parameter.Default != nil {
		state.seenDefault = true
	} else if state.seenDefault {
		return parser.errorAt(parameter.Range, "non-default parameter follows default parameter", false)
	}
	state.positional = append(state.positional, parameter)
	return nil
}

func (parser *parserState) parseAnnotationExpression() (compilerast.Expr, error) {
	star, matched, err := parser.take(lexer.Star)
	if err != nil {
		return nil, err
	}
	value, err := parser.parseConditionalExpression()
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

// parseParameter parses one name with optional annotation and default according
// to the surrounding function or lambda parameter form.
func (parser *parserState) parseParameter(annotations, defaults bool) (compilerast.Parameter, error) {
	name, err := parser.expect(lexer.Name, "expected parameter name")
	if err != nil {
		return compilerast.Parameter{}, err
	}
	if isHardKeyword(name.Text) {
		return compilerast.Parameter{}, parser.syntaxError(name, "expected parameter name")
	}
	parameter := compilerast.Parameter{Range: name.Span, Name: name.Text}
	if annotations {
		if _, matched, err := parser.take(lexer.Colon); err != nil {
			return compilerast.Parameter{}, err
		} else if matched {
			parameter.Annotation, err = parser.parseAnnotationExpression()
			if err != nil {
				return compilerast.Parameter{}, err
			}
			parameter.Range = joinSpans(parameter.Range, parameter.Annotation.Span())
		}
	}
	if _, matched, err := parser.take(lexer.Equal); err != nil {
		return compilerast.Parameter{}, err
	} else if matched {
		if !defaults {
			return compilerast.Parameter{}, parser.syntaxError(name, "variadic parameter cannot have a default")
		}
		parameter.Default, err = parser.parseConditionalExpression()
		if err != nil {
			return compilerast.Parameter{}, err
		}
		parameter.Range = joinSpans(parameter.Range, parameter.Default.Span())
	}
	return parameter, nil
}

func (parser *parserState) recordParameterName(state *parameterListState, parameter compilerast.Parameter) error {
	if _, duplicate := state.names[parameter.Name]; duplicate {
		return parser.errorAt(parameter.Range, "duplicate parameter name", false)
	}
	state.names[parameter.Name] = struct{}{}
	return nil
}

func (parser *parserState) finishParameterItem(terminator lexer.Kind) (bool, error) {
	if _, matched, err := parser.take(terminator); err != nil {
		return false, err
	} else if matched {
		return true, nil
	}
	if _, err := parser.expect(lexer.Comma, "expected ',' between parameters"); err != nil {
		return false, err
	}
	_, matched, err := parser.take(terminator)
	return matched, err
}

func (parser *parserState) finishParameterList(state *parameterListState) (compilerast.Parameters, error) {
	if state.bareStar && len(state.parameters.KeywordOnly) == 0 {
		return compilerast.Parameters{}, parser.errorAt(lexer.Span{}, "named arguments must follow bare *", false)
	}
	state.parameters.Positional = state.positional
	return state.parameters, nil
}
