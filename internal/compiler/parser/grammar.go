package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

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
	return compilerast.NewModule(span, body, parser.source), nil
}
