package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// finishCall parses positional, starred, named, and dictionary-unpacked
// arguments after an opening parenthesis.
func (parser *parserState) finishCall(function compilerast.Expr) (compilerast.Expr, error) {
	close, empty, err := parser.take(lexer.RParen)
	if err != nil {
		return nil, err
	}
	if empty {
		return &compilerast.CallExpr{
			Range:    joinSpans(function.Span(), close.Span),
			Function: function,
		}, nil
	}

	var arguments []compilerast.Expr
	var keywords []compilerast.KeywordArgument
	keywordNames := make(map[string]struct{})
	seenKeyword := false
	seenDoubleStar := false
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		if token.Kind == lexer.Comma {
			return nil, parser.syntaxError(token, "expected argument")
		}

		switch token.Kind {
		case lexer.Star:
			if seenDoubleStar {
				return nil, parser.syntaxError(token, "iterable unpacking cannot follow keyword unpacking")
			}
			star, err := parser.advance()
			if err != nil {
				return nil, err
			}
			value, err := parser.parseDisjunction()
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, &compilerast.StarredExpr{
				Range:   joinSpans(star.Span, value.Span()),
				Value:   value,
				Context: compilerast.Load,
			})
		case lexer.DoubleStar:
			unpack, err := parser.advance()
			if err != nil {
				return nil, err
			}
			value, err := parser.parseDisjunction()
			if err != nil {
				return nil, err
			}
			keywords = append(keywords, compilerast.KeywordArgument{
				Range: joinSpans(unpack.Span, value.Span()),
				Value: value,
			})
			seenKeyword = true
			seenDoubleStar = true
		default:
			keyword, err := parser.callKeywordName()
			if err != nil {
				return nil, err
			}
			if keyword != nil {
				if _, duplicate := keywordNames[keyword.Text]; duplicate {
					return nil, parser.syntaxError(*keyword, "keyword argument repeated")
				}
				if isHardKeyword(keyword.Text) {
					return nil, parser.syntaxError(*keyword, "expected argument")
				}
				if _, err := parser.advance(); err != nil {
					return nil, err
				}
				if _, err := parser.advance(); err != nil {
					return nil, err
				}
				value, err := parser.parseDisjunction()
				if err != nil {
					return nil, err
				}
				keywords = append(keywords, compilerast.KeywordArgument{
					Range: joinSpans(keyword.Span, value.Span()),
					Name:  keyword.Text,
					Value: value,
				})
				keywordNames[keyword.Text] = struct{}{}
				seenKeyword = true
			} else {
				if seenKeyword {
					return nil, parser.syntaxError(token, "positional argument follows keyword argument")
				}
				argument, err := parser.parseDisjunction()
				if err != nil {
					return nil, err
				}
				arguments = append(arguments, argument)
			}
		}

		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, err
		}
		if !comma {
			break
		}
		close, empty, err = parser.take(lexer.RParen)
		if err != nil {
			return nil, err
		}
		if empty {
			return &compilerast.CallExpr{
				Range:     joinSpans(function.Span(), close.Span),
				Function:  function,
				Arguments: arguments,
				Keywords:  keywords,
			}, nil
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
		Keywords:  keywords,
	}, nil
}

func (parser *parserState) callKeywordName() (*lexer.Token, error) {
	token, err := parser.peek(0)
	if err != nil || token.Kind != lexer.Name {
		return nil, err
	}
	next, err := parser.peek(1)
	if err != nil || next.Kind != lexer.Equal {
		return nil, err
	}
	return &token, nil
}
